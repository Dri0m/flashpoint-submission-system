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
	StoreSubmission(dbs DBSession) (int64, error)
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
}

type DBSession interface {
	Commit() error
	Rollback() error
	Tx() *sql.Tx
	Ctx() context.Context
}
