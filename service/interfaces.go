package service

import (
	"context"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"mime/multipart"
	"time"
)

type Validator interface {
	Validate(ctx context.Context, filePath string, sid, fid int64) (*types.ValidatorResponse, error)
}

type MultipartFileProvider interface {
	Filename() string
	Size() int64
	Open() (multipart.File, error)
}

type Clock interface {
	Now() time.Time
	Unix(sec int64, nsec int64) time.Time
}
