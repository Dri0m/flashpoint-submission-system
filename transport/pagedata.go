package transport

import "github.com/Dri0m/flashpoint-submission-system/types"

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
