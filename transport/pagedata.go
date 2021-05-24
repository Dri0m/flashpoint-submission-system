package transport

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
)

type basePageData struct {
	Username                string
	AvatarURL               string
	IsAuthorizedToUseSystem bool
}

type submissionsPageData struct {
	basePageData
	Submissions []*types.ExtendedSubmission
}

type viewSubmissionPageData struct {
	submissionsPageData
	CurationMeta *types.CurationMeta
	Comments     []*types.ExtendedComment
}

type validatorResponse struct {
	Filename         string             `json:"filename"`
	Path             string             `json:"path"`
	CurationErrors   []string           `json:"curation_errors"`
	CurationWarnings []string           `json:"curation_warnings"`
	IsExtreme        bool               `json:"is_extreme"`
	CurationType     int                `json:"curation_type"`
	Meta             types.CurationMeta `json:"meta"`
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (a *App) GetBasePageData(ctx context.Context) (*basePageData, error) {
	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		return &basePageData{}, nil
	}

	discordUser, err := a.DB.GetDiscordUser(ctx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from database")
	}

	isAuthorized, err := a.DB.IsDiscordUserAuthorized(ctx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user authorization")
	}

	bpd := &basePageData{
		Username:                discordUser.Username,
		AvatarURL:               utils.FormatAvatarURL(discordUser.ID, discordUser.Avatar),
		IsAuthorizedToUseSystem: isAuthorized,
	}

	return bpd, nil
}
