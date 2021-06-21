package notificationbot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type bot struct {
	session               *discordgo.Session
	flashpointServerID    string
	notificationChannelID string
	l                     *logrus.Entry
}

func NewBot(botSession *discordgo.Session, flashpointServerID string, notificationChannelID string, l *logrus.Entry) *bot {
	return &bot{
		session:               botSession,
		flashpointServerID:    flashpointServerID,
		notificationChannelID: notificationChannelID,
		l:                     l,
	}
}

// ConnectBot connects bot or panics
func ConnectBot(l *logrus.Logger, token string) *discordgo.Session {
	l.Infoln("connecting thr discord notification bot...")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		l.Fatal(err)
	}
	return dg
}

// SendNotification sends a message
func (b *bot) SendNotification(msg string) error {
	b.l.Debug("sending message")
	_, err := b.session.ChannelMessageSend(b.notificationChannelID, msg)
	return err
}
