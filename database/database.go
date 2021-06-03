package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type DB struct {
	Conn *sql.DB
}

// OpenDB opens DB or panics
func OpenDB(l *logrus.Logger, conf *config.Config) *sql.DB {

	rootUser := conf.DBRootUser
	rootPass := conf.DBRootPassword
	ip := conf.DBIP
	port := conf.DBPort
	dbName := conf.DBName

	rootDB, err := sql.Open("mysql",
		fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?multiStatements=true", rootUser, rootPass, ip, port, dbName))
	if err != nil {
		l.Fatal(err)
	}
	driver, err := mysql.WithInstance(rootDB, &mysql.Config{})
	if err != nil {
		l.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations/",
		"mysql",
		driver,
	)
	if err != nil {
		l.Fatal(err)
	}
	err = m.Up()
	if err != nil && err.Error() != "no change" {
		l.Fatal(err)
	}
	m.Close()
	driver.Close()
	rootDB.Close()

	user := conf.DBUser
	pass := conf.DBPassword

	db, err := sql.Open("mysql",
		fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?multiStatements=true", user, pass, ip, port, dbName))
	if err != nil {
		l.Fatal(err)
	}

	return db
}

// StoreSession store session into the DB with set expiration date
func (db *DB) StoreSession(ctx context.Context, tx *sql.Tx, key string, uid int64, durationSeconds int64) error {
	expiration := time.Now().Add(time.Second * time.Duration(durationSeconds)).Unix()
	_, err := tx.ExecContext(ctx, `INSERT INTO session (secret, uid, expires_at) VALUES (?, ?, ?)`, key, uid, expiration)
	return err
}

// DeleteSession deletes specific session
func (db *DB) DeleteSession(ctx context.Context, tx *sql.Tx, secret string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM session WHERE secret=?`, secret)
	return err
}

// GetUIDFromSession returns user ID and/or expiration state
func (db *DB) GetUIDFromSession(ctx context.Context, tx *sql.Tx, key string) (int64, bool, error) {
	var row *sql.Row
	if tx == nil {
		row = db.Conn.QueryRowContext(ctx, `SELECT uid, expires_at FROM session WHERE secret=?`, key)
	} else {
		row = tx.QueryRowContext(ctx, `SELECT uid, expires_at FROM session WHERE secret=?`, key)
	}

	var uid int64
	var expiration int64
	err := row.Scan(&uid, &expiration)
	if err != nil {
		return 0, false, err
	}

	if expiration <= time.Now().Unix() {
		return 0, false, nil
	}

	return uid, true, nil
}

// StoreDiscordUser store discord user or replace with new data
func (db *DB) StoreDiscordUser(ctx context.Context, tx *sql.Tx, discordUser *types.DiscordUser) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO discord_user (id, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			   ON DUPLICATE KEY UPDATE username=?, avatar=?, discriminator=?, public_flags=?, flags=?, locale=?, mfa_enabled=?`,
		discordUser.ID, discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, discordUser.MFAEnabled,
		discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, discordUser.MFAEnabled)
	return err
}

// GetDiscordUser returns DiscordUserResponse
func (db *DB) GetDiscordUser(ctx context.Context, tx *sql.Tx, uid int64) (*types.DiscordUser, error) {
	row := tx.QueryRowContext(ctx, `SELECT username, avatar, discriminator, public_flags, flags, locale, mfa_enabled FROM discord_user WHERE id=?`, uid)

	discordUser := &types.DiscordUser{ID: uid}
	err := row.Scan(&discordUser.Username, &discordUser.Avatar, &discordUser.Discriminator, &discordUser.PublicFlags, &discordUser.Flags, &discordUser.Locale, &discordUser.MFAEnabled)
	if err != nil {
		return nil, err
	}

	return discordUser, nil
}

// StoreDiscordServerRoles store discord user or replace with new data
func (db *DB) StoreDiscordServerRoles(ctx context.Context, tx *sql.Tx, roles []types.DiscordRole) error {
	if len(roles) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(roles)*3)
	for _, role := range roles {
		data = append(data, role.ID, role.Name, role.Color)
	}

	const valuePlaceholder = `(?, ?, ?)`
	_, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO discord_role (id, name, color) VALUES `+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(roles)-1),
		data...)
	return err
}

// StoreDiscordUserRoles store discord user roles
func (db *DB) StoreDiscordUserRoles(ctx context.Context, tx *sql.Tx, uid int64, roles []int64) error {
	if len(roles) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(roles)*3)
	for _, role := range roles {
		data = append(data, uid, role)
	}

	_, err := tx.ExecContext(ctx, `DELETE FROM discord_user_role WHERE fk_uid = ?`, uid)
	if err != nil {
		return err
	}

	const valuePlaceholder = `(?, ?)`
	_, err = tx.ExecContext(ctx,
		`INSERT INTO discord_user_role (fk_uid, fk_rid) VALUES `+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(roles)-1),
		data...)
	return err
}

// GetDiscordUserRoles returns all user roles
func (db *DB) GetDiscordUserRoles(ctx context.Context, tx *sql.Tx, uid int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT (SELECT name FROM discord_role WHERE discord_role.id=discord_user_role.fk_rid) FROM discord_user_role WHERE fk_uid=?`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]string, 0)

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}

	return result, nil
}

