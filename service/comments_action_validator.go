package service

import (
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"net/http"
)

func uidIn(uid int64, ids []int64) bool {
	for _, assignedUserID := range ids {
		if uid == assignedUserID {
			return true
		}
	}
	return false
}

func isActionValidForSubmission(uid int64, formAction string, submission *types.ExtendedSubmission) error {
	markedAdded := false
	sid := submission.SubmissionID

	for _, distinctAction := range submission.DistinctActions {
		if constants.ActionMarkAdded == distinctAction {
			markedAdded = true
			break
		}
	}

	// don't let last uploader decide on the submission
	if formAction == constants.ActionAssignTesting || formAction == constants.ActionUnassignVerification {
		if uid == submission.LastUploaderID {
			return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot assign it", sid), http.StatusBadRequest)
		}
	}
	if formAction == constants.ActionApprove {
		if uid == submission.LastUploaderID {
			return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot approve it", sid), http.StatusBadRequest)
		}
	}
	if formAction == constants.ActionRequestChanges {
		if uid == submission.LastUploaderID {
			return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot request changes on it", sid), http.StatusBadRequest)
		}
	}
	if formAction == constants.ActionVerify {
		if uid == submission.LastUploaderID {
			return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot verify it", sid), http.StatusBadRequest)
		}
	}

	// stop (or ignore) double actions
	if formAction == constants.ActionAssignTesting {
		if uidIn(uid, submission.AssignedTestingUserIDs) {
			return perr(fmt.Sprintf("you are already assigned to test submission %d", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionUnassignTesting {
		if !uidIn(uid, submission.AssignedTestingUserIDs) {
			return perr(fmt.Sprintf("you are not assigned to test submission %d", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionAssignVerification {
		if uidIn(uid, submission.AssignedVerificationUserIDs) {
			return perr(fmt.Sprintf("you are already assigned to verify submission %d", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionUnassignVerification {
		if !uidIn(uid, submission.AssignedVerificationUserIDs) {
			return perr(fmt.Sprintf("you are not assigned to verify submission %d", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionApprove {
		if uidIn(uid, submission.ApprovedUserIDs) {
			return perr(fmt.Sprintf("you have already approved submission %d", sid), http.StatusBadRequest)
		}

	} else if formAction == constants.ActionRequestChanges {
		if markedAdded {
			return perr(fmt.Sprintf("submission %d is alrady marked as added so you cannot request changes on it, please submit a bug report or a pending fix if there is a problem with the submission", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionVerify {
		if uidIn(uid, submission.VerifiedUserIDs) {
			return perr(fmt.Sprintf("you have already verified submission %d", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionMarkAdded {
		if markedAdded {
			return perr(fmt.Sprintf("submission %d is alrady marked as added so it cannot be marked again", sid), http.StatusBadRequest)
		}
	}

	// don't let the same user assign the submission to himself for more than one type of assignment
	if formAction == constants.ActionAssignTesting {
		if uidIn(uid, submission.AssignedVerificationUserIDs) {
			return perr(fmt.Sprintf("you are already assigned to verify submission %d so you cannot assign it for verification", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionAssignVerification {
		if uidIn(uid, submission.AssignedTestingUserIDs) {
			return perr(fmt.Sprintf("you are already assigned to test submission %d so you cannot assign it for testing", sid), http.StatusBadRequest)
		}
	}

	// don't let the same user approve and verify the submission
	if formAction == constants.ActionAssignTesting {
		if uidIn(uid, submission.VerifiedUserIDs) {
			return perr(fmt.Sprintf("you have already verified submission %d so you cannot assign it for testing", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionApprove {
		if uidIn(uid, submission.VerifiedUserIDs) {
			return perr(fmt.Sprintf("you have already verified submission %d so you cannot approve it", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionAssignVerification {
		if uidIn(uid, submission.ApprovedUserIDs) {
			return perr(fmt.Sprintf("you have already approved (tested) submission %d so you cannot assign it for verification", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionVerify {
		if uidIn(uid, submission.ApprovedUserIDs) {
			return perr(fmt.Sprintf("you have already approved (tested) submission %d so you cannot verify it", sid), http.StatusBadRequest)
		}
	}

	// don't let users do actions without assigning first
	if formAction == constants.ActionApprove {
		if !uidIn(uid, submission.AssignedTestingUserIDs) {
			return perr(fmt.Sprintf("you are not assigned to test submission %d so you cannot approve it", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionVerify {
		if !uidIn(uid, submission.AssignedVerificationUserIDs) {
			return perr(fmt.Sprintf("you are not assigned to verify submission %d so you cannot verify it", sid), http.StatusBadRequest)
		}
	}

	// don't let users verify before approve
	if formAction == constants.ActionAssignVerification {
		if len(submission.ApprovedUserIDs) == 0 {
			return perr(fmt.Sprintf("submission %d is not approved (tested) so you cannot assign it for verification", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionVerify {
		if len(submission.ApprovedUserIDs) == 0 {
			return perr(fmt.Sprintf("submission %d is not approved (tested) so you cannot verify it", sid), http.StatusBadRequest)
		}
	}

	// don't let users assign submission they have already confirmed to be good
	if formAction == constants.ActionAssignTesting {
		if uidIn(uid, submission.ApprovedUserIDs) {
			return perr(fmt.Sprintf("you have already approved submission %d so you cannot assign it for testing", sid), http.StatusBadRequest)
		}
	} else if formAction == constants.ActionAssignVerification {
		if uidIn(uid, submission.VerifiedUserIDs) {
			return perr(fmt.Sprintf("you have already verified submission %d so you cannot assign it for verification", sid), http.StatusBadRequest)
		}
	}

	// don't let user mark submission as added until it's verified
	if formAction == constants.ActionMarkAdded {
		if len(submission.VerifiedUserIDs) == 0 {
			return perr(fmt.Sprintf("submission %d is not verified so you cannot mark it as added", sid), http.StatusBadRequest)
		}
	}

	// cannot assign if marked as added
	if formAction == constants.ActionAssignTesting {
		if markedAdded {
			return perr(fmt.Sprintf("submission %d is alrady marked as added so you cannot assign it for testing, please submit a bug report or a pending fix if there is a problem with the submission", sid), http.StatusBadRequest)
		}
	}

	// cannot reject if marked as added
	if formAction == constants.ActionReject {
		if markedAdded {
			return perr(fmt.Sprintf("submission %d is alrady marked as added so you cannot reject it", sid), http.StatusBadRequest)
		}
	}

	// cannot double reject
	if formAction == constants.ActionReject {
		for _, distinctAction := range submission.DistinctActions {
			if constants.ActionReject == distinctAction {
				return perr(fmt.Sprintf("submission %d is alrady rejected so you cannot reject it", sid), http.StatusBadRequest)
			}
		}
	}

	// cannot upload if rejected
	if formAction == constants.ActionUpload {
		for _, distinctAction := range submission.DistinctActions {
			if constants.ActionReject == distinctAction {
				return perr(fmt.Sprintf("submission %d is alrady rejected so you cannot upload a new version", sid), http.StatusBadRequest)
			}
		}
	}

	return nil
}
