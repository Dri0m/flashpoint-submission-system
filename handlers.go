package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"time"
)

func (a *App) handleRequests(l *logrus.Logger, srv *http.Server, router *mux.Router) {
	// oauth
	router.Handle("/auth", http.HandlerFunc(a.HandleDiscordAuth)).Methods("GET")
	router.Handle("/auth/callback", http.HandlerFunc(a.HandleDiscordCallback)).Methods("GET")

	// logout
	router.Handle("/logout", http.HandlerFunc(a.HandleLogout)).Methods("GET")

	//file server
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	//pages
	router.Handle("/", http.HandlerFunc(a.HandleRootPage)).Methods("GET")
	router.Handle("/profile", http.HandlerFunc(a.UserAuth(a.HandleProfilePage))).Methods("GET")
	router.Handle("/submit", http.HandlerFunc(a.UserAuth(a.HandleSubmitPage))).Methods("GET")

	//API
	router.Handle("/submission-receiver", http.HandlerFunc(a.UserAuth(a.HandleSubmissionReceiver))).Methods("POST")
	err := srv.ListenAndServe()
	if err != nil {
		l.Fatal(err)
	}
}

func (a *App) HandleSubmissionReceiver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "invalid cookie", http.StatusInternalServerError)
		return
	}

	// limit RAM usage to 100MB
	r.ParseMultipartForm(100 * 1000 * 1000)

	file, fileHandler, err := r.FormFile("file")
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "could not retrieve the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	LogCtx(ctx).Debugf("Received a file '%s' - %d bytes, MIME Header: %+v", fileHandler.Filename, fileHandler.Size, fileHandler.Header)

	const dir = "submissions"

	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "could not make directory structure", http.StatusInternalServerError)
		return
	}

	destinationFilename := RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", dir, destinationFilename)

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "could not create destination file", http.StatusInternalServerError)
		return
	}
	defer destination.Close()

	LogCtx(ctx).Debugf("copying received file to '%s'...", destinationFilePath)

	nBytes, err := io.Copy(destination, file)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "could not copy file to destination", http.StatusInternalServerError)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return
	}
	if nBytes != fileHandler.Size {
		LogCtx(ctx).Error(err)
		http.Error(w, "incorrect number of bytes copied to destination", http.StatusInternalServerError)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return
	}

	s := &Submission{
		ID:               0,
		UploaderID:       userID,
		OriginalFilename: fileHandler.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHandler.Size,
		UploadedAt:       time.Now().Unix(),
	}

	if err := a.db.StoreSubmission(s); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "incorrect number of bytes copied to destination", http.StatusInternalServerError)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return
	}

}

func (a *App) HandleRootPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.GetBasePageData(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/root.gohtml")
}

func (a *App) HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.GetBasePageData(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/profile.gohtml")
}

func (a *App) HandleSubmitPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.GetBasePageData(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submit.gohtml")
}
