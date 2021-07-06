package service

import (
	"context"
	"errors"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_siteService_ReceiveComments_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}
	var notifiedUsers []int64

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: sid,
			SubmitterID:  uid,
		},
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("GetUsersForNotification", uid, sid, formAction).Return(notifiedUsers, nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(nil)

	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_SearchSubmissions(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("SearchSubmissions", filter).Return(([]*types.ExtendedSubmission)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_StoreComment(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: sid,
			SubmitterID:  uid,
		},
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("StoreComment", c).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_GetUsersForNotification(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: sid,
			SubmitterID:  uid,
		},
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dbs.On("Ctx").Return(ctx)

	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("GetUsersForNotification", uid, sid, formAction).Return(([]int64)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_UpdateSubmissionCacheTable(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}
	var notifiedUsers []int64

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: sid,
			SubmitterID:  uid,
		},
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("GetUsersForNotification", uid, sid, formAction).Return(notifiedUsers, nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}
	var notifiedUsers []int64

	formAction := constants.ActionComment
	formMessage := "foo"
	formIgnoreDupeActions := "false"

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: sid,
			SubmitterID:  uid,
		},
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("GetUsersForNotification", uid, sid, formAction).Return(notifiedUsers, nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(nil)

	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////
