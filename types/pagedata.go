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
	Submissions  []*ExtendedSubmission
	Filter       SubmissionsFilter
	FilterLayout string
}

type ViewSubmissionPageData struct {
	SubmissionsPageData
	CurationMeta         *CurationMeta
	Comments             []*ExtendedComment
	IsUserSubscribed     bool
	CurationImageIDs     []int64
	NextSubmissionID     *int64
	PreviousSubmissionID *int64
}

type SubmissionsFilesPageData struct {
	BasePageData
	SubmissionFiles []*ExtendedSubmissionFile
}
