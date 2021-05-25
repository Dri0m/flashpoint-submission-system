package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/bot"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

type Service struct {
	Bot                      bot.Bot
	DB                       database.DB
	ValidatorServerURL       string
	SessionExpirationSeconds int64
}

// GetBasePageData loads base user data, does not return error if user is not logged in
func (s *Service) GetBasePageData(ctx context.Context) (*basePageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		return &basePageData{}, nil
	}

	discordUser, err := s.DB.GetDiscordUser(ctx, tx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to get user data from database")
	}

	isAuthorized, err := s.DB.IsDiscordUserAuthorized(ctx, tx, uid)
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

func (s *Service) ProcessReceivedSubmissions(ctx context.Context, sid *int64, fileHeaders []*multipart.FileHeader) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	destinationFilenames := make([]string, 0)

	for _, fileHeader := range fileHeaders {
		destinationFilename, err := s.ProcessReceivedSubmission(ctx, tx, fileHeader, sid)

		if destinationFilename != nil {
			destinationFilenames = append(destinationFilenames, *destinationFilename)
		}

		if err != nil {
			for _, df := range destinationFilenames {
				utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
				utils.LogIfErr(ctx, os.Remove(df))
			}
			return fmt.Errorf("file '%s': %s", fileHeader.Filename, err.Error())
		}
	}

	if err := tx.Commit(); err != nil {
		for _, df := range destinationFilenames {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", df)
			utils.LogIfErr(ctx, os.Remove(df))
		}
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *Service) ProcessReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader, sid *int64) (*string, error) {
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
		submissionID, err = s.DB.StoreSubmission(ctx, tx)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return &destinationFilePath, fmt.Errorf("failed to store submission")
		}
	} else {
		submissionID = *sid
	}

	sf := &types.SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      utils.UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now(),
	}

	fid, err := s.DB.StoreSubmissionFile(ctx, tx, sf)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store submission file")
	}

	c := &types.Comment{
		AuthorID:     userID,
		SubmissionID: submissionID,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    time.Now(),
	}

	if err := s.DB.StoreComment(ctx, tx, c); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store uploader comment")
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	resp, err := utils.UploadFile(ctx, s.ValidatorServerURL, destinationFilePath)
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

	if err := s.DB.StoreCurationMeta(ctx, tx, &vr.Meta); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, fmt.Errorf("failed to store curation meta")
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := convertValidatorResponseToComment(&vr)
	if err := s.DB.StoreComment(ctx, tx, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
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

func (s *Service) beginTx() (*sql.Tx, error) {
	return s.DB.Conn.Begin()
}

func (s *Service) rollbackTx(ctx context.Context, tx *sql.Tx) {
	utils.LogIfErr(ctx, tx.Rollback())
}

func (s *Service) ProcessReceivedComment(ctx context.Context, uid, sid int64, formAction, formMessage string) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

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

	if err := s.DB.StoreComment(ctx, tx, c); err != nil {
		return fmt.Errorf("failed to store comment")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction")
	}

	return nil
}

func (s *Service) ProcessViewSubmission(ctx context.Context, sid int64) (*viewSubmissionPageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	filter := &types.SubmissionsFilter{
		SubmissionID: &sid,
	}

	submissions, err := s.DB.SearchSubmissions(ctx, tx, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submission")
	}

	if len(submissions) == 0 {
		return nil, fmt.Errorf("submission not found")
	}

	submission := submissions[0]

	meta, err := s.DB.GetCurationMetaBySubmissionFileID(ctx, tx, submission.FileID)
	if err != nil && err != sql.ErrNoRows {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation meta")
	}

	comments, err := s.DB.GetExtendedCommentsBySubmissionID(ctx, tx, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load curation comments")
	}

	pageData := &viewSubmissionPageData{
		submissionsPageData: submissionsPageData{
			basePageData: *bpd,
			Submissions:  submissions,
		},
		CurationMeta: meta,
		Comments:     comments,
	}

	return pageData, nil
}

func (s *Service) ProcessSearchSubmissions(ctx context.Context, filter *types.SubmissionsFilter) (*submissionsPageData, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	bpd, err := s.GetBasePageData(ctx)
	if err != nil {
		return nil, err
	}

	submissions, err := s.DB.SearchSubmissions(ctx, tx, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to load submissions")
	}

	pageData := &submissionsPageData{basePageData: *bpd, Submissions: submissions}
	return pageData, nil
}

func (s *Service) ProcessDownloadSubmission(ctx context.Context, sid int64) (*types.ExtendedSubmission, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	filter := &types.SubmissionsFilter{
		SubmissionID: &sid,
	}

	submissions, err := s.DB.SearchSubmissions(ctx, tx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to load submission")
	}

	if len(submissions) == 0 {
		return nil, fmt.Errorf("submission not found")
	}

	return submissions[0], nil
}

func (s *Service) ProcessDiscordCallback(ctx context.Context, discordUser *types.DiscordUser) (*AuthToken, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	// save discord user data
	if err := s.DB.StoreDiscordUser(ctx, tx, discordUser); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord user")
	}

	// get and save discord user authorization
	isAuthorized, err := s.Bot.IsUserAuthorized(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to obtain discord user's roles")
	}
	if err := s.DB.StoreDiscordUserAuthorization(ctx, tx, discordUser.ID, isAuthorized); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store discord user's authorization")
	}

	// create cookie and save session
	authToken, err := CreateAuthToken(discordUser.ID)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to generate auth token")
	}

	if err = s.DB.StoreSession(ctx, tx, authToken.Secret, discordUser.ID, s.SessionExpirationSeconds); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, fmt.Errorf("failed to store session")
	}

	return authToken, nil
}

func (s *Service) ProcessLogout(ctx context.Context, secret string) error {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	if err := s.DB.DeleteSession(ctx, tx, secret); err != nil {
		utils.LogCtx(ctx).Error(err)
		return fmt.Errorf("unable to delete session")
	}

	return nil
}

func (s *Service) GetUserAuthorization(ctx context.Context, uid int64) (bool, error) {
	tx, err := s.beginTx()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return false, fmt.Errorf("failed to begin transaction")
	}
	defer s.rollbackTx(ctx, tx)

	isAuthorized, err := s.DB.IsDiscordUserAuthorized(ctx, tx, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return false, fmt.Errorf("failed to load user authorization")
	}

	return isAuthorized, nil
}
