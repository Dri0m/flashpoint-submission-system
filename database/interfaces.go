package database

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/types"
)

type DAL interface {
	BeginTx() (*sql.Tx, error)
	StoreSession(ctx context.Context, tx *sql.Tx, key string, uid int64, durationSeconds int64) error
	DeleteSession(ctx context.Context, tx *sql.Tx, secret string) error
	GetUIDFromSession(ctx context.Context, tx *sql.Tx, key string) (int64, bool, error)
	StoreDiscordUser(ctx context.Context, tx *sql.Tx, discordUser *types.DiscordUser) error
	GetDiscordUser(ctx context.Context, tx *sql.Tx, uid int64) (*types.DiscordUser, error)
	StoreDiscordServerRoles(ctx context.Context, tx *sql.Tx, roles []types.DiscordRole) error
	StoreDiscordUserRoles(ctx context.Context, tx *sql.Tx, uid int64, roles []int64) error
	GetDiscordUserRoles(ctx context.Context, tx *sql.Tx, uid int64) ([]string, error)
	StoreSubmission(ctx context.Context, tx *sql.Tx) (int64, error)
	StoreSubmissionFile(ctx context.Context, tx *sql.Tx, s *types.SubmissionFile) (int64, error)
	GetSubmissionFiles(ctx context.Context, tx *sql.Tx, sfids []int64) ([]*types.SubmissionFile, error)
	GetExtendedSubmissionFilesBySubmissionID(ctx context.Context, tx *sql.Tx, sid int64) ([]*types.ExtendedSubmissionFile, error)
	SearchSubmissions(ctx context.Context, tx *sql.Tx, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error)
	StoreCurationMeta(ctx context.Context, tx *sql.Tx, cm *types.CurationMeta) error
	GetCurationMetaBySubmissionFileID(ctx context.Context, tx *sql.Tx, sfid int64) (*types.CurationMeta, error)
	StoreComment(ctx context.Context, tx *sql.Tx, c *types.Comment) error
	GetExtendedCommentsBySubmissionID(ctx context.Context, tx *sql.Tx, sid int64) ([]*types.ExtendedComment, error)
	SoftDeleteSubmissionFile(ctx context.Context, tx *sql.Tx, sfid int64) error
}
