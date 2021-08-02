package service

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/sirupsen/logrus"
)

func dberr(err error) error {
	return constants.DatabaseError{Err: err}
}

func perr(msg string, status int) error {
	return constants.PublicError{Msg: msg, Status: status}
}

func resumableLog(ctx context.Context, resumableParams *types.ResumableParams) *logrus.Entry {
	if resumableParams == nil {
		panic("invalid arguments provided")
	}

	return utils.LogCtx(ctx).WithFields(logrus.Fields{
		"resumableChunkNumber":      resumableParams.ResumableChunkNumber,
		"resumableChunkSize":        resumableParams.ResumableChunkSize,
		"resumableTotalSize":        resumableParams.ResumableTotalSize,
		"resumableIdentifier":       resumableParams.ResumableIdentifier,
		"resumableFilename":         resumableParams.ResumableFilename,
		"resumableRelativePath":     resumableParams.ResumableRelativePath,
		"resumableCurrentChunkSize": resumableParams.ResumableCurrentChunkSize,
		"resumableTotalChunks":      resumableParams.ResumableTotalChunks,
	})
}
