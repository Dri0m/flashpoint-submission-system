package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	cache2 "github.com/patrickmn/go-cache"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
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

	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/go-sql-driver/mysql"
	"github.com/kofalt/go-memoize"
	"golang.org/x/sync/errgroup"

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
	pgdal                     database.PGDAL
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
	metadataStatsCache        *memoize.Memoizer
	resumableUploadService    *resumableuploadservice.ResumableUploadService
	archiveIndexerServerURL   string
	flashfreezeIngestDir      string
	fixesDir                  string
	SSK                       SubmissionStatusKeeper
	DataPacksIndexer          ZipIndexer
}

func New(l *logrus.Entry, db *sql.DB, pgdb *pgxpool.Pool, authBotSession, notificationBotSession *discordgo.Session,
	flashpointServerID, notificationChannelID, curationFeedChannelID, validatorServerURL string,
	sessionExpirationSeconds int64, submissionsDir, submissionImagesDir, flashfreezeDir string, isDev bool,
	rsu *resumableuploadservice.ResumableUploadService, archiveIndexerServerURL, flashfreezeIngestDir, fixesDir string,
	dataPacksDir string) *SiteService {

	return &SiteService{
		authBot:                   authbot.NewBot(authBotSession, flashpointServerID, l.WithField("botName", "authBot"), isDev),
		notificationBot:           notificationbot.NewBot(notificationBotSession, flashpointServerID, notificationChannelID, curationFeedChannelID, l.WithField("botName", "notificationBot"), isDev),
		dal:                       database.NewMysqlDAL(db),
		pgdal:                     database.NewPostgresDAL(pgdb),
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
		metadataStatsCache:        memoize.NewMemoizer(1*time.Minute, cache2.NoExpiration),
		resumableUploadService:    rsu,
		archiveIndexerServerURL:   archiveIndexerServerURL,
		flashfreezeIngestDir:      flashfreezeIngestDir,
		fixesDir:                  fixesDir,
		SSK: SubmissionStatusKeeper{
			m: make(map[string]*types.SubmissionStatus),
		},
		DataPacksIndexer: NewZipIndexer(pgdb, dataPacksDir, l.WithField("botName", "dataPackIndexer")),
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

func (s *SiteService) DeleteGame(ctx context.Context, gameId string, reason string, imagesPath string, gamesPath string, deletedImagesPath string, deletedGamesPath string) error {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	uid := utils.UserID(ctx)

	// Soft delete database entry
	err = s.pgdal.DeleteGame(dbs, gameId, uid, reason, imagesPath, gamesPath, deletedImagesPath, deletedGamesPath)
	if err != nil {
		return err
	}

	// Move game data and images (where exist)
	// @TODO

	err = dbs.Commit()
	if err != nil {
		return dberr(err)
	}

	return nil
}

func (s *SiteService) RestoreGame(ctx context.Context, gameId string, reason string, imagesPath string, gamesPath string, deletedImagesPath string, deletedGamesPath string) error {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	uid := utils.UserID(ctx)

	// Restore database entry
	err = s.pgdal.RestoreGame(dbs, gameId, uid, reason, imagesPath, gamesPath, deletedImagesPath, deletedGamesPath)
	if err != nil {
		return err
	}

	err = dbs.Commit()
	if err != nil {
		return dberr(err)
	}

	return nil
}

func (s *SiteService) GetGamePageData(ctx context.Context, gameId string, imageCdn string, compressedImages bool, revisionDate string) (*types.GamePageData, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	msqldbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer msqldbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	game, err := s.pgdal.GetGame(dbs, gameId)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, perr("game not found", http.StatusNotFound)
	}

	user, err := s.dal.GetDiscordUser(msqldbs, game.UserID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, perr("failed to fetch user info for version", http.StatusNotFound)
	}

	revisions, err := s.pgdal.GetGameRevisionInfo(dbs, gameId)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, perr("failed to find revision info", http.StatusNotFound)
	}
	err = s.dal.PopulateRevisionInfo(msqldbs, revisions)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, perr("failed to populate revision info with user details", http.StatusNotFound)
	}

	// Desc sort revisions
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
	})

	logoUrl := fmt.Sprintf("%s/Logos/%s/%s/%s.png", imageCdn, game.ID[:2], game.ID[2:4], game.ID)
	if compressedImages {
		logoUrl = logoUrl + "?type=jpg"
	}
	ssUrl := fmt.Sprintf("%s/Screenshots/%s/%s/%s.png", imageCdn, game.ID[:2], game.ID[2:4], game.ID)
	if compressedImages {
		ssUrl = ssUrl + "?type=jpg"
	}

	validDeleteReasons := constants.GetValidDeleteReasons()
	validRestoreReasons := constants.GetValidRestoreReasons()

	pageData := &types.GamePageData{
		ImagesCdn:           imageCdn,
		Game:                game,
		LogoUrl:             logoUrl,
		ScreenshotUrl:       ssUrl,
		Revisions:           revisions,
		GameUsername:        user.Username,
		GameAvatarURL:       utils.FormatAvatarURL(user.ID, user.Avatar),
		GameAuthorID:        user.ID,
		BasePageData:        *bpd,
		ValidDeleteReasons:  validDeleteReasons,
		ValidRestoreReasons: validRestoreReasons,
	}

	return pageData, nil
}

