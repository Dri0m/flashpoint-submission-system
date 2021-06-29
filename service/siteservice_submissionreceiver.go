package service

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/go-sql-driver/mysql"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (s *SiteService) ReceiveSubmissions(ctx context.Context, sid *int64, fileProviders []MultipartFileProvider) ([]int64, error) {
	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}
	defer dbs.Rollback()

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, dberr(err)
	}

	if constants.IsInAudit(userRoles) && len(fileProviders) > 1 {
		return nil, perr("cannot upload more than one submission at once when user is in audit", http.StatusForbidden)
	}

	if constants.IsInAudit(userRoles) && fileProviders[0].Size() > constants.UserInAuditSumbissionMaxFilesize {
		return nil, perr("submission filesize limited to 200MB for users in audit", http.StatusForbidden)
	}

	var submissionLevel string

	if constants.IsInAudit(userRoles) {
		submissionLevel = constants.SubmissionLevelAudition
	} else if constants.IsTrialCurator(userRoles) {
		submissionLevel = constants.SubmissionLevelTrial
	} else if constants.IsStaff(userRoles) {
		submissionLevel = constants.SubmissionLevelStaff
	}

	destinationFilenames := make([]string, 0)
	imageFilePaths := make([]string, 0)

	cleanup := func() {
		for _, fp := range destinationFilenames {
			utils.LogCtx(ctx).Debugf("cleaning up file '%s'...", fp)
			if err := os.Remove(fp); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
		for _, fp := range imageFilePaths {
			utils.LogCtx(ctx).Debugf("cleaning up image file '%s'...", fp)
			if err := os.Remove(fp); err != nil {
				utils.LogCtx(ctx).Error(err)
			}
		}
	}

	var submissionIDs = make([]int64, 0)

	for _, fileProvider := range fileProviders {
		destinationFilename, ifp, submissionID, err := s.processReceivedSubmission(ctx, dbs, fileProvider, sid, submissionLevel)

		if destinationFilename != nil {
			destinationFilenames = append(destinationFilenames, *destinationFilename)
		}
		for _, imageFilePath := range ifp {
			imageFilePaths = append(imageFilePaths, imageFilePath)
		}

		if err != nil {
			cleanup()
			return nil, err
		}

		submissionIDs = append(submissionIDs, submissionID)
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return nil, dberr(err)
	}

	s.announceNotification()

	return submissionIDs, nil
}

