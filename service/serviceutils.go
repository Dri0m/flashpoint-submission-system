package service

import (
	"context"
	"sync"

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

type SubmissionStatusKeeper struct {
	m map[string]*types.SubmissionStatus
	sync.Mutex
}

func (s *SubmissionStatusKeeper) SetReceived(tempName string) {
	s.Lock()
	defer s.Unlock()
	s.m[tempName] = &types.SubmissionStatus{Status: constants.SubmissionStatusReceived}
}

func (s *SubmissionStatusKeeper) SetCopying(tempName string, message string) {
	s.Lock()
	defer s.Unlock()
	s.m[tempName].Status = constants.SubmissionStatusCopying
	s.m[tempName].Message = &message
}

func (s *SubmissionStatusKeeper) SetValidating(tempName string) {
	s.Lock()
	defer s.Unlock()
	s.m[tempName].Status = constants.SubmissionStatusValidating
	s.m[tempName].Message = nil
}

func (s *SubmissionStatusKeeper) SetFinalizing(tempName string) {
	s.Lock()
	defer s.Unlock()
	s.m[tempName].Status = constants.SubmissionStatusFinalizing
	s.m[tempName].Message = nil
}

func (s *SubmissionStatusKeeper) SetFailed(tempName, message string) {
	s.Lock()
	defer s.Unlock()
	s.m[tempName].Status = constants.SubmissionStatusFailed
	s.m[tempName].Message = &message
}

func (s *SubmissionStatusKeeper) SetSuccess(tempName string, sid int64) {
	s.Lock()
	defer s.Unlock()
	s.m[tempName].Status = constants.SubmissionStatusSuccess
	s.m[tempName].Message = nil
	s.m[tempName].SubmissionID = &sid
}

func (s *SubmissionStatusKeeper) Get(tempName string) *types.SubmissionStatus {
	s.Lock()
	defer s.Unlock()
	ss, ok := s.m[tempName]
	if !ok {
		return nil
	}
	return ss
}
