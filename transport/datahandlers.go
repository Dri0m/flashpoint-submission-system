package transport

import (
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (a *App) HandleDownloadSubmissionFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionFileID := params[constants.ResourceKeyFileID]

	sfid, err := strconv.ParseInt(submissionFileID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission file id", http.StatusBadRequest))
		return
	}

	sfs, err := a.Service.GetSubmissionFiles(ctx, []int64{sfid})
	if err != nil {
		writeError(ctx, w, err)
		return
	}
	sf := sfs[0]

	f, err := os.Open(fmt.Sprintf("%s/%s", a.Conf.SubmissionsDirFullPath, sf.CurrentFilename))

	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to read file", http.StatusInternalServerError))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sf.CurrentFilename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, sf.CurrentFilename, sf.UploadedAt, f)
}

func (a *App) HandleDownloadSubmissionBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionFileIDs := strings.Split(params[constants.ResourceKeyFileIDs], ",")
	sfids := make([]int64, 0, len(submissionFileIDs))

	for _, submissionFileID := range submissionFileIDs {
		sfid, err := strconv.ParseInt(submissionFileID, 10, 64)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, perr("invalid submission file id", http.StatusBadRequest))
			return
		}
		sfids = append(sfids, sfid)
	}

	sfs, err := a.Service.GetSubmissionFiles(ctx, sfids)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	filePaths := make([]string, 0, len(sfs))

	for _, sf := range sfs {
		filePaths = append(filePaths, fmt.Sprintf("%s/%s", a.Conf.SubmissionsDirFullPath, sf.CurrentFilename))
	}

	filename := fmt.Sprintf("fpfss-batch-%dfiles-%s.tar", len(sfs), utils.NewRealRandomStringProvider().RandomString(16))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	if err := utils.WriteTarball(w, filePaths); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to create tarball", http.StatusInternalServerError))
		return
	}
}

func (a *App) HandleDownloadCurationImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	curationImageID := params[constants.ResourceKeyCurationImageID]

	ciid, err := strconv.ParseInt(curationImageID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid curation image id", http.StatusBadRequest))
		return
	}

	ci, err := a.Service.GetCurationImage(ctx, ciid)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	f, err := os.Open(fmt.Sprintf("%s/%s", a.Conf.SubmissionImagesDirFullPath, ci.Filename))
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to read file", http.StatusInternalServerError))
		return
	}

	fi, err := f.Stat()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to read file", http.StatusInternalServerError))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "image")
	http.ServeContent(w, r, ci.Filename, fi.ModTime(), f)
}

func (a *App) HandleDownloadFlashfreezeRootFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	fileID := params[constants.ResourceKeyFlashfreezeRootFileID]

	fid, err := strconv.ParseInt(fileID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid root file id", http.StatusBadRequest))
		return
	}

	ci, err := a.Service.GetFlashfreezeRootFile(ctx, fid)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	f, err := os.Open(fmt.Sprintf("%s/%s", a.Conf.FlashfreezeDirFullPath, ci.CurrentFilename))
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to read file", http.StatusInternalServerError))
		return
	}

	fi, err := f.Stat()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to read file", http.StatusInternalServerError))
		return
	}
	defer f.Close()

	filename := fmt.Sprintf("flashfreeze-%d-%s", fid, ci.CurrentFilename)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, filename, fi.ModTime(), f)
}

func (a *App) HandleDownloadFixesFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	fixFileID := params[constants.ResourceKeyFixFileID]

	ffid, err := strconv.ParseInt(fixFileID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid fixes file id", http.StatusBadRequest))
		return
	}

	ffs, err := a.Service.GetFixesFiles(ctx, []int64{ffid})
	if err != nil {
		writeError(ctx, w, err)
		return
	}
	ff := ffs[0]

	f, err := os.Open(fmt.Sprintf("%s/%s", a.Conf.FixesDirFullPath, ff.CurrentFilename))

	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to read file", http.StatusInternalServerError))
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", ff.CurrentFilename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, ff.CurrentFilename, ff.UploadedAt, f)
}