// StoreSubmission stores plain submission
func (db *DB) StoreSubmission(ctx context.Context, tx *sql.Tx) (int64, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO submission (id) VALUES (DEFAULT)`)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// StoreSubmissionFile stores submission file
func (db *DB) StoreSubmissionFile(ctx context.Context, tx *sql.Tx, s *types.SubmissionFile) (int64, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO submission_file (fk_uploader_id, fk_submission_id, original_filename, current_filename, size, uploaded_at, md5sum, sha256sum) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SubmitterID, s.SubmissionID, s.OriginalFilename, s.CurrentFilename, s.Size, s.UploadedAt.Unix(), s.MD5Sum, s.SHA256Sum)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetSubmissionFiles gets submission files, returns error if input len != output len
func (db *DB) GetSubmissionFiles(ctx context.Context, tx *sql.Tx, sfids []int64) ([]*types.SubmissionFile, error) {
	if len(sfids) == 0 {
		return nil, nil
	}

	data := make([]interface{}, len(sfids))
	for i, d := range sfids {
		data[i] = d
	}

	q := `
		SELECT fk_uploader_id, fk_submission_id, original_filename, current_filename, size, uploaded_at, md5sum, sha256sum 
		FROM submission_file 
		WHERE id IN(?` + strings.Repeat(",?", len(sfids)-1) + `)
		AND deleted_at IS NULL
		ORDER BY uploaded_at DESC`

	var rows *sql.Rows
	var err error
	if tx == nil {
		rows, err = db.Conn.QueryContext(ctx, q, data...)
	} else {
		rows, err = tx.QueryContext(ctx, q, data...)
	}
	if err != nil {
		return nil, err
	}

	var result = make([]*types.SubmissionFile, 0, len(sfids))
	for rows.Next() {
		sf := &types.SubmissionFile{}
		var uploadedAt int64
		err := rows.Scan(&sf.SubmitterID, &sf.SubmissionID, &sf.OriginalFilename, &sf.CurrentFilename, &sf.Size, &uploadedAt, &sf.MD5Sum, &sf.SHA256Sum)
		if err != nil {
			return nil, err
		}
		sf.UploadedAt = time.Unix(uploadedAt, 0)
		result = append(result, sf)
	}

	if len(result) != len(sfids) {
		return nil, fmt.Errorf("%s files were not found", len(result)-len(sfids))
	}

	return result, nil
}

// GetExtendedSubmissionFilesBySubmissionID returns all extended submission files for a given submission
func (db *DB) GetExtendedSubmissionFilesBySubmissionID(ctx context.Context, tx *sql.Tx, sid int64) ([]*types.ExtendedSubmissionFile, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT submission_file.id, fk_uploader_id, username, avatar, 
		       original_filename, current_filename, size, uploaded_at, md5sum, sha256sum 
		FROM submission_file 
		LEFT JOIN discord_user ON fk_uploader_id=discord_user.id
		WHERE fk_submission_id=?
		AND submission_file.deleted_at IS NULL
		ORDER BY uploaded_at DESC`, sid)
	if err != nil {
		return nil, err
	}
	var result = make([]*types.ExtendedSubmissionFile, 0)
	var avatar string
	var uploadedAt int64
	for rows.Next() {
		sf := &types.ExtendedSubmissionFile{SubmissionID: sid}
		err := rows.Scan(&sf.FileID, &sf.SubmitterID, &sf.SubmitterUsername, &avatar,
			&sf.OriginalFilename, &sf.CurrentFilename, &sf.Size, &uploadedAt, &sf.MD5Sum, &sf.SHA256Sum)
		if err != nil {
			return nil, err
		}
		sf.SubmitterAvatarURL = utils.FormatAvatarURL(sf.SubmitterID, avatar)
		sf.UploadedAt = time.Unix(uploadedAt, 0)
		result = append(result, sf)
	}
	return result, nil
}

