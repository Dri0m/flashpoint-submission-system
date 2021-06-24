package constants

const ValidatorID = 810112564787675166
const SubmissionsDir = "files/submissions"
const SubmissionImagesDir = "files/submissions-images"
const UserInAuditSumbissionMaxFilesize = 200000000

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
	}
}

func GetActionsWithMandatoryMessage() []string {
	return []string{
		ActionComment,
		ActionRequestChanges,
	}
}

func GetActionsWithNotification() []string {
	return []string{
		ActionComment,
		ActionApprove,
		ActionRequestChanges,
		ActionMarkAdded,
		ActionUpload,
	}
}

const (
	ResourceKeySubmissionID    = "submission-id"
	ResourceKeySubmissionIDs   = "submission-ids"
	ResourceKeyFileID          = "file-id"
	ResourceKeyFileIDs         = "file-ids"
	ResourceKeyCommentID       = "comment-id"
	ResourceKeyCurationImageID = "curation-image-id"
)

const (
	NotificationDefault      = "notification"
	NotificationCurationFeed = "curation-feed"
)
