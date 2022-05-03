package database

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"time"
)

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
}

type DBSession interface {
	Commit() error
	Rollback() error
	Tx() *sql.Tx
	Ctx() context.Context
}
