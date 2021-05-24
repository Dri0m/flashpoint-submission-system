package types

import "time"

type CurationMeta struct {
	SubmissionID        int64
	SubmissionFileID    int64
	ApplicationPath     *string `json:"Application Path"`
	Developer           *string `json:"Developer"`
	Extreme             *string `json:"Extreme"`
	GameNotes           *string `json:"Game Notes"`
	Languages           *string `json:"Languages"`
	LaunchCommand       *string `json:"Launch Command"`
	OriginalDescription *string `json:"Original Description"`
	PlayMode            *string `json:"Play Mode"`
	Platform            *string `json:"Platform"`
	Publisher           *string `json:"Publisher"`
	ReleaseDate         *string `json:"Release Date"`
	Series              *string `json:"Series"`
	Source              *string `json:"Source"`
	Status              *string `json:"Status"`
	Tags                *string `json:"Tags"`
	TagCategories       *string `json:"Tag Categories"`
	Title               *string `json:"Title"`
	AlternateTitles     *string `json:"Alternate Title"`
	Library             *string `json:"Library"`
	Version             *string `json:"Version"`
	CurationNotes       *string `json:"Curation Notes"`
	MountParameters     *string `json:"Mount Parameters"`
	//AdditionalApplications *CurationFormatAddApps `json:"Additional Applications"`
}

type Comment struct {
	AuthorID     int64
	SubmissionID int64
	Action       string
	Message      *string
	CreatedAt    time.Time
}

type SubmissionFile struct {
	SubmitterID      int64
	SubmissionID     int64
	OriginalFilename string
	CurrentFilename  string
	Size             int64
	UploadedAt       time.Time
}

type ExtendedSubmission struct {
	SubmissionID            int64
	SubmitterID             int64     // oldest file
	SubmitterUsername       string    // oldest file
	SubmitterAvatarURL      string    // oldest file
	UpdaterID               int64     // newest file
	UpdaterUsername         string    // newest file
	UpdaterAvatarURL        string    // newest file
	FileID                  int64     // newest file
	OriginalFilename        string    // newest file
	CurrentFilename         string    // newest file
	Size                    int64     // newest file
	UploadedAt              time.Time // oldest file
	UpdatedAt               time.Time // newest file
	CurationTitle           *string   // newest file
	CurationAlternateTitles *string   //newest file
	CurationLaunchCommand   *string   // newest file
	BotAction               string
	LatestAction            string
}

type SubmissionsFilter struct {
	SubmissionID *int64
	SubmitterID  *int64
}

type ExtendedComment struct {
	AuthorID     int64
	Username     string
	AvatarURL    string
	SubmissionID int64
	Action       string
	Message      []string
	CreatedAt    time.Time
}
