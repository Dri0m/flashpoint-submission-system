package transport

import (
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"net/http"
)

func (a *App) handleRequests(l *logrus.Logger, srv *http.Server, router *mux.Router) {
	isStaff := func(r *http.Request, uid int64) (bool, error) {
		return a.UserHasAnyRole(r, uid, constants.StaffRoles())
	}
	isTrialCurator := func(r *http.Request, uid int64) (bool, error) {
		return a.UserHasAnyRole(r, uid, constants.TrialCuratorRoles())
	}
	isDeletor := func(r *http.Request, uid int64) (bool, error) {
		return a.UserHasAnyRole(r, uid, constants.DeletorRoles())
	}
	isInAudit := func(r *http.Request, uid int64) (bool, error) {
		s, err := a.UserHasAnyRole(r, uid, constants.StaffRoles())
		if err != nil {
			return false, err
		}
		t, err := a.UserHasAnyRole(r, uid, constants.TrialCuratorRoles())
		if err != nil {
			return false, err
		}
		return !(s || t), nil
	}
	userOwnsSubmission := func(r *http.Request, uid int64) (bool, error) {
		return a.UserOwnsResource(r, uid, constants.ResourceKeySubmissionID)
	}
	userOwnsAllSubmissions := func(r *http.Request, uid int64) (bool, error) {
		return a.UserOwnsResource(r, uid, constants.ResourceKeySubmissionIDs)
	}
	userHasNoSubmissions := func(r *http.Request, uid int64) (bool, error) {
		return a.IsUserWithinResourceLimit(r, uid, constants.ResourceKeySubmissionID, 1)
	}

	// oauth
	router.Handle("/auth", http.HandlerFunc(a.HandleDiscordAuth)).Methods("GET")
	router.Handle("/auth/callback", http.HandlerFunc(a.HandleDiscordCallback)).Methods("GET")

	// logout
	router.Handle("/logout", http.HandlerFunc(a.HandleLogout)).Methods("GET")

	// file server
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// pages
	router.Handle("/",
		http.HandlerFunc(a.HandleRootPage)).Methods("GET")

	router.Handle("/profile",
		http.HandlerFunc(a.UserAuthMux(a.HandleProfilePage))).Methods("GET")

	router.Handle("/submit",
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSubmitPage, muxAny(isStaff, isTrialCurator, isInAudit)))).Methods("GET")

	router.Handle("/submissions",
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSubmissionsPage, muxAny(isStaff)))).Methods("GET")

	router.Handle("/my-submissions",
		http.HandlerFunc(a.UserAuthMux(
			a.HandleMySubmissionsPage, muxAny(isStaff, isTrialCurator, isInAudit)))).Methods("GET")

	router.Handle(fmt.Sprintf("/submission/{%s}", constants.ResourceKeySubmissionID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleViewSubmissionPage,
			muxAny(isStaff,
				muxAll(isTrialCurator, userOwnsSubmission),
				muxAll(isInAudit, userOwnsSubmission))))).Methods("GET")

	router.Handle(fmt.Sprintf("/submission/{%s}/files", constants.ResourceKeySubmissionID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleViewSubmissionFilesPage,
			muxAny(isStaff,
				muxAll(isTrialCurator, userOwnsSubmission),
				muxAll(isInAudit, userOwnsSubmission))))).Methods("GET")

	// receivers
	router.Handle("/submission-receiver",
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSubmissionReceiver, muxAny(
				isStaff,
				isTrialCurator,
				muxAll(isInAudit, userHasNoSubmissions))))).Methods("POST")

	router.Handle(fmt.Sprintf("/submission-receiver/{%s}", constants.ResourceKeySubmissionID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSubmissionReceiver,
			muxAny(isStaff,
				muxAll(isTrialCurator, userOwnsSubmission),
				muxAll(isInAudit, userOwnsSubmission))))).Methods("POST")

	router.Handle(fmt.Sprintf("/submission-batch/{%s}/comment", constants.ResourceKeySubmissionIDs),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleCommentReceiverBatch, muxAny(
				muxAll(isStaff, a.UserCanCommentAction),
				muxAll(isTrialCurator, userOwnsAllSubmissions),
				muxAll(isInAudit, userOwnsAllSubmissions))))).Methods("POST")

	router.Handle("/api/notification-settings",
		http.HandlerFunc(a.UserAuthMux(
			a.HandleUpdateNotificationSettings, muxAny(isStaff, isTrialCurator, isInAudit)))).Methods("PUT")

	router.Handle(fmt.Sprintf("/submission/{%s}/subscription-settings", constants.ResourceKeySubmissionID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleUpdateSubscriptionSettings, muxAny(isStaff, isTrialCurator, isInAudit)))).Methods("PUT")

	// providers
	router.Handle(fmt.Sprintf("/submission/{%s}/file/{%s}", constants.ResourceKeySubmissionID, constants.ResourceKeyFileID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleDownloadSubmissionFile,
			muxAny(isStaff,
				muxAll(isTrialCurator, userOwnsSubmission),
				muxAll(isInAudit, userOwnsSubmission))))).Methods("GET")

	router.Handle(fmt.Sprintf("/submission-file-batch/{%s}", constants.ResourceKeyFileIDs),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleDownloadSubmissionBatch, muxAny(
				isStaff,
				muxAll(isTrialCurator, userOwnsAllSubmissions))))).Methods("GET")

	router.Handle(fmt.Sprintf("/submission/{%s}/curation-image/{%s}.png", constants.ResourceKeySubmissionID, constants.ResourceKeyCurationImageID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleDownloadCurationImage,
			muxAny(isStaff,
				muxAll(isTrialCurator, userOwnsSubmission),
				muxAll(isInAudit, userOwnsSubmission))))).Methods("GET")

	// soft delete
	router.Handle(fmt.Sprintf("/submission/{%s}/file/{%s}", constants.ResourceKeySubmissionID, constants.ResourceKeyFileID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSoftDeleteSubmissionFile, muxAll(isDeletor)))).Methods("DELETE")

	router.Handle(fmt.Sprintf("/submission/{%s}", constants.ResourceKeySubmissionID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSoftDeleteSubmission, muxAll(isDeletor)))).Methods("DELETE")

	router.Handle(fmt.Sprintf(
		"/submission/{%s}/comment/{%s}", constants.ResourceKeySubmissionID, constants.ResourceKeyCommentID),
		http.HandlerFunc(a.UserAuthMux(
			a.HandleSoftDeleteComment, muxAll(isDeletor)))).Methods("DELETE")

	err := srv.ListenAndServe()
	if err != nil {
		l.Fatal(err)
	}
}
