package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/authbot"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/notificationbot"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/go-sql-driver/mysql"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type MultipartFileWrapper struct {
	fileHeader *multipart.FileHeader
}

func NewMutlipartFileWrapper(fileHeader *multipart.FileHeader) *MultipartFileWrapper {
	return &MultipartFileWrapper{
		fileHeader: fileHeader,
	}
}

func (m *MultipartFileWrapper) Filename() string {
	return m.fileHeader.Filename
}

func (m *MultipartFileWrapper) Size() int64 {
	return m.fileHeader.Size
}

func (m *MultipartFileWrapper) Open() (multipart.File, error) {
	return m.fileHeader.Open()
}

type RealClock struct {
}

func (r *RealClock) Now() time.Time {
	return time.Now()
}

func (r *RealClock) Unix(sec int64, nsec int64) time.Time {
	return time.Unix(sec, nsec)
}

// authToken is authToken
type authToken struct {
	Secret string
	UserID string
}

type authTokenProvider struct {
}

func NewAuthTokenProvider() *authTokenProvider {
	return &authTokenProvider{}
}

type AuthTokenizer interface {
	CreateAuthToken(userID int64) (*authToken, error)
}

func (a *authTokenProvider) CreateAuthToken(userID int64) (*authToken, error) {
	s, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	return &authToken{
		Secret: s.String(),
		UserID: fmt.Sprint(userID),
	}, nil
}

// ParseAuthToken parses map into token
func ParseAuthToken(value map[string]string) (*authToken, error) {
	secret, ok := value["Secret"]
	if !ok {
		return nil, fmt.Errorf("missing Secret")
	}
	userID, ok := value["userID"]
	if !ok {
		return nil, fmt.Errorf("missing userid")
	}
	return &authToken{
		Secret: secret,
		UserID: userID,
	}, nil
}

func MapAuthToken(token *authToken) map[string]string {
	return map[string]string{"Secret": token.Secret, "userID": token.UserID}
}

type siteService struct {
	authBot                  authbot.DiscordRoleReader
	notificationBot          notificationbot.DiscordNotificationSender
	dal                      database.DAL
	validator                Validator
	clock                    Clock
	randomStringProvider     utils.RandomStringer
	authTokenProvider        AuthTokenizer
	sessionExpirationSeconds int64
	submissionsDir           string
}

func NewSiteService(l *logrus.Logger, db *sql.DB, authBotSession, notificationBotSession *discordgo.Session, flashpointServerID, notificationChannelID, validatorServerURL string, sessionExpirationSeconds int64, submissionsDir string) *siteService {
	return &siteService{
		authBot:                  authbot.NewBot(authBotSession, flashpointServerID, l.WithField("botName", "authBot")),
		notificationBot:          notificationbot.NewBot(notificationBotSession, flashpointServerID, notificationChannelID, l.WithField("botName", "notificationBot")),
		dal:                      database.NewMysqlDAL(db),
		validator:                NewValidator(validatorServerURL),
		clock:                    &RealClock{},
		randomStringProvider:     utils.NewRealRandomStringProvider(),
		authTokenProvider:        NewAuthTokenProvider(),
		sessionExpirationSeconds: sessionExpirationSeconds,
		submissionsDir:           submissionsDir,
	}
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (s *siteService) GetBasePageData(ctx context.Context) (*types.BasePageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		return &types.BasePageData{}, nil
	}

	discordUser, err := s.dal.GetDiscordUser(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from database")
	}

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user authorization")
	}

	bpd := &types.BasePageData{
		Username:  discordUser.Username,
		UserID:    discordUser.ID,
		AvatarURL: utils.FormatAvatarURL(discordUser.ID, discordUser.Avatar),
		UserRoles: userRoles,
	}

	return bpd, nil
}

