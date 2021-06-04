package service

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"mime/multipart"
)

type Service interface {
	GetBasePageData(ctx context.Context) (*types.BasePageData, error)
	ReceiveSubmissions(ctx context.Context, sid *int64, fileHeaders []*multipart.FileHeader) error
	ReceiveComments(ctx context.Context, uid int64, sids []int64, formAction, formMessage string) error
	GetViewSubmissionPageData(ctx context.Context, sid int64) (*types.ViewSubmissionPageData, error)
	GetSubmissionsFilesPageData(ctx context.Context, sid int64) (*types.SubmissionsFilesPageData, error)
	GetSubmissionsPageData(ctx context.Context, filter *types.SubmissionsFilter) (*types.SubmissionsPageData, error)
	SearchSubmissions(ctx context.Context, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error)
	GetSubmissionFiles(ctx context.Context, sfids []int64) ([]*types.SubmissionFile, error)
	GetUIDFromSession(ctx context.Context, key string) (int64, bool, error)
	SoftDeleteSubmissionFile(ctx context.Context, sfid int64) error
	SaveUser(ctx context.Context, discordUser *types.DiscordUser) (*utils.AuthToken, error)
	Logout(ctx context.Context, secret string) error
	GetUserRoles(ctx context.Context, uid int64) ([]string, error)
}
