package service

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"net/http"
	"time"
)

func (s *SiteService) ReceiveComments(ctx context.Context, uid int64, sids []int64, formAction, formMessage, formIgnoreDupeActions string) error {
	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	var message *string
	if formMessage != "" {
		message = &formMessage
	}

	// TODO refactor these validators into a function and cover with tests
	actions := constants.GetAllowedActions()
	isActionValid := false
	for _, a := range actions {
		if formAction == a {
			isActionValid = true
			break
		}
	}

	if !isActionValid {
		return perr("invalid comment action", http.StatusBadRequest)
	}

	actionsWithMandatoryMessage := constants.GetActionsWithMandatoryMessage()
	isActionWithMandatoryMessage := false
	for _, a := range actionsWithMandatoryMessage {
		if formAction == a {
			isActionWithMandatoryMessage = true
			break
		}
	}

	if isActionWithMandatoryMessage && (message == nil || *message == "") {
		return perr(fmt.Sprintf("cannot post comment action '%s' without a message", formAction), http.StatusBadRequest)
	}

	ignoreDupeActions := false
	if formIgnoreDupeActions == "true" {
		ignoreDupeActions = true
	}

	// stop request changes on comment batches
	if formAction == constants.ActionRequestChanges && len(sids) > 1 {
		return perr(fmt.Sprintf("cannot request changes on multiple submissions at once"), http.StatusBadRequest)
	}

	utils.LogCtx(ctx).Debugf("searching submissions for comment batch")
	foundSubmissions, err := s.dal.SearchSubmissions(dbs, &types.SubmissionsFilter{SubmissionIDs: sids})
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	for _, sid := range sids {
		found := false
		for _, s := range foundSubmissions {
			if sid == s.SubmissionID {
				found = true
			}
		}
		if !found {
			return perr(fmt.Sprintf("submission %d not found", sid), http.StatusNotFound)
		}
	}

	// TODO optimize batch operation even more
SubmissionLoop:
	for _, submission := range foundSubmissions {
		sid := submission.SubmissionID

		uidIn := func(ids []int64) bool {
			for _, assignedUserID := range ids {
				if uid == assignedUserID {
					return true
				}
			}
			return false
		}

		markedAdded := false

		for _, distinctAction := range submission.DistinctActions {
			if constants.ActionAssignVerification == distinctAction {
				markedAdded = true
				break
			}
		}

		// don't let last uploader decide on the submission
		if formAction == constants.ActionAssignTesting || formAction == constants.ActionUnassignVerification {
			if uid == submission.LastUploaderID {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot assign it", sid), http.StatusBadRequest)
			}
		}
		if formAction == constants.ActionApprove {
			if uid == submission.LastUploaderID {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot approve it", sid), http.StatusBadRequest)
			}
		}
		if formAction == constants.ActionRequestChanges {
			if uid == submission.LastUploaderID {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot request changes on it", sid), http.StatusBadRequest)
			}
		}
		if formAction == constants.ActionVerify {
			if uid == submission.LastUploaderID {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are the uploader of the newest version of submission %d, so you cannot verify it", sid), http.StatusBadRequest)
			}
		}

		// stop (or ignore) double actions
		if formAction == constants.ActionAssignTesting {
			if uidIn(submission.AssignedTestingUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are already assigned to test submission %d", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionUnassignTesting {
			if !uidIn(submission.AssignedTestingUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are not assigned to test submission %d", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionAssignVerification {
			if uidIn(submission.AssignedVerificationUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are already assigned to verify submission %d", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionUnassignVerification {
			if !uidIn(submission.AssignedVerificationUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are not assigned to verify submission %d", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionApprove {
			if uidIn(submission.ApprovedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already approved submission %d", sid), http.StatusBadRequest)
			}

		} else if formAction == constants.ActionRequestChanges {
			if markedAdded {
				return perr(fmt.Sprintf("submission %d is alrady marked as added so you cannot request changes on it, please submit a bug report or a pending fix if there is a problem with the submission", sid), http.StatusBadRequest)
			}
			if uidIn(submission.RequestedChangesUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already requested changes on submission %d", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionVerify {
			if uidIn(submission.VerifiedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already verified submission %d", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionMarkAdded {
			if markedAdded {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("submission %d is alrady marked as added so it cannot be marked again", sid), http.StatusBadRequest)
			}
		}

		// don't let the same user assign the submission to himself for more than one type of assignment
		if formAction == constants.ActionAssignTesting {
			if uidIn(submission.AssignedVerificationUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are already assigned to verify submission %d so you cannot assign it for verification", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionAssignVerification {
			if uidIn(submission.AssignedTestingUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are already assigned to test submission %d so you cannot assign it for testing", sid), http.StatusBadRequest)
			}
		}

		// don't let the same user approve and verify the submission
		if formAction == constants.ActionAssignTesting {
			if uidIn(submission.VerifiedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already verified submission %d so you cannot assign it for testing", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionApprove {
			if uidIn(submission.VerifiedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already verified submission %d so you cannot approve it", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionAssignVerification {
			if uidIn(submission.ApprovedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already approved (tested) submission %d so you cannot assign it for verification", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionVerify {
			if uidIn(submission.ApprovedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already approved (tested) submission %d so you cannot verify it", sid), http.StatusBadRequest)
			}
		}

		// don't let users do actions without assigning first
		if formAction == constants.ActionApprove {
			if !uidIn(submission.AssignedTestingUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are not assigned to test submission %d so you cannot approve it", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionVerify {
			if !uidIn(submission.AssignedVerificationUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you are not assigned to verify submission %d so you cannot verify it", sid), http.StatusBadRequest)
			}
		}

		// don't let users verify before approve
		if formAction == constants.ActionAssignVerification {
			if len(submission.ApprovedUserIDs) == 0 {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("submission %d is not approved (tested) so you cannot assign it for verification", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionVerify {
			if len(submission.ApprovedUserIDs) == 0 {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("submission %d is not approved (tested) so you cannot verify it", sid), http.StatusBadRequest)
			}
		}

		// don't let users assign submission they have already confirmed to be good
		if formAction == constants.ActionAssignTesting {
			if uidIn(submission.ApprovedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already approved submission %d so you cannot assign it for testing", sid), http.StatusBadRequest)
			}
		} else if formAction == constants.ActionAssignVerification {
			if uidIn(submission.VerifiedUserIDs) {
				if ignoreDupeActions {
					continue SubmissionLoop
				}
				return perr(fmt.Sprintf("you have already verified submission %d so you cannot assign it for verification", sid), http.StatusBadRequest)
			}
		}

		// actually store the comment
		c := &types.Comment{
			AuthorID:     uid,
			SubmissionID: sid,
			Message:      message,
			Action:       formAction,
			CreatedAt:    s.clock.Now(),
		}

		// clear messages for assigns and unassigns
		if formAction == constants.ActionAssignTesting ||
			formAction == constants.ActionUnassignTesting ||
			formAction == constants.ActionAssignVerification ||
			formAction == constants.ActionUnassignVerification {
			c.Message = nil
		}

		if err := s.dal.StoreComment(dbs, c); err != nil {
			utils.LogCtx(ctx).Error(err)
			return dberr(err)
		}

		// unassign if needed
		if formAction == constants.ActionApprove {
			c = &types.Comment{
				AuthorID:     uid,
				SubmissionID: sid,
				Message:      nil,
				Action:       constants.ActionUnassignTesting,
				CreatedAt:    s.clock.Now().Add(time.Second),
			}

			if err := s.dal.StoreComment(dbs, c); err != nil {
				utils.LogCtx(ctx).Error(err)
				return dberr(err)
			}
		} else if formAction == constants.ActionVerify {
			c = &types.Comment{
				AuthorID:     uid,
				SubmissionID: sid,
				Message:      nil,
				Action:       constants.ActionUnassignVerification,
				CreatedAt:    s.clock.Now().Add(time.Second),
			}

			if err := s.dal.StoreComment(dbs, c); err != nil {
				utils.LogCtx(ctx).Error(err)
				return dberr(err)
			}
		}

		if err := s.createNotification(dbs, uid, sid, formAction); err != nil {
			utils.LogCtx(ctx).Error(err)
			return dberr(err)
		}

		if err := s.dal.UpdateSubmissionCacheTable(dbs, sid); err != nil {
			utils.LogCtx(ctx).Error(err)
			return dberr(err)
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	s.announceNotification()

	return nil
}
