package authbot

import (
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"strconv"
)

type bot struct {
	session            *discordgo.Session
	flashpointServerID string
	l                  *logrus.Entry
	isDev              bool
}

func NewBot(botSession *discordgo.Session, flashpointServerID string, l *logrus.Entry, isDev bool) *bot {
	return &bot{
		session:            botSession,
		flashpointServerID: flashpointServerID,
		l:                  l,
		isDev:              isDev,
	}
}

// ConnectBot connects bot or panics
func ConnectBot(l *logrus.Entry, token string) *discordgo.Session {
	l.Infoln("connecting the discord auth bot...")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		l.Fatal(err)
	}
	l.Infoln("discord auth bot connected")

	return dg
}

// GetFlashpointRoleIDsForUser returns user role IDs
func (b *bot) GetFlashpointRoleIDsForUser(uid int64) ([]string, error) {
	b.l.WithField("uid", uid).Info("getting flashpoint role ID for user")
	member, err := b.session.GuildMember(b.flashpointServerID, fmt.Sprint(uid))
	if err != nil {
		return nil, err
	}

	return member.Roles, nil
}

// GetFlashpointRoles returns list of flashpoint server roles
func (b *bot) GetFlashpointRoles() ([]types.DiscordRole, error) {
	b.l.Info("getting flashpoint roles")
	roles, err := b.session.GuildRoles(b.flashpointServerID)
	if err != nil {
		return nil, err
	}

	result, err := formatDiscordgoRoles(roles)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func formatDiscordgoRoles(roles []*discordgo.Role) ([]types.DiscordRole, error) {
	formattedRoles := make([]types.DiscordRole, 0, len(roles))
	for _, role := range roles {
		id, err := strconv.ParseInt(role.ID, 10, 64)
		if err != nil {
			return nil, err
		}
		formattedRoles = append(formattedRoles, types.DiscordRole{ID: id, Name: role.Name, Color: fmt.Sprintf("#%06x", role.Color)})
	}
	return formattedRoles, nil
}