func (s *SiteService) processReceivedSubmission(ctx context.Context, dbs database.DBSession, fileHeader MultipartFileProvider, sid *int64, submissionLevel string) (*string, []string, int64, error) {
	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, 0, err
	}
	defer file.Close()

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes", fileHeader.Filename(), fileHeader.Size())

	if err := os.MkdirAll(s.submissionsDir, os.ModeDir); err != nil {
		return nil, nil, 0, err
	}
	if err := os.MkdirAll(s.submissionImagesDir, os.ModeDir); err != nil {
		return nil, nil, 0, err
	}

	ext := filepath.Ext(fileHeader.Filename())

	if ext != ".7z" && ext != ".zip" {
		return nil, nil, 0, perr("unsupported file extension", http.StatusBadRequest)
	}

	var destinationFilename string
	var destinationFilePath string
	for {
		destinationFilename = s.randomStringProvider.RandomString(64) + ext
		destinationFilePath = fmt.Sprintf("%s/%s", s.submissionsDir, destinationFilename)
		if !utils.FileExists(destinationFilePath) {
			break
		}
	}

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		return nil, nil, 0, err
	}
	defer destination.Close()

	utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	md5sum := md5.New()
	sha256sum := sha256.New()
	multiWriter := io.MultiWriter(destination, sha256sum, md5sum)

	nBytes, err := io.Copy(multiWriter, file)
	if err != nil {
		return &destinationFilePath, nil, 0, err
	}
	if nBytes != fileHeader.Size() {
		return &destinationFilePath, nil, 0, fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("sending the submission for validation...")
	vr, err := s.validator.Validate(ctx, destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, 0, perr(fmt.Sprintf("validator bot: %s", err.Error()), http.StatusInternalServerError)
	}

	// FIXME remove this lazy solution to prevent database deadlocks and fix it properly
	s.submissionReceiverMutex.Lock()
	defer s.submissionReceiverMutex.Unlock()
	utils.LogCtx(ctx).Debug("storing submission...")

	var submissionID int64
	isSubmissionNew := true

	if sid == nil {
		submissionID, err = s.dal.StoreSubmission(dbs, submissionLevel)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			return &destinationFilePath, nil, 0, dberr(err)
		}
	} else {
		submissionID = *sid
		isSubmissionNew = false
	}

	// send notification about new file uploaded
	if !isSubmissionNew {
		if err := s.createNotification(dbs, uid, submissionID, constants.ActionUpload); err != nil {
			return &destinationFilePath, nil, 0, err
		}
	}

	if err := s.dal.SubscribeUserToSubmission(dbs, uid, submissionID); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, 0, dberr(err)
	}

	sf := &types.SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      uid,
		OriginalFilename: fileHeader.Filename(),
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size(),
		UploadedAt:       s.clock.Now(),
		MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
		SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
	}

	fid, err := s.dal.StoreSubmissionFile(dbs, sf)
	if err != nil {
		me, ok := err.(*mysql.MySQLError)
		if ok {
			if me.Number == 1062 {
				return &destinationFilePath, nil, 0, perr(fmt.Sprintf("file '%s' with checksums md5:%s sha256:%s already present in the DB", fileHeader.Filename(), sf.MD5Sum, sf.SHA256Sum), http.StatusConflict)
			}
		}
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, 0, dberr(err)
	}

	utils.LogCtx(ctx).Debug("storing submission comment...")

	c := &types.Comment{
		AuthorID:     uid,
		SubmissionID: submissionID,
		Message:      nil,
		Action:       constants.ActionUpload,
		CreatedAt:    s.clock.Now(),
	}

	if err := s.dal.StoreComment(dbs, c); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, 0, dberr(err)
	}

	utils.LogCtx(ctx).Debug("processing curation meta...")

	if vr.IsExtreme {
		yes := "Yes"
		vr.Meta.Extreme = &yes
	} else {
		no := "No"
		vr.Meta.Extreme = &no
	}

	vr.Meta.SubmissionID = submissionID
	vr.Meta.SubmissionFileID = fid

	if err := s.dal.StoreCurationMeta(dbs, &vr.Meta); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, 0, dberr(err)
	}

	// feed the curation feed
	isCurationValid := len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) == 0
	if err := s.createCurationFeedMessage(dbs, uid, submissionID, isSubmissionNew, isCurationValid, &vr.Meta); err != nil {
		return &destinationFilePath, nil, 0, dberr(err)
	}

	// save images
	imageFilePaths := make([]string, 0, len(vr.Images))
	for _, image := range vr.Images {
		imageData, err := base64.StdEncoding.DecodeString(image.Data)
		if err != nil {
			return &destinationFilePath, imageFilePaths, 0, err
		}

		var imageFilename string
		var imageFilenameFilePath string
		for {
			imageFilename = s.randomStringProvider.RandomString(64)
			imageFilenameFilePath = fmt.Sprintf("%s/%s", s.submissionImagesDir, imageFilename)
			if !utils.FileExists(imageFilenameFilePath) {
				break
			}
		}

		imageFilePaths = append(imageFilePaths, imageFilenameFilePath)

		if err := ioutil.WriteFile(imageFilenameFilePath, imageData, 0644); err != nil {
			return &destinationFilePath, imageFilePaths, 0, err
		}

		ci := &types.CurationImage{
			SubmissionFileID: fid,
			Type:             image.Type,
			Filename:         imageFilename,
		}

		if _, err := s.dal.StoreCurationImage(dbs, ci); err != nil {
			utils.LogCtx(ctx).Error(err)
			return &destinationFilePath, imageFilePaths, 0, dberr(err)
		}
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := s.convertValidatorResponseToComment(vr)
	if err := s.dal.StoreComment(dbs, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, imageFilePaths, 0, dberr(err)
	}

	if err := s.dal.UpdateSubmissionCacheTable(dbs, submissionID); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, imageFilePaths, 0, dberr(err)
	}

	return &destinationFilePath, imageFilePaths, submissionID, nil
}

// convertValidatorResponseToComment produces appropriate comment based on validator response
func (s *SiteService) convertValidatorResponseToComment(vr *types.ValidatorResponse) *types.Comment {
	c := &types.Comment{
		AuthorID:     constants.ValidatorID,
		SubmissionID: vr.Meta.SubmissionID,
		CreatedAt:    s.clock.Now().Add(time.Second),
	}

	approvalMessage := "Looks good to me! 🤖"
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
