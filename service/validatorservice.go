package service

import (
	"context"
	"encoding/json"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/kofalt/go-memoize"
	"io"
	"time"
)

var cache = memoize.NewMemoizer(10*time.Minute, 60*time.Minute)

type curationValidator struct {
	validatorServerURL string
}

func NewValidator(validatorServerURL string) *curationValidator {
	return &curationValidator{
		validatorServerURL: validatorServerURL,
	}
}

func (c *curationValidator) Validate(ctx context.Context, file io.Reader, filename string) (*types.ValidatorResponse, error) {
	resp, err := utils.UploadMultipartFile(ctx, c.validatorServerURL+"/upload", file, filename)
	if err != nil {
		return nil, err
	}

	var vr types.ValidatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		return nil, err
	}

	return &vr, nil
}

func (c *curationValidator) GetTags(ctx context.Context) ([]types.Tag, error) {
	f := func() (interface{}, error) {
		return c.getTags(ctx)
	}

	resp, err, cached := cache.Memoize("GetTags", f)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, err
	}

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("getting tags from validator")

	tags := resp.([]types.Tag)
	return tags, nil
}

func (c *curationValidator) getTags(ctx context.Context) ([]types.Tag, error) {
	resp, err := utils.GetURL(c.validatorServerURL + "/tags")
	if err != nil {
		return nil, err
	}

	var tr types.ValidatorTagResponse
	err = json.Unmarshal(resp, &tr)
	if err != nil {
		return nil, err
	}

	return tr.Tags, nil
}
