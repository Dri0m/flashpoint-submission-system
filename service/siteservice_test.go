package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

////////////////////////////////////////////////

type mockDBSession struct {
	mock.Mock
}

func (m *mockDBSession) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDBSession) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDBSession) Tx() *sql.Tx {
	args := m.Called()
	return args.Get(0).(*sql.Tx)
}

func (m *mockDBSession) Ctx() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

////////////////////////////////////////////////

type mockDAL struct {
	mock.Mock
}

func (m *mockDAL) NewSession(_ context.Context) (database.DBSession, error) {
	args := m.Called()
	return args.Get(0).(*mockDBSession), args.Error(1)
}

func (m *mockDAL) StoreSession(_ database.DBSession, key string, uid int64, durationSeconds int64) error {
	args := m.Called(key, uid, durationSeconds)
	return args.Error(0)
}

func (m *mockDAL) DeleteSession(_ database.DBSession, secret string) error {
	args := m.Called(secret)
	return args.Error(0)
}

func (m *mockDAL) GetUIDFromSession(_ database.DBSession, key string) (int64, bool, error) {
	args := m.Called(key)
	return args.Get(0).(int64), args.Bool(1), args.Error(2)
}

func (m *mockDAL) StoreDiscordUser(_ database.DBSession, discordUser *types.DiscordUser) error {
	args := m.Called(discordUser)
	return args.Error(0)
}

func (m *mockDAL) GetDiscordUser(_ database.DBSession, uid int64) (*types.DiscordUser, error) {
	args := m.Called(uid)
	return args.Get(0).(*types.DiscordUser), args.Error(1)
}

func (m *mockDAL) StoreDiscordServerRoles(_ database.DBSession, roles []types.DiscordRole) error {
	args := m.Called(roles)
	return args.Error(0)
}

func (m *mockDAL) StoreDiscordUserRoles(_ database.DBSession, uid int64, roles []int64) error {
	args := m.Called(uid, roles)
	return args.Error(0)
}

func (m *mockDAL) GetDiscordUserRoles(_ database.DBSession, uid int64) ([]string, error) {
	args := m.Called(uid)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDAL) StoreSubmission(_ database.DBSession) (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) StoreSubmissionFile(_ database.DBSession, s *types.SubmissionFile) (int64, error) {
	args := m.Called(s)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) GetSubmissionFiles(_ database.DBSession, sfids []int64) ([]*types.SubmissionFile, error) {
	args := m.Called(sfids)
	return args.Get(0).([]*types.SubmissionFile), args.Error(1)
}

func (m *mockDAL) GetExtendedSubmissionFilesBySubmissionID(_ database.DBSession, sid int64) ([]*types.ExtendedSubmissionFile, error) {
	args := m.Called(sid)
	return args.Get(0).([]*types.ExtendedSubmissionFile), args.Error(1)
}

func (m *mockDAL) SearchSubmissions(_ database.DBSession, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	args := m.Called(filter)
	return args.Get(0).([]*types.ExtendedSubmission), args.Error(1)
}

func (m *mockDAL) StoreCurationMeta(_ database.DBSession, cm *types.CurationMeta) error {
	args := m.Called(cm)
	return args.Error(0)
}

func (m *mockDAL) GetCurationMetaBySubmissionFileID(_ database.DBSession, sfid int64) (*types.CurationMeta, error) {
	args := m.Called(sfid)
	return args.Get(0).(*types.CurationMeta), args.Error(1)
}

func (m *mockDAL) StoreComment(_ database.DBSession, c *types.Comment) error {
	args := m.Called(c)
	return args.Error(0)
}

func (m *mockDAL) GetExtendedCommentsBySubmissionID(_ database.DBSession, sid int64) ([]*types.ExtendedComment, error) {
	args := m.Called(sid)
	return args.Get(0).([]*types.ExtendedComment), args.Error(1)
}

func (m *mockDAL) SoftDeleteSubmissionFile(_ database.DBSession, sfid int64) error {
	args := m.Called(sfid)
	return args.Error(0)
}

////////////////////////////////////////////////

type mockBot struct {
	mock.Mock
}

func (m *mockBot) GetFlashpointRoleIDsForUser(uid int64) ([]string, error) {
	args := m.Called(uid)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockBot) GetFlashpointRoles() ([]types.DiscordRole, error) {
	args := m.Called()
	return args.Get(0).([]types.DiscordRole), args.Error(1)
}

////////////////////////////////////////////////

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Validate(ctx context.Context, filePath string, sid, fid int64) (*types.ValidatorResponse, error) {
	args := m.Called(filePath, sid, fid)
	return args.Get(0).(*types.ValidatorResponse), args.Error(1)
}

////////////////////////////////////////////////

type testService struct {
	s         *siteService
	bot       *mockBot
	dal       *mockDAL
	dbs       *mockDBSession
	validator *mockValidator
}

func NewTestSiteService() *testService {
	bot := &mockBot{}
	dal := &mockDAL{}
	dbs := &mockDBSession{}
	validator := &mockValidator{}

	return &testService{
		s: &siteService{
			bot:                      bot,
			dal:                      dal,
			validator:                validator,
			sessionExpirationSeconds: 0,
		},
		bot:       bot,
		dal:       dal,
		dbs:       dbs,
		validator: validator,
	}
}

func (ts *testService) assertExpectations(t *testing.T) {
	ts.bot.AssertExpectations(t)
	ts.dal.AssertExpectations(t)
	ts.dbs.AssertExpectations(t)
	ts.validator.AssertExpectations(t)
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
		AvatarURL: avatarURL,
		UserRoles: userRoles,
	}
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetBasePageData(ctx)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////
