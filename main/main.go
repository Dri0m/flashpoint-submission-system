// Extremely barebones server to demonstrate OAuth 2.0 flow with Discord
// Uses native net/http to be dependency-less and easy to run.
// No sessions logic implemented, re-login needed each visit.
// Edit the config lines a little bit then go build/run it as normal.
package main

import (
	"github.com/Dri0m/flashpoint-submission-system/authbot"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/logging"
	"github.com/Dri0m/flashpoint-submission-system/notificationbot"
	"github.com/Dri0m/flashpoint-submission-system/transport"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log := logging.InitLogger()
	log.Infoln("hi")

	conf := config.GetConfig(log)
	l := log.WithField("commit", conf.Commit)
	db := database.OpenDB(l, conf)
	defer db.Close()
	authBot := authbot.ConnectBot(l, conf.AuthBotToken)
	notificationBot := notificationbot.ConnectBot(l, conf.NotificationBotToken)

	transport.InitApp(l, conf, db, authBot, notificationBot)
}
