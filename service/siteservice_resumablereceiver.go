package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

func (s *SiteService) ReceiveSubmissionChunk(ctx context.Context, sid *int64, resumableParams *types.ResumableParams, chunk []byte) (*int64, error) {
	l := resumableLog(ctx, resumableParams)

	uid := utils.UserID(ctx)
	if uid == 0 {
		l.Panic("no user associated with request")
	}

	err := s.resumableUploadService.PutChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, chunk)
	if err != nil {
		l.Error(err)
		return nil, err
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalSize)
	if err != nil {
		l.Error(err)
		return nil, err
	}

	if isComplete {
		utils.LogCtx(ctx).Debug("resumable upload finished")
		return s.processReceivedResumableSubmission(ctx, uid, sid, resumableParams)
	}

	return nil, nil
}

type resumableUpload struct {
	uid    int64
	fileID string
	rsu    *resumableuploadservice.ResumableUploadService
}

func (ru resumableUpload) GetReadCloser() (io.ReadCloser, error) {
	return ru.rsu.NewFileReader(ru.uid, ru.fileID)
}

func (s *SiteService) processReceivedResumableSubmission(ctx context.Context, uid int64, sid *int64, resumableParams *types.ResumableParams) (*int64, error) {
	var destinationFilename *string
	imageFilePaths := make([]string, 0)

	cleanup := func() {
		if destinationFilename != nil {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", *destinationFilename)
			if err := os.Remove(*destinationFilename); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
		for _, fp := range imageFilePaths {
			utils.LogCtx(ctx).Debugf("cleaning up image file '%s'...", fp)
			if err := os.Remove(fp); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if constants.IsInAudit(userRoles) && resumableParams.ResumableTotalSize > constants.UserInAuditSubmissionMaxFilesize {
		return nil, perr("submission filesize limited to 500MB for users in audit", http.StatusForbidden)
	}

	var submissionLevel string

	if constants.IsInAudit(userRoles) {
		submissionLevel = constants.SubmissionLevelAudition
	} else if constants.IsTrialCurator(userRoles) {
		submissionLevel = constants.SubmissionLevelTrial
	} else if constants.IsStaff(userRoles) {
		submissionLevel = constants.SubmissionLevelStaff
	}

	ru := &resumableUpload{
		uid:    uid,
		fileID: resumableParams.ResumableIdentifier,
		rsu:    s.resumableUploadService,
	}
	destinationFilename, ifp, submissionID, err := s.processReceivedSubmission(ctx, dbs, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize, sid, submissionLevel)

	for _, imageFilePath := range ifp {
		imageFilePaths = append(imageFilePaths, imageFilePath)
	}

	if err != nil {
		cleanup()
		return nil, err
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
	if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier); err != nil {
		utils.LogCtx(ctx).Error(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("submissions received")
	s.announceNotification()

	return &submissionID, nil
}

func (s *SiteService) IsChunkReceived(ctx context.Context, resumableParams *types.ResumableParams) (bool, error) {
	l := resumableLog(ctx, resumableParams)

	uid := utils.UserID(ctx)
	if uid == 0 {
		l.Panic("no user associated with request")
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalSize)
	if err != nil {
		l.Error(err)
		return false, err
	}

	if isComplete {
		return false, perr("file already fully received", http.StatusConflict)
	}

	isReceived, err := s.resumableUploadService.TestChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber)
	if err != nil {
		l.Error(err)
		return false, err
	}

	return isReceived, nil
}

func (s *SiteService) ReceiveFlashfreezeChunk(ctx context.Context, resumableParams *types.ResumableParams, chunk []byte) (*int64, error) {
	l := resumableLog(ctx, resumableParams)

	uid := utils.UserID(ctx)
	if uid == 0 {
		l.Panic("no user associated with request")
	}

	err := s.resumableUploadService.PutChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, chunk)
	if err != nil {
		l.Error(err)
		return nil, err
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalSize)
	if err != nil {
		l.Error(err)
		return nil, err
	}

	if isComplete {
		utils.LogCtx(ctx).Debug("resumable upload finished")
		return s.processReceivedResumableFlashfreeze(ctx, uid, resumableParams)
	}

	return nil, nil
}

func (s *SiteService) processReceivedResumableFlashfreeze(ctx context.Context, uid int64, resumableParams *types.ResumableParams) (*int64, error) {
	var destinationFilename *string

	cleanup := func() {
		if destinationFilename != nil {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", *destinationFilename)
			if err := os.Remove(*destinationFilename); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	ru := &resumableUpload{
		uid:    uid,
		fileID: resumableParams.ResumableIdentifier,
		rsu:    s.resumableUploadService,
	}
	destinationFilename, fid, err := s.processReceivedFlashfreezeItem(ctx, dbs, uid, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize)
	if err != nil {
		return nil, err
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
	if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier); err != nil {
		utils.LogCtx(ctx).Error(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("flashfreeze items received")

	l := utils.LogCtx(ctx).WithFields(logrus.Fields{"flashfreezeFileID": *fid, "destinationFilename": destinationFilename})
	go s.indexReceivedFlashfreezeFile(l, *fid, *destinationFilename)

	return fid, nil
}

func (s *SiteService) indexReceivedFlashfreezeFile(l *logrus.Entry, fid int64, filePath string) {
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l)
	utils.LogCtx(ctx).Debug("indexing flashfreeze file")

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}
	defer dbs.Rollback()

	files, err := uploadArchiveForIndexing(ctx, filePath, s.archiveIndexerServerURL+"/upload")
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	batch := make([]*types.IndexedFileEntry, 0, 1000)

	for i, g := range files {
		batch = append(batch, g)

		if i%1000 == 0 || i == len(files)-1 {
			utils.LogCtx(ctx).Debug("inserting flashfreeze file contents batch into fpfssdb")
			err = s.dal.StoreFlashfreezeFileContents(dbs, fid, batch)
			if err != nil {
				utils.LogCtx(ctx).Error(err)
				return
			}
			batch = make([]*types.IndexedFileEntry, 0, 1000)
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		return
	}

	utils.LogCtx(ctx).Debug("flashfreeze file indexed")
}

func uploadArchiveForIndexing(ctx context.Context, filePath string, url string) ([]*types.IndexedFileEntry, error) {
	client := http.Client{}
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	ss := strings.Split(filePath, "/")
	filename := ss[len(ss)-1]

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	f, err := os.Open(filePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	utils.LogCtx(ctx).WithField("filepath", filePath).Debug("copying file into multipart writer")
	if _, err := io.Copy(fw, f); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}
	w.Close()

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	utils.LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("uploading file")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusInternalServerError {
			return nil, fmt.Errorf("The archive indexer has exploded, please send the following stack trace to @Dri0m on discord: %s", string(bodyBytes))
		}
		return nil, fmt.Errorf("unexpected response: %s", resp.Status)
	}

	utils.LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("response OK")

	var ir types.IndexerResp
	err = json.Unmarshal(bodyBytes, &ir)
	if err != nil {
		return nil, err
	}

	return ir.Files, nil
}
