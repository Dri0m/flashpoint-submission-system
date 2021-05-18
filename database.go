package main

import "time"

// StoreSession save session into the DB with set expiration date
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