func (s *SiteService) GetIndexMatchesHash(ctx context.Context, hashType string, hashStr string) (*types.IndexMatchResult, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	matches, err := s.pgdal.GetIndexMatchesHash(dbs, hashType, hashStr)
	if err != nil {
		if err != pgx.ErrNoRows {
			utils.LogCtx(ctx).Error(err)
			return nil, dberr(err)
		} else {
			// No results, let an empty result happen
			matches = make([]*types.IndexMatchData, 0)
		}
	}

	result := &types.IndexMatchResultData{
		HashType: hashType,
		Hash:     hashStr,
		Matches:  matches,
	}
	data := &types.IndexMatchResult{
		Results: []*types.IndexMatchResultData{result},
	}

	return data, nil
}

func (s *SiteService) GetGameDataIndexPageData(ctx context.Context, gameId string, date int64) (*types.GameDataIndexPageData, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	index, err := s.pgdal.GetGameDataIndex(dbs, gameId, date)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	pageData := &types.GameDataIndexPageData{
		BasePageData: *bpd,
		Index:        index,
	}

	return pageData, nil
}

func (s *SiteService) SaveTag(ctx context.Context, tag *types.Tag) error {
	uid := utils.UserID(ctx)
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	err = s.pgdal.SaveTag(dbs, tag, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	return nil
}

func (s *SiteService) GetTagPageData(ctx context.Context, tagIdStr string) (*types.TagPageData, error) {
	msqldbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer msqldbs.Rollback()

	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	categories, err := s.pgdal.GetTagCategories(dbs)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	var tag *types.Tag
	tagId, err := strconv.Atoi(tagIdStr)
	if err != nil {
		// Not an ID, check against tag name instead
		tag, err = s.pgdal.GetTagByName(dbs, tagIdStr)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, perr("tag not found", http.StatusNotFound)
		}
	} else {
		// Is an ID, use that
		tag, err = s.pgdal.GetTag(dbs, int64(tagId))
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, perr("tag not found", http.StatusNotFound)
		}
	}

	revisions, err := s.pgdal.GetTagRevisionInfo(dbs, tag.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, perr("tag revisions not found?", http.StatusInternalServerError)
	}
	err = s.dal.PopulateRevisionInfo(msqldbs, revisions)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, perr("failed to populate revision info with user details", http.StatusInternalServerError)
	}

	// Desc sort revisions
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
	})

	gamesUsing, err := s.pgdal.GetGamesUsingTagTotal(dbs, tag.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	pageData := &types.TagPageData{
		Tag:          tag,
		Categories:   categories,
		GamesUsing:   gamesUsing,
		Revisions:    revisions,
		BasePageData: *bpd,
	}

	return pageData, nil
}

