package bot

import "github.com/Dri0m/flashpoint-submission-system/types"

type DiscordRoleReader interface {
	GetFlashpointRoleIDsForUser(uid int64) ([]string, error)
	GetFlashpointRoles() ([]types.DiscordRole, error)
}