func (s *siteService) ReceiveSubmissions(ctx context.Context, sid *int64, fileProviders []MultipartFileProvider) error {
	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		return fmt.Errorf("no user associated with request")
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to get discord user roles")
	}

	if constants.IsInAudit(userRoles) && len(fileProviders) > 1 {
		return fmt.Errorf("cannot upload more than one submission at once when user is in audit")
	}

	if constants.IsInAudit(userRoles) && fileProviders[0].Size() > constants.UserInAuditSumbissionMaxFilesize {
		return fmt.Errorf("submission filesize limited to 200MB for users in audit")
	}

	var submissionLevel string

	if constants.IsInAudit(userRoles) {
		submissionLevel = constants.SubmissionLevelAudition
	} else if constants.IsTrialCurator(userRoles) {
		submissionLevel = constants.SubmissionLevelTrial
	} else if constants.IsStaff(userRoles) {
		submissionLevel = constants.SubmissionLevelStaff
	}

	destinationFilenames := make([]string, 0)

	for _, fileProvider := range fileProviders {
		destinationFilename, err := s.processReceivedSubmission(ctx, dbs, fileProvider, sid, submissionLevel)

		if destinationFilename != nil {
			destinationFilenames = append(destinationFilenames, *destinationFilename)
		}

		if err != nil {
			for _, df := range destinationFilenames {
				utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
				utils.LogIfErr(ctx, os.Remove(df))
			}
			return fmt.Errorf("file '%s': %s", fileProvider.Filename(), err.Error())
		}
	}

	if err := dbs.Commit(); err != nil {
		for _, df := range destinationFilenames {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
			utils.LogIfErr(ctx, os.Remove(df))
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) processReceivedSubmission(ctx context.Context, dbs database.DBSession, fileHeader MultipartFileProvider, sid *int64, submissionLevel string) (*string, error) {
	userID := utils.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("no user associated with request")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open received file")
	}
	defer file.Close()

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes", fileHeader.Filename(), fileHeader.Size())

	if err := os.MkdirAll(s.submissionsDir, os.ModeDir); err != nil {
		return nil, fmt.Errorf("failed to make directory structure")
	}

	ext := filepath.Ext(fileHeader.Filename())

	if ext != ".7z" && ext != ".zip" {
		return nil, fmt.Errorf("unsupported file extension")
	}

	destinationFilename := s.randomStringProvider.RandomString(64) + ext
	destinationFilePath := fmt.Sprintf("%s/%s", s.submissionsDir, destinationFilename)

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
	if nBytes != fileHeader.Size() {
		return &destinationFilePath, fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("storing submission...")

	var submissionID int64

	if sid == nil {
		submissionID, err = s.dal.StoreSubmission(dbs, submissionLevel)
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
		OriginalFilename: fileHeader.Filename(),
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size(),
		UploadedAt:       s.clock.Now(),
		MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
		SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
	}

	fid, err := s.dal.StoreSubmissionFile(dbs, sf)
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
		CreatedAt:    s.clock.Now(),
	}

	if err := s.dal.StoreComment(dbs, c); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store uploader comment")
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	vr, err := s.validator.Validate(ctx, destinationFilePath, submissionID, fid)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("validator: %w", err)
	}

	if err := s.dal.StoreCurationMeta(dbs, &vr.Meta); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store curation meta")
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := s.convertValidatorResponseToComment(vr)
	if err := s.dal.StoreComment(dbs, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store validator comment")
	}

	return &destinationFilePath, nil
}

// convertValidatorResponseToComment produces appropriate comment based on validator response
func (s *siteService) convertValidatorResponseToComment(vr *types.ValidatorResponse) *types.Comment {
	c := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: vr.Meta.SubmissionID,
		CreatedAt:    s.clock.Now(),
	}

	approvalMessage := "LGTM 🤖"
	message := ""

	if len(vr.CurationErrors) > 0 {
		message += "Your curation is invalid:\n"
	}
	if len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) > 0 {
		message += "Your curation might have some problems:\n"
	}

	for _, e := range vr.CurationErrors {
		message += fmt.Sprintf("🚫 %s\n", e)
	}
	for _, w := range vr.CurationWarnings {
		message += fmt.Sprintf("🚫 %s\n", w)
	}

	c.Message = &message

	c.Action = constants.ActionRequestChanges
	if len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) == 0 {
		c.Action = constants.ActionApprove
		c.Message = &approvalMessage
	}

	return c
}

