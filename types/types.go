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
	SubmissionID                int64
	SubmissionLevel             string
	SubmitterID                 int64     // oldest file
	SubmitterUsername           string    // oldest file
	SubmitterAvatarURL          string    // oldest file
	UpdaterID                   int64     // newest file
	UpdaterUsername             string    // newest file
	UpdaterAvatarURL            string    // newest file
	FileID                      int64     // newest file
	OriginalFilename            string    // newest file
	CurrentFilename             string    // newest file
	Size                        int64     // newest file
	UploadedAt                  time.Time // oldest file
	UpdatedAt                   time.Time // newest file
	CurationTitle               *string   // newest file
	CurationAlternateTitles     *string   // newest file
	CurationPlatform            *string   // newest file
	CurationLaunchCommand       *string   // newest file
	CurationLibrary             *string   // newest file
	BotAction                   string
	FileCount                   uint64
	AssignedTestingUserIDs      []int64
	AssignedVerificationUserIDs []int64
	RequestedChangesUserIDs     []int64
	ApprovedUserIDs             []int64
	VerifiedUserIDs             []int64
	DistinctActions             []string
}

type SubmissionsFilter struct {
	SubmissionIDs                []int64  `schema:"submission-id"`
	SubmitterID                  *int64   `schema:"submitter-id"`
	TitlePartial                 *string  `schema:"title-partial"`
	SubmitterUsernamePartial     *string  `schema:"submitter-username-partial"`
	PlatformPartial              *string  `schema:"platform-partial"`
	LibraryPartial               *string  `schema:"library-partial"`
	OriginalFilenamePartialAny   *string  `schema:"original-filename-partial-any"`
	CurrentFilenamePartialAny    *string  `schema:"current-filename-partial-any"`
	MD5SumPartialAny             *string  `schema:"md5sum-partial-any"`
	SHA256SumPartialAny          *string  `schema:"sha256sum-partial-any"`
	BotActions                   []string `schema:"bot-action"`
	ActionsAfterMyLastComment    []string `schema:"post-last-action"`
	ResultsPerPage               *int64   `schema:"results-per-page"`
	Page                         *int64   `schema:"page"`
	AssignedStatusTesting        *string  `schema:"assigned-status-testing"`
	AssignedStatusVerification   *string  `schema:"assigned-status-verification"`
	RequestedChangedStatus       *string  `schema:"requested-changes-status"`
	ApprovalsStatus              *string  `schema:"approvals-status"`
	SubmissionLevels             []string `schema:"sumbission-level"`
	AssignedStatusTestingMe      *string  `schema:"assigned-status-testing-me"`
	AssignedStatusVerificationMe *string  `schema:"assigned-status-verification-me"`
	RequestedChangedStatusMe     *string  `schema:"requested-changes-status-me"`
	ApprovalsStatusMe            *string  `schema:"approvals-status-me"`
	IsExtreme                    *string  `schema:"is-extreme"`
	DistinctActions              []string `schema:"distinct-action"`
	DistinctActionsNot           []string `schema:"distinct-action-not"`
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

	for _, sid := range sf.SubmissionIDs {
		if sid < 1 {
			{
				return fmt.Errorf("submission id must be >= 1")
			}
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

	if sf.AssignedStatusTesting != nil && *sf.AssignedStatusTesting != "unassigned" && *sf.AssignedStatusTesting != "assigned" {
		return fmt.Errorf("invalid assigned-status-testing")
	}
	if sf.AssignedStatusVerification != nil && *sf.AssignedStatusVerification != "unassigned" && *sf.AssignedStatusVerification != "assigned" {
		return fmt.Errorf("invalid assigned-status-verification")
	}
	if sf.AssignedStatusTesting != nil && *sf.AssignedStatusTesting != "unassigned" && *sf.AssignedStatusTesting != "assigned" {
		return fmt.Errorf("invalid assigned-status")
	}
	if sf.RequestedChangedStatus != nil && *sf.RequestedChangedStatus != "none" && *sf.RequestedChangedStatus != "ongoing" {
		return fmt.Errorf("invalid requested-changes-status")
	}
	if sf.ApprovalsStatus != nil && *sf.ApprovalsStatus != "none" && *sf.ApprovalsStatus != "one" && *sf.ApprovalsStatus != "more-than-one" {
		return fmt.Errorf("invalid approvals-status")
	}

	if sf.AssignedStatusTestingMe != nil && *sf.AssignedStatusTestingMe != "unassigned" && *sf.AssignedStatusTestingMe != "assigned" {
		return fmt.Errorf("invalid assigned-status-testing-me")
	}
	if sf.AssignedStatusVerificationMe != nil && *sf.AssignedStatusVerificationMe != "unassigned" && *sf.AssignedStatusVerificationMe != "assigned" {
		return fmt.Errorf("invalid assigned-status-verification-me")
	}
	if sf.RequestedChangedStatusMe != nil && *sf.RequestedChangedStatusMe != "none" && *sf.RequestedChangedStatusMe != "ongoing" {
		return fmt.Errorf("invalid requested-changes-status-me")
	}
	if sf.ApprovalsStatusMe != nil && *sf.ApprovalsStatusMe != "no" && *sf.ApprovalsStatusMe != "yes" {
		return fmt.Errorf("invalid approvals-status-me")
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
	Message      *string
	CreatedAt    time.Time
}

type UpdateNotificationSettings struct {
	NotificationActions []string `schema:"notification-action"`
}

type UpdateSubscriptionSettings struct {
	Subscribe bool `schema:"subscribe"`
}

type Notification struct {
	ID        int64
	Type      string
	Message   string
	CreatedAt time.Time
	SentAt    time.Time
}

type CurationImage struct {
	ID               int64
	SubmissionFileID int64
	Type             string
	Filename         string
}

type ValidatorResponseImage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type ValidatorResponse struct {
	Filename         string                   `json:"filename"`
	Path             string                   `json:"path"`
	CurationErrors   []string                 `json:"curation_errors"`
	CurationWarnings []string                 `json:"curation_warnings"`
	IsExtreme        bool                     `json:"is_extreme"`
	CurationType     int                      `json:"curation_type"`
	Meta             CurationMeta             `json:"meta"`
	Images           []ValidatorResponseImage `json:"images"`
}