func (s *SiteService) GetTagsPageData(ctx context.Context, modifiedAfter *string) (*types.TagsPageData, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	categories, err := s.pgdal.GetTagCategories(dbs)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	tags, err := s.pgdal.SearchTags(dbs, modifiedAfter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	pageData := &types.TagsPageData{
		Tags:         tags,
		Categories:   categories,
		BasePageData: *bpd,
		TotalCount:   int64(len(tags)),
	}

	return pageData, nil
}

func (s *SiteService) GetPlatformsPageData(ctx context.Context, modifiedAfter *string) (*types.PlatformsPageData, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	platforms, err := s.pgdal.SearchPlatforms(dbs, modifiedAfter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	pageData := &types.PlatformsPageData{
		Platforms:    platforms,
		BasePageData: *bpd,
		TotalCount:   int64(len(platforms)),
	}

	return pageData, nil
}

func (s *SiteService) GetApplyContentPatchPageData(ctx context.Context, sid int64) (*types.ApplyContentPatchPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	pgdbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer pgdbs.Rollback()

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

	if meta.UUID == nil || *meta.UUID == "" || !meta.GameExists {
		return nil, types.NotContentPatch{}
	}

	game, err := s.pgdal.GetGame(pgdbs, *meta.UUID)
	if err != nil {
		return nil, dberr(err)
	}

	pageData := &types.ApplyContentPatchPageData{
		BasePageData: *bpd,
		SubmissionID: submission.SubmissionID,
		CurationMeta: meta,
		ExistingMeta: game,
	}

	return pageData, nil
}

func (s *SiteService) GetViewSubmissionPageData(ctx context.Context, uid, sid int64) (*types.ViewSubmissionPageData, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	pgdbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer pgdbs.Rollback()

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

	if c.AuthorID == constants.ValidatorID {
		err := perr("cannot delete validator's comment", http.StatusForbidden)
		utils.LogCtx(ctx).Error(err)
		return err
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

func (s *SiteService) OverrideBot(ctx context.Context, sid int64) error {
	uid := utils.UserID(ctx)

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	msg := fmt.Sprintf("Approval override by user %d", uid)

	c := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &msg,
		Action:       constants.ActionApprove,
		CreatedAt:    s.clock.Now(),
	}

	if err := s.dal.StoreComment(dbs, c); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := s.dal.UpdateSubmissionCacheTable(dbs, sid); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

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
	getUserRolesKey := fmt.Sprintf("getUserRoles-%d", discordUser.ID)

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

func (s *SiteService) GenAuthToken(ctx context.Context, uid int64) (map[string]string, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	authToken, err := s.authTokenProvider.CreateAuthToken(uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	if err = s.dal.StoreSession(dbs, authToken.Secret, uid, s.sessionExpirationSeconds); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	return MapAuthToken(authToken), nil
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

func (s *SiteService) CreateFixFirstStep(ctx context.Context, uid int64, c *types.CreateFixFirstStep) (int64, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}
	defer dbs.Rollback()

	fid, err := s.dal.StoreFixFirstStep(dbs, uid, c)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}

	return fid, nil
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

func (s *SiteService) UpdateMasterDB(ctx context.Context) error {

	// downloading fresh master db is not required anymore, we sticking to 10.1 which is the last version before FPFSS

	// utils.LogCtx(ctx).Debug("downloading new masterdb")
	// databaseBytes, err := utils.GetURL("https://bluebot.unstable.life/master-db")
	// if err != nil {
	// 	utils.LogCtx(ctx).Error(err)
	// 	return err
	// }

	// utils.LogCtx(ctx).Debug("writing masterdb to temp file")
	// tmpDB, err := ioutil.TempFile("", "db*.sqlite3")
	// if err != nil {
	// 	utils.LogCtx(ctx).Error(err)
	// 	return err
	// }
	// defer func() {
	// 	tmpDB.Close()
	// 	os.Remove(tmpDB.Name())
	// }()

	// _, err = tmpDB.Write(databaseBytes)
	// if err != nil {
	// 	utils.LogCtx(ctx).Error(err)
	// 	return err
	// }
	// tmpDB.Close()

	// utils.LogCtx(ctx).Debug("opening masterdb")
	// db, err := sql.Open("sqlite3", tmpDB.Name()+"?mode=ro")
	// if err != nil {
	// 	utils.LogCtx(ctx).Error(err)
	// 	return err
	// }

	utils.LogCtx(ctx).Debug("opening masterdb")
	db, err := sql.Open("sqlite3", "masterdb.sqlite?mode=ro")
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

	// TODO refactor this into something more proper
	// change minimum match for some specific launch commands
	if launchCommand != nil {
		// itch.io games
		if strings.Contains(*launchCommand, "ssl.hwcdn.net/html") {
			minimumMatch = 0.95
		}
	}

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

func (s *SiteService) processReceivedFlashfreezeItem(ctx context.Context, dbs database.DBSession, uid int64, fileReadCloserProvider resumableuploadservice.ReadCloserInformerProvider, filename string, filesize int64) (*string, *int64, error) {
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

	readCloser, err := fileReadCloserProvider.GetReadCloserInformer()
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

	utils.LogCtx(ctx).Debugf("copying flashfreeze file to '%s'...", destinationFilePath)

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

func (s *SiteService) processReceivedResumableSubmission(ctx context.Context, uid int64, sid *int64, resumableParams *types.ResumableParams, tempName string) error {
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
		s.SSK.SetFailed(tempName, "internal error")
		return dberr(err)
	}
	defer dbs.Rollback()

	pgdbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		s.SSK.SetFailed(tempName, "internal error")
		return dberr(err)
	}
	defer pgdbs.Rollback()

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		s.SSK.SetFailed(tempName, "internal error")
		return dberr(err)
	}

	if constants.IsInAudit(userRoles) && resumableParams.ResumableTotalSize > constants.UserInAuditSubmissionMaxFilesize {
		msg := "submission filesize limited to 500MB for users in audit"
		s.SSK.SetFailed(tempName, msg)
		return perr(msg, http.StatusForbidden)
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
	destinationFilename, ifp, submissionID, err := s.processReceivedSubmission(ctx, dbs, pgdbs, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize, sid, submissionLevel, tempName)

	imageFilePaths = append(imageFilePaths, ifp...)

	if err != nil {
		cleanup()
		return err
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		s.SSK.SetFailed(tempName, "internal error")
		cleanup()
		return dberr(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("submissions received")
	s.announceNotification()

	s.SSK.SetSuccess(tempName, submissionID)

	return nil
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

func provideArchiveForIndexing(filePath string, baseUrl string) ([]*types.IndexedFileEntry, uint64, error) {
	client := http.Client{}
	resp, err := client.Post(fmt.Sprintf("%s/provide-path?path=%s", baseUrl, url.QueryEscape(filePath)), "application/json;charset=utf-8", nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

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

func (s *SiteService) ingestGivenFlashfreezeItems(l *logrus.Entry, files []fs.FileInfo, rootDir string) {
	guard := make(chan struct{}, 3)

	mutex := sync.Mutex{}

	for _, fileInfo := range files {
		guard <- struct{}{}
		fileInfo := fileInfo
		go func() {
			defer func() { <-guard }()
			if fileInfo.IsDir() {
				return
			}
			ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l.WithField("filename", fileInfo.Name()))
			fullFilepath := rootDir + "/" + fileInfo.Name()

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

			md5sum := md5.New()
			sha256sum := sha256.New()
			multiWriter := io.MultiWriter(sha256sum, md5sum)

			f, err := os.Open(fullFilepath)
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

			mutex.Lock()
			defer mutex.Unlock()
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

			if err := os.Rename(fullFilepath, destinationFilePath); err != nil {
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

func (s *SiteService) IngestFlashfreezeItems(l *logrus.Entry) {
	l.WithField("directory", s.flashfreezeIngestDir).Debug("listing directory")

	files, err := ioutil.ReadDir(s.flashfreezeIngestDir)
	if err != nil {
		l.Error(err)
		return
	}

	s.ingestGivenFlashfreezeItems(l, files, s.flashfreezeIngestDir)
}

func (s *SiteService) IngestUnknownFlashfreezeItems(l *logrus.Entry) {
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l)

	utils.LogCtx(ctx).WithField("directory", s.flashfreezeDir).Debug("listing directory")

	allFiles, err := ioutil.ReadDir(s.flashfreezeDir)
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

	ingestedFiles, err := s.dal.GetAllFlashfreezeRootFiles(dbs)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	files := make([]fs.FileInfo, 0, len(allFiles)-len(ingestedFiles))

	for _, candidateFile := range allFiles {
		if candidateFile.IsDir() {
			continue
		}
		found := false
		for _, ingestedFile := range ingestedFiles {
			if candidateFile.Name() == ingestedFile.CurrentFilename {
				found = true
				break
			}
		}

		if !found {
			files = append(files, candidateFile)
		}
	}

	if len(files) == 0 {
		utils.LogCtx(ctx).Debug("found no unknown flashfreeze items")
		return
	}

	utils.LogCtx(ctx).WithField("unknownFlashfreezeItems", len(files)).Debug("found some unknown flashfreeze items")
	s.ingestGivenFlashfreezeItems(l, files, s.flashfreezeDir)
}

func (s *SiteService) RecomputeSubmissionCacheAll(ctx context.Context) {

	var perPage int64 = 10000
	var count int64 = 1
	var recomputedCount int64 = 0

	for recomputedCount < count {
		var submissions []*types.ExtendedSubmission
		var err error
		submissions, count, err = s.SearchSubmissions(ctx, &types.SubmissionsFilter{ResultsPerPage: &perPage, ExcludeLegacy: true})
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return
		}
		utils.LogCtx(ctx).WithField("perPage", perPage).WithField("recomputedCount", recomputedCount).WithField("totalSubmissions", count).Debug("processing a page of submissions")

		for _, submission := range submissions {
			func() {
				utils.LogCtx(ctx).WithField("submissionID", submission.SubmissionID).Debug("recomputing cache for submission")

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

		recomputedCount += count
	}
}

func (s *SiteService) IndexUnindexedFlashfreezeItems(l *logrus.Entry) {
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l)

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}
	defer dbs.Rollback()

	unindexedFiles, err := s.dal.GetAllUnindexedFlashfreezeRootFiles(dbs)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	if len(unindexedFiles) == 0 {
		utils.LogCtx(ctx).Debug("found no unindexed flashfreeze files")
	}

	utils.LogCtx(ctx).WithField("unindexedFlashfreezeItems", len(unindexedFiles)).Debug("found some unindexed flashfreeze files")

	for _, unindexedFile := range unindexedFiles {
		destinationFilePath := s.flashfreezeDir + "/" + unindexedFile.CurrentFilename
		s.indexReceivedFlashfreezeFile(l, unindexedFile.ID, destinationFilePath)
	}
}

func (s *SiteService) GetFixByID(ctx context.Context, fid int64) (*types.Fix, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	f, err := s.dal.GetFixByID(dbs, fid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return f, nil
}

func (s *SiteService) DeleteUserSessions(ctx context.Context, uid int64) (int64, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}
	defer dbs.Rollback()

	count, err := s.dal.DeleteUserSessions(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, dberr(err)
	}

	return count, nil
}

func (s *SiteService) GetStatisticsPageData(ctx context.Context) (*types.StatisticsPageData, error) {
	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	errs, ectx := errgroup.WithContext(ctx)

	var sc int64
	var scbh int64
	var scbs int64
	var sca int64
	var scv int64
	var scr int64
	var scif int64
	var uc int64
	var cc int64
	var ffc int64
	var fffc int64
	var tss int64
	var tffs int64

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		_, sc, err = s.dal.SearchSubmissions(dbs, nil)
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		_, scbh, err = s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{BotActions: []string{"approve"}})
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		_, scbs, err = s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{BotActions: []string{"request-changes"}})
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		approved := "approved"
		_, sca, err = s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{ApprovalsStatus: &approved})
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		verified := "verified"
		_, scv, err = s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{VerificationStatus: &verified})
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		_, scr, err = s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{DistinctActions: []string{"reject"}})
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		_, scif, err = s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{DistinctActions: []string{"mark-added"}})
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		uc, err = s.dal.GetTotalUserCount(dbs)
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		cc, err = s.dal.GetTotalCommentsCount(dbs)
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		ffc, err = s.dal.GetTotalFlashfreezeCount(dbs)
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		fffc, err = s.dal.GetTotalFlashfreezeFileCount(dbs)
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		tss, err = s.dal.GetTotalSubmissionFilesize(dbs)
		return err
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error
		tffs, err = s.dal.GetTotalFlashfreezeFilesize(dbs)
		return err
	})

	if err := errs.Wait(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	pageData := &types.StatisticsPageData{
		BasePageData:                *bpd,
		SubmissionCount:             sc,
		SubmissionCountBotHappy:     scbh,
		SubmissionCountBotSad:       scbs,
		SubmissionCountApproved:     sca,
		SubmissionCountVerified:     scv,
		SubmissionCountRejected:     scr,
		SubmissionCountInFlashpoint: scif,
		UserCount:                   uc,
		CommentCount:                cc,
		FlashfreezeCount:            ffc,
		FlashfreezeFileCount:        fffc,
		TotalSubmissionSize:         tss,
		TotalFlashfreezeSize:        tffs,
	}
	return pageData, nil
}

func (s *SiteService) GetUsers(ctx context.Context) ([]*types.User, error) {
	dbs, _ := s.dal.NewSession(ctx)
	defer dbs.Rollback()
	return s.dal.GetUsers(dbs)
}

func (s *SiteService) GetUserStatistics(ctx context.Context, uid int64) (*types.UserStatistics, error) {
	user, isTrial, isStaff, err := func() (*types.DiscordUser, bool, bool, error) {
		dbs, _ := s.dal.NewSession(ctx)
		defer dbs.Rollback()
		du, err := s.dal.GetDiscordUser(dbs, uid)
		roles, err := s.GetUserRoles(ctx, uid)
		return du, constants.IsTrialCurator(roles), constants.IsStaff(roles), err
	}()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, perr("user not found", http.StatusNotFound)
		}
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	role := "User"
	if isTrial {
		role = constants.RoleTrialCurator
	}
	if isStaff {
		role = "Staff"
	}

	us := &types.UserStatistics{
		UserID:   user.ID,
		Username: user.Username,
		Role:     role,
	}

	errs, ectx := errgroup.WithContext(ctx)

	// get the latest user activity, which is a pain in the ass to obtain

	var lastUploadedSubmission *time.Time
	var lastUpdatedSubmission *time.Time
	//var lastUploadedFlashfreezeSubmission *time.Time
	var latestUserActivity time.Time

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error

		filter := &types.SubmissionsFilter{
			SubmitterID:    &user.ID,
			ResultsPerPage: utils.Int64Ptr(1),
			OrderBy:        utils.StrPtr("uploaded"),
			AscDesc:        utils.StrPtr("desc"),
		}

		subs, _, err := s.dal.SearchSubmissions(dbs, filter)
		if err != nil {
			return err
		}
		if len(subs) > 0 {
			lastUploadedSubmission = &subs[0].UploadedAt
		}

		return nil
	})

	errs.Go(func() error {
		dbs, _ := s.dal.NewSession(ectx)
		defer dbs.Rollback()
		var err error

		filter := &types.SubmissionsFilter{
			UpdatedByID:    &user.ID,
			ResultsPerPage: utils.Int64Ptr(1),
			OrderBy:        utils.StrPtr("updated"),
			AscDesc:        utils.StrPtr("desc"),
		}

		subs, _, err := s.dal.SearchSubmissions(dbs, filter)
		if err != nil {
			return err
		}

		if len(subs) > 0 {
			lastUpdatedSubmission = &subs[0].UpdatedAt
		}

		return nil
	})

	// TODO: flashfreeze is slow as fuck
	// errs.Go(func() error {
	// 	dbs, _ := s.dal.NewSession(ectx)
	// 	defer dbs.Rollback()
	// 	var err error

	// 	filter := &types.FlashfreezeFilter{
	// 		SubmitterID:    &user.ID,
	// 		ResultsPerPage: utils.Int64Ptr(1),
	// 		// flashfreeze search is sorted by descending upload date
	// 	}

	// 	files, _, err := s.dal.SearchFlashfreezeFiles(dbs, filter)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	if len(files) > 0 {
	// 		lastUploadedFlashfreezeSubmission = files[0].UploadedAt
	// 	}

	// 	return nil
	// })

	if err := errs.Wait(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	// latestUserActivity = utils.NilTime(lastUploadedFlashfreezeSubmission)
	// if utils.NilTime(lastUpdatedSubmission).After(latestUserActivity) {
	// 	latestUserActivity = utils.NilTime(lastUpdatedSubmission)
	// }
	latestUserActivity = utils.NilTime(lastUpdatedSubmission)
	if utils.NilTime(lastUploadedSubmission).After(latestUserActivity) {
		latestUserActivity = utils.NilTime(lastUploadedSubmission)
	}

	us.LastUserActivity = latestUserActivity

	if !us.LastUserActivity.IsZero() {

		// get the user actions

		errs, ectx = errgroup.WithContext(ctx)

		var commentedCount int64
		var requestedChangesCount int64
		var approvedCount int64
		var verifiedCount int64
		var addedToFlashpointCount int64
		var rejectedCount int64

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			comments, err := s.dal.GetCommentsByUserIDAndAction(dbs, user.ID, constants.ActionComment)
			if err != nil {
				return err
			}

			commentedCount = int64(len(comments))

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			comments, err := s.dal.GetCommentsByUserIDAndAction(dbs, user.ID, constants.ActionRequestChanges)
			if err != nil {
				return err
			}

			requestedChangesCount = int64(len(comments))

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			comments, err := s.dal.GetCommentsByUserIDAndAction(dbs, user.ID, constants.ActionApprove)
			if err != nil {
				return err
			}

			approvedCount = int64(len(comments))

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			comments, err := s.dal.GetCommentsByUserIDAndAction(dbs, user.ID, constants.ActionVerify)
			if err != nil {
				return err
			}

			verifiedCount = int64(len(comments))

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			comments, err := s.dal.GetCommentsByUserIDAndAction(dbs, user.ID, constants.ActionMarkAdded)
			if err != nil {
				return err
			}

			addedToFlashpointCount = int64(len(comments))

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			comments, err := s.dal.GetCommentsByUserIDAndAction(dbs, user.ID, constants.ActionReject)
			if err != nil {
				return err
			}

			rejectedCount = int64(len(comments))

			return nil
		})

		if err := errs.Wait(); err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, err
		}

		us.UserCommentedCount = commentedCount
		us.UserRequestedChangesCount = requestedChangesCount
		us.UserApprovedCount = approvedCount
		us.UserVerifiedCount = verifiedCount
		us.UserAddedToFlashpointCount = addedToFlashpointCount
		us.UserRejectedCount = rejectedCount

		// actions on user's submissions

		errs, ectx = errgroup.WithContext(ctx)

		var submissionsCount int64
		var submissionsBotHappyCount int64
		var submissionsBotUnhappyCount int64
		var submissionsRequestedChangesCount int64
		var submissionsApprovedCount int64
		var submissionsVerifiedCount int64
		var submissionsAddedToFlashpointCount int64
		var submissionsRejectedCount int64

		errs, ectx = errgroup.WithContext(ctx)

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID: &user.ID,
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID: &user.ID,
				BotActions:  []string{constants.ActionApprove},
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsBotHappyCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID: &user.ID,
				BotActions:  []string{constants.ActionRequestChanges},
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsBotUnhappyCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID:            &user.ID,
				RequestedChangedStatus: utils.StrPtr("ongoing"),
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsRequestedChangesCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID:        &user.ID,
				ApprovalsStatus:    utils.StrPtr("approved"),
				VerificationStatus: utils.StrPtr("none"),
				DistinctActionsNot: []string{constants.ActionReject, constants.ActionMarkAdded},
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsApprovedCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID:        &user.ID,
				VerificationStatus: utils.StrPtr("verified"),
				DistinctActionsNot: []string{constants.ActionReject, constants.ActionMarkAdded},
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsVerifiedCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID:        &user.ID,
				DistinctActions:    []string{constants.ActionMarkAdded},
				DistinctActionsNot: []string{constants.ActionReject},
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsAddedToFlashpointCount = c

			return nil
		})

		errs.Go(func() error {
			dbs, _ := s.dal.NewSession(ectx)
			defer dbs.Rollback()
			var err error

			filter := &types.SubmissionsFilter{
				SubmitterID:     &user.ID,
				DistinctActions: []string{constants.ActionReject},
			}

			_, c, err := s.dal.SearchSubmissions(dbs, filter)
			if err != nil {
				return err
			}

			submissionsRejectedCount = c

			return nil
		})

		if err := errs.Wait(); err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, err
		}

		us.SubmissionsCount = submissionsCount
		us.SubmissionsBotHappyCount = submissionsBotHappyCount
		us.SubmissionsBotUnhappyCount = submissionsBotUnhappyCount
		us.SubmissionsRequestedChangesCount = submissionsRequestedChangesCount
		us.SubmissionsApprovedCount = submissionsApprovedCount
		us.SubmissionsVerifiedCount = submissionsVerifiedCount
		us.SubmissionsAddedToFlashpointCount = submissionsAddedToFlashpointCount
		us.SubmissionsRejectedCount = submissionsRejectedCount
	}

	return us, nil
}

