package transport

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/logging"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/gorilla/securecookie"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// App is App
type App struct {
	Conf    *config.Config
	CC      utils.CookieCutter
	Service service.Service
	decoder *schema.Decoder
}

func InitApp(l *logrus.Logger, conf *config.Config, db *sql.DB, authBotSession, notificationBotSession *discordgo.Session) {
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
		Service: service.NewSiteService(l, db, authBotSession, notificationBotSession, conf.FlashpointServerID, conf.NotificationChannelID, conf.ValidatorServerURL, conf.SessionExpirationSeconds, constants.SubmissionsDir),
		decoder: decoder,
	}

	l.WithField("port", conf.Port).Infoln("starting the server...")
	go func() {
		a.handleRequests(l, srv, router)
	}()

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-term

	l.Infoln("closing the auth bot session...")
	authBotSession.Close()

	l.Infoln("closing the notification bot session...")
	notificationBotSession.Close()

	l.Infoln("shutting down the server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		l.WithError(err).Errorln("server shutdown failed")
	}

	l.Infoln("goodbye")
}
