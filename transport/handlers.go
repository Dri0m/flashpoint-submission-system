package transport

import (
	"context"
	"fmt"
	"github.com/kofalt/go-memoize"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/gorilla/mux"
)

var epoch = time.Unix(0, 0).Format(time.RFC1123)

var noCacheHeaders = map[string]string{
	"Expires":         epoch,
	"Cache-Control":   "no-cache, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}

var etagHeaders = []string{
	"ETag",
	"If-Modified-Since",
	"If-Match",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
}

func NoCache(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Delete any ETag headers that may have been set
		for _, v := range etagHeaders {
			if r.Header.Get(v) != "" {
				r.Header.Del(v)
			}
		}

		// Set our NoCache headers
		for k, v := range noCacheHeaders {
			w.Header().Set(k, v)
		}

		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

var pageDataCache = memoize.NewMemoizer(24*time.Hour, 48*time.Hour)

func (a *App) HandleCommentReceiverBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)

	params := mux.Vars(r)
	submissionIDs := strings.Split(params["submission-ids"], ",")
	sids := make([]int64, 0, len(submissionIDs))

	for _, submissionFileID := range submissionIDs {
		sid, err := strconv.ParseInt(submissionFileID, 10, 64)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
			return
		}
		sids = append(sids, sid)
	}

	if err := r.ParseForm(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse form", http.StatusBadRequest))
		return
	}

	// TODO use gorilla/schema
	formAction := r.FormValue("action")
	formMessage := r.FormValue("message")
	formIgnoreDupeActions := r.FormValue("ignore-duplicate-actions")

	if len([]rune(formMessage)) > 20000 {
		err := fmt.Errorf("message cannot be longer than 20000 characters")
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, constants.PublicError{Msg: err.Error(), Status: http.StatusBadRequest})
		return
	}

	if err := a.Service.ReceiveComments(ctx, uid, sids, formAction, formMessage, formIgnoreDupeActions); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp("success", http.StatusOK), http.StatusOK)
}

func (a *App) HandleSoftDeleteSubmissionFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionFileID := params[constants.ResourceKeyFileID]

	sfid, err := strconv.ParseInt(submissionFileID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission file id", http.StatusBadRequest))
		return
	}

	if err := r.ParseForm(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse form", http.StatusBadRequest))
		return
	}

	deleteReason := r.FormValue("reason")
	if len(deleteReason) < 3 {
		writeError(ctx, w, perr("reason must be at least 3 characters long", http.StatusBadRequest))
		return
	} else if len(deleteReason) > 255 {
		writeError(ctx, w, perr("reason cannot be longer than 255 characters", http.StatusBadRequest))
		return
	}

	if err := a.Service.SoftDeleteSubmissionFile(ctx, sfid, deleteReason); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, nil, http.StatusNoContent)
}

func (a *App) HandleSoftDeleteSubmission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
		return
	}

	deleteReason := r.FormValue("reason")
	if len(deleteReason) < 3 {
		writeError(ctx, w, perr("reason must be at least 3 characters long", http.StatusBadRequest))
		return
	} else if len(deleteReason) > 255 {
		writeError(ctx, w, perr("reason cannot be longer than 255 characters", http.StatusBadRequest))
		return
	}

	if err := a.Service.SoftDeleteSubmission(ctx, sid, deleteReason); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, nil, http.StatusNoContent)
}

func (a *App) HandleSoftDeleteComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	commentID := params[constants.ResourceKeyCommentID]

	cid, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid comment id", http.StatusBadRequest))
		return
	}

	deleteReason := r.FormValue("reason")
	if len(deleteReason) < 3 {
		writeError(ctx, w, perr("reason must be at least 3 characters long", http.StatusBadRequest))
		return
	} else if len(deleteReason) > 255 {
		writeError(ctx, w, perr("reason cannot be longer than 255 characters", http.StatusBadRequest))
		return
	}

	if err := a.Service.SoftDeleteComment(ctx, cid, deleteReason); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, nil, http.StatusNoContent)
}

func (a *App) HandleOverrideBot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
		return
	}

	if err := a.Service.OverrideBot(ctx, sid); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, nil, http.StatusNoContent)
}

