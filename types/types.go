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
	AlternateTitles     *string `json:"Alternate Titles"`
	Library             *string `json:"Library"`
	Version             *string `json:"Version"`
	CurationNotes       *string `json:"Curation Notes"`
	MountParameters     *string `json:"Mount Parameters"`
	//AdditionalApplications *CurationFormatAddApps `json:"Additional Applications"`
}

type MasterDatabaseGame struct {
	UUID                string
	Title               *string
	AlternateTitles     *string
	Series              *string
	Developer           *string
	Publisher           *string
	Platform            *string
	Extreme             *string
	PlayMode            *string
	Status              *string
	GameNotes           *string
	Source              *string
	LaunchCommand       *string
	ReleaseDate         *string
	Version             *string
	OriginalDescription *string
	Languages           *string
	Library             *string
	Tags                *string
	DateAdded           time.Time
	DateModified        time.Time
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
	LastUploaderID              int64     // newest file
	CurationTitle               *string   // newest file
	CurationAlternateTitles     *string   // newest file
	CurationPlatform            *string   // newest file
	CurationLaunchCommand       *string   // newest file
	CurationLibrary             *string   // newest file
	CurationExtreme             *string   // newest file
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
	VerificationStatus           *string  `schema:"verification-status"`
	SubmissionLevels             []string `schema:"sumbission-level"`
	AssignedStatusTestingMe      *string  `schema:"assigned-status-testing-me"`
	AssignedStatusVerificationMe *string  `schema:"assigned-status-verification-me"`
	RequestedChangedStatusMe     *string  `schema:"requested-changes-status-me"`
	ApprovalsStatusMe            *string  `schema:"approvals-status-me"`
	VerificationStatusMe         *string  `schema:"verification-status-me"`
	IsExtreme                    *string  `schema:"is-extreme"`
	DistinctActions              []string `schema:"distinct-action"`
	DistinctActionsNot           []string `schema:"distinct-action-not"`
	LaunchCommandFuzzy           *string  `schema:"launch-command-fuzzy"`
	LastUploaderNotMe            *string  `schema:"last-uploader-not-me"`
	OrderBy                      *string  `schema:"order-by"`
	AscDesc                      *string  `schema:"asc-desc"`
	SubscribedMe                 *string  `schema:"subscribed-me"`
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
	if sf.ApprovalsStatus != nil && *sf.ApprovalsStatus != "none" && *sf.ApprovalsStatus != "approved" {
		return fmt.Errorf("invalid approvals-status")
	}
	if sf.VerificationStatus != nil && *sf.VerificationStatus != "none" && *sf.VerificationStatus != "verified" {
		return fmt.Errorf("invalid verificaton-status")
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
	if sf.VerificationStatusMe != nil && *sf.VerificationStatusMe != "no" && *sf.VerificationStatusMe != "yes" {
		return fmt.Errorf("invalid verificaton-status-me")
	}
	if sf.LastUploaderNotMe != nil && *sf.LastUploaderNotMe != "yes" {
		return fmt.Errorf("last-uploader-not-me")
	}
	if sf.OrderBy != nil && *sf.OrderBy != "uploaded" && *sf.OrderBy != "updated" {
		return fmt.Errorf("invalid order-by")
	}
	if sf.AscDesc != nil && *sf.AscDesc != "asc" && *sf.AscDesc != "desc" {
		return fmt.Errorf("invalid asc-desc")
	}
	if sf.SubscribedMe != nil && *sf.SubscribedMe != "no" && *sf.SubscribedMe != "yes" {
		return fmt.Errorf("invalid subscribed-me")
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

type ReceiveFileResp struct {
	Message string  `json:"message"`
	URL     *string `json:"url"`
}

type SimilarityAttributes struct {
	ID                 string
	Title              *string
	LaunchCommand      *string
	TitleRatio         float64
	LaunchCommandRatio float64
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ValidatorTagResponse struct {
	Tags []Tag `json:"tags"`
}

type ResumableParams struct {
	ResumableChunkNumber      int    `schema:"resumableChunkNumber"`
	ResumableChunkSize        uint64 `schema:"resumableChunkSize"`
	ResumableTotalSize        int64  `schema:"resumableTotalSize"`
	ResumableIdentifier       string `schema:"resumableIdentifier"`
	ResumableFilename         string `schema:"resumableFilename"`
	ResumableRelativePath     string `schema:"resumableRelativePath"`
	ResumableCurrentChunkSize int64  `schema:"resumableCurrentChunkSize"`
	ResumableTotalChunks      int    `schema:"resumableTotalChunks"`
}

type FlashfreezeFile struct {
	ID               int64
	UserID           int64
	OriginalFilename string
	CurrentFilename  string
	Size             int64
	UploadedAt       time.Time
	MD5Sum           string
	SHA256Sum        string
}

type IndexerResp struct {
	ArchiveFilename string              `json:"archive_filename"`
	Files           []*IndexedFileEntry `json:"files"`
	IndexingErrors  uint64              `json:"indexing_errors"`
}

type IndexedFileEntry struct {
	Name             string `json:"name"`
	SizeCompressed   int64  `json:"size_compressed"`
	SizeUncompressed int64  `json:"size_uncompressed"`
	FileUtilOutput   string `json:"file_util_output"`
	SHA256           string `json:"sha256"`
	MD5              string `json:"md5"`
}

type ExtendedFlashfreezeFile struct {
	FileID            int64
	SubmitterID       int64
	SubmitterUsername string
	OriginalFilename  string
	MD5Sum            string
	SHA256Sum         string
	Size              int64
	UploadedAt        *time.Time // only for root files
	Description       *string    // only for inner files
	IsRootFile        bool
	IsDeepFile        bool
	IndexingTime      *time.Duration // only for root files
	FileCount         *int64         // only for root files
	IndexingErrors    *int64         // only for root files
}

type FlashfreezeFilter struct {
	FileIDs     []int64 `schema:"file-id"`
	SubmitterID *int64  `schema:"submitter-id"`

	NameFulltext        *string `schema:"name-fulltext"`
	DescriptionFulltext *string `schema:"description-fulltext"` // only for inner files

	NamePrefix        *string `schema:"name-prefix"`
	DescriptionPrefix *string `schema:"description-prefix"` // only for inner files

	SizeMin *int64 `schema:"size-min"`
	SizeMax *int64 `schema:"size-max"`

	SubmitterUsernamePartial *string `schema:"submitter-username-partial"`
	MD5SumPartial            *string `schema:"md5sum-partial"`
	SHA256SumPartial         *string `schema:"sha256sum-partial"`

	SearchFiles            *bool `schema:"search-files"`
	SearchFilesRecursively *bool `schema:"search-files-recursively"`

	ResultsPerPage *int64 `schema:"results-per-page"`
	Page           *int64 `schema:"page"`
}

func (ff *FlashfreezeFilter) Validate() error {

	v := reflect.ValueOf(ff).Elem() // fucking schema zeroing out my nil pointers
	t := reflect.TypeOf(ff).Elem()
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

	if ff.SubmitterID != nil && *ff.SubmitterID < 1 {
		if *ff.SubmitterID == 0 {
			ff.SubmitterID = nil
		} else {
			return fmt.Errorf("submitter id must be >= 1")
		}
	}
	if ff.SizeMin != nil && *ff.SizeMin < 1 {
		if *ff.SizeMin == 0 {
			ff.SizeMin = nil
		} else {
			return fmt.Errorf("size-uncompressed-min must be >= 1")
		}
	}
	if ff.SizeMax != nil && *ff.SizeMax < 1 {
		if *ff.SizeMax == 0 {
			ff.SizeMax = nil
		} else {
			return fmt.Errorf("size-uncompressed-max must be >= 1")
		}

		if ff.SizeMin != nil && *ff.SizeMin > *ff.SizeMax {
			return fmt.Errorf("size-uncompressed-min cannot be greater than size-uncompressed-max")
		}
	}

	if ff.ResultsPerPage != nil && *ff.ResultsPerPage < 1 {
		if *ff.ResultsPerPage == 0 {
			ff.ResultsPerPage = nil
		} else {
			return fmt.Errorf("results per page must be >= 1")
		}
	}
	if ff.Page != nil && *ff.Page < 1 {
		if *ff.Page == 0 {
			ff.Page = nil
		} else {
			return fmt.Errorf("page must be >= 1")
		}
	}

	return nil
}
