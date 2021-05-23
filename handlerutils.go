package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

type DiscordRole struct {
	ID    int64
	Name  string
	Color string
}

type basePageData struct {
	Username                string
	AvatarURL               string
	IsAuthorizedToUseSystem bool
}

func FormatAvatarURL(uid int64, avatar string) string {
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", uid, avatar)
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (a *App) GetBasePageData(r *http.Request) (*basePageData, error) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		return &basePageData{}, nil
	}

	if userID == 0 {
		return &basePageData{}, nil
	}

	discordUser, err := a.db.GetDiscordUser(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from db")
	}

	isAuthorized, err := a.db.IsDiscordUserAuthorized(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load user authorization")
	}

	bpd := &basePageData{
		Username:                discordUser.Username,
		AvatarURL:               FormatAvatarURL(discordUser.ID, discordUser.Avatar),
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
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse html templates", http.StatusInternalServerError)
		return
	}
	templateBuffer := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(templateBuffer, "layout", data)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to execute html templates", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(templateBuffer.Bytes()); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to write page data", http.StatusInternalServerError)
		return
	}
}

func (a *App) LogIfErr(ctx context.Context, err error) {
	if err != nil {
		LogCtx(ctx).Error(err)
	}
}

type ValidatorResponse struct {
	Filename         string       `json:"filename"`
	Path             string       `json:"path"`
	CurationErrors   []string     `json:"curation_errors"`
	CurationWarnings []string     `json:"curation_warnings"`
	IsExtreme        bool         `json:"is_extreme"`
	CurationType     int          `json:"curation_type"`
	Meta             CurationMeta `json:"meta"`
}

type CurationMeta struct {
	SubmissionID        int64
	SubmissionFileID    int64
	ApplicationPath     *string `json:"Application Path"`
	Developer           *string `json:"Developer"`
	Extreme             *string `json:"Extreme"`
	GameNotes           *string `json:"Game Notes"`
	Languages           *string `json:"Languages"`
	LaunchCommand       *string `json:"Launch Command"`
	OriginalDescription *string `json:"Original Description"`
	PlayMode            *string `json:"Play Mode"`
	Platform            *string `json:"Platform"`
	Publisher           *string `json:"Publisher"`
	ReleaseDate         *string `json:"Release Date"`
	Series              *string `json:"Series"`
	Source              *string `json:"Source"`
	Status              *string `json:"Status"`
	Tags                *string `json:"Tags"`
	TagCategories       *string `json:"Tag Categories"`
	Title               *string `json:"Title"`
	AlternateTitles     *string `json:"Alternate Title"`
	Library             *string `json:"Library"`
	Version             *string `json:"Version"`
	CurationNotes       *string `json:"Curation Notes"`
	MountParameters     *string `json:"Mount Parameters"`
	//AdditionalApplications *CurationFormatAddApps `json:"Additional Applications"`
}

func (a *App) ProcessReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader, sid *int64) error {
	userID := UserIDFromContext(ctx)
	if userID == 0 {
		err := fmt.Errorf("no user associated with request")
		LogCtx(ctx).Error(err)
		return err
	}
	file, err := fileHeader.Open()
	if err != nil {
		LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to open received file")
	}
	defer file.Close()

	LogCtx(ctx).Debugf("received a file '%s' - %d bytes, MIME header: %+v", fileHeader.Filename, fileHeader.Size, fileHeader.Header)

	const dir = "submissions"

	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to make directory structure")
	}

	destinationFilename := RandomString(64) + filepath.Ext(fileHeader.Filename)
	destinationFilePath := fmt.Sprintf("%s/%s", dir, destinationFilename)

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to create destination file")
	}
	defer destination.Close()

	LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	nBytes, err := io.Copy(destination, file)
	if err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("incorrect number of bytes copied to destination")
	}

	LogCtx(ctx).Debug("storing submission...")

	var submissionID int64

	if sid == nil {
		submissionID, err = a.db.StoreSubmission(tx)
		if err != nil {
			LogCtx(ctx).Error(err)
			a.LogIfErr(ctx, destination.Close())
			a.LogIfErr(ctx, os.Remove(destinationFilePath))
			return fmt.Errorf("failed to store submission")
		}
	} else {
		submissionID = *sid
	}

	s := &SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now(),
	}

	fid, err := a.db.StoreSubmissionFile(tx, s)
	if err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store submission")
	}

	c := &Comment{
		AuthorID:     userID,
		SubmissionID: submissionID,
		Message:      nil,
		Action:       ActionUpload,
		CreatedAt:    time.Now(),
	}

	if err := a.db.StoreComment(tx, c); err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store uploader comment")
	}

	LogCtx(ctx).Debug("processing curation meta...")

	resp, err := a.UploadFile(ctx, a.conf.ValidatorServerURL, destinationFilePath)
	if err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("validator: %w", err)
	}

	var vr ValidatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to decode validator response")
	}

	vr.Meta.SubmissionID = submissionID
	vr.Meta.SubmissionFileID = fid

	if err := a.db.StoreCurationMeta(tx, &vr.Meta); err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store curation meta")
	}

	LogCtx(ctx).Debug("processing bot event...")

	bc := ProcessValidatorResponse(&vr)
	if err := a.db.StoreComment(tx, bc); err != nil {
		LogCtx(ctx).Error(err)
		a.LogIfErr(ctx, destination.Close())
		a.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store validator comment")
	}

	return nil
}

// ProcessValidatorResponse determines if the validation is OK and produces appropriate comment
func ProcessValidatorResponse(vr *ValidatorResponse) *Comment {
	c := &Comment{
		AuthorID:     validatorID,
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

	c.Action = ActionRequestChanges
	if len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) == 0 {
		c.Action = ActionApprove
		c.Message = &approvalMessage
	}

	return c
}

// UploadFile POSTs a given file to a given URL via multipart writer and returns the response body if OK
func (a *App) UploadFile(ctx context.Context, url string, filePath string) ([]byte, error) {
	LogCtx(ctx).WithField("filepath", filePath).Debug("opening file for upload")
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

	LogCtx(ctx).WithField("filepath", filePath).Debug("copying file into multipart writer")
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
	LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("uploading file")

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

	LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("response OK")

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
