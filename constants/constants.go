package constants

const ValidatorID = 810112564787675166
const SystemID = 844246603102945333
const SubmissionsDir = "files/submissions"
const SubmissionImagesDir = "files/submissions-images"
const UserInAuditSubmissionMaxFilesize = 500000000

const (
	ActionComment              = "comment"
	ActionApprove              = "approve"
	ActionRequestChanges       = "request-changes"
	ActionMarkAdded            = "mark-added"
	ActionUpload               = "upload-file"
	ActionVerify               = "verify"
	ActionAssignTesting        = "assign-testing"
	ActionUnassignTesting      = "unassign-testing"
	ActionAssignVerification   = "assign-verification"
	ActionUnassignVerification = "unassign-verification"
	ActionSystem               = "system"
	ActionReject               = "reject"
)

const (
	SubmissionLevelAudition = "audition"
	SubmissionLevelTrial    = "trial"
	SubmissionLevelStaff    = "staff"
)

func GetAllowedActions() []string {
	return []string{
		ActionComment,
		ActionApprove,
		ActionRequestChanges,
		ActionMarkAdded,
		ActionUpload,
		ActionVerify,
		ActionAssignTesting,
		ActionUnassignTesting,
		ActionAssignVerification,
		ActionUnassignVerification,
		ActionReject,
	}
}

func GetActionsWithMandatoryMessage() []string {
	return []string{
		ActionComment,
		ActionRequestChanges,
		ActionReject,
	}
}

func GetActionsWithNotification() []string {
	return []string{
		ActionComment,
		ActionApprove,
		ActionRequestChanges,
		ActionMarkAdded,
		ActionUpload,
		ActionReject,
	}
}

const (
	ResourceKeySubmissionID          = "submission-id"
	ResourceKeySubmissionIDs         = "submission-ids"
	ResourceKeyFileID                = "file-id"
	ResourceKeyFileIDs               = "file-ids"
	ResourceKeyCommentID             = "comment-id"
	ResourceKeyCurationImageID       = "curation-image-id"
	ResourceKeyFlashfreezeRootFileID = "flashfreeze-root-file-id"
)

const (
	NotificationDefault      = "notification"
	NotificationCurationFeed = "curation-feed"
)

const (
	RequestWeb  = "web"
	RequestJSON = "json"
	RequestData = "data"
)

type PublicResponse struct {
	Msg    *string `json:"message"`
	Status int     `json:"status"`
}