func (s *siteService) ReceiveComments(ctx context.Context, uid int64, sids []int64, formAction, formMessage, formIgnoreDupeActions string) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	var message *string
	if formMessage != "" {
		message = &formMessage
	}

	// TODO refactor these validators into a function and cover with tests
	actions := constants.GetAllowedActions()
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

	actionsWithMandatoryMessage := constants.GetActionsWithMandatoryMessage()
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

	ignoreDupeActions := false
	if formIgnoreDupeActions == "true" {
		ignoreDupeActions = true
	}

	// TODO optimize batch operation
SubmissionLoop:
	for _, sid := range sids {
		c := &types.Comment{
			AuthorID:     uid,
			SubmissionID: sid,
			Message:      message,
			Action:       formAction,
			CreatedAt:    s.clock.Now(),
		}

		utils.LogCtx(ctx).Debugf("searching submission %d for comment batch", sid)
		submissions, err := s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{SubmissionID: &sid})
		if err != nil {
			return fmt.Errorf("failed to load submission with id %d", sid)
		}

		if len(submissions) == 0 {
			return fmt.Errorf("submission with id %d not found", sid)
		}

		submission := submissions[0]

		if formAction == constants.ActionAssign {
			for _, assignedUserID := range submission.AssignedUserIDs {
				if uid == assignedUserID {
					if ignoreDupeActions {
						continue SubmissionLoop
					}
					return fmt.Errorf("you are already assigned to submission %d", sid)
				}
			}
		} else if formAction == constants.ActionUnassign {
			found := false
			for _, assignedUserID := range submission.AssignedUserIDs {
				if uid == assignedUserID {
					found = true
				}
			}
			if !found {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return fmt.Errorf("you are not assigned to submission %d", sid)
			}
		} else if formAction == constants.ActionApprove {
			for _, assignedUserID := range submission.ApprovedUserIDs {
				if uid == assignedUserID {
					if ignoreDupeActions {
						continue SubmissionLoop
					}
					return fmt.Errorf("you have already approved submission %d", sid)
				}
			}
		} else if formAction == constants.ActionRequestChanges {
			for _, assignedUserID := range submission.RequestedChangesUserIDs {
				if uid == assignedUserID {
					if ignoreDupeActions {
						continue SubmissionLoop
					}
					return fmt.Errorf("you have already requested changes on submission %d", sid)
				}
			}
		}

		if err := s.dal.StoreComment(dbs, c); err != nil {
			return fmt.Errorf("failed to store comment")
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) GetViewSubmissionPageData(ctx context.Context, uid, sid int64) (*types.ViewSubmissionPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	filter := &types.SubmissionsFilter{
		SubmissionID: &sid,
	}

	submissions, err := s.dal.SearchSubmissions(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission")
	}

	if len(submissions) == 0 {
		return nil, fmt.Errorf("submission not found")
	}

	submission := submissions[0]

	meta, err := s.dal.GetCurationMetaBySubmissionFileID(dbs, submission.FileID)
	if err != nil && err != sql.ErrNoRows {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation meta")
	}

	comments, err := s.dal.GetExtendedCommentsBySubmissionID(dbs, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation comments")
	}

	isUserSubscribed, err := s.dal.IsUserSubscribedToSubmission(dbs, uid, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation comments")
	}

	pageData := &types.ViewSubmissionPageData{
		SubmissionsPageData: types.SubmissionsPageData{
			BasePageData: *bpd,
			Submissions:  submissions,
		},
		CurationMeta:     meta,
		Comments:         comments,
		IsUserSubscribed: isUserSubscribed,
	}

	return pageData, nil
}

func (s *siteService) GetSubmissionsFilesPageData(ctx context.Context, sid int64) (*types.SubmissionsFilesPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	sf, err := s.dal.GetExtendedSubmissionFilesBySubmissionID(dbs, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission")
	}

	pageData := &types.SubmissionsFilesPageData{
		BasePageData:    *bpd,
		SubmissionFiles: sf,
	}

	return pageData, nil
}

func (s *siteService) GetSubmissionsPageData(ctx context.Context, filter *types.SubmissionsFilter) (*types.SubmissionsPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	submissions, err := s.dal.SearchSubmissions(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submissions")
	}

	pageData := &types.SubmissionsPageData{
		BasePageData: *bpd,
		Submissions:  submissions,
		Filter:       *filter,
	}

	return pageData, nil
}

