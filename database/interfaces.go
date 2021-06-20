package database

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/types"
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
	SearchSubmissions(dbs DBSession, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error)
	StoreCurationMeta(dbs DBSession, cm *types.CurationMeta) error
	GetCurationMetaBySubmissionFileID(dbs DBSession, sfid int64) (*types.CurationMeta, error)
	StoreComment(dbs DBSession, c *types.Comment) error
	GetExtendedCommentsBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedComment, error)
	SoftDeleteSubmissionFile(dbs DBSession, sfid int64) error
	SoftDeleteSubmission(dbs DBSession, sid int64) error
	SoftDeleteComment(dbs DBSession, cid int64) error
	StoreNotificationSettings(dbs DBSession, uid int64, actions []string) error
	GetNotificationSettingsByUserID(dbs DBSession, uid int64) ([]string, error)
	SubscribeUserToSubmission(dbs DBSession, uid, sid int64) error
	UnsubscribeUserFromSubmission(dbs DBSession, uid, sid int64) error
	IsUserSubscribedToSubmission(dbs DBSession, uid, sid int64) (bool, error)
	StoreNotification(dbs DBSession, msg string) error
	GetUsersForNotification(dbs DBSession, authorID, sid int64, action string) ([]int64, error)
	GetOldestUnsentNotification(dbs DBSession) (*types.Notification, error)
	MarkNotificationAsSent(dbs DBSession, nid int64) error
	StoreCurationImage(dbs DBSession, c *types.CurationImage) (int64, error)
	GetCurationImagesBySubmissionFileID(dbs DBSession, sfid int64) ([]*types.CurationImage, error)
	GetCurationImage(dbs DBSession, ciid int64) (*types.CurationImage, error)
}

type DBSession interface {
	Commit() error
	Rollback() error
	Tx() *sql.Tx
	Ctx() context.Context
}
