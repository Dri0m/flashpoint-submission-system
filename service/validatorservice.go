package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/kofalt/go-memoize"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
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

func (c *curationValidator) Validate(ctx context.Context, file multipart.File, filename, filepath string) (*types.ValidatorResponse, error) {
	resp, err := uploadFile(ctx, c.validatorServerURL+"/upload", file, filename, filepath)
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

// uploadFile POSTs a given file to a given URL via multipart writer and returns the response body if OK
func uploadFile(ctx context.Context, url string, f multipart.File, filename, filePath string) ([]byte, error) {
	client := http.Client{}
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}

	utils.LogCtx(ctx).WithField("filepath", filePath).Debug("copying file into multipart writer")
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}

	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return nil, err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	utils.LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("uploading file")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check the response
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusInternalServerError {
			return nil, fmt.Errorf("The validator bot has exploded, please send the following stack trace to @Dri0m or @CurationBotGuy on discord: %s", string(bodyBytes))
		}
		return nil, fmt.Errorf("unexpected response: %s", resp.Status)
	}

	utils.LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("response OK")

	return bodyBytes, nil
}