func (a *App) HandleSubmissionReceiverResumable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

	// get submission ID
	var sid *int64
	if submissionID != "" {
		sidParsed, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
			return
		}
		sid = &sidParsed
	}

	chunk, resumableParams, err := a.parseResumableRequest(ctx, r)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	// then a magic happens
	tn, err := a.Service.ReceiveSubmissionChunk(ctx, sid, resumableParams, chunk)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	resp := types.ReceiveFileTempNameResp{
		Message:  "success",
		TempName: tn,
	}

	writeResponse(ctx, w, resp, http.StatusOK)
}

func (a *App) HandleReceiverResumableTestChunk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// parse resumable params
	resumableParams := &types.ResumableParams{}

	if err := a.decoder.Decode(resumableParams, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode resumable query params", http.StatusInternalServerError))
		return
	}

	// then a magic happens
	alreadyReceived, err := a.Service.IsChunkReceived(ctx, resumableParams)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if !alreadyReceived {
		writeResponse(ctx, w, nil, http.StatusNotFound)
		return
	}

	writeResponse(ctx, w, nil, http.StatusOK)
}

func (a *App) HandleRootPage(w http.ResponseWriter, r *http.Request) {
	uid, err := a.GetUserIDFromCookie(r)
	ctx := r.Context()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.UnsetCookie(w, utils.Cookies.Login)
		http.Redirect(w, r, "/web", http.StatusFound)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.UserID, uid))
	ctx = r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.UnsetCookie(w, utils.Cookies.Login)
		http.Redirect(w, r, "/web", http.StatusFound)
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/root.gohtml")
}

func (a *App) HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)

	pageData, err := a.Service.GetProfilePageData(ctx, uid)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/profile.gohtml")
}

func (a *App) HandleSubmitPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submit.gohtml")
}

func (a *App) HandleFlashfreezeSubmitPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/flashfreeze-submit.gohtml")
}

func (a *App) HandleFixesSubmitPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/fixes-submit.gohtml")
}

func (a *App) HandleFixesSubmitGenericPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/fixes-submit-generic.gohtml")
}

func (a *App) HandleFixesSubmitGenericPageUploadFilesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	fixID := params[constants.ResourceKeyFixID]

	// get fix ID
	var fid *int64
	if fixID != "" {
		sidParsed, err := strconv.ParseInt(fixID, 10, 64)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, perr("invalid fix id", http.StatusBadRequest))
			return
		}
		fid = &sidParsed
	}

	bpd, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	pageData := types.SubmitFixesFilesPageData{
		BasePageData: *bpd,
		FixID:        *fid,
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/fixes-submit-generic-upload-files.gohtml")
}

func (a *App) HandleSubmissionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := &types.SubmissionsFilter{}

	if err := a.decoder.Decode(filter, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := filter.Validate(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(err.Error(), http.StatusBadRequest))
		return
	}

	pageData, err := a.Service.GetSubmissionsPageData(ctx, filter)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	pageData.FilterLayout = r.FormValue("filter-layout")

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/submissions.gohtml",
		"templates/submission-filter.gohtml",
		"templates/submission-table.gohtml",
		"templates/submission-pagenav.gohtml",
		"templates/submission-filter-chunks.gohtml",
		"templates/comment-form.gohtml")
}

func (a *App) HandleMySubmissionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)

	filter := &types.SubmissionsFilter{}

	if err := a.decoder.Decode(filter, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := filter.Validate(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, err)
		return
	}

	filter.SubmitterID = &uid

	pageData, err := a.Service.GetSubmissionsPageData(ctx, filter)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	pageData.FilterLayout = r.FormValue("filter-layout")

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/my-submissions.gohtml",
		"templates/submission-filter.gohtml",
		"templates/submission-table.gohtml",
		"templates/submission-pagenav.gohtml",
		"templates/submission-filter-chunks.gohtml",
		"templates/comment-form.gohtml")
}

func (a *App) HandleViewSubmissionPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)
	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
		return
	}

	pageData, err := a.Service.GetViewSubmissionPageData(ctx, uid, sid)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/submission.gohtml",
		"templates/submission-table.gohtml",
		"templates/comment-form.gohtml",
		"templates/view-submission-nav.gohtml")
}

func (a *App) HandleViewSubmissionFilesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
		return
	}

	pageData, err := a.Service.GetSubmissionsFilesPageData(ctx, sid)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submission-files.gohtml", "templates/submission-files-table.gohtml")
}

