package service

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"io"
	"net/http"
	"os"
)

func (s *SiteService) ReceiveSubmissionChunk(ctx context.Context, sid *int64, resumableParams *types.ResumableParams, chunk []byte) (*int64, error) {
	l := resumableLog(ctx, resumableParams)

	uid := utils.UserID(ctx)
	if uid == 0 {
		l.Panic("no user associated with request")
	}

	l.Debug("storing chunk...")
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

	utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
	if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier); err != nil {
		utils.LogCtx(ctx).Error(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
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

	l.Debug("testing chunk")
	isReceived, err := s.resumableUploadService.TestChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber)
	if err != nil {
		l.Error(err)
		return false, err
	}

	if isReceived {
		l.Debug("chunk already received")
	} else {
		l.Debug("chunk not received yet")
	}

	return isReceived, nil
}

func (s *SiteService) ReceiveFlashfreezeChunk(ctx context.Context, resumableParams *types.ResumableParams, chunk []byte) (*int64, error) {
	l := resumableLog(ctx, resumableParams)

	uid := utils.UserID(ctx)
	if uid == 0 {
		l.Panic("no user associated with request")
	}

	l.Debug("storing chunk...")
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

	utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
	if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier); err != nil {
		utils.LogCtx(ctx).Error(err)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	utils.LogCtx(ctx).WithField("amount", 1).Debug("flashfreeze item received")

	return fid, nil
}
