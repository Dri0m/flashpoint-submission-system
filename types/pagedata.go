package types

type BasePageData struct {
	Username                string
	AvatarURL               string
	IsAuthorizedToUseSystem bool
}

type SubmissionsPageData struct {
	BasePageData
	Submissions []*ExtendedSubmission
}

type ViewSubmissionPageData struct {
	SubmissionsPageData
	CurationMeta *CurationMeta
	Comments     []*ExtendedComment
}

type ValidatorResponse struct {
	Filename         string       `json:"filename"`
	Path             string       `json:"path"`
	CurationErrors   []string     `json:"curation_errors"`
	CurationWarnings []string     `json:"curation_warnings"`
	IsExtreme        bool         `json:"is_extreme"`
	CurationType     int          `json:"curation_type"`
	Meta             CurationMeta `json:"meta"`
}
