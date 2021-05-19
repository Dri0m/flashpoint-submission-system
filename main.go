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
	db   *sql.DB
	bot  *discordgo.Session
}

func main() {
	l := InitLogger()
	l.Infoln("hi")

	conf := GetConfig(l)
	db := OpenDB(l)
	defer db.Close()
	bot := ConnectBot(l, conf.BotToken)

	runServer(l, conf, db, bot)

	l.Infoln("goodbye")
}

func runServer(l *logrus.Logger, conf *Config, db *sql.DB, bot *discordgo.Session) {
	l.Infoln("initializing the server")
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", conf.Port),
		Handler: LogRequestHandler(l, router),
	}

	a := &App{
		conf: conf,
		db:   db,
		bot:  bot,
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
}
