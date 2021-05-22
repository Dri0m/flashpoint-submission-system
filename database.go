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
func (db *DB) StoreSession(key string, uid int64, durationSeconds int64) error {
	expiration := time.Now().Add(time.Second * time.Duration(durationSeconds)).Unix()
	_, err := db.conn.Exec(`INSERT INTO session (secret, uid, expires_at) VALUES (?, ?, ?)`, key, uid, expiration)
	return err
}

// DeleteSession deletes specific session
func (db *DB) DeleteSession(secret string) error {
	_, err := db.conn.Exec(`DELETE FROM session WHERE secret=?`, secret)
	return err
}

// GetUIDFromSession returns user ID and/or expiration state
func (db *DB) GetUIDFromSession(key string) (string, bool, error) {
	row := db.conn.QueryRow(`SELECT uid, expires_at FROM session WHERE secret=?`, key)

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
func (db *DB) StoreDiscordUser(discordUser *DiscordUser) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO discord_user (id, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		discordUser.ID, discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, discordUser.MFAEnabled)
	return err
}

// GetDiscordUser returns DiscordUserResponse
func (db *DB) GetDiscordUser(uid int64) (*DiscordUser, error) {
	row := db.conn.QueryRow(`SELECT username, avatar, discriminator, public_flags, flags, locale, mfa_enabled FROM discord_user WHERE id=?`, uid)

	discordUser := &DiscordUser{ID: uid}
	err := row.Scan(&discordUser.Username, &discordUser.Avatar, &discordUser.Discriminator, &discordUser.PublicFlags, &discordUser.Flags, &discordUser.Locale, &discordUser.MFAEnabled)
	if err != nil {
		return nil, err
	}

	return discordUser, nil
}

// StoreDiscordUserAuthorization stores discord user auth state
func (db *DB) StoreDiscordUserAuthorization(uid int64, isAuthorized bool) error {
	a := 0
	if isAuthorized {
		a = 1
	}

	_, err := db.conn.Exec(`INSERT OR REPLACE INTO authorization (fk_uid, authorized) VALUES (?, ?)`, uid, a)
	return err
}

// IsDiscordUserAuthorized returns discord user auth state
func (db *DB) IsDiscordUserAuthorized(uid int64) (bool, error) {
	row := db.conn.QueryRow(`SELECT authorized FROM authorization WHERE fk_uid=?`, uid)
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

type Submission struct {
	ID               int64
	UploaderID       int64
	OriginalFilename string
	CurrentFilename  string
	Size             int64
	UploadedAt       time.Time
}

// StoreSubmission stores submission entry
func (db *DB) StoreSubmission(tx *sql.Tx, s *Submission) (int64, error) {
	res, err := tx.Exec(`INSERT INTO submission (fk_uploader_id, original_filename, current_filename, size, uploaded_at) VALUES (?, ?, ?, ?, ?)`,
		s.UploaderID, s.OriginalFilename, s.CurrentFilename, s.Size, s.UploadedAt.Unix())
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetSubmissionsByUserID returns all submissions for a given user, sorted by date
func (db *DB) GetSubmissionsByUserID(uid int64) ([]*Submission, error) {
	rows, err := db.conn.Query(`SELECT id, original_filename, current_filename, size, uploaded_at FROM submission WHERE fk_uploader_id=? ORDER BY uploaded_at DESC, id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Submission, 0)

	var uploadedAt int64

	for rows.Next() {
		s := &Submission{UploaderID: uid}
		if err := rows.Scan(&s.ID, &s.OriginalFilename, &s.CurrentFilename, &s.Size, &uploadedAt); err != nil {
			return nil, err
		}
		s.UploadedAt = time.Unix(uploadedAt, 0)
		result = append(result, s)
	}

	return result, nil
}

// GetSubmission returns DiscordUserResponse
func (db *DB) GetSubmission(sid int64) (*Submission, error) {
	row := db.conn.QueryRow(`SELECT fk_uploader_id, original_filename, current_filename, size, uploaded_at FROM submission WHERE id=?`, sid)

	s := &Submission{ID: sid}
	var uploadedAt int64
	err := row.Scan(&s.UploaderID, &s.OriginalFilename, &s.CurrentFilename, &s.Size, &uploadedAt)
	if err != nil {
		return nil, err
	}
	s.UploadedAt = time.Unix(uploadedAt, 0)

	return s, nil
}

// GetAllSubmissions returns all submissions
func (db *DB) GetAllSubmissions() ([]*Submission, error) {
	rows, err := db.conn.Query(`SELECT id, fk_uploader_id, original_filename, current_filename, size, uploaded_at FROM submission ORDER BY uploaded_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Submission, 0)

	var uploadedAt int64

	for rows.Next() {
		s := &Submission{}
		if err := rows.Scan(&s.ID, &s.UploaderID, &s.OriginalFilename, &s.CurrentFilename, &s.Size, &uploadedAt); err != nil {
			return nil, err
		}
		s.UploadedAt = time.Unix(uploadedAt, 0)
		result = append(result, s)
	}

	return result, nil
}

// StoreCurationMeta stores curation meta
func (db *DB) StoreCurationMeta(tx *sql.Tx, cm *CurationMeta) error {
	_, err := tx.Exec(`INSERT INTO curation_meta (fk_submission_id, application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters) 
                           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cm.SubmissionID, cm.ApplicationPath, cm.Developer, cm.Extreme, cm.GameNotes, cm.Languages,
		cm.LaunchCommand, cm.OriginalDescription, cm.PlayMode, cm.Platform, cm.Publisher, cm.ReleaseDate, cm.Series, cm.Source, cm.Status,
		cm.Tags, cm.TagCategories, cm.Title, cm.AlternateTitles, cm.Library, cm.Version, cm.CurationNotes, cm.MountParameters)
	return err
}

// GetCurationMetaBySubmissionID returns curation meta for given submission
func (db *DB) GetCurationMetaBySubmissionID(sid int64) (*CurationMeta, error) {
	row := db.conn.QueryRow(`SELECT application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters FROM curation_meta WHERE fk_submission_id=?`, sid)

	c := &CurationMeta{SubmissionID: sid}
	err := row.Scan(&c.ApplicationPath, &c.Developer, &c.Extreme, &c.GameNotes, &c.Languages,
		&c.LaunchCommand, &c.OriginalDescription, &c.PlayMode, &c.Platform, &c.Publisher, &c.ReleaseDate, &c.Series, &c.Source, &c.Status,
		&c.Tags, &c.TagCategories, &c.Title, &c.AlternateTitles, &c.Library, &c.Version, &c.CurationNotes, &c.MountParameters)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// StoreComment stores curation meta
func (db *DB) StoreComment(tx *sql.Tx, c *Comment) error {
	_, err := tx.Exec(`INSERT INTO comment (fk_author_id, fk_submission_id, message, is_approving, created_at) 
                           VALUES (?, ?, ?, ?, ?)`,
		c.AuthorID, c.SubmissionID, c.Message, c.IsApproving, c.CreatedAt.Unix())
	return err
}

// GetCommentsBySubmissionID stores curation meta
func (db *DB) GetCommentsBySubmissionID(sid int64) ([]*Comment, error) {
	rows, err := db.conn.Query(`SELECT fk_author_id, message, is_approving, created_at FROM comment WHERE fk_submission_id=? ORDER BY created_at DESC`, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*Comment, 0)

	var createdAt int64

	for rows.Next() {
		c := &Comment{SubmissionID: sid}
		if err := rows.Scan(&c.AuthorID, &c.Message, &c.IsApproving, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(createdAt, 0)
		result = append(result, c)
	}

	return result, nil
}
