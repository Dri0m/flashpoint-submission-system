package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"mime/multipart"
	"strconv"
	"testing"
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

func (m *mockValidator) Validate(_ context.Context, filePath string, sid, fid int64) (*types.ValidatorResponse, error) {
	args := m.Called(filePath, sid, fid)
	return args.Get(0).(*types.ValidatorResponse), args.Error(1)
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

type testService struct {
	s                    *siteService
	bot                  *mockBot
	dal                  *mockDAL
	dbs                  *mockDBSession
	validator            *mockValidator
	multipartFileWrapper *mockMultipartFileWrapper
	authTokenProvider    *mockAuthTokenProvider
}

func NewTestSiteService() *testService {
	bot := &mockBot{}
	dal := &mockDAL{}
	dbs := &mockDBSession{}
	validator := &mockValidator{}
	multipartFileWrapper := &mockMultipartFileWrapper{}
	authTokenProvider := &mockAuthTokenProvider{}

	return &testService{
		s: &siteService{
			bot:                      bot,
			dal:                      dal,
			validator:                validator,
			clock:                    &fakeClock{},
			randomStringProvider:     &fakeRandomStringProvider{},
			authTokenProvider:        authTokenProvider,
			sessionExpirationSeconds: 0,
		},
		bot:                  bot,
		dal:                  dal,
		dbs:                  dbs,
		validator:            validator,
		multipartFileWrapper: multipartFileWrapper,
		authTokenProvider:    authTokenProvider,
	}
}

func (ts *testService) assertExpectations(t *testing.T) {
	ts.bot.AssertExpectations(t)
	ts.dal.AssertExpectations(t)
	ts.dbs.AssertExpectations(t)
	ts.validator.AssertExpectations(t)
	ts.multipartFileWrapper.AssertExpectations(t)
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

func Test_siteService_ReceiveSubmissions_OK(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    ts.s.clock.Now(),
	}

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
	}

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
	}

	approvalMessage := "LGTM 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.validator.On("Validate", destinationFilePath, sid, fid).Return(vr, nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreComment", bc).Return(nil)

	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.NoError(t, err)

	assert.FileExists(t, destinationFilePath) // submission file was copied successfully

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreSubmission(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreSubmissionFile(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreUploadComment(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_Validate(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.validator.On("Validate", destinationFilePath, sid, fid).Return((*types.ValidatorResponse)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreCurationMeta(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    ts.s.clock.Now(),
	}

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
	}

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.validator.On("Validate", destinationFilePath, sid, fid).Return(vr, nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreBotComment(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    ts.s.clock.Now(),
	}

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
	}

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
	}

	approvalMessage := "LGTM 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.validator.On("Validate", destinationFilePath, sid, fid).Return(vr, nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreComment", bc).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	sf := &types.SubmissionFile{
		SubmissionID:     sid,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             size,
		UploadedAt:       ts.s.clock.Now(),
		MD5Sum:           "d41d8cd98f00b204e9800998ecf8427e",                                 // empty file hash
		SHA256Sum:        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // empty file hash
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    ts.s.clock.Now(),
	}

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
	}

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
	}

	approvalMessage := "LGTM 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.dal.On("StoreSubmission").Return(sid, nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.validator.On("Validate", destinationFilePath, sid, fid).Return(vr, nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreComment", bc).Return(nil)

	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err = ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_ReceiveComments_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}

	formAction := constants.ActionComment
	formMessage := "foo"

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("StoreComment", c).Return(nil)

	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage)

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

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("StoreComment", c).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveComments_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var sids = []int64{sid}

	formAction := constants.ActionComment
	formMessage := "foo"

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      &formMessage,
		Action:       constants.ActionComment,
		CreatedAt:    ts.s.clock.Now(),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("StoreComment", c).Return(nil)

	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.ReceiveComments(ctx, uid, sids, formAction, formMessage)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

////////////////////////////////////////////////

func Test_siteService_GetViewSubmissionPageData_OK(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	bpd := createAssertBPD(ts, uid)

	filter := &types.SubmissionsFilter{
		SubmissionID: &sid,
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

	expected := &types.ViewSubmissionPageData{
		SubmissionsPageData: types.SubmissionsPageData{
			BasePageData: *bpd,
			Submissions:  submissions,
		},
		CurationMeta: cm,
		Comments:     comments,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(comments, nil)
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, sid)

	assert.Equal(t, expected, actual)
	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_GetViewSubmissionPageData_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var sid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	actual, err := ts.s.GetViewSubmissionPageData(ctx, sid)

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
		SubmissionID: &sid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(([]*types.ExtendedSubmission)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, sid)

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
		SubmissionID: &sid,
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return((*types.CurationMeta)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, sid)

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
		SubmissionID: &sid,
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	cm := &types.CurationMeta{}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SearchSubmissions", filter).Return(submissions, nil)
	ts.dal.On("GetCurationMetaBySubmissionFileID", fid).Return(cm, nil)
	ts.dal.On("GetExtendedCommentsBySubmissionID", sid).Return(([]*types.ExtendedComment)(nil), errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.GetViewSubmissionPageData(ctx, sid)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
		SubmissionID: &sid,
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
		SubmissionID: &sid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
		SubmissionID: &sid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
		SubmissionID: &sid,
	}

	submissions := []*types.ExtendedSubmission{
		{
			SubmissionID: uid,
			SubmitterID:  sid,
			FileID:       fid,
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
		SubmissionID: &sid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
		SubmissionID: &sid,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
	var fid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid).Return(nil)
	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid)

	assert.NoError(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_SoftDeleteSubmissionFile(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid).Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid)

	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SoftDeleteSubmissionFile_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1
	var fid int64 = 2

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("SoftDeleteSubmissionFile", fid).Return(nil)
	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	err := ts.s.SoftDeleteSubmissionFile(ctx, fid)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(userRolesIDs, nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(serverRoles, nil)
	ts.bot.On("GetFlashpointRoleIDsForUser", uid).Return(([]string)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(nil)

	ts.bot.On("GetFlashpointRoles").Return(([]types.DiscordRole)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_StoreDiscordUser(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)
	ts.dal.On("StoreDiscordUser", discordUser).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	actual, err := ts.s.SaveUser(ctx, discordUser)

	assert.Nil(t, actual)
	assert.Error(t, err)

	ts.assertExpectations(t)
}

func Test_siteService_SaveUser_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	var uid int64 = 1

	discordUser := &types.DiscordUser{
		ID:       uid,
		Username: "foo",
		Avatar:   "bar",
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.New())
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
