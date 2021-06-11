package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/bot"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
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
	bot                      bot.DiscordRoleReader
	dal                      database.DAL
	validator                Validator
	clock                    Clock
	randomStringProvider     utils.RandomStringer
	authTokenProvider        AuthTokenizer
	sessionExpirationSeconds int64
	submissionsDir           string
}

func NewSiteService(l *logrus.Logger, db *sql.DB, botSession *discordgo.Session, flashpointServerID, validatorServerURL string, sessionExpirationSeconds int64, submissionsDir string) *siteService {
	return &siteService{
		bot:                      bot.NewBot(botSession, flashpointServerID, l),
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
		AvatarURL: utils.FormatAvatarURL(discordUser.ID, discordUser.Avatar),
		UserRoles: userRoles,
	}

	return bpd, nil
}

func (s *siteService) ReceiveSubmissions(ctx context.Context, sid *int64, fileProviders []MultipartFileProvider) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf(constants.ErrorFailedToBeginTransaction)
	}
	defer dbs.Rollback()

	destinationFilenames := make([]string, 0)

	for _, fileProvider := range fileProviders {
		destinationFilename, err := s.processReceivedSubmission(ctx, dbs, fileProvider, sid)

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

func (s *siteService) processReceivedSubmission(ctx context.Context, dbs database.DBSession, fileHeader MultipartFileProvider, sid *int64) (*string, error) {
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

	destinationFilename := s.randomStringProvider.RandomString(64) + filepath.Ext(fileHeader.Filename())
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
		submissionID, err = s.dal.StoreSubmission(dbs)
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

func (s *siteService) ReceiveComments(ctx context.Context, uid int64, sids []int64, formAction, formMessage string) error {
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
	actions := constants.GetActions()
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

	for _, sid := range sids {
		c := &types.Comment{
			AuthorID:     uid,
			SubmissionID: sid,
			Message:      message,
			Action:       formAction,
			CreatedAt:    s.clock.Now(),
		}

		// TODO optimize into batch insert
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

func (s *siteService) GetViewSubmissionPageData(ctx context.Context, sid int64) (*types.ViewSubmissionPageData, error) {
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
