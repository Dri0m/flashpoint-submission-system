// Extremely barebones server to demonstrate OAuth 2.0 flow with Discord
// Uses native net/http to be dependency-less and easy to run.
// No sessions logic implemented, re-login needed each visit.
// Edit the config lines a little bit then go build/run it as normal.
package main

import (
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	l := InitLogger()
	l.Infoln("hi")

	conf := GetConfig(l)
	db := OpenDB(l)
	defer db.Close()
	bot := ConnectBot(l, conf.BotToken)

	InitApp(l, conf, db, bot)
}
