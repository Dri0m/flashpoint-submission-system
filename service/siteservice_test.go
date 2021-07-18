package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/kofalt/go-memoize"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"strconv"
	"testing"
	"time"
)

const b64Example = "iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAAHElEQVQI12P4//8/w38GIAXDIBKE0DHxgljNBAAO9TXL0Y4OHwAAAABJRU5ErkJggg==" // some red dot PNG

////////////////////////////////////////////////

type testService struct {
	s                    *SiteService
	authBot              *mockAuthBot
	notificationBot      *mockNotificationBot
	dal                  *mockDAL
	dbs                  *mockDBSession
	validator            *mockValidator
	multipartFileWrapper *mockMultipartFileWrapper
	authTokenProvider    *mockAuthTokenProvider
}

func NewTestSiteService() *testService {
	authBot := &mockAuthBot{}
	notificationBot := &mockNotificationBot{}
	dal := &mockDAL{}
	dbs := &mockDBSession{}
	validator := &mockValidator{}
	multipartFileWrapper := &mockMultipartFileWrapper{}
	authTokenProvider := &mockAuthTokenProvider{}

	return &testService{
		s: &SiteService{
			authBot:                  authBot,
			notificationBot:          notificationBot,
			dal:                      dal,
			validator:                validator,
			clock:                    &fakeClock{},
			randomStringProvider:     &fakeRandomStringProvider{},
			authTokenProvider:        authTokenProvider,
			sessionExpirationSeconds: 0,
			discordRoleCache:         memoize.NewMemoizer(2*time.Minute, 60*time.Minute),
		},
		authBot:              authBot,
		notificationBot:      notificationBot,
		dal:                  dal,
		dbs:                  dbs,
		validator:            validator,
		multipartFileWrapper: multipartFileWrapper,
		authTokenProvider:    authTokenProvider,
	}
}

func (ts *testService) assertExpectations(t *testing.T) {
	ts.authBot.AssertExpectations(t)
	ts.notificationBot.AssertExpectations(t)
	ts.dal.AssertExpectations(t)
	ts.dbs.AssertExpectations(t)
	ts.validator.AssertExpectations(t)
	ts.multipartFileWrapper.AssertExpectations(t)
	ts.authTokenProvider.AssertExpectations(t)
}

////////////////////////////////////////////////

func createAssertBPD(ts *testService, uid int64) *types.BasePageData {
	username := "username"
	avatar := "avatar"
	avatarURL := fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", uid, avatar)
	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: username,
		Avatar:   avatar,
	}
	userRoles := []string{"a"}
	bpd := &types.BasePageData{
		Username:  username,
		UserID:    uid,
		AvatarURL: avatarURL,
		UserRoles: userRoles,
	}

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	return bpd
}

////////////////////////////////////////////////

