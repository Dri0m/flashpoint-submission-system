package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/kofalt/go-memoize"
)

var resumableMemoizer = memoize.NewMemoizer(time.Hour*24, time.Hour*24)

func (ru resumableUpload) GetReadCloserInformer() (resumableuploadservice.ReadCloserInformer, error) {
	return ru.rsu.NewFileReader(ru.uid, ru.fileID, ru.chunkCount)
}

func (s *SiteService) ReceiveSubmissionChunk(ctx context.Context, sid *int64, resumableParams *types.ResumableParams, chunk []byte) (*string, error) {
	ctx = context.WithValue(ctx, utils.CtxKeys.Log, resumableLog(ctx, resumableParams))

	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	utils.LogCtx(ctx).Debug("storing submission chunk")
	err := s.resumableUploadService.PutChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, chunk)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks, resumableParams.ResumableTotalSize)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	if isComplete {
		// tempName is used as the ID of the submission while the submission is being processed, used by the client to poll for status
		tempName := s.randomStringProvider.RandomString(32)
		s.SSK.SetReceived(tempName)

		go func() {
			ctx := utils.ValueOnlyContext{ctx}
			utils.LogCtx(ctx).Debug("submission resumable upload finished")

			processReceivedResumableSubmission := func() (interface{}, error) {
				return nil, s.processReceivedResumableSubmission(ctx, uid, sid, resumableParams, tempName)
			}
			processReceivedResumableSubmissionKey := fmt.Sprintf("%d-%s", uid, resumableParams.ResumableIdentifier)

			_, err, cached := resumableMemoizer.Memoize(processReceivedResumableSubmissionKey, processReceivedResumableSubmission)
			utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("processed resumable submission upload")

			if err != nil {
				resumableMemoizer.Storage.Delete(processReceivedResumableSubmissionKey)
				utils.LogCtx(ctx).Error(err)
			}

			if !cached {
				utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
				if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks); err != nil {
					utils.LogCtx(ctx).Error(err)
				}

				utils.LogCtx(ctx).WithField("memoizerKey", processReceivedResumableSubmissionKey).Debug("deleting the memoized call")
				resumableMemoizer.Storage.Delete(processReceivedResumableSubmissionKey)
			}
		}()

		return &tempName, nil
	}

	return nil, nil
}

type resumableUpload struct {
	uid        int64
	fileID     string
	chunkCount int
	rsu        *resumableuploadservice.ResumableUploadService
}

func newResumableUpload(uid int64, fileID string, chunkCount int, rsu *resumableuploadservice.ResumableUploadService) *resumableUpload {
	return &resumableUpload{
		uid:        uid,
		fileID:     fileID,
		chunkCount: chunkCount,
		rsu:        rsu,
	}
}

func (s *SiteService) ReceiveFlashfreezeChunk(ctx context.Context, resumableParams *types.ResumableParams, chunk []byte) (*int64, error) {
	ctx = context.WithValue(ctx, utils.CtxKeys.Log, resumableLog(ctx, resumableParams))

	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	utils.LogCtx(ctx).Debug("storing flashfreeze chunk")
	err := s.resumableUploadService.PutChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, chunk)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks, resumableParams.ResumableTotalSize)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	if isComplete {
		utils.LogCtx(ctx).Debug("flashfreeze resumable upload finished")

		processReceivedResumableFlashfreeze := func() (interface{}, error) {
			return s.processReceivedResumableFlashfreeze(ctx, uid, resumableParams)
		}
		processReceivedResumableFlashfreezeKey := fmt.Sprintf("%d-%s", uid, resumableParams.ResumableIdentifier)

		ifid, err, cached := resumableMemoizer.Memoize(processReceivedResumableFlashfreezeKey, processReceivedResumableFlashfreeze)
		utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("processed resumable flashfreeze upload")

		if err != nil {
			resumableMemoizer.Storage.Delete(processReceivedResumableFlashfreezeKey)
			utils.LogCtx(ctx).Error(err)
			return nil, err
		}

		if !cached {
			utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
			if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks); err != nil {
				utils.LogCtx(ctx).Error(err)
			}

			utils.LogCtx(ctx).WithField("memoizerKey", processReceivedResumableFlashfreezeKey).Debug("deleting the memoized call")
			resumableMemoizer.Storage.Delete(processReceivedResumableFlashfreezeKey)
		}

		return ifid.(*int64), nil
	}

	return nil, nil
}

func (s *SiteService) ReceiveFixesChunk(ctx context.Context, fixID int64, resumableParams *types.ResumableParams, chunk []byte) error {
	ctx = context.WithValue(ctx, utils.CtxKeys.Log, resumableLog(ctx, resumableParams))

	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	utils.LogCtx(ctx).Debug("storing fixes chunk")
	err := s.resumableUploadService.PutChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, chunk)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks, resumableParams.ResumableTotalSize)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return err
	}

	if isComplete {
		utils.LogCtx(ctx).Debug("fixes resumable upload finished")

		processReceivedResumableFlashfreeze := func() (interface{}, error) {
			return s.processReceivedResumableFixesFile(ctx, uid, fixID, resumableParams)
		}
		processReceivedResumableFixesKey := fmt.Sprintf("%d-%s", uid, resumableParams.ResumableIdentifier)

		_, err, cached := resumableMemoizer.Memoize(processReceivedResumableFixesKey, processReceivedResumableFlashfreeze)
		utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("processed resumable fixes upload")

		if err != nil {
			resumableMemoizer.Storage.Delete(processReceivedResumableFixesKey)
			utils.LogCtx(ctx).Error(err)
			return err
		}

		if !cached {
			utils.LogCtx(ctx).Debug("deleting the resumable file chunks")
			if err := s.resumableUploadService.DeleteFile(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks); err != nil {
				utils.LogCtx(ctx).Error(err)
			}

			utils.LogCtx(ctx).WithField("memoizerKey", processReceivedResumableFixesKey).Debug("deleting the memoized call")
			resumableMemoizer.Storage.Delete(processReceivedResumableFixesKey)
		}

		return nil
	}

	return nil
}

///////////

func (s *SiteService) IsChunkReceived(ctx context.Context, resumableParams *types.ResumableParams) (bool, error) {
	ctx = context.WithValue(ctx, utils.CtxKeys.Log, resumableLog(ctx, resumableParams))

	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	isComplete, err := s.resumableUploadService.IsUploadFinished(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableTotalChunks, resumableParams.ResumableTotalSize)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return false, err
	}

	if isComplete {
		return false, perr("file already fully received", http.StatusConflict)
	}

	utils.LogCtx(ctx).Debug("testing chunk")
	isReceived, err := s.resumableUploadService.TestChunk(uid, resumableParams.ResumableIdentifier, resumableParams.ResumableChunkNumber, resumableParams.ResumableCurrentChunkSize)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return false, err
	}

	return isReceived, nil
}
