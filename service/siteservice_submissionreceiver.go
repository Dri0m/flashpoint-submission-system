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
	"golang.org/x/sync/errgroup"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *SiteService) processReceivedSubmission(ctx context.Context, dbs database.DBSession, fileReadCloserProvider ReadCloserProvider, filename string, filesize int64, sid *int64, submissionLevel string) (*string, []string, int64, error) {
	uid := utils.UserID(ctx)
	if uid == 0 {
		utils.LogCtx(ctx).Panic("no user associated with request")
	}

	var err error

	utils.LogCtx(ctx).Debugf("received a file '%s' - %d bytes", filename, filesize)

	if err := os.MkdirAll(s.submissionsDir, os.ModeDir); err != nil {
		return nil, nil, 0, err
	}
	if err := os.MkdirAll(s.submissionImagesDir, os.ModeDir); err != nil {
		return nil, nil, 0, err
	}

	ext := filepath.Ext(filename)

	if ext != ".7z" && ext != ".zip" {
		return nil, nil, 0, perr("unsupported file extension", http.StatusUnsupportedMediaType)
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

	errs, ectx := errgroup.WithContext(ctx)

	md5sum := md5.New()
	sha256sum := sha256.New()

	errs.Go(func() error {
		utils.LogCtx(ectx).Debug("processing submission file in goroutine...")

		var err error

		readCloser, err := fileReadCloserProvider.GetReadCloser()
		if err != nil {
			return err
		}
		defer readCloser.Close()

		destination, err := os.Create(destinationFilePath)
		if err != nil {
			return err
		}
		defer destination.Close()

		utils.LogCtx(ctx).Debugf("copying submission file to '%s'...", destinationFilePath)

		multiWriter := io.MultiWriter(destination, sha256sum, md5sum)

		nBytes, err := io.Copy(multiWriter, readCloser)
		if err != nil {
			return err
		}
		if nBytes != filesize {
			return fmt.Errorf("incorrect number of bytes copied to destination")
		}

		return nil
	})

	var vr *types.ValidatorResponse
	var msg *string

	errs.Go(func() error {
		utils.LogCtx(ectx).Debug("sending the submission for validation in goroutine...")

		var err error

		readCloser, err := fileReadCloserProvider.GetReadCloser()
		if err != nil {
			return err
		}
		defer readCloser.Close()

		vr, err = s.validator.Validate(ectx, readCloser, destinationFilename)
		if err != nil {
			utils.LogCtx(ectx).Error(err)
			return perr(fmt.Sprintf("validator bot: %s", err.Error()), http.StatusInternalServerError)
		}

		utils.LogCtx(ectx).Debug("computing similarity in goroutine...")
		msg, err = s.computeSimilarityComment(dbs, sid, &vr.Meta)
		if err != nil {
			utils.LogCtx(ectx).Error(err)
			return err
		}

		return nil
	})

	if err := errs.Wait(); err != nil {
		return &destinationFilePath, nil, 0, err
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

	// subscribe the author
	if err := s.dal.SubscribeUserToSubmission(dbs, uid, submissionID); err != nil {
		utils.LogCtx(ctx).Error(err)
		return &destinationFilePath, nil, 0, dberr(err)
	}

	isAudition := submissionLevel == constants.SubmissionLevelAudition

	// also subscribe all those that want to subscribe to new audition uploads
	if isSubmissionNew && isAudition {
		auditionSubscribeUserIDs, err := s.dal.GetUsersForUniversalNotification(dbs, uid, constants.ActionAuditionSubscribe)
		if err != nil {
			utils.LogCtx(dbs.Ctx()).Error(err)
			return &destinationFilePath, nil, 0, dberr(err)
		}

		for _, subUID := range auditionSubscribeUserIDs {
			if err := s.dal.SubscribeUserToSubmission(dbs, subUID, submissionID); err != nil {
				utils.LogCtx(ctx).Error(err)
				return &destinationFilePath, nil, 0, dberr(err)
			}
		}
	}

	sf := &types.SubmissionFile{
		SubmissionID:     submissionID,
		SubmitterID:      uid,
		OriginalFilename: filename,
		CurrentFilename:  destinationFilename,
		Size:             filesize,
		UploadedAt:       s.clock.Now(),
		MD5Sum:           hex.EncodeToString(md5sum.Sum(nil)),
		SHA256Sum:        hex.EncodeToString(sha256sum.Sum(nil)),
	}

	fid, err := s.dal.StoreSubmissionFile(dbs, sf)
	if err != nil {
		me, ok := err.(*mysql.MySQLError)
		if ok {
			if me.Number == 1062 {
				return &destinationFilePath, nil, 0, perr(fmt.Sprintf("file '%s' with checksums md5:%s sha256:%s already present in the DB", filename, sf.MD5Sum, sf.SHA256Sum), http.StatusConflict)
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
	if err := s.createCurationFeedMessage(dbs, uid, submissionID, isSubmissionNew, isCurationValid, &vr.Meta, isAudition); err != nil {
		return &destinationFilePath, nil, 0, dberr(err)
	}

	errs, ectx = errgroup.WithContext(ctx)

	// save images
	imageFilePaths := make([]string, 0, len(vr.Images))
	cis := make([]*types.CurationImage, 0, len(vr.Images))

	errs.Go(func() error {
		utils.LogCtx(ectx).Debug("processing meta images in goroutine")
		for _, image := range vr.Images {
			imageData, err := base64.StdEncoding.DecodeString(image.Data)
			if err != nil {
				return err
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
				return err
			}

			ci := &types.CurationImage{
				SubmissionFileID: fid,
				Type:             image.Type,
				Filename:         imageFilename,
			}

			cis = append(cis, ci)
		}
		utils.LogCtx(ectx).Debug("meta images processed in goroutine")
		return nil
	})

	if err := errs.Wait(); err != nil {
		return &destinationFilePath, imageFilePaths, 0, err
	}

	for _, ci := range cis {
		if _, err := s.dal.StoreCurationImage(dbs, ci); err != nil {
			utils.LogCtx(ectx).Error(err)
			return &destinationFilePath, imageFilePaths, 0, dberr(err)
		}
	}

	var sc *types.Comment

	if len(*msg) > 0 {
		sc = &types.Comment{
			AuthorID:     constants.SystemID,
			SubmissionID: submissionID,
			Message:      msg,
			Action:       constants.ActionSystem,
			CreatedAt:    s.clock.Now().Add(time.Second * 2),
		}
	} else {
		utils.LogCtx(dbs.Ctx()).Debug("no similar curations found")
	}

	if sc != nil {
		if err := s.dal.StoreComment(dbs, sc); err != nil {
			utils.LogCtx(ectx).Error(err)
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

func (s *SiteService) computeSimilarityComment(dbs database.DBSession, sid *int64, meta *types.CurationMeta) (*string, error) {
	titleSimilarities, launchCommandSimilarities, err := s.getSimilarityScores(dbs, 0.9, meta.Title, meta.LaunchCommand)
	if err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
		return nil, dberr(err)
	}

	var sb strings.Builder

	if len(titleSimilarities) > 1 || len(launchCommandSimilarities) > 1 {

		strID := ""
		if sid != nil {
			strID = strconv.FormatInt(*sid, 10)
		}

		if len(titleSimilarities) > 1 {
			sb.Write([]byte("Curations with similar titles have been found:\n"))

			for _, ts := range titleSimilarities {
				if sid != nil && ts.ID == strID {
					continue
				}
				sb.Write([]byte(fmt.Sprintf("(%.1f%%) ID %s - Title - '%s'\n", ts.TitleRatio*100, ts.ID, *ts.Title)))
			}
		}

		if len(launchCommandSimilarities) > 1 {
			sb.Write([]byte("\n"))
			sb.Write([]byte("Curations with similar launch commands have been found:\n"))

			for _, ts := range launchCommandSimilarities {
				if sid != nil && ts.ID == strID {
					continue
				}
				sb.Write([]byte(fmt.Sprintf("(%.1f%%) ID %s - Launch Command - '%s'\n", ts.LaunchCommandRatio*100, ts.ID, *ts.LaunchCommand)))
			}
		}

		sb.Write([]byte("\n"))
		sb.Write([]byte("This could mean that your submission is a duplicate."))
	}

	msg := sb.String()

	if len(msg) == 0 {
		utils.LogCtx(dbs.Ctx()).Debug("no similar curations found")
	}

	return &msg, nil
}