// SearchSubmissions returns extended submissions based on given filter
func (db *DB) SearchSubmissions(ctx context.Context, tx *sql.Tx, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	filters := make([]string, 0)
	data := make([]interface{}, 0)

	data = append(data, constants.ValidatorID, constants.ValidatorID)

	const defaultLimit int64 = 1000
	const defaultOffset int64 = 0

	currentLimit := defaultLimit
	currentOffset := defaultOffset

	limit := "LIMIT ?"
	offset := "OFFSET ?"

	if filter != nil {
		if filter.SubmissionID != nil {
			filters = append(filters, "submission.id=?")
			data = append(data, *filter.SubmissionID)
		}
		if filter.SubmitterID != nil {
			filters = append(filters, "uploader.id=?")
			data = append(data, *filter.SubmitterID)
		}
		if filter.TitlePartial != nil {
			filters = append(filters, "meta.title LIKE ?")
			data = append(data, utils.FormatLike(*filter.TitlePartial))
		}
		if filter.SubmitterUsernamePartial != nil {
			filters = append(filters, "uploader.username LIKE ?")
			data = append(data, utils.FormatLike(*filter.SubmitterUsernamePartial))
		}
		if filter.PlatformPartial != nil {
			filters = append(filters, "meta.platform LIKE ?")
			data = append(data, utils.FormatLike(*filter.PlatformPartial))
		}
		if len(filter.BotActions) != 0 {
			filters = append(filters, `bot_comment.action IN(?`+strings.Repeat(",?", len(filter.BotActions)-1)+`)`)
			for _, ba := range filter.BotActions {
				data = append(data, ba)
			}
		}

		if filter.ResultsPerPage != nil {
			currentLimit = *filter.ResultsPerPage
		} else {
			currentLimit = defaultLimit
		}
		if filter.Page != nil {
			currentOffset = (*filter.Page - 1) * currentLimit
		} else {
			currentOffset = defaultOffset
		}
	}

	data = append(data, currentLimit, currentOffset)

	and := ""
	if len(filters) > 0 {
		and = " AND "
	}

	finalQuery := `
		SELECT  submission.id        AS submission_id 
			   ,uploader.id          AS uploader_id 
			   ,uploader.username    AS uploader_username 
			   ,uploader.avatar      AS uploader_avatar 
			   ,updater.id           AS updater_id 
			   ,updater.username     AS updater_username 
			   ,updater.avatar       AS updater_avatar 
			   ,newest.id            AS submission_file_id 
			   ,newest.original_filename 
			   ,newest.current_filename 
			   ,newest.size 
			   ,oldest.uploaded_at 
			   ,newest.updated_at 
			   ,meta.title 
			   ,meta.alternate_titles
               ,meta.platform
			   ,meta.launch_command 
			   ,bot_comment.action   AS bot_action 
			   ,latest_action.action AS latest_action 
			   ,file_counter.file_count
		FROM submission
		LEFT JOIN 
		(WITH ranked_file AS (
			SELECT  s.* 
				   ,ROW_NUMBER() OVER (PARTITION BY fk_submission_id ORDER BY uploaded_at ASC) AS rn
			FROM submission_file AS s)
			SELECT  fk_uploader_id AS uploader_id 
				   ,fk_submission_id 
				   ,uploaded_at    AS uploaded_at
			FROM ranked_file
			WHERE rn = 1  
		) AS oldest
		ON oldest.fk_submission_id = submission.id
		LEFT JOIN 
		(WITH ranked_file AS (
			SELECT  s.* 
				   ,ROW_NUMBER() OVER (PARTITION BY fk_submission_id ORDER BY uploaded_at DESC) AS rn
			FROM submission_file AS s)
			SELECT  id
				   ,fk_uploader_id AS updater_id 
				   ,fk_submission_id 
				   ,original_filename 
				   ,current_filename 
				   ,size 
				   ,uploaded_at    AS updated_at
			FROM ranked_file
			WHERE rn = 1  
		) AS newest
		ON newest.fk_submission_id = submission.id
		LEFT JOIN discord_user uploader
		ON oldest.uploader_id = uploader.id
		LEFT JOIN discord_user updater
		ON newest.updater_id = updater.id
		LEFT JOIN curation_meta meta
		ON meta.fk_submission_file_id = newest.id
		LEFT JOIN 
		(WITH ranked_comment AS (
			SELECT  c.* 
				   ,ROW_NUMBER() OVER (PARTITION BY fk_submission_id ORDER BY created_at DESC) AS rn
			FROM comment AS c
			WHERE c.fk_author_id = ?) 
			SELECT  ranked_comment.fk_submission_id AS submission_id 
				   ,(
			SELECT  name
			FROM action
			WHERE action.id = ranked_comment.fk_action_id) AS action 
			FROM ranked_comment
			WHERE rn = 1  
		) AS bot_comment
		ON bot_comment.submission_id = submission.id
		LEFT JOIN 
		(WITH ranked_comment AS (
			SELECT  c.* 
				   ,ROW_NUMBER() OVER (PARTITION BY fk_submission_id ORDER BY created_at DESC) AS rn
			FROM comment AS c
			WHERE c.fk_author_id != ?
			AND c.fk_action_id != (SELECT id FROM action WHERE name="comment")) 
			SELECT  ranked_comment.fk_submission_id AS submission_id 
				   ,ranked_comment.created_at 
				   ,(
			SELECT  name
			FROM action
			WHERE action.id = ranked_comment.fk_action_id) AS action 
			FROM ranked_comment
			WHERE rn = 1  
		) AS latest_action
		ON latest_action.submission_id = submission.id
		LEFT JOIN 
		(
			SELECT  fk_submission_id 
				   ,COUNT(id) AS file_count
			FROM submission_file
			WHERE submission_file.deleted_at IS NULL
			GROUP BY  fk_submission_id 
		) AS file_counter
		ON file_counter.fk_submission_id = submission.id
		WHERE submission.deleted_at IS NULL` + and + strings.Join(filters, " AND ") + `
		GROUP BY submission.id
		ORDER BY newest.updated_at DESC
		` + limit + ` ` + offset

	var rows *sql.Rows
	var err error
	if tx == nil {
		rows, err = db.Conn.QueryContext(ctx, finalQuery, data...)
	} else {
		rows, err = tx.QueryContext(ctx, finalQuery, data...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*types.ExtendedSubmission, 0)

	var uploadedAt int64
	var updatedAt int64
	var submitterAvatar string
	var updaterAvatar string

	for rows.Next() {
		s := &types.ExtendedSubmission{}
		if err := rows.Scan(
			&s.SubmissionID,
			&s.SubmitterID, &s.SubmitterUsername, &submitterAvatar,
			&s.UpdaterID, &s.UpdaterUsername, &updaterAvatar,
			&s.FileID, &s.OriginalFilename, &s.CurrentFilename, &s.Size,
			&uploadedAt, &updatedAt,
			&s.CurationTitle, &s.CurationAlternateTitles, &s.CurationPlatform, &s.CurationLaunchCommand,
			&s.BotAction,
			&s.LatestAction,
			&s.FileCount); err != nil {
			return nil, err
		}
		s.SubmitterAvatarURL = utils.FormatAvatarURL(s.SubmitterID, submitterAvatar)
		s.UpdaterAvatarURL = utils.FormatAvatarURL(s.UpdaterID, updaterAvatar)
		s.UploadedAt = time.Unix(uploadedAt, 0)
		s.UpdatedAt = time.Unix(updatedAt, 0)
		result = append(result, s)
	}

	return result, nil
}

// StoreCurationMeta stores curation meta
func (db *DB) StoreCurationMeta(ctx context.Context, tx *sql.Tx, cm *types.CurationMeta) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO curation_meta (fk_submission_file_id, application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters) 
                           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cm.SubmissionFileID, cm.ApplicationPath, cm.Developer, cm.Extreme, cm.GameNotes, cm.Languages,
		cm.LaunchCommand, cm.OriginalDescription, cm.PlayMode, cm.Platform, cm.Publisher, cm.ReleaseDate, cm.Series, cm.Source, cm.Status,
		cm.Tags, cm.TagCategories, cm.Title, cm.AlternateTitles, cm.Library, cm.Version, cm.CurationNotes, cm.MountParameters)
	return err
}

