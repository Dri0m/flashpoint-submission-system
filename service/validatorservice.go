package service

import (
	"context"
	"encoding/json"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
)

type curationValidator struct {
	validatorServerURL string
}

func NewValidator(validatorServerURL string) *curationValidator {
	return &curationValidator{
		validatorServerURL: validatorServerURL,
	}
}

func (c *curationValidator) Validate(ctx context.Context, filePath string, sid, fid int64) (*types.ValidatorResponse, error) {
	resp, err := utils.UploadFile(ctx, c.validatorServerURL, filePath)
	if err != nil {
		return nil, err
	}

	var vr types.ValidatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		return nil, err
	}

	vr.Meta.SubmissionID = sid
	vr.Meta.SubmissionFileID = fid

	return &vr, nil
}