func (s *SiteService) processReceivedResumableFixesFile(ctx context.Context, uid int64, fixID int64, resumableParams *types.ResumableParams) (*int64, error) {
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
	fid, err := s.processReceivedFixesItem(ctx, dbs, uid, fixID, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize)
	if err != nil {
		return nil, err
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("fixes items received")

	return fid, nil
}

func (s *SiteService) processReceivedFixesItem(ctx context.Context, dbs database.DBSession, uid int64, fixID int64, fileReadCloserProvider resumableuploadservice.ReadCloserInformerProvider, filename string, filesize int64) (*int64, error) {
	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes", filename, filesize)

	if err := os.MkdirAll(s.fixesDir, os.ModeDir); err != nil {
		return nil, err
	}

	var destinationFilename string
	var destinationFilePath string

	for {
		destinationFilename = s.randomStringProvider.RandomString(64) + filepath.Ext(filename)
		destinationFilePath = fmt.Sprintf("%s/%s", s.fixesDir, destinationFilename)
		if !utils.FileExists(destinationFilePath) {
			break
		}
	}

	var err error

	readCloser, err := fileReadCloserProvider.GetReadCloserInformer()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}
	defer readCloser.Close()

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}
	defer destination.Close()

	utils.LogCtx(ctx).Debugf("copying fixes file to '%s'...", destinationFilePath)

	md5sum := md5.New()
	sha256sum := sha256.New()
	multiWriter := io.MultiWriter(destination, sha256sum, md5sum)

	nBytes, err := io.Copy(multiWriter, readCloser)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}
	if nBytes != filesize {
		err := fmt.Errorf("incorrect number of bytes copied to destination")
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	ff := &types.FixesFile{
		UserID:           uid,
		FixID:            fixID,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             filesize,
		UploadedAt:       s.clock.Now(),
		MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
		SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
	}

	fid, err := s.dal.StoreFixesFile(dbs, ff)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	return &fid, nil
}

