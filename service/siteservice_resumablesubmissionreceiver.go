package service

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
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

	l.Debug("storing chunk")
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
		var intos int64 = -1337
		return &intos, nil
	}

	return nil, nil
}

func (s *SiteService) IsSubmissionChunkReceived(ctx context.Context, resumableParams *types.ResumableParams) (bool, error) {
	l := utils.LogCtx(ctx).WithFields(logrus.Fields{
		"resumableChunkNumber":      resumableParams.ResumableChunkNumber,
		"resumableChunkSize":        resumableParams.ResumableChunkSize,
		"resumableTotalSize":        resumableParams.ResumableTotalSize,
		"resumableIdentifier":       resumableParams.ResumableIdentifier,
		"resumableFilename":         resumableParams.ResumableFilename,
		"resumableRelativePath":     resumableParams.ResumableRelativePath,
		"resumableCurrentChunkSize": resumableParams.ResumableCurrentChunkSize,
	})

	l.Debug("testing chunk")
	isReceived, err := s.resumableUploadService.TestChunk(resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber)
	if err != nil {
		l.Error(err)
		return false, err
	}

	return isReceived, nil
}
