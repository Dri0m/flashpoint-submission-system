package types

import (
	"fmt"
	"reflect"
	"time"
)

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
	MD5Sum           string
	SHA256Sum        string
}

type ExtendedSubmissionFile struct {
	FileID             int64
	SubmissionID       int64
	SubmitterID        int64
	SubmitterUsername  string
	SubmitterAvatarURL string
	OriginalFilename   string
	CurrentFilename    string
	Size               int64
	UploadedAt         time.Time
	MD5Sum             string
	SHA256Sum          string
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
	CurationPlatform        *string   //newest file
	CurationLaunchCommand   *string   // newest file
	BotAction               string
	LatestAction            string
	FileCount               uint64
}

type SubmissionsFilter struct {
	SubmissionID              *int64   `schema:"submission-id"`
	SubmitterID               *int64   `schema:"submitter-id"`
	TitlePartial              *string  `schema:"title-partial"`
	SubmitterUsernamePartial  *string  `schema:"submitter-username-partial"`
	PlatformPartial           *string  `schema:"platform-partial"`
	BotActions                []string `schema:"bot-action"`
	ActionsAfterMyLastComment []string `schema:"post-last-action"`
	ResultsPerPage            *int64   `schema:"results-per-page"`
	Page                      *int64   `schema:"page"`
}

func (sf *SubmissionsFilter) Validate() error {

	v := reflect.ValueOf(sf).Elem() // fucking schema zeroing out my nil pointers
	t := reflect.TypeOf(sf).Elem()
	for i := 0; i < v.NumField(); i++ {
		if t.Field(i).Type.Kind() == reflect.Ptr {
			f := v.Field(i)
			e := f.Elem()
			if e.Kind() == reflect.Int64 && e.Int() == 0 {
				f.Set(reflect.Zero(f.Type()))
			}
			if e.Kind() == reflect.String && e.String() == "" {
				f.Set(reflect.Zero(f.Type()))
			}
		}
	}

	if sf.SubmissionID != nil && *sf.SubmissionID < 1 {
		if *sf.SubmissionID == 0 { // schema parser zeroes out pointer values ffs
			sf.SubmissionID = nil
		} else {
			return fmt.Errorf("submission id must be >= 1")
		}

	}
	if sf.SubmitterID != nil && *sf.SubmitterID < 1 {
		if *sf.SubmitterID == 0 {
			sf.SubmitterID = nil
		} else {
			return fmt.Errorf("submitter id must be >= 1")
		}
	}
	if sf.ResultsPerPage != nil && *sf.ResultsPerPage < 1 {
		if *sf.ResultsPerPage == 0 {
			sf.ResultsPerPage = nil
		} else {
			return fmt.Errorf("results per page must be >= 1")
		}
	}
	if sf.Page != nil && *sf.Page < 1 {
		if *sf.Page == 0 {
			sf.Page = nil
		} else {
			return fmt.Errorf("page must be >= 1")
		}
	}
	return nil
}

type ExtendedComment struct {
	CommentID    int64
	AuthorID     int64
	Username     string
	AvatarURL    string
	SubmissionID int64
	Action       string
	Message      []string
	CreatedAt    time.Time
}
