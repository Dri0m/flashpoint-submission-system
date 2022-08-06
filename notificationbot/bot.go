package notificationbot

import (
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type bot struct {
	session               *discordgo.Session
	flashpointServerID    string
	notificationChannelID string
	curationFeedChannelID string
	l                     *logrus.Entry
	isDev                 bool
}

func NewBot(botSession *discordgo.Session, flashpointServerID, notificationChannelID, curationFeedChannelID string, l *logrus.Entry, isDev bool) *bot {
	return &bot{
		session:               botSession,
		flashpointServerID:    flashpointServerID,
		notificationChannelID: notificationChannelID,
		curationFeedChannelID: curationFeedChannelID,
		l:                     l,
		isDev:                 isDev,
	}
}

// ConnectBot connects bot or panics
func ConnectBot(l *logrus.Entry, token string) *discordgo.Session {
	l.Infoln("connecting the discord notification bot...")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		l.Fatal(err)
	}
	l.Infoln("discord notification bot connected")

	return dg
}

// SendNotification sends a message
func (b *bot) SendNotification(msg, notificationType string) error {
	if b.isDev {
		b.l.Debugf("dev mode active, not sending notificaiton")
		return nil
	}

	var err error

	b.l.Debugf("attempting to send a message of type %s", notificationType)
	if notificationType == constants.NotificationDefault {
		_, err = b.session.ChannelMessageSend(b.notificationChannelID, msg)
	} else if notificationType == constants.NotificationCurationFeed {
		_, err = b.session.ChannelMessageSend(b.curationFeedChannelID, msg)
	} else {
		b.l.Panic("invalid notification type")
	}

	return err
}