func (a *App) HandleUpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)

	notificationSettings := &types.UpdateNotificationSettings{}

	if err := a.decoder.Decode(notificationSettings, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := a.Service.UpdateNotificationSettings(ctx, uid, notificationSettings.NotificationActions); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp("success", http.StatusOK), http.StatusOK)
}

func (a *App) HandleUpdateSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)
	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
		return
	}

	subscriptionSettings := &types.UpdateSubscriptionSettings{}

	if err := a.decoder.Decode(subscriptionSettings, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := a.Service.UpdateSubscriptionSettings(ctx, uid, sid, subscriptionSettings.Subscribe); err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp("success", http.StatusOK), http.StatusOK)
}

func (a *App) HandleReceiveFixesSubmitGeneric(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserID(ctx)

	createFixFirstStep := &types.CreateFixFirstStep{}

	if err := r.ParseForm(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse form", http.StatusBadRequest))
		return
	}

	if err := a.decoder.Decode(createFixFirstStep, r.PostForm); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := createFixFirstStep.Validate(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(err.Error(), http.StatusBadRequest))
		return
	}

	fid, err := a.Service.CreateFixFirstStep(ctx, uid, createFixFirstStep)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/web/fixes/submit/generic/%d", fid), http.StatusFound)
}

func (a *App) HandleInternalPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, err)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/internal.gohtml")
}

// TODO create a closure function thingy to handle this automatically? already 3+ guards like these hang around the code
var updateMasterDBGuard = make(chan struct{}, 1)

func (a *App) HandleUpdateMasterDB(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	select {
	case updateMasterDBGuard <- struct{}{}:
		utils.LogCtx(ctx).Debug("starting update master db")
	default:
		writeResponse(ctx, w, presp("update master db already running", http.StatusForbidden), http.StatusForbidden)
		return
	}

	go func() {
		err := a.Service.UpdateMasterDB(context.WithValue(context.Background(), utils.CtxKeys.Log, utils.LogCtx(ctx)))
		if err != nil {
			utils.LogCtx(ctx).Error(err)
		}
		<-updateMasterDBGuard
	}()

	writeResponse(ctx, w, presp("starting update master db", http.StatusOK), http.StatusOK)
}

func (a *App) HandleHelpPage(w http.ResponseWriter, r *http.Request) {
	// TODO all auth-free pages should use a middleware to remove all of this user ID handling from the handlers
	ctx := r.Context()
	uid, err := a.GetUserIDFromCookie(r)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		utils.UnsetCookie(w, utils.Cookies.Login)
		http.Redirect(w, r, "/web", http.StatusFound)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.UserID, uid))
	ctx = r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.UnsetCookie(w, utils.Cookies.Login)
		http.Redirect(w, r, "/web", http.StatusFound)
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/help.gohtml")
}

func (a *App) parseResumableRequest(ctx context.Context, r *http.Request) ([]byte, *types.ResumableParams, error) {
	// parse resumable params
	resumableParams := &types.ResumableParams{}

	if err := a.decoder.Decode(resumableParams, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, perr("failed to decode resumable query params", http.StatusBadRequest)
	}

	// get chunk data
	if err := r.ParseMultipartForm(64 * 1000 * 1000); err != nil {
		utils.LogCtx(ctx).Error(err)
		return nil, nil, perr("failed to parse form", http.StatusUnprocessableEntity)
	}

	fileHeaders := r.MultipartForm.File["file"]

	if len(fileHeaders) == 0 {
		err := fmt.Errorf("no files received")
		utils.LogCtx(ctx).Error(err)
		return nil, nil, perr(err.Error(), http.StatusBadRequest)
	}

	file, err := fileHeaders[0].Open()
	if err != nil {
		return nil, nil, perr("failed to open received file", http.StatusInternalServerError)
	}
	defer file.Close()

	utils.LogCtx(ctx).Debug("reading received chunk")

	chunk := make([]byte, resumableParams.ResumableCurrentChunkSize)
	n, err := file.Read(chunk)
	if err != nil || int64(n) != resumableParams.ResumableCurrentChunkSize {
		return nil, nil, perr("failed to read received file", http.StatusUnprocessableEntity)
	}

	return chunk, resumableParams, nil
}

