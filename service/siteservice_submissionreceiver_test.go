package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"testing"
	"time"
)

func Test_siteService_ReceiveSubmissions_OK(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
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

	approvalMessage := "Looks good to me! 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now().Add(time.Second),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreComment", bc).Return(nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(nil)

	ts.dal.On("GetAllSimilarityAttributes").Return([]*types.SimilarityAttributes{}, nil)
	ts.dbs.On("Ctx").Return(ctx)

	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Equal(t, []int64{sid}, sids)
	assert.NoError(t, err)

	assert.FileExists(t, destinationFilePath) // submission file was copied successfully

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_OK_WithSubmissionImage(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	imageDestinationFilename := ts.s.randomStringProvider.RandomString(64)
	imageDestinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionImagesDir, imageDestinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	var ciid int64 = 4

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
	}

	imageType := "logo"

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
		Images: []types.ValidatorResponseImage{
			{
				Type: imageType,
				Data: b64Example,
			},
		},
	}

	ci := &types.CurationImage{
		SubmissionFileID: fid,
		Type:             imageType,
		Filename:         imageDestinationFilename,
	}

	approvalMessage := "Looks good to me! 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now().Add(time.Second),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreCurationImage", ci).Return(ciid, nil)
	ts.dal.On("StoreComment", bc).Return(nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(nil)

	ts.dal.On("GetAllSimilarityAttributes").Return([]*types.SimilarityAttributes{}, nil)
	ts.dbs.On("Ctx").Return(ctx)

	ts.dbs.On("Commit").Return(nil)
	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Equal(t, []int64{sid}, sids)
	assert.NoError(t, err)

	assert.FileExists(t, destinationFilePath)      // submission file was copied successfully
	assert.FileExists(t, imageDestinationFilePath) // image file was copied successfully

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_NewSession(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return((*mockDBSession)(nil), errors.New(""))

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_GetDiscordUserRoles(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(([]string)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreSubmission(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
		Extreme:          &extreme,
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_SubscribeUserToSubmission(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
		Extreme:          &extreme,
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreSubmissionFile(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
		Extreme:          &extreme,
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreUploadComment(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

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

	extreme := "No"

	meta := types.CurationMeta{
		SubmissionID:     sid,
		SubmissionFileID: fid,
		Extreme:          &extreme,
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_Validate(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	userRoles := []string{}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return((*types.ValidatorResponse)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreCurationMeta(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
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

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreNotification(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	imageDestinationFilename := ts.s.randomStringProvider.RandomString(64)
	imageDestinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionImagesDir, imageDestinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
	}

	imageType := "logo"

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
		Images: []types.ValidatorResponseImage{
			{
				Type: imageType,
				Data: b64Example,
			},
		},
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath)      // cleanup when upload fails
	assert.NoFileExists(t, imageDestinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreCurationImage(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	imageDestinationFilename := ts.s.randomStringProvider.RandomString(64)
	imageDestinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionImagesDir, imageDestinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3
	var ciid int64 = 4

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
	}

	imageType := "logo"

	vr := &types.ValidatorResponse{
		Filename:         "",
		Path:             "",
		CurationErrors:   []string{},
		CurationWarnings: []string{},
		IsExtreme:        false,
		CurationType:     0,
		Meta:             meta,
		Images: []types.ValidatorResponseImage{
			{
				Type: imageType,
				Data: b64Example,
			},
		},
	}

	ci := &types.CurationImage{
		SubmissionFileID: fid,
		Type:             imageType,
		Filename:         imageDestinationFilename,
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreCurationImage", ci).Return(ciid, errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath)      // cleanup when upload fails
	assert.NoFileExists(t, imageDestinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_StoreBotComment(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
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

	approvalMessage := "Looks good to me! 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now().Add(time.Second),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreComment", bc).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_UpdateSubmissionCacheTable(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
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

	approvalMessage := "Looks good to me! 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now().Add(time.Second),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreComment", bc).Return(nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_GetAllSimilarityAttributes(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
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

	approvalMessage := "Looks good to me! 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now().Add(time.Second),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreComment", bc).Return(nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(nil)

	ts.dbs.On("Ctx").Return(ctx)
	ts.dal.On("GetAllSimilarityAttributes").Return(([]*types.SimilarityAttributes)(nil), errors.New(""))

	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

func Test_siteService_ReceiveSubmissions_Fail_Commit(t *testing.T) {
	ts := NewTestSiteService()

	tmpDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionsDir = tmpDir

	tmpImageDir, err := ioutil.TempDir("", "Test_siteService_ReceiveSubmissions_OK_dir")
	assert.NoError(t, err)
	ts.s.submissionImagesDir = tmpImageDir

	tmpFile, err := ioutil.TempFile("", "Test_siteService_ReceiveSubmissions_OK*.7z")
	assert.NoError(t, err)

	filename := tmpFile.Name()
	var size int64 = 0

	destinationFilename := ts.s.randomStringProvider.RandomString(64) + ".7z"
	destinationFilePath := fmt.Sprintf("%s/%s", ts.s.submissionsDir, destinationFilename)

	var uid int64 = 1
	var sid int64 = 2
	var fid int64 = 3

	userRoles := []string{}
	submissionLevel := constants.SubmissionLevelAudition

	extreme := "No"

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
		Extreme:          &extreme,
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

	approvalMessage := "Looks good to me! 🤖"
	bc := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: sid,
		Message:      &approvalMessage,
		Action:       constants.ActionApprove,
		CreatedAt:    ts.s.clock.Now().Add(time.Second),
	}

	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, logrus.NewEntry(logrus.New()))
	ctx = context.WithValue(ctx, utils.CtxKeys.UserID, uid)

	ts.dal.On("NewSession").Return(ts.dbs, nil)

	ts.dal.On("GetDiscordUserRoles", uid).Return(userRoles, nil)

	ts.multipartFileWrapper.On("Open").Return(tmpFile, nil)
	ts.multipartFileWrapper.On("Filename").Return(filename)
	ts.multipartFileWrapper.On("Size").Return(size)

	ts.validator.On("Validate", destinationFilePath).Return(vr, nil)

	ts.dal.On("StoreSubmission", submissionLevel).Return(sid, nil)
	ts.dal.On("SubscribeUserToSubmission", uid, sid).Return(nil)
	ts.dal.On("StoreSubmissionFile", sf).Return(fid, nil)
	ts.dal.On("StoreComment", c).Return(nil)

	ts.dal.On("StoreCurationMeta", &meta).Return(nil)
	ts.dal.On("StoreNotification", mock.AnythingOfType("string"), constants.NotificationCurationFeed).Return(nil)
	ts.dal.On("StoreComment", bc).Return(nil)
	ts.dal.On("UpdateSubmissionCacheTable", sid).Return(nil)

	ts.dal.On("GetAllSimilarityAttributes").Return([]*types.SimilarityAttributes{}, nil)
	ts.dbs.On("Ctx").Return(ctx)

	ts.dbs.On("Commit").Return(errors.New(""))
	ts.dbs.On("Rollback").Return(nil)

	sids, err := ts.s.ReceiveSubmissions(ctx, nil, []MultipartFileProvider{ts.multipartFileWrapper})

	assert.Nil(t, sids)
	assert.Error(t, err)

	assert.NoFileExists(t, destinationFilePath) // cleanup when upload fails

	ts.assertExpectations(t)
}

////////////////////////////////////////////////
