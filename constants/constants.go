package constants

const ValidatorID = 810112564787675166
const SubmissionsDir = "submissions"

const (
	ActionComment        = "comment"
	ActionApprove        = "approve"
	ActionRequestChanges = "request-changes"
	ActionAccept         = "accept"
	ActionMarkAdded      = "mark-added"
	ActionReject         = "reject"
	ActionUpload         = "upload-file"
)

func GetActions() []string {
	return []string{
		ActionComment,
		ActionApprove,
		ActionRequestChanges,
		ActionAccept,
		ActionMarkAdded,
		ActionReject,
		ActionUpload,
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
