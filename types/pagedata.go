package types

type BasePageData struct {
	Username      string
	UserID        int64
	AvatarURL     string
	UserRoles     []string
	IsDevInstance bool
}

type ProfilePageData struct {
	BasePageData
	NotificationActions []string
}

type SubmissionsPageData struct {
	BasePageData
	Submissions  []*ExtendedSubmission
	TotalCount   int64
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
	TagList              []Tag
}

type SubmissionsFilesPageData struct {
	BasePageData
	SubmissionFiles []*ExtendedSubmissionFile
}

type SearchFlashfreezePageData struct {
	BasePageData
	FlashfreezeFiles []*ExtendedFlashfreezeFile
	Filter           FlashfreezeFilter
}