func (s *SiteService) GetSearchFixesData(ctx context.Context, filter *types.FixesFilter) (*types.SearchFixesPageData, error) {
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

	fixes, count, err := s.dal.SearchFixes(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	pageData := &types.SearchFixesPageData{
		BasePageData: *bpd,
		Fixes:        fixes,
		TotalCount:   count,
		Filter:       *filter,
	}

	return pageData, nil
}

func (s *SiteService) GetViewFixPageData(ctx context.Context, fid int64) (*types.ViewFixPageData, error) {
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

	filter := &types.FixesFilter{
		FixIDs: []int64{fid},
	}

	fixes, _, err := s.dal.SearchFixes(dbs, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	files, err := s.dal.GetFilesForFix(dbs, fid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if len(fixes) == 0 {
		return nil, perr("fix not found", http.StatusNotFound)
	}

	pageData := &types.ViewFixPageData{
		SearchFixesPageData: types.SearchFixesPageData{
			BasePageData: *bpd,
			Fixes:        fixes,
		},
		FixesFiles: files,
	}

	return pageData, nil
}

func (s *SiteService) GetFixesFiles(ctx context.Context, ffids []int64) ([]*types.FixesFile, error) {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	ffs, err := s.dal.GetFixesFiles(dbs, ffids)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	return ffs, nil
}

func (s *SiteService) DeveloperTagDescFromValidator(ctx context.Context) error {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	tagsList, err := s.validator.GetTags(ctx)
	if err != nil {
		return err
	}

	err = s.pgdal.UpdateTagsFromTagsList(dbs, tagsList)
	if err != nil {
		return err
	}

	err = dbs.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *SiteService) DeveloperImportDatabaseJson(ctx context.Context, data *types.LauncherDump) error {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	err = s.pgdal.DeveloperImportDatabaseJson(dbs, data)
	if err != nil {
		dbs.Rollback()
		dbs, err := s.pgdal.NewSession(ctx)
		// Enable triggers
		_, err = dbs.Tx().Exec(dbs.Ctx(), `SET session_replication_role = DEFAULT`)
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	err = dbs.Commit()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	dbs, err = s.pgdal.NewSession(ctx)
	// Enable triggers
	_, err = dbs.Tx().Exec(dbs.Ctx(), `SET session_replication_role = DEFAULT`)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	utils.LogCtx(ctx).Debug("commited database import")

	return nil
}

func (s *SiteService) SaveGame(ctx context.Context, game *types.Game) error {
	uid := utils.UserID(ctx)
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	err = s.pgdal.SaveGame(dbs, game, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	return nil
}

func (s *SiteService) GetMetadataStatsPageData(ctx context.Context) (*types.MetadataStatsPageData, error) {
	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	memo, err, _ := s.metadataStatsCache.Memoize("metadataStats", func() (interface{}, error) {
		dbs, err := s.pgdal.NewSession(ctx)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, err
		}
		defer dbs.Rollback()

		data, err := s.pgdal.GetMetadataStats(dbs)
		if err != nil {
			return nil, err
		}

		return data, err
	})
	if err != nil {
		return nil, dberr(err)
	}

	pageData := types.MetadataStatsPageData{
		BasePageData:              *bpd,
		MetadataStatsPageDataBare: *memo.(*types.MetadataStatsPageDataBare),
	}

	return &pageData, nil
}

func (s *SiteService) GetDeletedGamePageData(ctx context.Context, modifiedAfter *string) ([]*types.DeletedGame, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	games, err := s.pgdal.SearchDeletedGames(dbs, modifiedAfter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	return games, nil
}

func (s *SiteService) GetGameCountSinceDate(ctx context.Context, modifiedAfter *string) (int, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return 0, nil
	}
	defer dbs.Rollback()

	total, err := s.pgdal.CountSinceDate(dbs, modifiedAfter)
	if err != nil {
		return 0, dberr(err)
	}

	return total, nil
}

func (s *SiteService) GetGamesPageData(ctx context.Context, modifierAfter *string, modifiedBefore *string, broad bool, afterId *string) ([]*types.Game, []*types.AdditionalApp, []*types.GameData, [][]string, [][]string, error) {
	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, nil, nil, nil, dberr(err)
	}
	defer dbs.Rollback()

	games, addApps, gameData, tagRelations, platformRelations, err := s.pgdal.SearchGames(dbs, modifierAfter, modifiedBefore, broad, afterId)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, nil, nil, nil, dberr(err)
	}

	return games, addApps, gameData, tagRelations, platformRelations, nil
}

func (s *SiteService) AddSubmissionToFlashpoint(ctx context.Context, submission *types.ExtendedSubmission, subDirFullPath string, dataPacksDir string, imagesDir string, r *http.Request) (*string, error) {
	// Lock the database for sequential write
	utils.MetadataMutex.Lock()
	defer utils.MetadataMutex.Unlock()

	dbs, err := s.pgdal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	sfs, err := s.GetSubmissionFiles(ctx, []int64{submission.FileID})
	if err != nil {
		return nil, err
	}
	sf := sfs[0]

	// Repack the curation via the validator and get fresh metadata
	originalPath := fmt.Sprintf("%s/%s", subDirFullPath, sf.CurrentFilename)
	vr, err := s.validator.ProvideArchiveForRepacking(originalPath)
	if err != nil {
		return nil, err
	}
	if vr.Error != nil {
		return nil, types.RepackError(*vr.Error)
	}
	defer RemoveRepackFolder(ctx, *vr.FilePath)

	// If UUID is given, check if game exists alreay
	var game *types.Game
	if vr.Meta.UUID != nil {
		game, _ = s.pgdal.GetGame(dbs, *vr.Meta.UUID)
		if game != nil {
			// If body exists, apply patch in metadata
			if r.ContentLength > 0 {
				var patch *types.GameContentPatch
				err = json.NewDecoder(r.Body).Decode(&patch)
				if err != nil {
					return nil, err
				}

				err := s.pgdal.ApplyGamePatch(dbs, utils.UserID(ctx), game, patch, vr.Meta.AdditionalApps)
				if err != nil {
					return nil, err
				}
			} else if vr.Meta.AdditionalApps != nil {
				// No body, just patch in add apps
				err := s.pgdal.ApplyGamePatch(dbs, utils.UserID(ctx), game, nil, vr.Meta.AdditionalApps)
				if err != nil {
					return nil, err
				}
			}

			// Game exists, add new game data instead
			data, err := s.pgdal.AddGameData(dbs, utils.UserID(ctx), game.ID, vr)
			if err != nil {
				return nil, err
			}
			game.Data = append(game.Data, data)
		}
	}

	if game == nil {
		game, err = s.pgdal.AddSubmissionFromValidator(dbs, utils.UserID(ctx), vr)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return nil, err
		}
	}

	msTime := game.Data[0].DateAdded.UnixMilli()

	utils.LogCtx(ctx).Debug("Adding sub from validator")
	// Add game into metadata

	// Get the base name of the data pack file
	base := filepath.Base(*vr.FilePath)

	// Add date added into filename
	newBase := fmt.Sprintf("%s-%d%s", game.ID, msTime, filepath.Ext(base))
	newFileName := filepath.Join(dataPacksDir, newBase)
	err = os.MkdirAll(dataPacksDir, 0777)
	if err != nil {
		return nil, err
	}

	// Copy renamed data pack file to data packs folder
	srcFile, err := os.Open(*vr.FilePath)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	destFile, err := os.Create(newFileName) // creates if file doesn't exist
	if err != nil {
		return nil, err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile) // check first var for number of bytes copied
	if err != nil {
		return nil, err
	}

	err = destFile.Sync()
	if err != nil {
		return nil, err
	}

	// Copy image data to new images
	if len(vr.Images) < 2 {
		return nil, types.NotEnoughImages(strconv.Itoa(len(vr.Images)))
	}

	logo := vr.Images[0]
	logoFilePath := fmt.Sprintf(`%s/Logos/%s/%s/%s.png`, imagesDir, game.ID[0:2], game.ID[2:4], game.ID)
	decodedLogo, err := base64.StdEncoding.DecodeString(logo.Data)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(logoFilePath), 0777)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(logoFilePath, decodedLogo, 0644)
	if err != nil {
		log.Fatal(err)
	}

	ss := vr.Images[1]
	ssFilePath := fmt.Sprintf(`%s/Screenshots/%s/%s/%s.png`, imagesDir, game.ID[0:2], game.ID[2:4], game.ID)
	decodedScreenshot, err := base64.StdEncoding.DecodeString(ss.Data)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(filepath.Dir(ssFilePath), 0777)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(ssFilePath, decodedScreenshot, 0644)
	if err != nil {
		log.Fatal(err)
	}

	err = dbs.Commit()
	if err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
		return nil, err
	}

	return &game.ID, nil
}

func RemoveRepackFolder(ctx context.Context, filePath string) {
	dir := filepath.Dir(filePath)
	// Remove the directory
	err := os.RemoveAll(dir)

	if err != nil {
		utils.LogCtx(ctx).Error(err)
	}
}