// GetCurationMetaBySubmissionFileID returns curation meta for given submission file
func (db *DB) GetCurationMetaBySubmissionFileID(ctx context.Context, tx *sql.Tx, sfid int64) (*types.CurationMeta, error) {
	row := tx.QueryRowContext(ctx, `SELECT submission_file.fk_submission_id, application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters 
		FROM curation_meta JOIN submission_file ON curation_meta.fk_submission_file_id = submission_file.id
		WHERE fk_submission_file_id=? AND submission_file.deleted_at IS NULL`, sfid)

	c := &types.CurationMeta{SubmissionFileID: sfid}
	err := row.Scan(&c.SubmissionID, &c.ApplicationPath, &c.Developer, &c.Extreme, &c.GameNotes, &c.Languages,
		&c.LaunchCommand, &c.OriginalDescription, &c.PlayMode, &c.Platform, &c.Publisher, &c.ReleaseDate, &c.Series, &c.Source, &c.Status,
		&c.Tags, &c.TagCategories, &c.Title, &c.AlternateTitles, &c.Library, &c.Version, &c.CurationNotes, &c.MountParameters)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// StoreComment stores curation meta
func (db *DB) StoreComment(ctx context.Context, tx *sql.Tx, c *types.Comment) error {
	var msg *string
	if c.Message != nil {
		s := strings.TrimSpace(*c.Message)
		msg = &s
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO comment (fk_author_id, fk_submission_id, message, fk_action_id, created_at) 
                           VALUES (?, ?, ?, (SELECT id FROM action WHERE name=?), ?)`,
		c.AuthorID, c.SubmissionID, msg, c.Action, c.CreatedAt.Unix())
	return err
}

// GetExtendedCommentsBySubmissionID returns all comments with author data for a given submission
func (db *DB) GetExtendedCommentsBySubmissionID(ctx context.Context, tx *sql.Tx, sid int64) ([]*types.ExtendedComment, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT discord_user.id, username, avatar, message, (SELECT name FROM action WHERE id=comment.fk_action_id) as action, created_at 
		FROM comment 
		JOIN discord_user ON discord_user.id = fk_author_id
		WHERE fk_submission_id=? 
		AND comment.deleted_at IS NULL
		ORDER BY created_at;`, sid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*types.ExtendedComment, 0)

	var createdAt int64
	var avatar string
	var message *string

	for rows.Next() {

		ec := &types.ExtendedComment{SubmissionID: sid}
		if err := rows.Scan(&ec.AuthorID, &ec.Username, &avatar, &message, &ec.Action, &createdAt); err != nil {
			return nil, err
		}
		ec.CreatedAt = time.Unix(createdAt, 0)
		ec.AvatarURL = utils.FormatAvatarURL(ec.AuthorID, avatar)
		if message != nil {
			ec.Message = strings.Split(*message, "\n")
		}
		result = append(result, ec)
	}

	return result, nil
}

// SoftDeleteSubmissionFile stores submission file
func (db *DB) SoftDeleteSubmissionFile(ctx context.Context, tx *sql.Tx, sfid int64) error {
	row := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM submission_file
		WHERE fk_submission_id = (SELECT fk_submission_id FROM submission_file WHERE id = ?)
        AND submission_file.deleted_at IS NULL
		GROUP BY fk_submission_id`,
		sfid)

	var count int64
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf(constants.ErrorCannotDeleteLastSubmissionFile)
	}

	_, err := tx.ExecContext(ctx, `
		UPDATE submission_file SET deleted_at = UNIX_TIMESTAMP() 
		WHERE id  = ?`,
		sfid)
	return err
}
