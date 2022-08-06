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
	"github.com/Dri0m/flashpoint-submission-system/resumableuploadservice"
	"github.com/Dri0m/flashpoint-submission-system/transport"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	log := logging.InitLogger()
	l := log.WithField("commit", config.EnvString("GIT_COMMIT")).WithField("runID", utils.NewRealRandomStringProvider().RandomString(8))
	l.Infoln("hi")

	l.Infoln("loading config...")
	conf := config.GetConfig(l)
	l.Infoln("config loaded")

	db := database.OpenDB(l, conf)
	defer db.Close()

	authBot := authbot.ConnectBot(l, conf.AuthBotToken)
	notificationBot := notificationbot.ConnectBot(l, conf.NotificationBotToken)

	l.Infoln("connecting to the resumable upload service")
	rsu, err := resumableuploadservice.New(conf.ResumableUploadDirFullPath)
	if err != nil {
		l.Fatal(err)
	}
	defer rsu.Close()
	l.Infoln("resumable upload service connected")

	transport.InitApp(l, conf, db, authBot, notificationBot, rsu)
}
