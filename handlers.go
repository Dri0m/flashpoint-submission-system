package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
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

	// file server
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// pages
	router.Handle("/", http.HandlerFunc(a.HandleRootPage)).Methods("GET")
	router.Handle("/profile", http.HandlerFunc(a.UserAuthentication(a.HandleProfilePage))).Methods("GET")
	router.Handle("/submit", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleSubmitPage)))).Methods("GET")
	router.Handle("/my-submissions", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleMySubmissionsPage)))).Methods("GET")

	// form receivers
	router.Handle("/submission-receiver", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleSubmissionReceiver)))).Methods("POST")
	err := srv.ListenAndServe()
	if err != nil {
		l.Fatal(err)
	}
}

func (a *App) LogIfErr(ctx context.Context, err error) {
	if err != nil {
		LogCtx(ctx).Error(err)
	}
}

func (a *App) ProcessReceivedSubmission(ctx context.Context, tx *sql.Tx, fileHeader *multipart.FileHeader) error {
	userID := UserIDFromContext(ctx)
	if userID == 0 {
		err := fmt.Errorf("no user associated with request")
		LogCtx(ctx).Error(err)
		return err
	}
	file, err := fileHeader.Open()
	if err != nil {
		LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to open received file")
	}
	defer file.Close()

	LogCtx(ctx).Debugf("received a file '%s' - %d bytes, MIME header: %+v", fileHeader.Filename, fileHeader.Size, fileHeader.Header)

	const dir = "submissions"

	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to make directory structure")
	}

	destinationFilename := RandomString(64)
	destinationFilePath := fmt.Sprintf("%s/%s", dir, destinationFilename)

	destination, err := os.Create(destinationFilePath)
	if err != nil {
		LogCtx(ctx).Error(err)
		return fmt.Errorf("failed to create destination file")
	}
	defer destination.Close()

	LogCtx(ctx).Debugf("copying received file to '%s'...", destinationFilePath)

	nBytes, err := io.Copy(destination, file)
	if err != nil {
		LogCtx(ctx).Error(err)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return fmt.Errorf("failed to copy file to destination")
	}
	if nBytes != fileHeader.Size {
		LogCtx(ctx).Error(err)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		return fmt.Errorf("incorrect number of bytes copied to destination")
	}

	s := &Submission{
		ID:               0,
		UploaderID:       UserIDFromContext(ctx),
		OriginalFilename: fileHeader.Filename,
		CurrentFilename:  destinationFilename,
		Size:             fileHeader.Size,
		UploadedAt:       time.Now().Unix(),
	}

	if err := a.db.StoreSubmission(tx, s); err != nil {
		LogCtx(ctx).Error(err)
		_ = destination.Close()
		_ = os.Remove(destinationFilePath)
		a.LogIfErr(ctx, tx.Rollback())
		return fmt.Errorf("failed to store submission")
	}

	return nil
}

func (a *App) HandleSubmissionReceiver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tx, err := a.db.conn.Begin()
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to begin transaction", http.StatusInternalServerError)
		return
	}

	// limit RAM usage to 100MB
	if err := r.ParseMultipartForm(100 * 1000 * 1000); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse form", http.StatusInternalServerError)
		return
	}

	fileHeaders := r.MultipartForm.File["files"]
	for _, fileHeader := range fileHeaders {
		err := a.ProcessReceivedSubmission(ctx, tx, fileHeader)
		if err != nil {
			LogCtx(ctx).Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			a.LogIfErr(ctx, tx.Rollback())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to commit transaction", http.StatusInternalServerError)
		a.LogIfErr(ctx, tx.Rollback())
	}

	http.Redirect(w, r, "/my-submissions", http.StatusFound)
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

type mySubmissionsPageData struct {
	basePageData
	Submissions []*Submission
}

func (a *App) HandleMySubmissionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := a.GetUserIDFromCookie(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "invalid cookie", http.StatusInternalServerError)
		return
	}

	bpd, err := a.GetBasePageData(r)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	submissions, err := a.db.GetSubmissionsForUser(userID)
	if err != nil {
		LogCtx(ctx).Error(err)
		http.Error(w, "failed to load user submissions", http.StatusInternalServerError)
		return
	}

	pageData := mySubmissionsPageData{basePageData: *bpd, Submissions: submissions}

	a.RenderTemplates(ctx, w, r, pageData, "templates/my-submissions.gohtml")
}
