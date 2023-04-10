package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/kofalt/go-memoize"
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

func (c *curationValidator) ProvideArchiveForRepacking(filePath string) (*types.ValidatorRepackResponse, error) {
	filePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 86400 * time.Second}
	resp, err := client.Post(fmt.Sprintf("%s/pack-path?path=%s", c.validatorServerURL, url.QueryEscape(filePath)), "application/json;charset=utf-8", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provide to remote error: %s", string(bytes))
	}

	var vr types.ValidatorRepackResponse
	err = json.Unmarshal(bytes, &vr)
	if err != nil {
		return nil, err
	}

	return &vr, nil
}

func (c *curationValidator) ProvideArchiveForValidation(filePath string) (*types.ValidatorResponse, error) {
	filePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	client := http.Client{Timeout: 86400 * time.Second}
	resp, err := client.Post(fmt.Sprintf("%s/provide-path?path=%s", c.validatorServerURL, url.QueryEscape(filePath)), "application/json;charset=utf-8", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provide to remote error: %s", string(bytes))
	}

	var vr types.ValidatorResponse
	err = json.Unmarshal(bytes, &vr)
	if err != nil {
		return nil, err
	}

	return &vr, nil
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
