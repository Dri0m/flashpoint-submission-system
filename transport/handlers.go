package transport

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/bot"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/logging"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/gorilla/securecookie"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

// App is App
type App struct {
	Conf    *config.Config
	CC      utils.CookieCutter
	Service service.Service
	decoder *schema.Decoder
}

func InitApp(l *logrus.Logger, conf *config.Config, db *sql.DB, botSession *discordgo.Session) {
	l.Infoln("initializing the server")
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", conf.Port),
		Handler: logging.LogRequestHandler(l, router),
	}

	decoder := schema.NewDecoder()
	decoder.ZeroEmpty(false)

	a := &App{
		Conf: conf,
		CC: utils.CookieCutter{
			Previous: securecookie.New([]byte(conf.SecurecookieHashKeyPrevious), []byte(conf.SecurecookieBlockKeyPrevious)),
			Current:  securecookie.New([]byte(conf.SecurecookieHashKeyCurrent), []byte(conf.SecurecookieBlockKeyPrevious)),
		},
		Service: service.Service{
			Bot: bot.Bot{
				Session:            botSession,
				FlashpointServerID: conf.FlashpointServerID,
				L:                  l,
			},
			DB: database.DB{
				Conn: db,
			},
			ValidatorServerURL:       conf.ValidatorServerURL,
			SessionExpirationSeconds: conf.SessionExpirationSeconds,
		},
		decoder: decoder,
	}

	l.WithField("port", conf.Port).Infoln("starting the server...")
	go func() {
		a.handleRequests(l, srv, router)
	}()

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-term

	l.Infoln("shutting down the server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		l.WithError(err).Errorln("server shutdown failed")
	}

	l.Infoln("goodbye")
}

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
	router.Handle("/submissions", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleSubmissionsPage)))).Methods("GET")
	router.Handle("/my-submissions", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleMySubmissionsPage)))).Methods("GET")
	router.Handle("/submission/{id}", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleViewSubmissionPage)))).Methods("GET")
	router.Handle("/submission/{id}/files", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleViewSubmissionFilesPage)))).Methods("GET")

	// receivers
	router.Handle("/submission-receiver", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleSubmissionReceiver)))).Methods("POST")
	router.Handle("/submission-receiver/{id}", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleSubmissionReceiver)))).Methods("POST")
	router.Handle("/submission/{id}/comment", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleCommentReceiver)))).Methods("POST")

	// providers
	router.Handle("/submission-file/{id}", http.HandlerFunc(a.UserAuthentication(a.UserAuthorization(a.HandleDownloadSubmissionFile)))).Methods("GET")

	err := srv.ListenAndServe()
	if err != nil {
		l.Fatal(err)
	}
}

func (a *App) HandleCommentReceiver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserIDFromContext(ctx)

	params := mux.Vars(r)
	submissionID := params["id"]
	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	formAction := r.FormValue("action")
	formMessage := r.FormValue("message")

	if len([]rune(formMessage)) > 20000 {
		err = fmt.Errorf("message cannot be longer than 20000 characters")
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.Service.ProcessReceivedComment(ctx, uid, sid, formAction, formMessage); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, fmt.Sprintf("comment processor: %s", err.Error()), http.StatusInternalServerError)
	}

	http.Redirect(w, r, fmt.Sprintf("/submission/%d", sid), http.StatusFound)
}

func (a *App) HandleDownloadSubmissionFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params["id"]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	sfs, err := a.Service.ProcessDownloadSubmissionFiles(ctx, []int64{sid})
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, fmt.Sprintf("download submission processor: %s", err.Error()), http.StatusBadRequest)
		return
	}
	sf := sfs[0]

	const dir = "submissions"
	f, err := os.Open(fmt.Sprintf("%s/%s", dir, sf.CurrentFilename))

	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sf.CurrentFilename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, sf.CurrentFilename, sf.UploadedAt, f)
}

func (a *App) HandleSubmissionReceiver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params["id"]

	var sid *int64

	if submissionID != "" {
		sidParsed, err := strconv.ParseInt(submissionID, 10, 64)
		if err != nil {
			utils.LogCtx(ctx).Error(err)
			http.Error(w, "invalid submission id", http.StatusBadRequest)
			return
		}
		sid = &sidParsed
	}

	// limit RAM usage to 100MB
	if err := r.ParseMultipartForm(100 * 1000 * 1000); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to parse form", http.StatusInternalServerError)
		return
	}

	fileHeaders := r.MultipartForm.File["files"]

	if len(fileHeaders) == 0 {
		err := fmt.Errorf("no files received")
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.Service.ProcessReceivedSubmissions(ctx, sid, fileHeaders); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, fmt.Sprintf("submission processor: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/my-submissions", http.StatusFound)
}

func (a *App) HandleRootPage(w http.ResponseWriter, r *http.Request) {
	uid, err := a.GetUserIDFromCookie(r)
	ctx := r.Context()
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.UserID, uid))
	ctx = r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/root.gohtml")
}

func (a *App) HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/profile.gohtml")
}

func (a *App) HandleSubmitPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageData, err := a.Service.GetBasePageData(ctx)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submit.gohtml")
}

func (a *App) HandleSubmissionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := &types.SubmissionsFilter{
		SubmissionID: nil,
		SubmitterID:  nil,
	}

	if err := a.decoder.Decode(filter, r.URL.Query()); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "failed to decode query params", http.StatusInternalServerError)
		return
	}

	if err := filter.Validate(); err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageData, err := a.Service.ProcessSearchSubmissions(ctx, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submissions.gohtml", "templates/submission-filter.gohtml", "templates/submission-table.gohtml")
}

func (a *App) HandleMySubmissionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := utils.UserIDFromContext(ctx)

	filter := &types.SubmissionsFilter{
		SubmitterID: &uid,
	}

	pageData, err := a.Service.ProcessSearchSubmissions(ctx, filter)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/my-submissions.gohtml", "templates/submission-table.gohtml")
}

func (a *App) HandleViewSubmissionPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params["id"]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	pageData, err := a.Service.ProcessViewSubmission(ctx, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submission.gohtml", "templates/submission-table.gohtml")
}

func (a *App) HandleViewSubmissionFilesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := mux.Vars(r)
	submissionID := params["id"]

	sid, err := strconv.ParseInt(submissionID, 10, 64)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	pageData, err := a.Service.ProcessViewSubmissionFiles(ctx, sid)
	if err != nil {
		utils.LogCtx(ctx).Error(err)
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	a.RenderTemplates(ctx, w, r, pageData, "templates/submission-files.gohtml", "templates/submission-files-table.gohtml")
}
