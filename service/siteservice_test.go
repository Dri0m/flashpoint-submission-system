package service

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

////////////////////////////////////////////////

type mockDAL struct {
	mock.Mock
}

func (m *mockDAL) BeginTx() (*sql.Tx, error) {
	args := m.Called()
	return args.Get(0).(*sql.Tx), args.Error(1)
}

func (m *mockDAL) StoreSession(ctx context.Context, tx *sql.Tx, key string, uid int64, durationSeconds int64) error {
	args := m.Called(key, uid, durationSeconds)
	return args.Error(0)
}

func (m *mockDAL) DeleteSession(ctx context.Context, tx *sql.Tx, secret string) error {
	args := m.Called(secret)
	return args.Error(0)
}

func (m *mockDAL) GetUIDFromSession(ctx context.Context, tx *sql.Tx, key string) (int64, bool, error) {
	args := m.Called(key)
	return args.Get(0).(int64), args.Bool(1), args.Error(2)
}

func (m *mockDAL) StoreDiscordUser(ctx context.Context, tx *sql.Tx, discordUser *types.DiscordUser) error {
	args := m.Called(discordUser)
	return args.Error(0)
}

func (m *mockDAL) GetDiscordUser(ctx context.Context, tx *sql.Tx, uid int64) (*types.DiscordUser, error) {
	args := m.Called(uid)
	return args.Get(0).(*types.DiscordUser), args.Error(1)
}

func (m *mockDAL) StoreDiscordServerRoles(ctx context.Context, tx *sql.Tx, roles []types.DiscordRole) error {
	args := m.Called(roles)
	return args.Error(0)
}

func (m *mockDAL) StoreDiscordUserRoles(ctx context.Context, tx *sql.Tx, uid int64, roles []int64) error {
	args := m.Called(uid, roles)
	return args.Error(0)
}

func (m *mockDAL) GetDiscordUserRoles(ctx context.Context, tx *sql.Tx, uid int64) ([]string, error) {
	args := m.Called(uid)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDAL) StoreSubmission(ctx context.Context, tx *sql.Tx) (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) StoreSubmissionFile(ctx context.Context, tx *sql.Tx, s *types.SubmissionFile) (int64, error) {
	args := m.Called(s)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) GetSubmissionFiles(ctx context.Context, tx *sql.Tx, sfids []int64) ([]*types.SubmissionFile, error) {
	args := m.Called(sfids)
	return args.Get(0).([]*types.SubmissionFile), args.Error(1)
}

func (m *mockDAL) GetExtendedSubmissionFilesBySubmissionID(ctx context.Context, tx *sql.Tx, sid int64) ([]*types.ExtendedSubmissionFile, error) {
	args := m.Called(sid)
	return args.Get(0).([]*types.ExtendedSubmissionFile), args.Error(1)
}

func (m *mockDAL) SearchSubmissions(ctx context.Context, tx *sql.Tx, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	args := m.Called(filter)
	return args.Get(0).([]*types.ExtendedSubmission), args.Error(1)
}

func (m *mockDAL) StoreCurationMeta(ctx context.Context, tx *sql.Tx, cm *types.CurationMeta) error {
	args := m.Called(cm)
	return args.Error(0)
}

func (m *mockDAL) GetCurationMetaBySubmissionFileID(ctx context.Context, tx *sql.Tx, sfid int64) (*types.CurationMeta, error) {
	args := m.Called(sfid)
	return args.Get(0).(*types.CurationMeta), args.Error(1)
}

func (m *mockDAL) StoreComment(ctx context.Context, tx *sql.Tx, c *types.Comment) error {
	args := m.Called(c)
	return args.Error(0)
}

func (m *mockDAL) GetExtendedCommentsBySubmissionID(ctx context.Context, tx *sql.Tx, sid int64) ([]*types.ExtendedComment, error) {
	args := m.Called(sid)
	return args.Get(0).([]*types.ExtendedComment), args.Error(1)
}

func (m *mockDAL) SoftDeleteSubmissionFile(ctx context.Context, tx *sql.Tx, sfid int64) error {
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

func NewTestSiteService(bot *mockBot, dal *mockDAL) *siteService {
	return &siteService{
		bot:                      bot,
		dal:                      dal,
		validatorServerURL:       "",
		sessionExpirationSeconds: 0,
	}
}

////////////////////////////////////////////////

func Test_siteService_GetBasePageData_OK(t *testing.T) {
	bot := &mockBot{}
	dal := &mockDAL{}
	s := NewTestSiteService(bot, dal)

	username := "username"
	avatarURL := "avatarURL"
	discordUser := &types.DiscordUser{
		ID:       42,
		Username: username,
		Avatar:   avatarURL,
	}
	userRoles := []string{"a"}
	uid := 1
	expected := &types.BasePageData{
		Username:  username,
		AvatarURL: avatarURL,
		UserRoles: userRoles,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.UserID, uid)

	dal.On("BeginTx").Return((*sql.Tx)(nil), nil)
	dal.On("GetDiscordUser", uid).Return(discordUser, nil)
	dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	actual, err := s.GetBasePageData(ctx)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	dal.AssertExpectations(t)
}
