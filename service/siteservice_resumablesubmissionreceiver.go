package service

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

func (s *SiteService) ReceiveSubmissionChunk(ctx context.Context, sid *int64, resumableParams *types.ResumableParams, chunk []byte) (*int64, error) {
	l := utils.LogCtx(ctx).WithFields(logrus.Fields{
		"resumableChunkNumber":      resumableParams.ResumableChunkNumber,
		"resumableChunkSize":        resumableParams.ResumableChunkSize,
		"resumableTotalSize":        resumableParams.ResumableTotalSize,
		"resumableIdentifier":       resumableParams.ResumableIdentifier,
		"resumableFilename":         resumableParams.ResumableFilename,
		"resumableRelativePath":     resumableParams.ResumableRelativePath,
		"resumableCurrentChunkSize": resumableParams.ResumableCurrentChunkSize,
	})

	l.Debug("storing chunk...")
	err := s.resumableUploadService.PutChunk(resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, chunk)
	if err != nil {
		l.Error(err)
		return nil, err
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(resumableParams.ResumableIdentifier, resumableParams.ResumableTotalSize)
	if err != nil {
		l.Error(err)
		return nil, err
	}

	if isComplete {
		utils.LogCtx(ctx).Debug("resumable upload finished")
		return s.processReceivedResumableSubmission(ctx, sid, resumableParams)
	}

	return nil, nil
}

type resumableUpload struct {
	fileID string
	rsu    *resumableuploadservice.ResumableUploadService
}

func (ru resumableUpload) GetReadCloser() (io.ReadCloser, error) {
	return ru.rsu.NewFileReader(ru.fileID)
}

func (s *SiteService) processReceivedResumableSubmission(ctx context.Context, sid *int64, resumableParams *types.ResumableParams) (*int64, error) {
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

	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
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

	if constants.IsInAudit(userRoles) && resumableParams.ResumableTotalSize > constants.UserInAuditSumbissionMaxFilesize {
		return nil, perr("submission filesize limited to 200MB for users in audit", http.StatusForbidden)
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
		fileID: resumableParams.ResumableIdentifier,
		rsu:    s.resumableUploadService,
	}
	destinationFilename, ifp, submissionID, err := s.processReceivedSubmission(ctx, dbs, ru, resumableParams.ResumableFilename, resumableParams.ResumableTotalSize, sid, submissionLevel)

	utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
	if e := s.resumableUploadService.DeleteFile(resumableParams.ResumableIdentifier); e != nil {
		utils.LogCtx(ctx).Error(e)
	}

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

	s.announceNotification()

	return &submissionID, nil
}

func (s *SiteService) IsSubmissionChunkReceived(ctx context.Context, sid *int64, resumableParams *types.ResumableParams) (bool, error) {
	l := utils.LogCtx(ctx).WithFields(logrus.Fields{
		"resumableChunkNumber":      resumableParams.ResumableChunkNumber,
		"resumableChunkSize":        resumableParams.ResumableChunkSize,
		"resumableTotalSize":        resumableParams.ResumableTotalSize,
		"resumableIdentifier":       resumableParams.ResumableIdentifier,
		"resumableFilename":         resumableParams.ResumableFilename,
		"resumableRelativePath":     resumableParams.ResumableRelativePath,
		"resumableCurrentChunkSize": resumableParams.ResumableCurrentChunkSize,
	})

	isComplete, err := s.resumableUploadService.IsUploadFinished(resumableParams.ResumableIdentifier, resumableParams.ResumableTotalSize)
	if err != nil {
		l.Error(err)
		return false, err
	}

	if isComplete {
		return false, perr("file already fully received", http.StatusConflict)
	}

	l.Debug("testing chunk")
	isReceived, err := s.resumableUploadService.TestChunk(resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber)
	if err != nil {
		l.Error(err)
		return false, err
	}

	if isReceived {
		l.Debug("chunk already received")
	} else {
		l.Debug("chunk not received yet")
	}

	// TODO handle case where file finishes upload here

	return isReceived, nil
}
