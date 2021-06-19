package types

type BasePageData struct {
	Username  string
	UserID    int64
	AvatarURL string
	UserRoles []string
}

type ProfilePageData struct {
	BasePageData
	NotificationActions []string
}

type SubmissionsPageData struct {
	BasePageData
	Submissions []*ExtendedSubmission
	Filter      SubmissionsFilter
}

type ViewSubmissionPageData struct {
	SubmissionsPageData
	CurationMeta     *CurationMeta
	Comments         []*ExtendedComment
	IsUserSubscribed bool
}

type SubmissionsFilesPageData struct {
	BasePageData
	SubmissionFiles []*ExtendedSubmissionFile
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
