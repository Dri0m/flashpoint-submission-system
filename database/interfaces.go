package database

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/jackc/pgx/v5"
	"time"
)

type PGDAL interface {
	NewSession(ctx context.Context) (PGDBSession, error)

	CountSinceDate(dbs PGDBSession, modifiedAfter *string) (int, error)

	SearchTags(dbs PGDBSession, modifiedAfter *string) ([]*types.Tag, error)
	SearchPlatforms(dbs PGDBSession, modifiedAfter *string) ([]*types.Platform, error)
	SearchGames(dbs PGDBSession, modifiedAfter *string, modifiedBefore *string, broad bool, afterId *string) ([]*types.Game, []*types.AdditionalApp, []*types.GameData, [][]string, [][]string, error)
	SearchDeletedGames(dbs PGDBSession, modifiedAfter *string) ([]*types.DeletedGame, error)

	GetTagCategories(dbs PGDBSession) ([]*types.TagCategory, error)
	GetGamesUsingTagTotal(dbs PGDBSession, tagId int64) (int64, error)
	SaveGame(dbs PGDBSession, game *types.Game, uid int64) error
	SaveTag(dbs PGDBSession, tag *types.Tag, uid int64) error
	DeveloperImportDatabaseJson(dbs PGDBSession, data *types.LauncherDump) error

	GetTagCategory(dbs PGDBSession, categoryId int64) (*types.TagCategory, error)
	GetTag(dbs PGDBSession, tagId int64) (*types.Tag, error)
	GetTagByName(dbs PGDBSession, tagName string) (*types.Tag, error)
	GetPlatform(dbs PGDBSession, platformId int64) (*types.Platform, error)
	GetPlatformByName(dbs PGDBSession, platformName string) (*types.Platform, error)
	GetGame(dbs PGDBSession, gameId string) (*types.Game, error)
	GetGameDataIndex(dbs PGDBSession, gameId string, date int64) (*types.GameDataIndex, error)
	GetGameRevisionInfo(dbs PGDBSession, gameId string) ([]*types.RevisionInfo, error)
	GetTagRevisionInfo(dbs PGDBSession, tagId int64) ([]*types.RevisionInfo, error)

	GetMetadataStats(dbs PGDBSession) (*types.MetadataStatsPageDataBare, error)

	DeleteGame(dbs PGDBSession, gameId string, uid int64, reason string, imagesPath string, gamesPath string, deletedImagesPath string, deletedGamesPath string) error

	RestoreGame(dbs PGDBSession, gameId string, uid int64, reason string, imagesPath string, gamesPath string, deletedImagesPath string, deletedGamesPath string) error

	GetOrCreateTagCategory(dbs PGDBSession, categoryName string) (*types.TagCategory, error)
	GetOrCreateTag(dbs PGDBSession, tagName string, tagCategory string, reason string, uid int64) (*types.Tag, error)
	GetOrCreatePlatform(dbs PGDBSession, platformName string, reason string, uid int64) (*types.Platform, error)

	AddSubmissionFromValidator(dbs PGDBSession, uid int64, vr *types.ValidatorRepackResponse) (*types.Game, error)
	AddGameData(dbs PGDBSession, uid int64, gameId string, vr *types.ValidatorRepackResponse) (*types.GameData, error)

	IndexerGetNext(ctx context.Context) (*types.GameData, error)
	IndexerInsert(ctx context.Context, crc32sum []byte, md5sum []byte, sha256sum []byte, sha1sum []byte,
		size uint64, path string, gameId string, zipDate time.Time) error
	IndexerMarkFailure(ctx context.Context, gameId string, zipDate time.Time) error

	GetIndexMatchesHash(dbs PGDBSession, hashType string, hashStr string) ([]*types.IndexMatchData, error)

	UpdateTagsFromTagsList(dbs PGDBSession, tagsList []types.Tag) error
	ApplyGamePatch(dbs PGDBSession, uid int64, game *types.Game, patch *types.GameContentPatch, addApps []*types.CurationAdditionalApp) error
}

