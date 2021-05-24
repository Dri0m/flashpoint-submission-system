// Extremely barebones server to demonstrate OAuth 2.0 flow with Discord
// Uses native net/http to be dependency-less and easy to run.
// No sessions logic implemented, re-login needed each visit.
// Edit the config lines a little bit then go build/run it as normal.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// App is App
type App struct {
	Conf *Config
	DB   DB
	Bot  Bot
	CC   CookieCutter
}

type Bot struct {
	Session            *discordgo.Session
	FlashpointServerID string
	L                  *logrus.Logger
}

type CookieCutter struct {
	Previous *securecookie.SecureCookie
	Current  *securecookie.SecureCookie
}

func main() {
	l := InitLogger()
	l.Infoln("hi")

	conf := GetConfig(l)
	db := OpenDB(l)
	defer db.Close()
	bot := ConnectBot(l, conf.BotToken)

	initApp(l, conf, db, bot)
}

func initApp(l *logrus.Logger, conf *Config, db *sql.DB, botSession *discordgo.Session) {
	l.Infoln("initializing the server")
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", conf.Port),
		Handler: LogRequestHandler(l, router),
	}

	a := &App{
		Conf: conf,
		DB: DB{
			Conn: db,
		},
		Bot: Bot{
			Session:            botSession,
			FlashpointServerID: conf.FlashpointServerID,
			L:                  l,
		},
		CC: CookieCutter{
			Previous: securecookie.New([]byte(conf.SecurecookieHashKeyPrevious), []byte(conf.SecurecookieBlockKeyPrevious)),
			Current:  securecookie.New([]byte(conf.SecurecookieHashKeyCurrent), []byte(conf.SecurecookieBlockKeyPrevious)),
		},
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
