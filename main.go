// Extremely barebones server to demonstrate OAuth 2.0 flow with Discord
// Uses native net/http to be dependency-less and easy to run.
// No sessions logic implemented, re-login needed each visit.
// Edit the config lines a little bit then go build/run it as normal.
package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// App is App
type App struct {
	conf *Config
}

func main() {
	l := InitLogger()
	l.Infoln("hi")

	conf := GetConfig(l)

	l.Infoln("initializing the server")
	router := mux.NewRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", conf.Port),
		Handler: LogRequestHandler(l, router),
	}

	a := &App{
		conf: conf,
	}

	l.WithField("port", conf.Port).Infoln("starting the server...")
	go func() {
		a.handleRequests(srv, router)
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
