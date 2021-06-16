package constants

const ValidatorID = 810112564787675166
const SubmissionsDir = "submissions"
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

const (
	ResourceKeySubmissionID  = "submission-id"
	ResourceKeySubmissionIDs = "submission-ids"
	ResourceKeyFileID        = "file-id"
	ResourceKeyFileIDs       = "file-ids"
	ResourceKeyCommentID     = "comment-id"
)
