package main

import "time"

// StoreSession store session into the DB with set expiration date
func (a *App) StoreSession(key string, uid string) error {
	expiration := time.Now().Add(time.Second * 30).Unix()
	_, err := a.db.Exec(`INSERT INTO session (secret, uid, expires_at) VALUES (?, ?, ?)`, key, uid, expiration)
	return err
}

// GetUIDFromSession returns user ID and/or expiration state
func (a *App) GetUIDFromSession(key string) (string, bool, error) {
	row := a.db.QueryRow(`SELECT uid, expires_at FROM session WHERE secret=?`, key)

	var uid string
	var expiration int64
	err := row.Scan(&uid, &expiration)
	if err != nil {
		return "", false, err
	}

	if expiration <= time.Now().Unix() {
		return "", false, nil
	}

	return uid, true, nil
}

// StoreDiscordUser store discord user or replace with new data
func (a *App) StoreDiscordUser(discordUser *DiscordUserResponse) error {
	mfa := 0
	if discordUser.MFAEnabled {
		mfa = 1
	}
	_, err := a.db.Exec(
		`INSERT OR REPLACE INTO discord_user (uid, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		discordUser.ID, discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, mfa)
	return err
}

// GetDiscordUser returns DiscordUserResponse
func (a *App) GetDiscordUser(uid string) (*DiscordUserResponse, error) {
	row := a.db.QueryRow(`SELECT username, avatar, discriminator, public_flags, flags, locale, mfa_enabled FROM discord_user WHERE uid=?`, uid)

	discordUser := &DiscordUserResponse{ID: uid}
	var mfa int64
	err := row.Scan(&discordUser.Username, &discordUser.Avatar, &discordUser.Discriminator, &discordUser.PublicFlags, &discordUser.Flags, &discordUser.Locale, &mfa)
	if err != nil {
		return nil, err
	}

	if mfa == 1 {
		discordUser.MFAEnabled = true
	}

	return discordUser, nil
}
