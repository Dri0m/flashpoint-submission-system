package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/bot"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type siteService struct {
	bot                      bot.Bot
	dal                      database.DAL
	validatorServerURL       string
	sessionExpirationSeconds int64
}

func NewSiteService(l *logrus.Logger, db *sql.DB, botSession *discordgo.Session, flashpointServerID, validatorServerURL string, sessionExpirationSeconds int64) *siteService {
	return &siteService{
		bot: bot.Bot{
			Session:            botSession,
			FlashpointServerID: flashpointServerID,
			L:                  l,
		},
		dal:                      database.NewMysqlDAL(db),
		validatorServerURL:       validatorServerURL,
		sessionExpirationSeconds: sessionExpirationSeconds,
	}
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (s *siteService) GetBasePageData(ctx context.Context) (*types.BasePageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		return &types.BasePageData{}, nil
	}

	discordUser, err := s.dal.GetDiscordUser(ctx, tx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from database")
	}

	userRoles, err := s.dal.GetDiscordUserRoles(ctx, tx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user authorization")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	bpd := &types.BasePageData{
		Username:  discordUser.Username,
		AvatarURL: utils.FormatAvatarURL(discordUser.ID, discordUser.Avatar),
		UserRoles: userRoles,
	}

	return bpd, nil
}

func (s *siteService) ReceiveSubmissions(ctx context.Context, sid *int64, fileHeaders []*multipart.FileHeader) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	destinationFilenames := make([]string, 0)

	for _, fileHeader := range fileHeaders {
		destinationFilename, err := s.processReceivedSubmission(ctx, tx, fileHeader, sid)

		if destinationFilename != nil {
			destinationFilenames = append(destinationFilenames, *destinationFilename)
		}

		if err != nil {
			for _, df := range destinationFilenames {
				utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
				utils.LogIfErr(ctx, os.Remove(df))
			}
			return fmt.Errorf("file '%s': %s", fileHeader.Filename, err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		for _, df := range destinationFilenames {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
			utils.LogIfErr(ctx, os.Remove(df))
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) processReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader, sid *int64) (*string, error) {
	userID := utils.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("no user associated with request")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open received file")
	}
	defer file.Close()

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes, MIME header: %+v", fileHeader.Filename, fileHeader.Size, fileHeader.Header)

	const dir = "submissions"

	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return nil, fmt.Errorf("failed to make directory structure")
	}

	destinationFilename := utils.RandomString(64) + filepath.Ext(fileHeader.Filename)
	destinationFilePath := fmt.Sprintf("%s/%s", dir, destinationFilename)

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file")
	}
	defer func() { utils.LogIfErr(ctx, destination.Close()) }()

	utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	md5sum := md5.New()
	sha256sum := sha256.New()
	multiWriter := io.MultiWriter(destination, sha256sum, md5sum)

	nBytes, err := io.Copy(multiWriter, file)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		return &destinationFilePath, fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("storing submission...")

	var submissionID int64

	if sid == nil {
		submissionID, err = s.dal.StoreSubmission(ctx, tx)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return &destinationFilePath, fmt.Errorf("failed to store submission")
		}
	} else {
		submissionID = *sid
	}

	sf := &types.SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      utils.UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now(),
		MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
		SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
	}

	fid, err := s.dal.StoreSubmissionFile(ctx, tx, sf)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		me, ok := err.(*mysql.MySQLError)
		if ok {
			if me.Number == 1062 {
				return &destinationFilePath, fmt.Errorf("file with checksums md5:%s sha256:%s already present in the DB", sf.MD5Sum, sf.SHA256Sum)
			}
		}
		return &destinationFilePath, fmt.Errorf("failed to store submission file")
	}

	c := &types.Comment{
		AuthorID:     userID,
		SubmissionID: submissionID,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    time.Now(),
	}

	if err := s.dal.StoreComment(ctx, tx, c); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store uploader comment")
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	resp, err := utils.UploadFile(ctx, s.validatorServerURL, destinationFilePath)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("validator: %w", err)
	}

	var vr types.ValidatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("failed to decode validator response")
	}

	vr.Meta.SubmissionID = submissionID
	vr.Meta.SubmissionFileID = fid

	if err := s.dal.StoreCurationMeta(ctx, tx, &vr.Meta); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store curation meta")
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := convertValidatorResponseToComment(&vr)
	if err := s.dal.StoreComment(ctx, tx, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store validator comment")
	}

	return &destinationFilePath, nil
}

