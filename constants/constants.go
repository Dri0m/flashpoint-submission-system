package constants

const ValidatorID = 810112564787675166
const SubmissionsDir = "files/submissions"
const SubmissionImagesDir = "files/submissions-images"
const UserInAuditSumbissionMaxFilesize = 200000000

const (
	ActionComment        = "comment"
	ActionApprove        = "approve"
	ActionRequestChanges = "request-changes"
	ActionAccept         = "accept"
	ActionMarkAdded      = "mark-added"
	ActionReject         = "reject"
	ActionUpload         = "upload-file"
	ActionAssign         = "assign"
	ActionUnassign       = "unassign"
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
		ActionAccept,
		ActionMarkAdded,
		ActionReject,
		ActionUpload,
		ActionAssign,
		ActionUnassign,
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
