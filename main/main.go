// Extremely barebones server to demonstrate OAuth 2.0 flow with Discord
// Uses native net/http to be dependency-less and easy to run.
// No sessions logic implemented, re-login needed each visit.
// Edit the config lines a little bit then go build/run it as normal.
package main

import (
	"github.com/Dri0m/flashpoint-submission-system/bot"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/logging"
	"github.com/Dri0m/flashpoint-submission-system/service"
	"github.com/Dri0m/flashpoint-submission-system/transport"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	l := logging.InitLogger()
	l.Infoln("hi")

	conf := config.GetConfig(l)
	db := service.OpenDB(l, conf)
	defer db.Close()
	b := bot.ConnectBot(l, conf.BotToken)

	transport.InitApp(l, conf, db, b)
}