type DAL interface {
	NewSession(ctx context.Context) (DBSession, error)
	StoreSession(dbs DBSession, key string, uid int64, durationSeconds int64) error
	DeleteSession(dbs DBSession, secret string) error
	GetUIDFromSession(dbs DBSession, key string) (int64, bool, error)

	StoreDiscordUser(dbs DBSession, discordUser *types.DiscordUser) error
	GetDiscordUser(dbs DBSession, uid int64) (*types.DiscordUser, error)
	StoreDiscordServerRoles(dbs DBSession, roles []types.DiscordRole) error
	StoreDiscordUserRoles(dbs DBSession, uid int64, roles []int64) error
	GetDiscordUserRoles(dbs DBSession, uid int64) ([]string, error)

	StoreSubmission(dbs DBSession, submissionLevel string) (int64, error)
	StoreSubmissionFile(dbs DBSession, s *types.SubmissionFile) (int64, error)
	GetSubmissionFiles(dbs DBSession, sfids []int64) ([]*types.SubmissionFile, error)
	GetExtendedSubmissionFilesBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedSubmissionFile, error)

	SearchSubmissions(dbs DBSession, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, int64, error)

	StoreCurationMeta(dbs DBSession, cm *types.CurationMeta) error
	GetCurationMetaBySubmissionFileID(dbs DBSession, sfid int64) (*types.CurationMeta, error)

	StoreComment(dbs DBSession, c *types.Comment) error
	GetExtendedCommentsBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedComment, error)
	GetCommentByID(dbs DBSession, cid int64) (*types.Comment, error)

	SoftDeleteSubmissionFile(dbs DBSession, sfid int64, deleteReason string) error
	SoftDeleteSubmission(dbs DBSession, sid int64, deleteReason string) error
	SoftDeleteComment(dbs DBSession, cid int64, deleteReason string) error

	StoreNotificationSettings(dbs DBSession, uid int64, actions []string) error
	GetNotificationSettingsByUserID(dbs DBSession, uid int64) ([]string, error)

	SubscribeUserToSubmission(dbs DBSession, uid, sid int64) error
	UnsubscribeUserFromSubmission(dbs DBSession, uid, sid int64) error
	IsUserSubscribedToSubmission(dbs DBSession, uid, sid int64) (bool, error)

	StoreNotification(dbs DBSession, msg, notificationType string) error
	GetUsersForNotification(dbs DBSession, authorID, sid int64, action string) ([]int64, error)
	GetUsersForUniversalNotification(dbs DBSession, authorID int64, action string) ([]int64, error)
	GetOldestUnsentNotification(dbs DBSession) (*types.Notification, error)
	MarkNotificationAsSent(dbs DBSession, nid int64) error

	StoreCurationImage(dbs DBSession, c *types.CurationImage) (int64, error)
	GetCurationImagesBySubmissionFileID(dbs DBSession, sfid int64) ([]*types.CurationImage, error)
	GetCurationImage(dbs DBSession, ciid int64) (*types.CurationImage, error)

	GetNextSubmission(dbs DBSession, sid int64) (int64, error)
	GetPreviousSubmission(dbs DBSession, sid int64) (int64, error)

	UpdateSubmissionCacheTable(dbs DBSession, sid int64) error

	ClearMasterDBGames(dbs DBSession) error
	StoreMasterDBGames(dbs DBSession, games []*types.MasterDatabaseGame) error

	GetAllSimilarityAttributes(dbs DBSession) ([]*types.SimilarityAttributes, error)

	StoreFlashfreezeRootFile(dbs DBSession, s *types.FlashfreezeFile) (int64, error)
	StoreFlashfreezeDeepFile(dbs DBSession, fid int64, entries []*types.IndexedFileEntry) error
	SearchFlashfreezeFiles(dbs DBSession, filter *types.FlashfreezeFilter) ([]*types.ExtendedFlashfreezeItem, int64, error)
	UpdateFlashfreezeRootFileIndexedState(dbs DBSession, fid int64, indexedAt *time.Time, indexingErrors uint64) error
	GetFlashfreezeRootFile(dbs DBSession, fid int64) (*types.FlashfreezeFile, error)
	GetAllFlashfreezeRootFiles(dbs DBSession) ([]*types.FlashfreezeFile, error)
	GetAllUnindexedFlashfreezeRootFiles(dbs DBSession) ([]*types.FlashfreezeFile, error)

	StoreFixFirstStep(dbs DBSession, uid int64, c *types.CreateFixFirstStep) (int64, error)
	GetFixByID(dbs DBSession, fid int64) (*types.Fix, error)
	StoreFixesFile(dbs DBSession, s *types.FixesFile) (int64, error)
	SearchFixes(dbs DBSession, filter *types.FixesFilter) ([]*types.ExtendedFixesItem, int64, error)
	GetFilesForFix(dbs DBSession, fid int64) ([]*types.ExtendedFixesFile, error)
	GetFixesFiles(dbs DBSession, ffids []int64) ([]*types.FixesFile, error)

	DeleteUserSessions(dbs DBSession, uid int64) (int64, error)

	GetTotalCommentsCount(dbs DBSession) (int64, error)
	GetTotalUserCount(dbs DBSession) (int64, error)
	GetTotalFlashfreezeCount(dbs DBSession) (int64, error)
	GetTotalFlashfreezeFileCount(dbs DBSession) (int64, error)
	GetTotalSubmissionFilesize(dbs DBSession) (int64, error)
	GetTotalFlashfreezeFilesize(dbs DBSession) (int64, error)

	GetUsers(dbs DBSession) ([]*types.User, error)
	GetCommentsByUserIDAndAction(dbs DBSession, uid int64, action string) ([]*types.Comment, error)

	PopulateRevisionInfo(dbs DBSession, revisions []*types.RevisionInfo) error
}

type DBSession interface {
	Commit() error
	Rollback() error
	Tx() *sql.Tx
	Ctx() context.Context
}

type PGDBSession interface {
	Commit() error
	Rollback() error
	Tx() pgx.Tx
	Ctx() context.Context
}
