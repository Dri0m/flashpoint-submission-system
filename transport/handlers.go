package transport

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"strings"
)

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

func (a *App) HandleSubmissionReceiver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params[constants.ResourceKeySubmissionID]

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

	// limit RAM usage to 10MB
	if err := r.ParseMultipartForm(10 * 1000 * 1000); err != nil {
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, perr("failed to parse form", http.StatusInternalServerError))
		return
	}

	fileHeaders := r.MultipartForm.File["files"]

	if len(fileHeaders) == 0 {
		err := fmt.Errorf("no files received")
		utils.LogCtx(ctx).Error(err)
		writeError(ctx, w, constants.PublicError{Msg: err.Error(), Status: http.StatusBadRequest})
		return
	}

	fileWrappers := make([]service.MultipartFileProvider, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		fileWrappers = append(fileWrappers, service.NewMutlipartFileWrapper(fileHeader))
	}

	sids, err := a.Service.ReceiveSubmissions(ctx, sid, fileWrappers)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	url := fmt.Sprintf("/submission/%d", sids[0])

	resp := types.ReceiveFileResp{
		Message: "success",
		URL:     &url,
	}

	writeResponse(ctx, w, resp, http.StatusOK)
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
	sid, err = a.Service.ReceiveSubmissionChunk(ctx, sid, resumableParams, chunk)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	var url *string
	if sid != nil {
		x := fmt.Sprintf("/submission/%d", *sid)
		url = &x
	}

	resp := types.ReceiveFileResp{
		Message: "success",
		URL:     url,
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

	a.RenderTemplates(ctx, w, r, pageData, "templates/flashfreeze.gohtml")
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

	err := a.Service.UpdateNotificationSettings(ctx, uid, notificationSettings.NotificationActions)
	if err != nil {
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

	err = a.Service.UpdateSubscriptionSettings(ctx, uid, sid, subscriptionSettings.Subscribe)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp("success", http.StatusOK), http.StatusOK)
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

func (a *App) HandleUpdateMasterDB(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := a.Service.UpdateMasterDB(ctx)
	if err != nil {
		writeError(ctx, w, err)
		return
	}

	writeResponse(ctx, w, presp("success", http.StatusOK), http.StatusOK)
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
	if err := r.ParseMultipartForm(10 * 1000 * 1000); err != nil {
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

func (a *App) HandleSearchFlasfhreezePage(w http.ResponseWriter, r *http.Request) {
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

func (a *App) HandleIngestFlashfreeze(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	writeResponse(ctx, w, presp("not implemented", http.StatusInternalServerError), http.StatusInternalServerError)
}