func (a *App) HandleFlashfreezeReceiverResumable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chunk, resumableParams, err := a.parseResumableRequest(ctx, r)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	// then a magic happens
	fid, err := a.Service.ReceiveFlashfreezeChunk(ctx, resumableParams, chunk)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	var url *string
	if fid != nil {
		x := fmt.Sprintf("/flashfreeze/files?file-id=%d", *fid)
		url = &x
	}

	resp := types.ReceiveFileResp{
		Message: "success",
		URL:     url,
	}
	writeResponse(ctx, w, resp, http.StatusOK)
}

func (a *App) HandleSearchFlashfreezePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := &types.FlashfreezeFilter{}

	if err := a.decoder.Decode(filter, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := filter.Validate(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(err.Error(), http.StatusBadRequest))
		return
	}

	pageData, err := a.Service.GetSearchFlashfreezeData(ctx, filter)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/flashfreeze-files.gohtml",
		"templates/flashfreeze-table.gohtml",
		"templates/flashfreeze-filter.gohtml",
		"templates/flashfreeze-pagenav.gohtml")
}

var ingestGuard = make(chan struct{}, 1)

func (a *App) HandleIngestFlashfreeze(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	select {
	case ingestGuard <- struct{}{}:
		utils.LogCtx(ctx).Debug("starting flashfreeze ingestion")
	default:
		writeResponse(ctx, w, presp("ingestion already running", http.StatusForbidden), http.StatusForbidden)
		return
	}

	go func() {
		a.Service.IngestFlashfreezeItems(utils.LogCtx(context.WithValue(context.Background(), utils.CtxKeys.Log, utils.LogCtx(ctx))))
		<-ingestGuard
	}()

	writeResponse(ctx, w, presp("starting flashfreeze ingestion", http.StatusOK), http.StatusOK)
}

var recomputeSubmissionCacheAllGuard = make(chan struct{}, 1)

func (a *App) HandleRecomputeSubmissionCacheAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	select {
	case recomputeSubmissionCacheAllGuard <- struct{}{}:
		utils.LogCtx(ctx).Debug("starting recompute submission cache all")
	default:
		writeResponse(ctx, w, presp("recompute submission cache all already running", http.StatusForbidden), http.StatusForbidden)
		return
	}

	go func() {
		a.Service.RecomputeSubmissionCacheAll(context.WithValue(context.Background(), utils.CtxKeys.Log, utils.LogCtx(ctx)))
		<-recomputeSubmissionCacheAllGuard
	}()

	writeResponse(ctx, w, presp("starting recompute submission cache all", http.StatusOK), http.StatusOK)
}

var ingestUnknownGuard = make(chan struct{}, 1)

// HandleIngestUnknownFlashfreeze ingests flashfreeze files which are in the flashfreeze directory, but not in the database.
// This should not be needed and such files are a result of a bug or human error.
func (a *App) HandleIngestUnknownFlashfreeze(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	select {
	case ingestUnknownGuard <- struct{}{}:
		utils.LogCtx(ctx).Debug("starting flashfreeze ingestion of unknown files")
	default:
		writeResponse(ctx, w, presp("ingestion already running", http.StatusForbidden), http.StatusForbidden)
		return
	}

	go func() {
		a.Service.IngestUnknownFlashfreezeItems(utils.LogCtx(context.WithValue(context.Background(), utils.CtxKeys.Log, utils.LogCtx(ctx))))
		<-ingestUnknownGuard
	}()

	writeResponse(ctx, w, presp("starting flashfreeze ingestion of unknown files", http.StatusOK), http.StatusOK)
}

var indexUnindexedGuard = make(chan struct{}, 1)

func (a *App) HandleIndexUnindexedFlashfreeze(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	select {
	case indexUnindexedGuard <- struct{}{}:
		utils.LogCtx(ctx).Debug("starting flashfreeze indexing of unindexed files")
	default:
		writeResponse(ctx, w, presp("indexing already running", http.StatusForbidden), http.StatusForbidden)
		return
	}

	go func() {
		a.Service.IndexUnindexedFlashfreezeItems(utils.LogCtx(context.WithValue(context.Background(), utils.CtxKeys.Log, utils.LogCtx(ctx))))
		<-indexUnindexedGuard
	}()

	writeResponse(ctx, w, presp("starting flashfreeze indexing of unindexed files", http.StatusOK), http.StatusOK)
}

func (a *App) HandleDeleteUserSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse form", http.StatusBadRequest))
		return
	}

	req := &types.DeleteUserSessionsRequest{}

	if err := a.decoder.Decode(req, r.PostForm); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	count, err := a.Service.DeleteUserSessions(ctx, req.DiscordID)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp(fmt.Sprintf("deleted %d sessions", count), http.StatusOK), http.StatusOK)
}

