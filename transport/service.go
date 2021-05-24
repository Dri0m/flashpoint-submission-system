package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"io"
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
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		utils.LogCtx(ctx).Error(err)
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("storing submission...")

	var submissionID int64

	if sid == nil {
		submissionID, err = a.DB.StoreSubmission(tx)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			utils.LogIfErr(ctx, destination.Close())
			utils.LogIfErr(ctx, os.Remove(destinationFilePath))
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
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
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
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store uploader comment")
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	resp, err := utils.UploadFile(ctx, a.Conf.ValidatorServerURL, destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("validator: %w", err)
	}

	var vr validatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to decode validator response")
	}

	vr.Meta.SubmissionID = submissionID
	vr.Meta.SubmissionFileID = fid

	if err := a.DB.StoreCurationMeta(tx, &vr.Meta); err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
		return fmt.Errorf("failed to store curation meta")
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := ProcessValidatorResponse(&vr)
	if err := a.DB.StoreComment(tx, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.LogIfErr(ctx, destination.Close())
		utils.LogIfErr(ctx, os.Remove(destinationFilePath))
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
