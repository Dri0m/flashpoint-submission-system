package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/go-sql-driver/mysql"
	"github.com/kofalt/go-memoize"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Dri0m/flashpoint-submission-system/authbot"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/notificationbot"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/agnivade/levenshtein"
	"github.com/bwmarrin/discordgo"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
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

type SiteService struct {
	authBot                   authbot.DiscordRoleReader
	notificationBot           notificationbot.DiscordNotificationSender
	dal                       database.DAL
	validator                 Validator
	clock                     Clock
	randomStringProvider      utils.RandomStringer
	authTokenProvider         AuthTokenizer
	sessionExpirationSeconds  int64
	submissionsDir            string
	submissionImagesDir       string
	flashfreezeDir            string
	notificationQueueNotEmpty chan bool
	isDev                     bool
	submissionReceiverMutex   sync.Mutex
	discordRoleCache          *memoize.Memoizer
	resumableUploadService    *resumableuploadservice.ResumableUploadService
	archiveIndexerServerURL   string
	flashfreezeIngestDir      string
}

func New(l *logrus.Entry, db *sql.DB, authBotSession, notificationBotSession *discordgo.Session,
	flashpointServerID, notificationChannelID, curationFeedChannelID, validatorServerURL string,
	sessionExpirationSeconds int64, submissionsDir, submissionImagesDir, flashfreezeDir string, isDev bool, rsu *resumableuploadservice.ResumableUploadService, archiveIndexerServerURL, flashfreezeIngestDir string) *SiteService {

	return &SiteService{
		authBot:                   authbot.NewBot(authBotSession, flashpointServerID, l.WithField("botName", "authBot"), isDev),
		notificationBot:           notificationbot.NewBot(notificationBotSession, flashpointServerID, notificationChannelID, curationFeedChannelID, l.WithField("botName", "notificationBot"), isDev),
		dal:                       database.NewMysqlDAL(db),
		validator:                 NewValidator(validatorServerURL),
		clock:                     &RealClock{},
		randomStringProvider:      utils.NewRealRandomStringProvider(),
		authTokenProvider:         NewAuthTokenProvider(),
		sessionExpirationSeconds:  sessionExpirationSeconds,
		submissionsDir:            submissionsDir,
		submissionImagesDir:       submissionImagesDir,
		flashfreezeDir:            flashfreezeDir,
		notificationQueueNotEmpty: make(chan bool, 1),
		isDev:                     isDev,
		discordRoleCache:          memoize.NewMemoizer(2*time.Minute, 60*time.Minute),
		resumableUploadService:    rsu,
		archiveIndexerServerURL:   archiveIndexerServerURL,
		flashfreezeIngestDir:      flashfreezeIngestDir,
	}
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (s *SiteService) GetBasePageData(ctx context.Context) (*types.BasePageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	uid := utils.UserID(ctx)
	if uid == 0 {
		return &types.BasePageData{}, nil
	}

	discordUser, err := s.dal.GetDiscordUser(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	bpd := &types.BasePageData{
		Username:      discordUser.Username,
		UserID:        discordUser.ID,
		AvatarURL:     utils.FormatAvatarURL(discordUser.ID, discordUser.Avatar),
		UserRoles:     userRoles,
		IsDevInstance: s.isDev,
	}

	return bpd, nil
}

func (s *SiteService) GetViewSubmissionPageData(ctx context.Context, uid, sid int64) (*types.ViewSubmissionPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions, _, err := s.dal.SearchSubmissions(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if len(submissions) == 0 {
		return nil, perr("submission not found", http.StatusNotFound)
	}

	submission := submissions[0]

	meta, err := s.dal.GetCurationMetaBySubmissionFileID(dbs, submission.FileID)
	if err != nil && err != sql.ErrNoRows {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	comments, err := s.dal.GetExtendedCommentsBySubmissionID(dbs, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	isUserSubscribed, err := s.dal.IsUserSubscribedToSubmission(dbs, uid, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	curationImages, err := s.dal.GetCurationImagesBySubmissionFileID(dbs, submission.FileID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	ciids := make([]int64, 0, len(curationImages))

	for _, curationImage := range curationImages {
		ciids = append(ciids, curationImage.ID)
	}

	var nextSID *int64
	var prevSID *int64

	nsid, err := s.dal.GetNextSubmission(dbs, sid)
	if err != nil {
		if err != sql.ErrNoRows {
			utils.LogCtx(ctx).Error(err)
			return nil, dberr(err)
		}
	} else {
		nextSID = &nsid
	}

	psid, err := s.dal.GetPreviousSubmission(dbs, sid)
	if err != nil {
		if err != sql.ErrNoRows {
			utils.LogCtx(ctx).Error(err)
			return nil, dberr(err)
		}
	} else {
		prevSID = &psid
	}

	tagList, err := s.validator.GetTags(ctx)
	if err != nil {
		return nil, err
	}

	pageData := &types.ViewSubmissionPageData{
		SubmissionsPageData: types.SubmissionsPageData{
			BasePageData: *bpd,
			Submissions:  submissions,
		},
		CurationMeta:         meta,
		Comments:             comments,
		IsUserSubscribed:     isUserSubscribed,
		CurationImageIDs:     ciids,
		NextSubmissionID:     nextSID,
		PreviousSubmissionID: prevSID,
		TagList:              tagList,
	}

	return pageData, nil
}

func (s *SiteService) GetSubmissionsFilesPageData(ctx context.Context, sid int64) (*types.SubmissionsFilesPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	sf, err := s.dal.GetExtendedSubmissionFilesBySubmissionID(dbs, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	pageData := &types.SubmissionsFilesPageData{
		BasePageData:    *bpd,
		SubmissionFiles: sf,
	}

	return pageData, nil
}

func (s *SiteService) GetSubmissionsPageData(ctx context.Context, filter *types.SubmissionsFilter) (*types.SubmissionsPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	submissions, count, err := s.dal.SearchSubmissions(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	pageData := &types.SubmissionsPageData{
		BasePageData: *bpd,
		TotalCount:   count,
		Submissions:  submissions,
		Filter:       *filter,
	}

	return pageData, nil
}

func (s *SiteService) SearchSubmissions(ctx context.Context, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, int64, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, 0, dberr(err)
	}
	defer dbs.Rollback()

	submissions, count, err := s.dal.SearchSubmissions(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, 0, dberr(err)
	}
	return submissions, count, nil
}

func (s *SiteService) GetSubmissionFiles(ctx context.Context, sfids []int64) ([]*types.SubmissionFile, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	sfs, err := s.dal.GetSubmissionFiles(dbs, sfids)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return sfs, nil
}

func (s *SiteService) GetUIDFromSession(ctx context.Context, key string) (int64, bool, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, false, dberr(err)
	}
	defer dbs.Rollback()

	uid, ok, err := s.dal.GetUIDFromSession(dbs, key)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, false, dberr(err)
	}

	return uid, ok, nil
}

func (s *SiteService) SoftDeleteSubmissionFile(ctx context.Context, sfid int64, deleteReason string) error {
	uid := utils.UserID(ctx)

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	sfs, err := s.dal.GetSubmissionFiles(dbs, []int64{sfid})
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	authorID := sfs[0].SubmitterID
	sid := sfs[0].SubmissionID

	if err := s.dal.SoftDeleteSubmissionFile(dbs, sfid, deleteReason); err != nil {
		if err.Error() == constants.ErrorCannotDeleteLastSubmissionFile {
			return perr(err.Error(), http.StatusBadRequest)
		}
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := s.createDeletionNotification(dbs, authorID, uid, &sid, nil, &sfid, deleteReason); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	s.announceNotification()

	return nil
}

func (s *SiteService) SoftDeleteSubmission(ctx context.Context, sid int64, deleteReason string) error {
	uid := utils.UserID(ctx)

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	submissions, _, err := s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{SubmissionIDs: []int64{sid}})
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	authorID := submissions[0].SubmitterID

	if err := s.dal.SoftDeleteSubmission(dbs, sid, deleteReason); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := s.createDeletionNotification(dbs, authorID, uid, &sid, nil, nil, deleteReason); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	s.announceNotification()

	return nil
}

func (s *SiteService) SoftDeleteComment(ctx context.Context, cid int64, deleteReason string) error {
	uid := utils.UserID(ctx)

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	c, err := s.dal.GetCommentByID(dbs, cid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := s.dal.SoftDeleteComment(dbs, cid, deleteReason); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := s.createDeletionNotification(dbs, c.AuthorID, uid, &c.SubmissionID, &cid, nil, deleteReason); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	s.announceNotification()

	return nil
}

func (s *SiteService) SaveUser(ctx context.Context, discordUser *types.DiscordUser) (*authToken, error) {
	getServerRoles := func() (interface{}, error) {
		return s.authBot.GetFlashpointRoles()
	}
	const getServerRolesKey = "getServerRoles"

	// get discord server roles
	sr, err, cached := s.discordRoleCache.Memoize(getServerRolesKey, getServerRoles)
	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("reading server roles from discord")

	if err != nil {
		s.discordRoleCache.Storage.Delete(getServerRolesKey)
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	serverRoles := sr.([]types.DiscordRole)

	getUserRoles := func() (interface{}, error) {
		return s.authBot.GetFlashpointRoleIDsForUser(discordUser.ID)
	}
	const getUserRolesKey = "getUserRoles"

	// get discord user roles
	urid, err, cached := s.discordRoleCache.Memoize(getUserRolesKey, getUserRoles)
	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("reading user roles from discord")

	if err != nil {
		s.discordRoleCache.Storage.Delete(getUserRolesKey)
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	userRoleIDs := urid.([]string)

	// start session
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	userExists := true
	_, err = s.dal.GetDiscordUser(dbs, discordUser.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			userExists = false
		} else {
			utils.LogCtx(ctx).Error(err)
			return nil, dberr(err)
		}
	}

	// save discord user data
	if err := s.dal.StoreDiscordUser(dbs, discordUser); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	// enable all notifications for a new user
	if !userExists {
		if err := s.dal.StoreNotificationSettings(dbs, discordUser.ID, constants.GetActionsWithNotification()); err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, dberr(err)
		}
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
		return nil, dberr(err)
	}
	if err := s.dal.StoreDiscordUserRoles(dbs, discordUser.ID, userRolesIDsNumeric); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	// create cookie and save session
	authToken, err := s.authTokenProvider.CreateAuthToken(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	if err = s.dal.StoreSession(dbs, authToken.Secret, discordUser.ID, s.sessionExpirationSeconds); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	return authToken, nil
}

func (s *SiteService) Logout(ctx context.Context, secret string) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	if err := s.dal.DeleteSession(dbs, secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	return nil
}

func (s *SiteService) GetUserRoles(ctx context.Context, uid int64) ([]string, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	roles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	return roles, nil
}

func (s *SiteService) GetProfilePageData(ctx context.Context, uid int64) (*types.ProfilePageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	notificationActions, err := s.dal.GetNotificationSettingsByUserID(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	pageData := &types.ProfilePageData{
		BasePageData:        *bpd,
		NotificationActions: notificationActions,
	}

	return pageData, nil
}

func (s *SiteService) UpdateNotificationSettings(ctx context.Context, uid int64, notificationActions []string) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	if err := s.dal.StoreNotificationSettings(dbs, uid, notificationActions); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	return nil
}

func (s *SiteService) UpdateSubscriptionSettings(ctx context.Context, uid, sid int64, subscribe bool) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	if subscribe {
		if err := s.dal.SubscribeUserToSubmission(dbs, uid, sid); err != nil {
			utils.LogCtx(ctx).Error(err)
			return dberr(err)
		}
	} else {
		if err := s.dal.UnsubscribeUserFromSubmission(dbs, uid, sid); err != nil {
			utils.LogCtx(ctx).Error(err)
			return dberr(err)
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	return nil
}

func (s *SiteService) GetCurationImage(ctx context.Context, ciid int64) (*types.CurationImage, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	ci, err := s.dal.GetCurationImage(dbs, ciid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return ci, nil
}

func (s *SiteService) GetNextSubmission(ctx context.Context, sid int64) (*int64, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	nsid, err := s.dal.GetNextSubmission(dbs, sid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return &nsid, nil
}

func (s *SiteService) GetPreviousSubmission(ctx context.Context, sid int64) (*int64, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	psid, err := s.dal.GetPreviousSubmission(dbs, sid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return &psid, nil
}

// UpdateMasterDB TODO internal, not covered by tests
func (s *SiteService) UpdateMasterDB(ctx context.Context) error {
	utils.LogCtx(ctx).Debug("downloading new masterdb")
	databaseBytes, err := utils.GetURL("https://bluebot.unstable.life/master-db")
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}

	utils.LogCtx(ctx).Debug("writing masterdb to temp file")
	tmpDB, err := ioutil.TempFile("", "db*.sqlite3")
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}
	defer func() {
		tmpDB.Close()
		os.Remove(tmpDB.Name())
	}()

	_, err = tmpDB.Write(databaseBytes)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}
	tmpDB.Close()

	utils.LogCtx(ctx).Debug("opening masterdb")
	db, err := sql.Open("sqlite3", tmpDB.Name()+"?mode=ro")
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}

	utils.LogCtx(ctx).Debug("reading masterdb")
	rows, err := db.Query(`
		SELECT id, title, alternateTitles, series, developer, publisher, platform, extreme, playMode, status, notes,
		       source, launchCommand, releaseDate, version, originalDescription, language, library, tagsStr, dateAdded, dateModified
		FROM game`)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}

	games := make([]*types.MasterDatabaseGame, 0, 100000)

	var isExtreme bool

	for rows.Next() {
		g := &types.MasterDatabaseGame{}
		err := rows.Scan(
			&g.UUID, &g.Title, &g.AlternateTitles, &g.Series, &g.Developer, &g.Publisher, &g.Platform,
			&isExtreme, &g.PlayMode, &g.Status, &g.GameNotes, &g.Source, &g.LaunchCommand, &g.ReleaseDate,
			&g.Version, &g.OriginalDescription, &g.Languages, &g.Library, &g.Tags, &g.DateAdded, &g.DateModified)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return err
		}
		if isExtreme {
			yes := "Yes"
			g.Extreme = &yes
		} else {
			no := "No"
			g.Extreme = &no
		}

		games = append(games, g)
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	utils.LogCtx(ctx).Debug("clearing masterdb in fpfssdb")
	err = s.dal.ClearMasterDBGames(dbs)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	utils.LogCtx(ctx).Debug("updating masterdb in fpfssdb")
	batch := make([]*types.MasterDatabaseGame, 0, 1000)

	for i, g := range games {
		batch = append(batch, g)

		if i%1000 == 0 || i == len(games)-1 {
			utils.LogCtx(ctx).Debug("inserting masterdb batch into fpfssdb")
			err = s.dal.StoreMasterDBGames(dbs, batch)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return dberr(err)
			}
			batch = make([]*types.MasterDatabaseGame, 0, 1000)
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	utils.LogCtx(ctx).Debug("masterdb update finished")
	return nil
}

func (s *SiteService) getSimilarityScores(dbs database.DBSession, minimumMatch float64, title, launchCommand *string) ([]*types.SimilarityAttributes, []*types.SimilarityAttributes, error) {
	ctx := dbs.Ctx()
	start := time.Now()

	sas, err := s.dal.GetAllSimilarityAttributes(dbs)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, dberr(err)
	}

	byTitle := make([]*types.SimilarityAttributes, 0)
	byLaunchCommand := make([]*types.SimilarityAttributes, 0)

	if len(sas) < 2 {
		return byTitle, byLaunchCommand, nil
	}

	normalize := func(s string) string {
		return strings.ReplaceAll(
			strings.ReplaceAll(
				strings.ReplaceAll(
					strings.ReplaceAll(
						s, "`", ""),
					" ", ""),
				"'", ""),
			`"`, "")
	}

	var nt string
	if title != nil {
		nt = normalize(*title)
	}

	var nlc string
	if launchCommand != nil {
		nlc = normalize(*launchCommand)
	}

	for _, sa := range sas {
		if title != nil && sa.Title != nil {
			nc := normalize(*sa.Title)
			distance := levenshtein.ComputeDistance(nt, nc)
			matchRatio := 1 - (float64(distance) / math.Max(float64(len(nt)), float64(len(nc))))

			if matchRatio > minimumMatch {
				sa.TitleRatio = matchRatio
				byTitle = append(byTitle, sa)
			}
		}
		if launchCommand != nil && sa.LaunchCommand != nil {
			nc := normalize(*sa.LaunchCommand)
			distance := levenshtein.ComputeDistance(nlc, nc)
			matchRatio := 1 - (float64(distance) / math.Max(float64(len(nlc)), float64(len(nc))))

			if matchRatio > minimumMatch {
				sa.LaunchCommandRatio = matchRatio
				byLaunchCommand = append(byLaunchCommand, sa)
			}
		}
	}

	sort.Slice(byTitle, func(i, j int) bool {
		return byTitle[i].TitleRatio > byTitle[j].TitleRatio
	})

	sort.Slice(byLaunchCommand, func(i, j int) bool {
		return byLaunchCommand[i].LaunchCommandRatio > byLaunchCommand[j].LaunchCommandRatio
	})

	duration := time.Since(start)
	utils.LogCtx(ctx).WithField("duration_ns", duration.Nanoseconds()).Debug("similarity scores calculated")

	return byTitle, byLaunchCommand, nil
}

func isFlasfhreezeExtensionValid(filename string) (bool, string) {
	ext := filepath.Ext(filename)

	extless := filename[:len(filename)-len(ext)]
	ext2 := filepath.Ext(extless)

	if ext2 == ".warc" || ext2 == ".tar" {
		ext = ext2 + ext
	}

	if ext != ".7z" && ext != ".zip" && ext != ".rar" &&
		ext != ".tar" && ext != ".tar.gz" && ext != ".tar.bz2" && ext != ".tar.xz" && ext != ".tar.zst" && ext != ".tar.zstd" && ext != ".tgz" &&
		ext != ".arc" && ext != ".warc" && ext != ".warc.gz" && ext != ".arc.gz" {
		return false, ext
	}

	return true, ext
}

func (s *SiteService) processReceivedFlashfreezeItem(ctx context.Context, dbs database.DBSession, uid int64, fileReadCloserProvider ReadCloserProvider, filename string, filesize int64) (*string, *int64, error) {
	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes", filename, filesize)

	if err := os.MkdirAll(s.flashfreezeDir, os.ModeDir); err != nil {
		return nil, nil, err
	}

	ok, ext := isFlasfhreezeExtensionValid(filename)
	if !ok {
		return nil, nil, perr("unsupported file extension", http.StatusUnsupportedMediaType)
	}

	var destinationFilename string
	var destinationFilePath string

	for {
		destinationFilename = s.randomStringProvider.RandomString(64) + ext
		destinationFilePath = fmt.Sprintf("%s/%s", s.flashfreezeDir, destinationFilename)
		if !utils.FileExists(destinationFilePath) {
			break
		}
	}

	var err error

	readCloser, err := fileReadCloserProvider.GetReadCloser()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, err
	}
	defer readCloser.Close()

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, err
	}
	defer destination.Close()

	utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	md5sum := md5.New()
	sha256sum := sha256.New()
	multiWriter := io.MultiWriter(destination, sha256sum, md5sum)

	nBytes, err := io.Copy(multiWriter, readCloser)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, err
	}
	if nBytes != filesize {
		err := fmt.Errorf("incorrect number of bytes copied to destination")
		utils.LogCtx(ctx).Error(err)
		return nil, nil, err
	}

	sf := &types.FlashfreezeFile{
		UserID:           uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             filesize,
		UploadedAt:       s.clock.Now(),
		MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
		SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
	}

	fid, err := s.dal.StoreFlashfreezeRootFile(dbs, sf)
	if err != nil {
		me, ok := err.(*mysql.MySQLError)
		if ok {
			if me.Number == 1062 {
				return &destinationFilePath, nil, perr(fmt.Sprintf("file '%s' with checksums md5:%s sha256:%s already present in the DB", filename, sf.MD5Sum, sf.SHA256Sum), http.StatusConflict)
			}
		}
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, dberr(err)
	}

	return &destinationFilePath, &fid, nil
}

func (s *SiteService) GetSearchFlashfreezeData(ctx context.Context, filter *types.FlashfreezeFilter) (*types.SearchFlashfreezePageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	flashfreezeFiles, count, err := s.dal.SearchFlashfreezeFiles(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	pageData := &types.SearchFlashfreezePageData{
		BasePageData:     *bpd,
		FlashfreezeFiles: flashfreezeFiles,
		TotalCount:       count,
		Filter:           *filter,
	}

	return pageData, nil
}

func (s *SiteService) GetFlashfreezeRootFile(ctx context.Context, fid int64) (*types.FlashfreezeFile, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	ci, err := s.dal.GetFlashfreezeRootFile(dbs, fid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return ci, nil
}

func (s *SiteService) processReceivedResumableSubmission(ctx context.Context, uid int64, sid *int64, resumableParams *types.ResumableParams) (*int64, error) {
	var destinationFilename *string
	imageFilePaths := make([]string, 0)

	cleanup := func() {
		if destinationFilename != nil {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", *destinationFilename)
			if err := os.Remove(*destinationFilename); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
		for _, fp := range imageFilePaths {
			utils.LogCtx(ctx).Debugf("cleaning up image file '%s'...", fp)
			if err := os.Remove(fp); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if constants.IsInAudit(userRoles) && resumableParams.ResumableTotalSize > constants.UserInAuditSubmissionMaxFilesize {
		return nil, perr("submission filesize limited to 500MB for users in audit", http.StatusForbidden)
	}

	var submissionLevel string

	if constants.IsInAudit(userRoles) {
		submissionLevel = constants.SubmissionLevelAudition
	} else if constants.IsTrialCurator(userRoles) {
		submissionLevel = constants.SubmissionLevelTrial
	} else if constants.IsStaff(userRoles) {
		submissionLevel = constants.SubmissionLevelStaff
	}

	ru := newResumableUpload(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks, s.resumableUploadService)
	destinationFilename, ifp, submissionID, err := s.processReceivedSubmission(ctx, dbs, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize, sid, submissionLevel)

	for _, imageFilePath := range ifp {
		imageFilePaths = append(imageFilePaths, imageFilePath)
	}

	if err != nil {
		cleanup()
		return nil, err
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("submissions received")
	s.announceNotification()

	return &submissionID, nil
}

func (s *SiteService) processReceivedResumableFlashfreeze(ctx context.Context, uid int64, resumableParams *types.ResumableParams) (*int64, error) {
	var destinationFilename *string

	cleanup := func() {
		if destinationFilename != nil {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", *destinationFilename)
			if err := os.Remove(*destinationFilename); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	ru := newResumableUpload(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks, s.resumableUploadService)
	destinationFilePath, fid, err := s.processReceivedFlashfreezeItem(ctx, dbs, uid, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize)
	if err != nil {
		return nil, err
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("flashfreeze items received")

	l := utils.LogCtx(ctx).WithFields(logrus.Fields{"flashfreezeFileID": *fid, "destinationFilePath": *destinationFilePath})
	go s.indexReceivedFlashfreezeFile(l, *fid, *destinationFilePath)

	return fid, nil
}

func (s *SiteService) indexReceivedFlashfreezeFile(l *logrus.Entry, fid int64, filePath string) {
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l)
	utils.LogCtx(ctx).Debug("indexing flashfreeze file")

	files, indexingErrors, err := provideArchiveForIndexing(filePath, s.archiveIndexerServerURL)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}
	defer dbs.Rollback()

	batch := make([]*types.IndexedFileEntry, 0, 1000)

	for i, g := range files {
		batch = append(batch, g)

		if i%1000 == 0 || i == len(files)-1 {
			utils.LogCtx(ctx).Debug("inserting flashfreeze file contents batch into fpfssdb")
			err = s.dal.StoreFlashfreezeDeepFile(dbs, fid, batch)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}
			batch = make([]*types.IndexedFileEntry, 0, 1000)
		}
	}

	t := s.clock.Now()
	err = s.dal.UpdateFlashfreezeRootFileIndexedState(dbs, fid, &t, indexingErrors)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	utils.LogCtx(ctx).Debug("flashfreeze file indexed")
}

func uploadArchiveForIndexing(ctx context.Context, filePath string, baseUrl string) ([]*types.IndexedFileEntry, uint64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}

	fn := strings.Split(filePath, "/")
	fakeFilename := fn[len(fn)-1]

	bytes, err := utils.UploadMultipartFile(ctx, baseUrl+"/upload", f, fakeFilename)

	var ir types.IndexerResp
	err = json.Unmarshal(bytes, &ir)
	if err != nil {
		return nil, 0, err
	}

	return ir.Files, ir.IndexingErrors, nil
}

func provideArchiveForIndexing(filePath string, baseUrl string) ([]*types.IndexedFileEntry, uint64, error) {
	client := http.Client{}
	resp, err := client.Post(fmt.Sprintf("%s/provide-path?path=%s", baseUrl, url.QueryEscape(filePath)), "application/json;charset=utf-8", nil)
	if err != nil {
		return nil, 0, err
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("provide to remote error: %s", string(bytes))
	}

	var ir types.IndexerResp
	err = json.Unmarshal(bytes, &ir)
	if err != nil {
		return nil, 0, err
	}

	return ir.Files, ir.IndexingErrors, nil
}

func (s *SiteService) IngestFlashfreezeItems(l *logrus.Entry) {
	guard := make(chan struct{}, 12)
	files, err := ioutil.ReadDir(s.flashfreezeIngestDir)
	if err != nil {
		l.Error(err)
	}

	for _, fileInfo := range files {
		guard <- struct{}{}
		fileInfo := fileInfo
		go func() {
			defer func() { <-guard }()
			ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l.WithField("filename", fileInfo.Name()))
			fullFilepath := s.flashfreezeIngestDir + "/" + fileInfo.Name()

			ok, ext := isFlasfhreezeExtensionValid(fileInfo.Name())
			if !ok {
				utils.LogCtx(ctx).Warn("unsupported file extension")
				return
			}

			var destinationFilename string
			var destinationFilePath string

			for {
				destinationFilename = s.randomStringProvider.RandomString(64) + ext
				destinationFilePath = fmt.Sprintf("%s/%s", s.flashfreezeDir, destinationFilename)
				if !utils.FileExists(destinationFilePath) {
					break
				}
			}

			if err := os.Rename(fullFilepath, destinationFilePath); err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}

			md5sum := md5.New()
			sha256sum := sha256.New()
			multiWriter := io.MultiWriter(sha256sum, md5sum)

			f, err := os.Open(destinationFilePath)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}

			utils.LogCtx(ctx).Debug("computing checksums...")
			nBytes, err := io.Copy(multiWriter, f)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}
			if nBytes != fileInfo.Size() {
				err := fmt.Errorf("incorrect number of bytes copied to destination")
				utils.LogCtx(ctx).Error(err)
				return
			}

			dbs, err := s.dal.NewSession(ctx)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}
			defer dbs.Rollback()

			sf := &types.FlashfreezeFile{
				UserID:           constants.SystemID,
				OriginalFilename: fileInfo.Name(),
				CurrentFilename:  destinationFilename,
				Size:             fileInfo.Size(),
				UploadedAt:       s.clock.Now(),
				MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
				SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
			}

			fid, err := s.dal.StoreFlashfreezeRootFile(dbs, sf)
			if err != nil {
				me, ok := err.(*mysql.MySQLError)
				if ok {
					if me.Number == 1062 {
						err := fmt.Errorf("file '%s' with checksums md5:%s sha256:%s already present in the DB", fileInfo.Name(), sf.MD5Sum, sf.SHA256Sum)
						utils.LogCtx(ctx).Error(err)
						return
					}
				}
				utils.LogCtx(ctx).Error(err)
				return
			}

			if err := dbs.Commit(); err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}

			utils.LogCtx(ctx).WithField("amount", 1).Debug("flashfreeze items received")

			l := utils.LogCtx(ctx).WithFields(logrus.Fields{"flashfreezeFileID": fid, "destinationFilepath": destinationFilePath})
			s.indexReceivedFlashfreezeFile(l, fid, destinationFilePath)
		}()
	}
}

func (s *SiteService) RecomputeSubmissionCacheAll(l *logrus.Entry) {
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l)

	submissions, _, err := s.SearchSubmissions(ctx, nil)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	for _, submission := range submissions {
		func() {
			dbs, err := s.dal.NewSession(ctx)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}
			defer dbs.Rollback()

			err = s.dal.UpdateSubmissionCacheTable(dbs, submission.SubmissionID)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}

			if err := dbs.Commit(); err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}
		}()
	}
}
