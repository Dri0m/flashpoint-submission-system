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

type TagsPageData struct {
	BasePageData
	Tags       []*Tag
	Categories []*TagCategory
	TotalCount int64
}

type TagsPageDataJSON struct {
	Tags       []*Tag         `json:"tags"`
	Categories []*TagCategory `json:"categories"`
}

type PlatformsPageData struct {
	BasePageData
	Platforms  []*Platform
	TotalCount int64
}

type TagPageData struct {
	BasePageData
	Tag        Tag
	Categories []*TagCategory
	GamesUsing int64
}

type GamePageData struct {
	BasePageData
	Game          Game
	LogoUrl       string
	ScreenshotUrl string
	ImagesCdn     string
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
	FlashfreezeFiles []*ExtendedFlashfreezeItem
	TotalCount       int64
	Filter           FlashfreezeFilter
}

type StatisticsPageData struct {
	BasePageData
	SubmissionCount             int64
	SubmissionCountBotHappy     int64
	SubmissionCountBotSad       int64
	SubmissionCountApproved     int64
	SubmissionCountVerified     int64
	SubmissionCountRejected     int64
	SubmissionCountInFlashpoint int64
	UserCount                   int64
	CommentCount                int64
	FlashfreezeCount            int64
	FlashfreezeFileCount        int64
	TotalSubmissionSize         int64
	TotalFlashfreezeSize        int64
}

type SubmitFixesFilesPageData struct {
	BasePageData
	FixID int64
}

type SearchFixesPageData struct {
	BasePageData
	Fixes      []*ExtendedFixesItem
	TotalCount int64
	Filter     FixesFilter
}

type ViewFixPageData struct {
	SearchFixesPageData
	FixesFiles []*ExtendedFixesFile
}

type UserStatisticsPageData struct {
	BasePageData
	Users []*UserStatistics
}

type DeviceAuthStates struct {
	Pending  int64
	Complete int64
	Expired  int64
	Denied   int64
}

type DeviceAuthPageData struct {
	BasePageData
	Token  *DeviceFlowToken
	States DeviceAuthStates
}
