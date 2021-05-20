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

const dbName = "db.db"

// App is App
type App struct {
	conf *Config
	db   DB
	bot  Bot
	cc   CookieCutter
}

type Bot struct {
	session            *discordgo.Session
	flashpointServerID string
	l                  *logrus.Logger
}

type DB struct {
	conn *sql.DB
}

type CookieCutter struct {
	previous *securecookie.SecureCookie
	current  *securecookie.SecureCookie
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
		conf: conf,
		db: DB{
			conn: db,
		},
		bot: Bot{
			session:            botSession,
			flashpointServerID: conf.FlashpointServerID,
			l:                  l,
		},
		cc: CookieCutter{
			previous: securecookie.New([]byte(conf.SecurecookieHashKeyPrevious), []byte(conf.SecurecookieBlockKeyPrevious)),
			current:  securecookie.New([]byte(conf.SecurecookieHashKeyCurrent), []byte(conf.SecurecookieBlockKeyPrevious)),
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