func Test_siteService_GetBasePageData_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	username := "username"
	avatar := "avatar"
	avatarURL := fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", uid, avatar)
	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: username,
		Avatar:   avatar,
	}
	userRoles := []string{"a"}
	expected := &types.BasePageData{
		Username:  username,
		UserID:    uid,
		AvatarURL: avatarURL,
		UserRoles: userRoles,
	}
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetBasePageData(ctx)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetBasePageData_Fail_GetDiscordUserRoles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	username := "username"
	avatar := "avatar"
	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: username,
		Avatar:   avatar,
	}
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("GetDiscordUserRoles", uid).Return(([]string)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetBasePageData(ctx)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetBasePageData_Fail_GetDiscordUser(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetDiscordUser", uid).Return((*types.DiscordUser)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetBasePageData(ctx)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetBasePageData_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetBasePageData(ctx)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetViewSubmissionPageData_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	var ciid int64 = 4
	var nsid int64 = 5
	var psid int64 = 6
	bpd := createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}
	comments := []*types.ExtendedComment{{}}
	curationImages := []*types.CurationImage{{ID: ciid}}

	tagList := []types.Tag{{Name: "foo", Description: "bar"}}

	expected := &types.ViewSubmissionPageData{
		SubmissionsPageData: types.SubmissionsPageData{
			BasePageData: *bpd,
			Submissions:  submissions,
		},
		CurationMeta:         cm,
		Comments:             comments,
		CurationImageIDs:     []int64{ciid},
		NextSubmissionID:     &nsid,
		PreviousSubmissionID: &psid,
		TagList:              tagList,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dal.On("IsUserSubscribedToSubmission", uid, sid).Return(false, nil)
	ts.dal.On("GetCurationImagesBySubmissionFileID", fid).Return(curationImages, nil)
	ts.dal.On("GetNextSubmission", sid).Return(nsid, nil)
	ts.dal.On("GetPreviousSubmission", sid).Return(psid, nil)
	ts.validator.On("GetTags", ctx).Return(tagList, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_SearchSubmissions(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(([]*types.ExtendedSubmission)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_GetCurationMetaBySubmissionFileID(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return((*types.CurationMeta)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_GetExtendedCommentsBySubmissionID(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(([]*types.ExtendedComment)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_IsUserSubscribedToSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}
	comments := []*types.ExtendedComment{{}}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dal.On("IsUserSubscribedToSubmission", uid, sid).Return(false, errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_GetCurationImagesBySubmissionFileID(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}
	comments := []*types.ExtendedComment{{}}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dal.On("IsUserSubscribedToSubmission", uid, sid).Return(false, nil)
	ts.dal.On("GetCurationImagesBySubmissionFileID", fid).Return(([]*types.CurationImage)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_GetNextSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	var ciid int64 = 4
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}
	comments := []*types.ExtendedComment{{}}
	curationImages := []*types.CurationImage{{ID: ciid}}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dal.On("IsUserSubscribedToSubmission", uid, sid).Return(false, nil)
	ts.dal.On("GetCurationImagesBySubmissionFileID", fid).Return(curationImages, nil)
	ts.dal.On("GetNextSubmission", sid).Return((int64)(0), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_GetPreviousSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	var ciid int64 = 4
	var nsid int64 = 5
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}
	comments := []*types.ExtendedComment{{}}
	curationImages := []*types.CurationImage{{ID: ciid}}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dal.On("IsUserSubscribedToSubmission", uid, sid).Return(false, nil)
	ts.dal.On("GetCurationImagesBySubmissionFileID", fid).Return(curationImages, nil)
	ts.dal.On("GetNextSubmission", sid).Return(nsid, nil)
	ts.dal.On("GetPreviousSubmission", sid).Return((int64)(0), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_GetTags(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	var ciid int64 = 4
	var nsid int64 = 5
	var psid int64 = 6
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}
	comments := []*types.ExtendedComment{{}}
	curationImages := []*types.CurationImage{{ID: ciid}}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dal.On("IsUserSubscribedToSubmission", uid, sid).Return(false, nil)
	ts.dal.On("GetCurationImagesBySubmissionFileID", fid).Return(curationImages, nil)
	ts.dal.On("GetNextSubmission", sid).Return(nsid, nil)
	ts.dal.On("GetPreviousSubmission", sid).Return(psid, nil)
	ts.validator.On("GetTags", ctx).Return(([]types.Tag)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, uid, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetSubmissionsFilesPageData_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	bpd := createAssertBPD(ts, uid)

	sf := []*types.ExtendedSubmissionFile{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	expected := &types.SubmissionsFilesPageData{
		BasePageData:    *bpd,
		SubmissionFiles: sf,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetExtendedSubmissionFilesBySubmissionID", sid).Return(sf, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetSubmissionsFilesPageData(ctx, sid)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetSubmissionsFilesPageData_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetSubmissionsFilesPageData(ctx, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetSubmissionsFilesPageData_Fail_GetExtendedSubmissionFilesBySubmissionID(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	createAssertBPD(ts, uid)

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetExtendedSubmissionFilesBySubmissionID", sid).Return(([]*types.ExtendedSubmissionFile)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetSubmissionsFilesPageData(ctx, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetSubmissionsPageData_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	bpd := createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	expected := &types.SubmissionsPageData{
		BasePageData: *bpd,
		Submissions:  submissions,
		Filter:       *filter,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetSubmissionsPageData(ctx, filter)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetSubmissionsPageData_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetSubmissionsPageData(ctx, filter)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetSubmissionsPageData_Fail_SearchSubmissions(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(([]*types.ExtendedSubmission)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetSubmissionsPageData(ctx, filter)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_SearchSubmissions_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SearchSubmissions(ctx, filter)

	assert.Equal(t, submissions, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SearchSubmissions_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.SearchSubmissions(ctx, filter)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SearchSubmissions_Fail_SearchSubmissions(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	filter := &types.SubmissionsFilter{
		SubmissionIDs: []int64{sid},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(([]*types.ExtendedSubmission)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SearchSubmissions(ctx, filter)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetSubmissionFiles_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sfids := []int64{fid}

	sf := []*types.SubmissionFile{
		{
			SubmitterID:  uid,
			SubmissionID: sid,
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", sfids).Return(sf, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetSubmissionFiles(ctx, sfids)

	assert.Equal(t, sf, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetSubmissionFiles_Fail_GetSubmissionFiles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 3

	sfids := []int64{fid}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", sfids).Return(([]*types.SubmissionFile)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetSubmissionFiles(ctx, sfids)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetSubmissionFiles_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 3

	sfids := []int64{fid}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetSubmissionFiles(ctx, sfids)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetUIDFromSession_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	key := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetUIDFromSession", key).Return(uid, true, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, ok, err := ts.s.GetUIDFromSession(ctx, key)

	assert.Equal(t, uid, actual)
	assert.True(t, ok)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetUIDFromSession_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	key := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))
	actual, ok, err := ts.s.GetUIDFromSession(ctx, key)

	assert.Zero(t, actual)
	assert.False(t, ok)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetUIDFromSession_Fail_GetUIDFromSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	key := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetUIDFromSession", key).Return((int64)(0), false, errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, ok, err := ts.s.GetUIDFromSession(ctx, key)

	assert.Zero(t, actual)
	assert.False(t, ok)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_SoftDeleteSubmissionFile_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3
	var fid int64 = 4

	sfs := []*types.SubmissionFile{
		{SubmissionID: sid},
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", []int64{fid}).Return(sfs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid, deleteReason).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid, deleteReason)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 2

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_GetSubmissionFiles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 4

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", []int64{fid}).Return(([]*types.SubmissionFile)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_SoftDeleteSubmissionFile(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3
	var fid int64 = 4

	sfs := []*types.SubmissionFile{
		{SubmissionID: sid},
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", []int64{fid}).Return(sfs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid, deleteReason).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_StoreNotification(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3
	var fid int64 = 4

	sfs := []*types.SubmissionFile{
		{SubmissionID: sid},
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", []int64{fid}).Return(sfs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid, deleteReason).Return(nil)
	ts.dbs.On("Ctx").Return(ctx)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3
	var fid int64 = 4

	sfs := []*types.SubmissionFile{
		{SubmissionID: sid},
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetSubmissionFiles", []int64{fid}).Return(sfs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid, deleteReason).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_SoftDeleteSubmission_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3

	submissions := []*types.ExtendedSubmission{
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", &types.SubmissionsFilter{SubmissionIDs: []int64{sid}}).Return(submissions, nil)
	ts.dal.On("SoftDeleteSubmission", sid, deleteReason).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmission(ctx, sid, deleteReason)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmission_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.SoftDeleteSubmission(ctx, sid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmission_Fail_SearchSubmissions(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", &types.SubmissionsFilter{SubmissionIDs: []int64{sid}}).Return(([]*types.ExtendedSubmission)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmission(ctx, sid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmission_Fail_SoftDeleteSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3

	submissions := []*types.ExtendedSubmission{
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", &types.SubmissionsFilter{SubmissionIDs: []int64{sid}}).Return(submissions, nil)
	ts.dal.On("SoftDeleteSubmission", sid, deleteReason).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmission(ctx, sid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmission_Fail_StoreNotification(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3

	submissions := []*types.ExtendedSubmission{
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", &types.SubmissionsFilter{SubmissionIDs: []int64{sid}}).Return(submissions, nil)
	ts.dal.On("SoftDeleteSubmission", sid, deleteReason).Return(nil)
	ts.dbs.On("Ctx").Return(ctx)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmission(ctx, sid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmission_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var authorID int64 = 3

	submissions := []*types.ExtendedSubmission{
		{SubmitterID: authorID},
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", &types.SubmissionsFilter{SubmissionIDs: []int64{sid}}).Return(submissions, nil)
	ts.dal.On("SoftDeleteSubmission", sid, deleteReason).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmission(ctx, sid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_SoftDeleteComment_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var cid int64 = 2
	var sid int64 = 3
	var authorID int64 = 4

	c := &types.Comment{
		SubmissionID: sid,
		AuthorID:     authorID,
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCommentByID", cid).Return(c, nil)
	ts.dal.On("SoftDeleteComment", cid, deleteReason).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteComment(ctx, cid, deleteReason)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteComment_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var cid int64 = 2

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.SoftDeleteComment(ctx, cid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteComment_Fail_GetCommentByID(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var cid int64 = 2

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCommentByID", cid).Return((*types.Comment)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteComment(ctx, cid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteComment_Fail_SoftDeleteComment(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var cid int64 = 2
	var sid int64 = 3
	var authorID int64 = 4

	c := &types.Comment{
		SubmissionID: sid,
		AuthorID:     authorID,
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCommentByID", cid).Return(c, nil)
	ts.dal.On("SoftDeleteComment", cid, deleteReason).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteComment(ctx, cid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteComment_Fail_StoreNotification(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var cid int64 = 2
	var sid int64 = 3
	var authorID int64 = 4

	c := &types.Comment{
		SubmissionID: sid,
		AuthorID:     authorID,
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCommentByID", cid).Return(c, nil)
	ts.dal.On("SoftDeleteComment", cid, deleteReason).Return(nil)
	ts.dbs.On("Ctx").Return(ctx)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteComment(ctx, cid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteComment_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var cid int64 = 2
	var sid int64 = 3
	var authorID int64 = 4

	c := &types.Comment{
		SubmissionID: sid,
		AuthorID:     authorID,
	}

	deleteReason := "foobar"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCommentByID", cid).Return(c, nil)
	ts.dal.On("SoftDeleteComment", cid, deleteReason).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationDefault).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteComment(ctx, cid, deleteReason)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_SaveUser_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	userRolesIDsNumeric := []int64{
		rid,
	}

	a := &authToken{
		Secret: "xyzzy",
		UserID: strconv.FormatInt(uid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("StoreDiscordServerRoles", serverRoles).Return(nil)
	ts.dal.On("StoreDiscordUserRoles", uid, userRolesIDsNumeric).Return(nil)

	ts.authTokenProvider.On("CreateAuthToken", uid).Return(a, nil)

	ts.dal.On("StoreSession", a.Secret, uid, ts.s.sessionExpirationSeconds).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Equal(t, a, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	userRolesIDsNumeric := []int64{
		rid,
	}

	a := &authToken{
		Secret: "xyzzy",
		UserID: strconv.FormatInt(uid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("StoreDiscordServerRoles", serverRoles).Return(nil)
	ts.dal.On("StoreDiscordUserRoles", uid, userRolesIDsNumeric).Return(nil)

	ts.authTokenProvider.On("CreateAuthToken", uid).Return(a, nil)

	ts.dal.On("StoreSession", a.Secret, uid, ts.s.sessionExpirationSeconds).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_StoreSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	userRolesIDsNumeric := []int64{
		rid,
	}

	a := &authToken{
		Secret: "xyzzy",
		UserID: strconv.FormatInt(uid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("StoreDiscordServerRoles", serverRoles).Return(nil)
	ts.dal.On("StoreDiscordUserRoles", uid, userRolesIDsNumeric).Return(nil)

	ts.authTokenProvider.On("CreateAuthToken", uid).Return(a, nil)

	ts.dal.On("StoreSession", a.Secret, uid, ts.s.sessionExpirationSeconds).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_CreateAuthToken(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	userRolesIDsNumeric := []int64{
		rid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("StoreDiscordServerRoles", serverRoles).Return(nil)
	ts.dal.On("StoreDiscordUserRoles", uid, userRolesIDsNumeric).Return(nil)

	ts.authTokenProvider.On("CreateAuthToken", uid).Return((*authToken)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_StoreDiscordUserRoles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	userRolesIDsNumeric := []int64{
		rid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("StoreDiscordServerRoles", serverRoles).Return(nil)
	ts.dal.On("StoreDiscordUserRoles", uid, userRolesIDsNumeric).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_StoreDiscordServerRoles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.dal.On("StoreDiscordServerRoles", serverRoles).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_GetFlashpointRoleIDsForUser(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(([]string)(nil), errors.New(""))

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_GetFlashpointRoles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.authBot.On("GetFlashpointRoles").Return(([]types.DiscordRole)(nil), errors.New(""))

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_StoreDiscordUser(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_GetDiscordUser(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUser", uid).Return((*types.DiscordUser)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var rid int64 = 2

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	serverRoles := []types.DiscordRole{
		{
			ID:    rid,
			Name:  "baz",
			Color: "octarine",
		},
	}

	userRolesIDs := []string{
		strconv.FormatInt(rid, 10),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.authBot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.authBot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_Logout_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	secret := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("DeleteSession", secret).Return(nil)

	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.Logout(ctx, secret)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_Logout_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	secret := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.Logout(ctx, secret)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_Logout_Fail_DeleteSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	secret := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("DeleteSession", secret).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.Logout(ctx, secret)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_Logout_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	secret := "foo"

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("DeleteSession", secret).Return(nil)

	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.Logout(ctx, secret)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetUserRoles_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	roles := []string{
		"foo",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetDiscordUserRoles", uid).Return(roles, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetUserRoles(ctx, uid)

	assert.Equal(t, roles, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetUserRoles_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetUserRoles(ctx, uid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetUserRoles_Fail_GetDiscordUserRoles(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetDiscordUserRoles", uid).Return(([]string)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetUserRoles(ctx, uid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetProfilePageData_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	bpd := createAssertBPD(ts, uid)

	actions := []string{
		"foo",
	}

	expected := &types.ProfilePageData{
		BasePageData:        *bpd,
		NotificationActions: actions,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetNotificationSettingsByUserID", uid).Return(actions, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetProfilePageData(ctx, uid)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetProfilePageData_Fail_GetNotificationSettingsByUserID(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	createAssertBPD(ts, uid)

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetNotificationSettingsByUserID", uid).Return(([]string)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetProfilePageData(ctx, uid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetProfilePageData_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetProfilePageData(ctx, uid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_UpdateNotificationSettings_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	actions := []string{
		"foo",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreNotificationSettings", uid, actions).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateNotificationSettings(ctx, uid, actions)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateNotificationSettings_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	actions := []string{
		"foo",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreNotificationSettings", uid, actions).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateNotificationSettings(ctx, uid, actions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateNotificationSettings_Fail_StoreNotificationSettings(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	actions := []string{
		"foo",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreNotificationSettings", uid, actions).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateNotificationSettings(ctx, uid, actions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateNotificationSettings_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	actions := []string{
		"foo",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.UpdateNotificationSettings(ctx, uid, actions)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_UpdateSubscriptionSettings_OK_Subscribe(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateSubscriptionSettings(ctx, uid, sid, true)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateSubscriptionSettings_OK_Unsubscribe(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("UnsubscribeUserFromSubmission", uid, sid).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateSubscriptionSettings(ctx, uid, sid, false)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateSubscriptionSettings_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.UpdateSubscriptionSettings(ctx, uid, sid, true)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateSubscriptionSettings_Fail_UnsubscribeUserFromSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("UnsubscribeUserFromSubmission", uid, sid).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateSubscriptionSettings(ctx, uid, sid, false)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateSubscriptionSettings_Fail_SubscribeUserToSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateSubscriptionSettings(ctx, uid, sid, true)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_UpdateSubscriptionSettings_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.UpdateSubscriptionSettings(ctx, uid, sid, true)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetCurationImage_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var ciid int64 = 2

	ci := &types.CurationImage{
		ID: ciid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCurationImage", ciid).Return(ci, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetCurationImage(ctx, ciid)

	assert.Equal(t, ci, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetCurationImage_Fail_GetCurationImage(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var ciid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetCurationImage", ciid).Return((*types.CurationImage)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetCurationImage(ctx, ciid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetCurationImage_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var ciid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetCurationImage(ctx, ciid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetNextSubmission_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var nsid int64 = 3

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetNextSubmission", sid).Return(nsid, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetNextSubmission(ctx, sid)

	assert.Equal(t, nsid, *actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetNextSubmission_Fail_GetNextSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetNextSubmission", sid).Return((int64)(0), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetNextSubmission(ctx, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetNextSubmission_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetNextSubmission(ctx, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetPreviousSubmission_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var psid int64 = 3

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetPreviousSubmission", sid).Return(psid, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetPreviousSubmission(ctx, sid)

	assert.Equal(t, psid, *actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetPreviousSubmission_Fail_GetPreviousSubmission(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("GetPreviousSubmission", sid).Return((int64)(0), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetPreviousSubmission(ctx, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetPreviousSubmission_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetPreviousSubmission(ctx, sid)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////