func (s *siteService) ReceiveComments(ctx context.Context, uid int64, sids []int64, formAction, formMessage string) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	var message *string
	if formMessage != "" {
		message = &formMessage
	}

	actions := []string{constants.ActionComment, constants.ActionApprove, constants.ActionRequestChanges, constants.ActionAccept, constants.ActionMarkAdded, constants.ActionReject, constants.ActionUpload}
	isActionValid := false
	for _, a := range actions {
		if formAction == a {
			isActionValid = true
			break
		}
	}

	if !isActionValid {
		return fmt.Errorf("invalid comment action")
	}

	actionsWithMandatoryMessage := []string{constants.ActionComment, constants.ActionRequestChanges, constants.ActionReject}
	isActionWithMandatoryMessage := false
	for _, a := range actionsWithMandatoryMessage {
		if formAction == a {
			isActionWithMandatoryMessage = true
			break
		}
	}

	if isActionWithMandatoryMessage && (message == nil || *message == "") {
		return fmt.Errorf("cannot post comment action '%s' without a message", formAction)
	}

	for _, sid := range sids {
		c := &types.Comment{
			AuthorID:     uid,
			SubmissionID: sid,
			Message:      message,
			Action:       formAction,
			CreatedAt:    time.Now(),
		}

		// TODO optimize into batch insert
		if err := s.dal.StoreComment(ctx, tx, c); err != nil {
			return fmt.Errorf("failed to store comment")
		}
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) GetViewSubmissionPageData(ctx context.Context, sid int64) (*types.ViewSubmissionPageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	filter := &types.SubmissionsFilter{
		SubmissionID: &sid,
	}

	submissions, err := s.dal.SearchSubmissions(ctx, tx, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission")
	}

	if len(submissions) == 0 {
		return nil, fmt.Errorf("submission not found")
	}

	submission := submissions[0]

	meta, err := s.dal.GetCurationMetaBySubmissionFileID(ctx, tx, submission.FileID)
	if err != nil && err != sql.ErrNoRows {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation meta")
	}

	comments, err := s.dal.GetExtendedCommentsBySubmissionID(ctx, tx, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation comments")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	pageData := &types.ViewSubmissionPageData{
		SubmissionsPageData: types.SubmissionsPageData{
			BasePageData: *bpd,
			Submissions:  submissions,
		},
		CurationMeta: meta,
		Comments:     comments,
	}

	return pageData, nil
}

func (s *siteService) GetSubmissionsFilesPageData(ctx context.Context, sid int64) (*types.SubmissionsFilesPageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	sf, err := s.dal.GetExtendedSubmissionFilesBySubmissionID(ctx, tx, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	pageData := &types.SubmissionsFilesPageData{
		BasePageData:    *bpd,
		SubmissionFiles: sf,
	}

	return pageData, nil
}

func (s *siteService) GetSubmissionsPageData(ctx context.Context, filter *types.SubmissionsFilter) (*types.SubmissionsPageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	submissions, err := s.dal.SearchSubmissions(ctx, tx, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submissions")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	pageData := &types.SubmissionsPageData{
		BasePageData: *bpd,
		Submissions:  submissions,
		Filter:       *filter,
	}

	return pageData, nil
}

func (s *siteService) SearchSubmissions(ctx context.Context, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	return s.dal.SearchSubmissions(ctx, nil, filter)
}

func (s *siteService) GetSubmissionFiles(ctx context.Context, sfids []int64) ([]*types.SubmissionFile, error) {
	sfs, err := s.dal.GetSubmissionFiles(ctx, nil, sfids)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission file")
	}
	return sfs, nil
}

func (s *siteService) GetUIDFromSession(ctx context.Context, key string) (int64, bool, error) {
	uid, ok, err := s.dal.GetUIDFromSession(ctx, nil, key)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, false, err
	}
	return uid, ok, nil
}

func (s *siteService) SoftDeleteSubmissionFile(ctx context.Context, sfid int64) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	if err := s.dal.SoftDeleteSubmissionFile(ctx, tx, sfid); err != nil {
		if err.Error() == constants.ErrorCannotDeleteLastSubmissionFile {
			return err
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to soft delete submission file")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) SaveUser(ctx context.Context, discordUser *types.DiscordUser) (*utils.AuthToken, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	// save discord user data
	if err := s.dal.StoreDiscordUser(ctx, tx, discordUser); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord user")
	}

	// get discord roles
	serverRoles, err := s.bot.GetFlashpointRoles() // TODO changes in roles need to be refreshed sometimes
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to obtain discord server roles")
	}
	userRoleIDs, err := s.bot.GetFlashpointRoleIDsForUser(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to obtain discord server roles")
	}

	userRolesIDsNumeric := make([]int64, 0, len(userRoleIDs))
	for _, userRoleID := range userRoleIDs {
		id, err := strconv.ParseInt(userRoleID, 10, 64)
		if err != nil {
			return nil, err
		}
		userRolesIDsNumeric = append(userRolesIDsNumeric, id)
	}

	// save discord roles
	if err := s.dal.StoreDiscordServerRoles(ctx, tx, serverRoles); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord server roles")
	}
	if err := s.dal.StoreDiscordUserRoles(ctx, tx, discordUser.ID, userRolesIDsNumeric); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord user roles")
	}

	// create cookie and save session
	authToken, err := utils.CreateAuthToken(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to generate auth token")
	}

	if err = s.dal.StoreSession(ctx, tx, authToken.Secret, discordUser.ID, s.sessionExpirationSeconds); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store session")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	return authToken, nil
}

func (s *siteService) Logout(ctx context.Context, secret string) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	if err := s.dal.DeleteSession(ctx, tx, secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("unable to delete session")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) GetUserRoles(ctx context.Context, uid int64) ([]string, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	roles, err := s.dal.GetDiscordUserRoles(ctx, tx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user roles")
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	return roles, nil
}
