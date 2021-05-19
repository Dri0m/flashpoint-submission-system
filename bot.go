package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// ConnectBot connects bot or panics
func ConnectBot(l *logrus.Logger, token string) *discordgo.Session {
	l.Infoln("connecting discord bot...")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		l.Fatal(err)
	}
	return dg
}

// GetFlashpointRoleIDsForUser returns user role IDs
func (a *App) GetFlashpointRoleIDsForUser(uid int64) ([]string, error) {
	member, err := a.bot.GuildMember(a.conf.FlashpointServerID, fmt.Sprint(uid))
	if err != nil {
		return nil, err
	}

	return member.Roles, nil
}

// GetFlashpointRolesForUser returns user role IDs
func (a *App) GetFlashpointRolesForUser(uid int64) ([]*discordgo.Role, error) {
	roleIds, err := a.GetFlashpointRoleIDsForUser(uid)
	if err != nil {
		return nil, err
	}
	roles, err := a.GetRolesFromRoleIDs(roleIds)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

// GetFlashpointRoles returns list of flashpoint server roles
func (a *App) GetFlashpointRoles() ([]*discordgo.Role, error) {
	roles, err := a.bot.GuildRoles(a.conf.FlashpointServerID)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

// GetRolesFromRoleIDs takes role IDs and returns full roles
func (a *App) GetRolesFromRoleIDs(roleIDs []string) ([]*discordgo.Role, error) {
	result := make([]*discordgo.Role, 0, len(roleIDs))

	roles, err := a.GetFlashpointRoles()
	if err != nil {
		return nil, err
	}

	for _, roleID := range roleIDs {
		for _, role := range roles {
			if role.ID == roleID {
				result = append(result, role)
			}
		}
	}

	return result, nil
}
