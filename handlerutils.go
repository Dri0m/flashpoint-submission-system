package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"io"
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
		AvatarURL:               fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", discordUser.ID, discordUser.Avatar),
		IsAuthorizedToUseSystem: isAuthorized,
	}

	return bpd, nil
}

// RenderTemplates is a helper for rendering templates
func (a *App) RenderTemplates(ctx context.Context, w http.ResponseWriter, r *http.Request, data interface{}, filenames ...string) {
	templates := []string{"templates/base.gohtml", "templates/navbar.gohtml"}
	templates = append(templates, filenames...)
	tmpl, err := template.ParseFiles(templates...)
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

func (a *App) ProcessReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader) error {
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

	LogCtx(ctx).Debugf("copying received file to '%s'...", destinationFilePath)

	nBytes, err := io.Copy(destination, file)
	if err != nil {
		LogCtx(ctx).Error(err)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		LogCtx(ctx).Error(err)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return fmt.Errorf("incorrect number of bytes copied to destination")
	}

	s := &Submission{
		ID:               0,
		UploaderID:       UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now(),
	}

	if err := a.db.StoreSubmission(tx, s); err != nil {
		LogCtx(ctx).Error(err)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		a.LogIfErr(ctx, tx.Rollback())
		return fmt.Errorf("failed to store submission")
	}

	return nil
}
