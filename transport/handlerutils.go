package transport

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/Masterminds/sprig"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type basePageData struct {
	Username                string
	AvatarURL               string
	IsAuthorizedToUseSystem bool
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (a *App) GetBasePageData(r *http.Request) (*basePageData, error) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return &basePageData{}, nil
	}

	if userID == 0 {
		return &basePageData{}, nil
	}

	discordUser, err := a.DB.GetDiscordUser(userID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from database")
	}

	isAuthorized, err := a.DB.IsDiscordUserAuthorized(userID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user authorization")
	}

	bpd := &basePageData{
		Username:                discordUser.Username,
		AvatarURL:               utils.FormatAvatarURL(discordUser.ID, discordUser.Avatar),
		IsAuthorizedToUseSystem: isAuthorized,
	}

	return bpd, nil
}

// RenderTemplates is a helper for rendering templates
func (a *App) RenderTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) {
	templates := []string{"templates/base.gohtml", "templates/navbar.gohtml"}
	templates = append(templates, filenames...)
	tmpl, err := template.New("base").Funcs(sprig.FuncMap()).Funcs(template.FuncMap{"boolString": BoolString}).ParseFiles(templates...)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	templateBuffer := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(templateBuffer, "layout", data)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to execute html templates", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(templateBuffer.Bytes()); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to write page data", http.StatusInternalServerError)
		return
	}
}

func (a *App) LogIfErr(ctx context.Context, err error) {
	if err != nil {
		utils.LogCtx(ctx).Error(err)
	}
}

type validatorResponse struct {
	Filename         string             `json:"filename"`
	Path             string             `json:"path"`
	CurationErrors   []string           `json:"curation_errors"`
	CurationWarnings []string           `json:"curation_warnings"`
	IsExtreme        bool               `json:"is_extreme"`
	CurationType     int                `json:"curation_type"`
	Meta             types.CurationMeta `json:"meta"`
}

func (a *App) ProcessReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader, sid *int64) error {
	userID := utils.UserIDFromContext(ctx)
	if userID == 0 {
		err := fmt.Errorf("no user associated with request")
		utils.LogCtx(ctx).Error(err)
		return err
	}
	file, err := fileHeader.Open()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to open received file")
	}
	defer file.Close()

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes, MIME header: %+v", fileHeader.Filename, fileHeader.Size, fileHeader.Header)

	const dir = "submissions"

	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to make directory structure")
	}

	destinationFilename := utils.RandomString(64) + filepath.Ext(fileHeader.Filename)
	destinationFilePath := fmt.Sprintf("%s/%s", dir, destinationFilename)

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to create destination file")
	}
	defer destination.Close()

	utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	nBytes, err := io.Copy(destination, file)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("storing submission...")

	var submissionID int64

	if sid == nil {
		submissionID, err = a.DB.StoreSubmission(tx)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			a.LogIfErr(ctx, destination.Close())
			a.LogIfErr(ctx, os.Remove(destinationFilePath))
			return fmt.Errorf("failed to store submission")
		}
	} else {
		submissionID = *sid
	}

	s := &database.SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      utils.UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now(),
	}

	fid, err := a.DB.StoreSubmissionFile(tx, s)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store submission")
	}

	c := &database.Comment{
		AuthorID:     userID,
		SubmissionID: submissionID,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    time.Now(),
	}

	if err := a.DB.StoreComment(tx, c); err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store uploader comment")
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	resp, err := a.UploadFile(ctx, a.Conf.ValidatorServerURL, destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("validator: %w", err)
	}

	var vr validatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to decode validator response")
	}

	vr.Meta.SubmissionID = submissionID
	vr.Meta.SubmissionFileID = fid

	if err := a.DB.StoreCurationMeta(tx, &vr.Meta); err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store curation meta")
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := ProcessValidatorResponse(&vr)
	if err := a.DB.StoreComment(tx, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store validator comment")
	}

	return nil
}

// ProcessValidatorResponse determines if the validation is OK and produces appropriate comment
func ProcessValidatorResponse(vr *validatorResponse) *database.Comment {
	c := &database.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: vr.Meta.SubmissionID,
		CreatedAt:    time.Now(),
	}

	approvalMessage := "LGTM 🤖"
	message := ""

	if len(vr.CurationErrors) > 0 {
		message += "Your curation is invalid:\n"
	}
	if len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) > 0 {
		message += "Your curation might have some problems:\n"
	}

	for _, e := range vr.CurationErrors {
		message += fmt.Sprintf("🚫 %s\n", e)
	}
	for _, w := range vr.CurationWarnings {
		message += fmt.Sprintf("🚫 %s\n", w)
	}

	c.Message = &message

	c.Action = constants.ActionRequestChanges
	if len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) == 0 {
		c.Action = constants.ActionApprove
		c.Message = &approvalMessage
	}

	return c
}

// UploadFile POSTs a given file to a given URL via multipart writer and returns the response body if OK
func (a *App) UploadFile(ctx context.Context, url string, filePath string) ([]byte, error) {
	utils.LogCtx(ctx).WithField("filepath", filePath).Debug("opening file for upload")
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	client := http.Client{}
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	var fw io.Writer

	if fw, err = w.CreateFormFile("file", f.Name()); err != nil {
		return nil, err
	}

	utils.LogCtx(ctx).WithField("filepath", filePath).Debug("copying file into multipart writer")
	if _, err = io.Copy(fw, f); err != nil {
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

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	utils.LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("response OK")

	return bodyBytes, nil
}

// BoolString is a little hack to make handling tri-state bool in go templates trivial
func BoolString(b *bool) string {
	if b == nil {
		return "nil"
	}
	if *b {
		return "true"
	}
	return "false"
}