func (s *siteService) SearchSubmissions(ctx context.Context, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	submissions, err := s.dal.SearchSubmissions(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to search submissions")
	}
	return submissions, nil
}

func (s *siteService) GetSubmissionFiles(ctx context.Context, sfids []int64) ([]*types.SubmissionFile, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	sfs, err := s.dal.GetSubmissionFiles(dbs, sfids)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission file")
	}
	return sfs, nil
}

func (s *siteService) GetUIDFromSession(ctx context.Context, key string) (int64, bool, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, false, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()
	uid, ok, err := s.dal.GetUIDFromSession(dbs, key)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, false, err
	}
	return uid, ok, nil
}

func (s *siteService) SoftDeleteSubmissionFile(ctx context.Context, sfid int64) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	if err := s.dal.SoftDeleteSubmissionFile(dbs, sfid); err != nil {
		if err.Error() == constants.ErrorCannotDeleteLastSubmissionFile {
			return err
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to soft delete submission file")
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) SoftDeleteSubmission(ctx context.Context, sid int64) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	if err := s.dal.SoftDeleteSubmission(dbs, sid); err != nil {
		if err.Error() == constants.ErrorCannotDeleteLastSubmissionFile {
			return err
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to soft delete submission file")
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) SoftDeleteComment(ctx context.Context, cid int64) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	if err := s.dal.SoftDeleteComment(dbs, cid); err != nil {
		if err.Error() == constants.ErrorCannotDeleteLastSubmissionFile {
			return err
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to soft delete comment")
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) SaveUser(ctx context.Context, discordUser *types.DiscordUser) (*authToken, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	// save discord user data
	if err := s.dal.StoreDiscordUser(dbs, discordUser); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord user")
	}

	// get discord roles
	serverRoles, err := s.authBot.GetFlashpointRoles() // TODO changes in roles need to be refreshed sometimes
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to obtain discord server roles")
	}
	userRoleIDs, err := s.authBot.GetFlashpointRoleIDsForUser(discordUser.ID)
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
	if err := s.dal.StoreDiscordServerRoles(dbs, serverRoles); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord server roles")
	}
	if err := s.dal.StoreDiscordUserRoles(dbs, discordUser.ID, userRolesIDsNumeric); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord user roles")
	}

	// create cookie and save session
	authToken, err := s.authTokenProvider.CreateAuthToken(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to generate auth token")
	}

	if err = s.dal.StoreSession(dbs, authToken.Secret, discordUser.ID, s.sessionExpirationSeconds); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store session")
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	return authToken, nil
}

func (s *siteService) Logout(ctx context.Context, secret string) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	if err := s.dal.DeleteSession(dbs, secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("unable to delete session")
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) GetUserRoles(ctx context.Context, uid int64) ([]string, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	roles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user roles")
	}

	return roles, nil
}

func (s *siteService) GetProfilePageData(ctx context.Context, uid int64) (*types.ProfilePageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	notificationActions, err := s.dal.GetNotificationSettingsByUserID(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation comments")
	}

	pageData := &types.ProfilePageData{
		BasePageData:        *bpd,
		NotificationActions: notificationActions,
	}

	return pageData, nil
}

func (s *siteService) UpdateNotificationSettings(ctx context.Context, uid int64, notificationActions []string) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	if err := s.dal.StoreNotificationSettings(dbs, uid, notificationActions); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("unable to store notification settings")
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *siteService) UpdateSubscriptionSettings(ctx context.Context, uid, sid int64, subscribe bool) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	if subscribe {
		if err := s.dal.SubscribeUserToSubmission(dbs, uid, sid); err != nil {
			utils.LogCtx(ctx).Error(err)
			return fmt.Errorf("unable to subscribe user to submission")
		}
	} else {
		if err := s.dal.UnsubscribeUserFromSubmission(dbs, uid, sid); err != nil {
			utils.LogCtx(ctx).Error(err)
			return fmt.Errorf("unable to unsubscribe user from submission")
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}
