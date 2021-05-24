package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"io"
	"mime/multipart"
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
func (a *App) GetBasePageData(ctx context.Context) (*basePageData, error) {
	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		return &basePageData{}, nil
	}

	discordUser, err := a.DB.GetDiscordUser(ctx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from database")
	}

	isAuthorized, err := a.DB.IsDiscordUserAuthorized(ctx, uid)
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

func (a *App) ProcessReceivedSubmissions(ctx context.Context, tx *sql.Tx, sid *int64, fileHeaders []*multipart.FileHeader) error {
	destinationFilenames := make([]string, 0)

	for _, fileHeader := range fileHeaders {
		destinationFilename, err := a.ProcessReceivedSubmission(ctx, tx, fileHeader, sid)

		if destinationFilename != nil {
			destinationFilenames = append(destinationFilenames, *destinationFilename)
		}

		if err != nil {
			utils.LogCtx(ctx).Error(err)
			utils.LogIfErr(ctx, tx.Rollback())
			for _, df := range destinationFilenames {
				utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
				utils.LogIfErr(ctx, os.Remove(df))
			}
			return fmt.Errorf("file '%s': %s", fileHeader.Filename, err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.LogIfErr(ctx, tx.Rollback())
		for _, df := range destinationFilenames {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
			utils.LogIfErr(ctx, os.Remove(df))
		}
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (a *App) ProcessReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader, sid *int64) (*string, error) {
	userID := utils.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("no user associated with request")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open received file")
	}
	defer file.Close()

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes, MIME header: %+v", fileHeader.Filename, fileHeader.Size, fileHeader.Header)

	const dir = "submissions"

	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return nil, fmt.Errorf("failed to make directory structure")
	}

	destinationFilename := utils.RandomString(64) + filepath.Ext(fileHeader.Filename)
	destinationFilePath := fmt.Sprintf("%s/%s", dir, destinationFilename)

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file")
	}
	defer func() { utils.LogIfErr(ctx, destination.Close()) }()

	utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	nBytes, err := io.Copy(destination, file)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		return &destinationFilePath, fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("storing submission...")

	var submissionID int64

	if sid == nil {
		submissionID, err = a.DB.StoreSubmission(ctx, tx)
		if err != nil {
			return &destinationFilePath, fmt.Errorf("failed to store submission")
		}
	} else {
		submissionID = *sid
	}

	s := &types.SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      utils.UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now(),
	}

	fid, err := a.DB.StoreSubmissionFile(ctx, tx, s)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("failed to store submission")
	}

	c := &types.Comment{
		AuthorID:     userID,
		SubmissionID: submissionID,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    time.Now(),
	}

	if err := a.DB.StoreComment(ctx, tx, c); err != nil {
		return &destinationFilePath, fmt.Errorf("failed to store uploader comment")
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	resp, err := utils.UploadFile(ctx, a.Conf.ValidatorServerURL, destinationFilePath)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("validator: %w", err)
	}

	var vr validatorResponse
	err = json.Unmarshal(resp, &vr)
	if err != nil {
		return &destinationFilePath, fmt.Errorf("failed to decode validator response")
	}

	vr.Meta.SubmissionID = submissionID
	vr.Meta.SubmissionFileID = fid

	if err := a.DB.StoreCurationMeta(ctx, tx, &vr.Meta); err != nil {
		return &destinationFilePath, fmt.Errorf("failed to store curation meta")
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := convertValidatorResponseToComment(&vr)
	if err := a.DB.StoreComment(ctx, tx, bc); err != nil {
		return &destinationFilePath, fmt.Errorf("failed to store validator comment")
	}

	return &destinationFilePath, nil
}

// convertValidatorResponseToComment produces appropriate comment based on validator response
func convertValidatorResponseToComment(vr *validatorResponse) *types.Comment {
	c := &types.Comment{
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

func (a *App) ProcessReceivedComment(ctx context.Context, tx *sql.Tx, uid, sid int64, formAction, formMessage string) error {
	var message *string
	if formMessage != "" {
		message = &formMessage
	}

	actions := []string{constants.ActionComment, constants.ActionApprove, constants.ActionRequestChanges, constants.ActionAccept, constants.ActionMarkAdded, constants.ActionReject, constants.ActionUpload}
	isActionValid := false
	for _, a := range actions {
		if formAction == a {
			isActionValid = true
			break
		}
	}

	if !isActionValid {
		return fmt.Errorf("invalid comment action")
	}

	actionsWithMandatoryMessage := []string{constants.ActionComment, constants.ActionRequestChanges, constants.ActionReject}
	isActionWithMandatoryMessage := false
	for _, a := range actionsWithMandatoryMessage {
		if formAction == a {
			isActionWithMandatoryMessage = true
			break
		}
	}

	if isActionWithMandatoryMessage && (message == nil || *message == "") {
		return fmt.Errorf("cannot post comment action '%s' without a message", formAction)
	}

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: sid,
		Message:      message,
		Action:       formAction,
		CreatedAt:    time.Now(),
	}

	if err := a.DB.StoreComment(ctx, tx, c); err != nil {
		return fmt.Errorf("failed to store comment")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (a *App) ProcessViewSubmission(ctx context.Context, sid int64) (*viewSubmissionPageData, error) {
	bpd, err := a.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	filter := &types.SubmissionsFilter{
		SubmissionID: &sid,
	}

	submissions, err := a.DB.SearchSubmissions(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to load submission")
	}

	if len(submissions) == 0 {
		return nil, fmt.Errorf("submission not found")
	}

	submission := submissions[0]

	meta, err := a.DB.GetCurationMetaBySubmissionFileID(ctx, submission.FileID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to load curation meta")
	}

	comments, err := a.DB.GetExtendedCommentsBySubmissionID(ctx, sid)
	if err != nil {
		return nil, fmt.Errorf("failed to load curation comments")
	}

	pageData := &viewSubmissionPageData{
		basePageData: *bpd,
		Submissions:  submissions,
		CurationMeta: meta,
		Comments:     comments,
	}

	return pageData, nil
}

func (a *App) ProcessSubmissionsPage(ctx context.Context) (*submissionsPageData, error) {
	bpd, err := a.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	submissions, err := a.DB.SearchSubmissions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load submissions")
	}

	pageData := &submissionsPageData{basePageData: *bpd, Submissions: submissions}
	return pageData, nil
}
