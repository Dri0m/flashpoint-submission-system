package service

import (
	"context"
	"database/sql"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/stretchr/testify/mock"
	"io"
	"mime/multipart"
	"time"
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

func (m *mockDAL) StoreSubmission(_ database.DBSession, submissionLevel string) (int64, error) {
	args := m.Called(submissionLevel)
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

func (m *mockDAL) GetCommentByID(_ database.DBSession, cid int64) (*types.Comment, error) {
	args := m.Called(cid)
	return args.Get(0).(*types.Comment), args.Error(1)
}

func (m *mockDAL) SoftDeleteSubmissionFile(_ database.DBSession, sfid int64, deleteReason string) error {
	args := m.Called(sfid, deleteReason)
	return args.Error(0)
}

func (m *mockDAL) SoftDeleteSubmission(_ database.DBSession, sid int64, deleteReason string) error {
	args := m.Called(sid, deleteReason)
	return args.Error(0)
}

func (m *mockDAL) SoftDeleteComment(_ database.DBSession, cid int64, deleteReason string) error {
	args := m.Called(cid, deleteReason)
	return args.Error(0)
}

func (m *mockDAL) StoreNotificationSettings(_ database.DBSession, uid int64, actions []string) error {
	args := m.Called(uid, actions)
	return args.Error(0)
}

func (m *mockDAL) GetNotificationSettingsByUserID(_ database.DBSession, uid int64) ([]string, error) {
	args := m.Called(uid)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDAL) SubscribeUserToSubmission(_ database.DBSession, uid, sid int64) error {
	args := m.Called(uid, sid)
	return args.Error(0)
}

func (m *mockDAL) UnsubscribeUserFromSubmission(_ database.DBSession, uid, sid int64) error {
	args := m.Called(uid, sid)
	return args.Error(0)
}

func (m *mockDAL) IsUserSubscribedToSubmission(_ database.DBSession, uid, sid int64) (bool, error) {
	args := m.Called(uid, sid)
	return args.Bool(0), args.Error(1)
}

func (m *mockDAL) StoreNotification(_ database.DBSession, msg, notificationType string) error {
	args := m.Called(msg, notificationType)
	return args.Error(0)
}

func (m *mockDAL) GetUsersForNotification(_ database.DBSession, authorID, sid int64, action string) ([]int64, error) {
	args := m.Called(authorID, sid, action)
	return args.Get(0).([]int64), args.Error(1)
}

func (m *mockDAL) GetOldestUnsentNotification(_ database.DBSession) (*types.Notification, error) {
	args := m.Called()
	return args.Get(0).(*types.Notification), args.Error(1)
}

func (m *mockDAL) MarkNotificationAsSent(_ database.DBSession, nid int64) error {
	args := m.Called(nid)
	return args.Error(0)
}

func (m *mockDAL) StoreCurationImage(_ database.DBSession, c *types.CurationImage) (int64, error) {
	args := m.Called(c)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) GetCurationImagesBySubmissionFileID(_ database.DBSession, sfid int64) ([]*types.CurationImage, error) {
	args := m.Called(sfid)
	return args.Get(0).([]*types.CurationImage), args.Error(1)
}

func (m *mockDAL) GetCurationImage(_ database.DBSession, ciid int64) (*types.CurationImage, error) {
	args := m.Called(ciid)
	return args.Get(0).(*types.CurationImage), args.Error(1)
}

func (m *mockDAL) GetNextSubmission(_ database.DBSession, sid int64) (int64, error) {
	args := m.Called(sid)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) GetPreviousSubmission(_ database.DBSession, sid int64) (int64, error) {
	args := m.Called(sid)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockDAL) UpdateSubmissionCacheTable(_ database.DBSession, sid int64) error {
	args := m.Called(sid)
	return args.Error(0)
}

func (m *mockDAL) ClearMasterDBGames(_ database.DBSession) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDAL) StoreMasterDBGames(_ database.DBSession, games []*types.MasterDatabaseGame) error {
	args := m.Called(games)
	return args.Error(0)
}

func (m *mockDAL) GetAllSimilarityAttributes(_ database.DBSession) ([]*types.SimilarityAttributes, error) {
	args := m.Called()
	return args.Get(0).([]*types.SimilarityAttributes), args.Error(1)
}

////////////////////////////////////////////////

type mockAuthBot struct {
	mock.Mock
}

func (m *mockAuthBot) GetFlashpointRoleIDsForUser(uid int64) ([]string, error) {
	args := m.Called(uid)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockAuthBot) GetFlashpointRoles() ([]types.DiscordRole, error) {
	args := m.Called()
	return args.Get(0).([]types.DiscordRole), args.Error(1)
}

////////////////////////////////////////////////

type mockNotificationBot struct {
	mock.Mock
}

func (m *mockNotificationBot) SendNotification(msg, notificationType string) error {
	args := m.Called(msg, notificationType)
	return args.Error(0)
}

////////////////////////////////////////////////

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Validate(_ context.Context, file io.Reader, filename, filepath string) (*types.ValidatorResponse, error) {
	args := m.Called(file, filename, filepath)
	return args.Get(0).(*types.ValidatorResponse), args.Error(1)
}

func (m *mockValidator) GetTags(ctx context.Context) ([]types.Tag, error) {
	args := m.Called(ctx)
	return args.Get(0).([]types.Tag), args.Error(1)
}

////////////////////////////////////////////////

type mockMultipartFileWrapper struct {
	mock.Mock
}

func (m *mockMultipartFileWrapper) Filename() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockMultipartFileWrapper) Size() int64 {
	args := m.Called()
	return args.Get(0).(int64)
}

func (m *mockMultipartFileWrapper) Open() (multipart.File, error) {
	args := m.Called()
	return args.Get(0).(multipart.File), args.Error(1)
}

////////////////////////////////////////////////

type fakeClock struct {
}

func (f fakeClock) Now() time.Time {
	return time.Time{}
}

func (f fakeClock) Unix(sec int64, nsec int64) time.Time {
	return time.Unix(sec, nsec)
}

////////////////////////////////////////////////

type fakeRandomStringProvider struct {
}

func (f fakeRandomStringProvider) RandomString(n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += "a"
	}
	return result
}

////////////////////////////////////////////////

type mockAuthTokenProvider struct {
	mock.Mock
}

func (m *mockAuthTokenProvider) CreateAuthToken(userID int64) (*authToken, error) {
	args := m.Called(userID)
	return args.Get(0).(*authToken), args.Error(1)
}

////////////////////////////////////////////////

type fakeMultipartFile struct{}

func (fmp *fakeMultipartFile) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (fmp *fakeMultipartFile) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, io.EOF
}

func (fmp *fakeMultipartFile) Seek(offset int64, whence int) (int64, error) {
	return 0, io.EOF
}

func (fmp *fakeMultipartFile) Close() error {
	return nil
}
