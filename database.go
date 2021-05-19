package main

import (
	"database/sql"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

// OpenDB opens DB or panics
func OpenDB(l *logrus.Logger) *sql.DB {
	l.Infof("opening database '%s'...", dbName)
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		l.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		l.Fatal(err)
	}

	db.SetMaxOpenConns(1)

	//_, err = db.Exec(`PRAGMA journal_mode = WAL`)
	//if err != nil {
	//	l.Fatal(err)
	//}

	file, err := os.ReadFile("sql.sql")
	if err != nil {
		l.Fatal(err)
	}

	_, err = db.Exec(string(file))
	if err != nil {
		l.Fatal(err)
	}

	return db
}

// StoreSession store session into the DB with set expiration date
func (d *DB) StoreSession(key string, uid int64) error {
	expiration := time.Now().Add(time.Hour * 24).Unix()
	_, err := d.conn.Exec(`INSERT INTO session (secret, uid, expires_at) VALUES (?, ?, ?)`, key, uid, expiration)
	return err
}

// DeleteSession deletes specific session
func (d *DB) DeleteSession(secret string) error {
	_, err := d.conn.Exec(`DELETE FROM session WHERE secret=?`, secret)
	return err
}

// GetUIDFromSession returns user ID and/or expiration state
func (d *DB) GetUIDFromSession(key string) (string, bool, error) {
	row := d.conn.QueryRow(`SELECT uid, expires_at FROM session WHERE secret=?`, key)

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
func (d *DB) StoreDiscordUser(discordUser *DiscordUser) error {
	mfa := 0
	if discordUser.MFAEnabled {
		mfa = 1
	}
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO discord_user (id, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		discordUser.ID, discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, mfa)
	return err
}

// GetDiscordUser returns DiscordUserResponse
func (d *DB) GetDiscordUser(uid int64) (*DiscordUser, error) {
	row := d.conn.QueryRow(`SELECT username, avatar, discriminator, public_flags, flags, locale, mfa_enabled FROM discord_user WHERE id=?`, uid)

	discordUser := &DiscordUser{ID: uid}
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

// StoreDiscordUserAuthorization stores discord user auth state
func (d *DB) StoreDiscordUserAuthorization(uid int64, isAuthorized bool) error {
	a := 0
	if isAuthorized {
		a = 1
	}

	_, err := d.conn.Exec(`INSERT OR REPLACE INTO authorization (fk_uid, authorized) VALUES (?, ?)`, uid, a)
	return err
}

// IsDiscordUserAuthorized returns discord user auth state
func (d *DB) IsDiscordUserAuthorized(uid int64) (bool, error) {
	row := d.conn.QueryRow(`SELECT authorized FROM authorization WHERE fk_uid=?`, uid)
	var a int64
	err := row.Scan(&a)
	if err != nil {
		return false, err
	}

	if a == 1 {
		return true, nil
	}
	return false, nil
}