func (a *App) HandleStatisticsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	f := func() (interface{}, error) {
		pageData, err := a.Service.GetStatisticsPageData(ctx)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, err)
			return nil, err
		}
		return pageData, nil
	}

	const key = "GetStatisticsPageData"

	pageDataI, err, cached := pageDataCache.Memoize(key, f)
	if err != nil {
		writeError(ctx, w, err)
		pageDataCache.Storage.Delete(key)
		return
	}

	pageData := pageDataI.(*types.StatisticsPageData)

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).Debug("getting statistics page data")

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/statistics.gohtml")
}

func (a *App) HandleUserStatisticsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/user-statistics.gohtml")
}

func (a *App) HandleSendRemindersAboutRequestedChanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	count, err := a.Service.ProduceRemindersAboutRequestedChanges(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp(fmt.Sprintf("%d notifications added to the queue", count), http.StatusOK), http.StatusOK)
}

func (a *App) HandleFixesReceiverResumable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	fixID := params[constants.ResourceKeyFixID]

	// get fix ID
	var fid *int64
	if fixID != "" {
		sidParsed, err := strconv.ParseInt(fixID, 10, 64)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, perr("invalid fix id", http.StatusBadRequest))
			return
		}
		fid = &sidParsed
	}

	chunk, resumableParams, err := a.parseResumableRequest(ctx, r)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	// then a magic happens
	err = a.Service.ReceiveFixesChunk(ctx, *fid, resumableParams, chunk)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	var url *string
	x := fmt.Sprintf("/TODO/files?file-id=%d", *fid)
	url = &x

	resp := types.ReceiveFileResp{
		Message: "success",
		URL:     url,
	}
	writeResponse(ctx, w, resp, http.StatusOK)
}

func (a *App) HandleSearchFixesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := &types.FixesFilter{}

	if err := a.decoder.Decode(filter, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to decode query params", http.StatusInternalServerError))
		return
	}

	if err := filter.Validate(); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr(err.Error(), http.StatusBadRequest))
		return
	}

	pageData, err := a.Service.GetSearchFixesData(ctx, filter)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/fixes.gohtml",
		"templates/fixes-table.gohtml",
		"templates/fixes-filter.gohtml",
		"templates/fixes-pagenav.gohtml")
}

func (a *App) HandleViewFixesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	fixID := params[constants.ResourceKeyFixID]

	fid, err := strconv.ParseInt(fixID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid fix id", http.StatusBadRequest))
		return
	}

	pageData, err := a.Service.GetViewFixPageData(ctx, fid)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	if utils.RequestType(ctx) != constants.RequestWeb {
		writeResponse(ctx, w, pageData, http.StatusOK)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData,
		"templates/fix.gohtml",
		"templates/fixes-table.gohtml")
}

func (a *App) HandleGetUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	users, err := a.Service.GetUsers(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, err)
		return
	}

	data := struct {
		Users []*types.User `json:"users"`
	}{
		users,
	}

	writeResponse(ctx, w, data, http.StatusOK)
}

func (a *App) HandleGetUserStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := mux.Vars(r)
	userID := params[constants.ResourceKeyUserID]

	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("invalid submission id", http.StatusBadRequest))
		return
	}

	f := func() (interface{}, error) {
		us, err := a.Service.GetUserStatistics(ctx, uid)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			writeError(ctx, w, err)
			return nil, err
		}
		return us, nil
	}

	key := fmt.Sprintf("GetUserStatistics-%d", uid)

	usI, err, cached := pageDataCache.Memoize(key, f)
	if err != nil {
		writeError(ctx, w, err)
		pageDataCache.Storage.Delete(key)
		return
	}

	us := usI.(*types.UserStatistics)

	utils.LogCtx(ctx).WithField("cached", utils.BoolToString(cached)).WithField("uid", uid).Debug("getting user statistics")

	writeResponse(ctx, w, us, http.StatusOK)
}

func (a *App) HandleGetUploadProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := mux.Vars(r)
	tempName := params[constants.ResourceKeyTempName]

	data := struct {
		Status *types.SubmissionStatus `json:"status"`
	}{
		a.Service.SSK.Get(tempName),
	}

	writeResponse(ctx, w, data, http.StatusOK)
}
