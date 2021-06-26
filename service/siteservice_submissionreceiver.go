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

func (s *SiteService) ReceiveSubmissions(ctx context.Context, sid *int64, fileProviders []MultipartFileProvider) error {
	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	dbs, err := s.dal.NewSession(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}
	defer dbs.Rollback()

	userRoles, err := s.dal.GetDiscordUserRoles(dbs, uid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return dberr(err)
	}

	if constants.IsInAudit(userRoles) && len(fileProviders) > 1 {
		return perr("cannot upload more than one submission at once when user is in audit", http.StatusForbidden)
	}

	if constants.IsInAudit(userRoles) && fileProviders[0].Size() > constants.UserInAuditSumbissionMaxFilesize {
		return perr("submission filesize limited to 200MB for users in audit", http.StatusForbidden)
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

	for _, fileProvider := range fileProviders {
		destinationFilename, ifp, err := s.processReceivedSubmission(ctx, dbs, fileProvider, sid, submissionLevel)

		if destinationFilename != nil {
			destinationFilenames = append(destinationFilenames, *destinationFilename)
		}
		for _, imageFilePath := range ifp {
			imageFilePaths = append(imageFilePaths, imageFilePath)
		}

		if err != nil {
			cleanup()
			return err
		}
	}

	if err := dbs.Commit(); err != nil {
		utils.LogCtx(ctx).Error(err)
		cleanup()
		return dberr(err)
	}

	s.announceNotification()

	return nil
}

func (s *SiteService) processReceivedSubmission(ctx context.Context, dbs database.DBSession, fileHeader MultipartFileProvider, sid *int64, submissionLevel string) (*string, []string, error) {
	uid := utils.UserIDFromContext(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes", fileHeader.Filename(), fileHeader.Size())

	if err := os.MkdirAll(s.submissionsDir, os.ModeDir); err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(s.submissionImagesDir, os.ModeDir); err != nil {
		return nil, nil, err
	}

	ext := filepath.Ext(fileHeader.Filename())

	if ext != ".7z" && ext != ".zip" {
		return nil, nil, perr("unsupported file extension", http.StatusBadRequest)
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
		return nil, nil, err
	}
	defer destination.Close()

	utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

	md5sum := md5.New()
	sha256sum := sha256.New()
	multiWriter := io.MultiWriter(destination, sha256sum, md5sum)

	nBytes, err := io.Copy(multiWriter, file)
	if err != nil {
		return &destinationFilePath, nil, err
	}
	if nBytes != fileHeader.Size() {
		return &destinationFilePath, nil, fmt.Errorf("incorrect number of bytes copied to destination")
	}

	utils.LogCtx(ctx).Debug("sending the submission for validation...")
	vr, err := s.validator.Validate(ctx, destinationFilePath)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, err
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
			return &destinationFilePath, nil, dberr(err)
		}
	} else {
		submissionID = *sid
		isSubmissionNew = false
	}

	// send notification about new file uploaded
	if !isSubmissionNew {
		if err := s.createNotification(dbs, uid, submissionID, constants.ActionUpload); err != nil {
			return &destinationFilePath, nil, err
		}
	}

	if err := s.dal.SubscribeUserToSubmission(dbs, uid, submissionID); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, dberr(err)
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
				return &destinationFilePath, nil, perr(fmt.Sprintf("file '%s' with checksums md5:%s sha256:%s already present in the DB", fileHeader.Filename(), sf.MD5Sum, sf.SHA256Sum), http.StatusConflict)
			}
		}
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, dberr(err)
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
		return &destinationFilePath, nil, dberr(err)
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
		return &destinationFilePath, nil, dberr(err)
	}

	// feed the curation feed
	isCurationValid := len(vr.CurationErrors) == 0 && len(vr.CurationWarnings) == 0
	if err := s.createCurationFeedMessage(dbs, uid, submissionID, isSubmissionNew, isCurationValid, &vr.Meta); err != nil {
		return &destinationFilePath, nil, dberr(err)
	}

	// save images
	imageFilePaths := make([]string, 0, len(vr.Images))
	for _, image := range vr.Images {
		imageData, err := base64.StdEncoding.DecodeString(image.Data)
		if err != nil {
			return &destinationFilePath, imageFilePaths, err
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
			return &destinationFilePath, imageFilePaths, err
		}

		ci := &types.CurationImage{
			SubmissionFileID: fid,
			Type:             image.Type,
			Filename:         imageFilename,
		}

		if _, err := s.dal.StoreCurationImage(dbs, ci); err != nil {
			utils.LogCtx(ctx).Error(err)
			return &destinationFilePath, imageFilePaths, dberr(err)
		}
	}

	utils.LogCtx(ctx).Debug("processing bot event...")

	bc := s.convertValidatorResponseToComment(vr)
	if err := s.dal.StoreComment(dbs, bc); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, imageFilePaths, dberr(err)
	}

	if err := s.dal.UpdateSubmissionCacheTable(dbs, submissionID); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, imageFilePaths, dberr(err)
	}

	return &destinationFilePath, imageFilePaths, nil
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
