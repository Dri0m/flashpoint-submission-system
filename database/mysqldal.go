package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/sirupsen/logrus"
)

type mysqlDAL struct {
	db *sql.DB
}

func NewMysqlDAL(conn *sql.DB) *mysqlDAL {
	return &mysqlDAL{
		db: conn,
	}
}

// OpenDB opens DAL or panics
func OpenDB(l *logrus.Entry, conf *config.Config) *sql.DB {
	l.Infoln("connecting to the database")

	user := conf.DBUser
	pass := conf.DBPassword
	ip := conf.DBIP
	port := conf.DBPort
	dbName := conf.DBName

	db, err := sql.Open("mysql",
		fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?multiStatements=true", user, pass, ip, port, dbName))
	if err != nil {
		l.Fatal(err)
	}

	l.Infoln("database connected")
	return db
}

type MysqlSession struct {
	context     context.Context
	transaction *sql.Tx
}

// NewSession begins a transaction
func (d *mysqlDAL) NewSession(ctx context.Context) (DBSession, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}

	return &MysqlSession{
		context:     ctx,
		transaction: tx,
	}, nil
}

func (dbs *MysqlSession) Commit() error {
	return dbs.transaction.Commit()
}

func (dbs *MysqlSession) Rollback() error {
	err := dbs.Tx().Rollback()
	if err != nil && err.Error() == "sql: transaction has already been committed or rolled back" {
		err = nil
	}
	if err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
	}
	return err
}

func (dbs *MysqlSession) Tx() *sql.Tx {
	return dbs.transaction
}

func (dbs *MysqlSession) Ctx() context.Context {
	return dbs.context
}

// StoreSession store session into the DAL with set expiration date
func (d *mysqlDAL) StoreSession(dbs DBSession, key string, uid int64, durationSeconds int64) error {
	expiration := time.Now().Add(time.Second * time.Duration(durationSeconds)).Unix()
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO session (secret, uid, expires_at) VALUES (?, ?, ?)`, key, uid, expiration)
	return err
}

// DeleteSession deletes specific session
func (d *mysqlDAL) DeleteSession(dbs DBSession, secret string) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `DELETE FROM session WHERE secret=?`, secret)
	return err
}

// GetUIDFromSession returns user ID and/or expiration state
func (d *mysqlDAL) GetUIDFromSession(dbs DBSession, key string) (int64, bool, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `SELECT uid, expires_at FROM session WHERE secret=?`, key)

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
func (d *mysqlDAL) StoreDiscordUser(dbs DBSession, discordUser *types.DiscordUser) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(),
		`INSERT INTO discord_user (id, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			   ON DUPLICATE KEY UPDATE username=?, avatar=?, discriminator=?, public_flags=?, flags=?, locale=?, mfa_enabled=?`,
		discordUser.ID, discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, discordUser.MFAEnabled,
		discordUser.Username, discordUser.Avatar, discordUser.Discriminator, discordUser.PublicFlags, discordUser.Flags, discordUser.Locale, discordUser.MFAEnabled)
	return err
}

// GetDiscordUser returns DiscordUserResponse
func (d *mysqlDAL) GetDiscordUser(dbs DBSession, uid int64) (*types.DiscordUser, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `SELECT username, avatar, discriminator, public_flags, flags, locale, mfa_enabled FROM discord_user WHERE id=?`, uid)

	discordUser := &types.DiscordUser{ID: uid}
	err := row.Scan(&discordUser.Username, &discordUser.Avatar, &discordUser.Discriminator, &discordUser.PublicFlags, &discordUser.Flags, &discordUser.Locale, &discordUser.MFAEnabled)
	if err != nil {
		return nil, err
	}

	return discordUser, nil
}

// StoreDiscordServerRoles store discord user or replace with new data
func (d *mysqlDAL) StoreDiscordServerRoles(dbs DBSession, roles []types.DiscordRole) error {
	if len(roles) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(roles)*3)
	for _, role := range roles {
		data = append(data, role.ID, role.Name, role.Color)
	}

	const valuePlaceholder = `(?, ?, ?)`
	_, err := dbs.Tx().ExecContext(dbs.Ctx(),
		`INSERT IGNORE INTO discord_role (id, name, color) VALUES `+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(roles)-1),
		data...)
	return err
}

// StoreDiscordUserRoles store discord user roles
func (d *mysqlDAL) StoreDiscordUserRoles(dbs DBSession, uid int64, roles []int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `DELETE FROM discord_user_role WHERE fk_uid = ?`, uid)
	if err != nil {
		return err
	}

	if len(roles) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(roles)*3)
	for _, role := range roles {
		data = append(data, uid, role)
	}

	const valuePlaceholder = `(?, ?)`
	_, err = dbs.Tx().ExecContext(dbs.Ctx(),
		`INSERT INTO discord_user_role (fk_uid, fk_rid) VALUES `+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(roles)-1),
		data...)
	return err
}

// GetDiscordUserRoles returns all user roles
func (d *mysqlDAL) GetDiscordUserRoles(dbs DBSession, uid int64) ([]string, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
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
func (d *mysqlDAL) StoreSubmission(dbs DBSession, submissionLevel string) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO submission (fk_submission_level_id) 
				VALUES ((SELECT id FROM submission_level WHERE name = ?))`,
		submissionLevel)
	if err != nil {
		return 0, err
	}
	sid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		INSERT INTO submission_cache (fk_submission_id) 
		VALUES (?)`,
		sid)
	if err != nil {
		return 0, err
	}

	return sid, nil
}

// StoreSubmissionFile stores submission file
func (d *mysqlDAL) StoreSubmissionFile(dbs DBSession, s *types.SubmissionFile) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO submission_file (fk_user_id, fk_submission_id, original_filename, current_filename, size, created_at, md5sum, sha256sum) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SubmitterID, s.SubmissionID, s.OriginalFilename, s.CurrentFilename, s.Size, s.UploadedAt.Unix(), s.MD5Sum, s.SHA256Sum)
	if err != nil {
		return 0, err
	}
	fid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return fid, nil
}

// GetSubmissionFiles gets submission files, returns error if input len != output len
func (d *mysqlDAL) GetSubmissionFiles(dbs DBSession, sfids []int64) ([]*types.SubmissionFile, error) {
	if len(sfids) == 0 {
		return nil, nil
	}

	data := make([]interface{}, len(sfids))
	for i, d := range sfids {
		data[i] = d
	}

	q := `
		SELECT fk_user_id, fk_submission_id, original_filename, current_filename, size, created_at, md5sum, sha256sum 
		FROM submission_file 
		WHERE id IN(?` + strings.Repeat(",?", len(sfids)-1) + `)
		AND deleted_at IS NULL
		ORDER BY created_at DESC`

	var rows *sql.Rows
	var err error
	rows, err = dbs.Tx().QueryContext(dbs.Ctx(), q, data...)
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
		return nil, fmt.Errorf("%d files were not found", len(result)-len(sfids))
	}

	return result, nil
}

// GetExtendedSubmissionFilesBySubmissionID returns all extended submission files for a given submission
func (d *mysqlDAL) GetExtendedSubmissionFilesBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedSubmissionFile, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT submission_file.id, fk_user_id, username, avatar, 
		       original_filename, current_filename, size, created_at, md5sum, sha256sum 
		FROM submission_file 
		LEFT JOIN discord_user ON fk_user_id=discord_user.id
		WHERE fk_submission_id=?
		AND submission_file.deleted_at IS NULL
		ORDER BY created_at DESC`, sid)
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

// StoreCurationMeta stores curation meta
func (d *mysqlDAL) StoreCurationMeta(dbs DBSession, cm *types.CurationMeta) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO curation_meta (fk_submission_file_id, application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters, uuid, game_exists,
                           primary_platform) 
                           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cm.SubmissionFileID, cm.ApplicationPath, cm.Developer, cm.Extreme, cm.GameNotes, cm.Languages,
		cm.LaunchCommand, cm.OriginalDescription, cm.PlayMode, cm.Platform, cm.Publisher, cm.ReleaseDate, cm.Series, cm.Source, cm.Status,
		cm.Tags, cm.TagCategories, cm.Title, cm.AlternateTitles, cm.Library, cm.Version, cm.CurationNotes, cm.MountParameters, cm.UUID, cm.GameExists,
		cm.PrimaryPlatform)
	return err
}

// GetCurationMetaBySubmissionFileID returns curation meta for given submission file
func (d *mysqlDAL) GetCurationMetaBySubmissionFileID(dbs DBSession, sfid int64) (*types.CurationMeta, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `SELECT submission_file.fk_submission_id, application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters, uuid, game_exists,
                           primary_platform
		FROM curation_meta JOIN submission_file ON curation_meta.fk_submission_file_id = submission_file.id
		WHERE fk_submission_file_id=? AND submission_file.deleted_at IS NULL`, sfid)

	c := &types.CurationMeta{SubmissionFileID: sfid}
	err := row.Scan(&c.SubmissionID, &c.ApplicationPath, &c.Developer, &c.Extreme, &c.GameNotes, &c.Languages,
		&c.LaunchCommand, &c.OriginalDescription, &c.PlayMode, &c.Platform, &c.Publisher, &c.ReleaseDate, &c.Series, &c.Source, &c.Status,
		&c.Tags, &c.TagCategories, &c.Title, &c.AlternateTitles, &c.Library, &c.Version, &c.CurationNotes, &c.MountParameters, &c.UUID, &c.GameExists,
		&c.PrimaryPlatform)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// StoreComment stores curation meta
func (d *mysqlDAL) StoreComment(dbs DBSession, c *types.Comment) error {
	var msg *string
	if c.Message != nil {
		s := strings.TrimSpace(*c.Message)
		msg = &s
	}
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		INSERT INTO comment (fk_user_id, fk_submission_id, message, fk_action_id, created_at) 
        VALUES (?, ?, ?, (SELECT id FROM action WHERE name=?), ?)`,
		c.AuthorID, c.SubmissionID, msg, c.Action, c.CreatedAt.Unix())
	if err != nil {
		return err
	}

	return nil
}

func (d *mysqlDAL) PopulateRevisionInfo(dbs DBSession, revisions []*types.RevisionInfo) error {
	for _, revision := range revisions {
		var avatar string
		err := dbs.Tx().QueryRowContext(dbs.Ctx(), `SELECT username, avatar
		FROM discord_user
		WHERE discord_user.id = ?`,
			revision.AuthorID).
			Scan(&revision.Username, &avatar)
		if err != nil {
			return err
		}
		revision.AvatarURL = utils.FormatAvatarURL(revision.AuthorID, avatar)
	}
	return nil
}

// GetExtendedCommentsBySubmissionID returns all comments with author data for a given submission
func (d *mysqlDAL) GetExtendedCommentsBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedComment, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT comment.id, discord_user.id, username, avatar, message, (SELECT name FROM action WHERE id=comment.fk_action_id) as action, created_at 
		FROM comment 
		JOIN discord_user ON discord_user.id = fk_user_id
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

	for rows.Next() {

		ec := &types.ExtendedComment{SubmissionID: sid}
		if err := rows.Scan(&ec.CommentID, &ec.AuthorID, &ec.Username, &avatar, &ec.Message, &ec.Action, &createdAt); err != nil {
			return nil, err
		}
		ec.CreatedAt = time.Unix(createdAt, 0)
		ec.AvatarURL = utils.FormatAvatarURL(ec.AuthorID, avatar)
		result = append(result, ec)
	}

	return result, nil
}

// GetCommentByID returns a comment
func (d *mysqlDAL) GetCommentByID(dbs DBSession, cid int64) (*types.Comment, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT fk_user_id, fk_submission_id, message, (SELECT name FROM action WHERE id=comment.fk_action_id), created_at
		FROM comment
		WHERE id = ?`,
		cid)

	c := &types.Comment{}
	var createdAt int64
	if err := row.Scan(&c.AuthorID, &c.SubmissionID, &c.Message, &c.Action, &createdAt); err != nil {
		return nil, err
	}
	c.CreatedAt = time.Unix(createdAt, 0)

	return c, nil
}

// SoftDeleteSubmissionFile marks submission file as deleted
func (d *mysqlDAL) SoftDeleteSubmissionFile(dbs DBSession, sfid int64, deleteReason string) error {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*), fk_submission_id FROM submission_file
		WHERE fk_submission_id = (SELECT fk_submission_id FROM submission_file WHERE id = ?)
        AND submission_file.deleted_at IS NULL
		GROUP BY fk_submission_id`,
		sfid)

	var count int64
	var sid int64
	if err := row.Scan(&count, &sid); err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf(constants.ErrorCannotDeleteLastSubmissionFile)
	}

	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_file SET deleted_at = UNIX_TIMESTAMP(), deleted_reason = ?
		WHERE id  = ?`,
		deleteReason, sfid)
	if err != nil {
		return err
	}

	err = d.UpdateSubmissionCacheTable(dbs, sid)
	if err != nil {
		return err
	}

	return nil
}

// SoftDeleteSubmission marks submission and its files as deleted
func (d *mysqlDAL) SoftDeleteSubmission(dbs DBSession, sid int64, deleteReason string) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_file SET deleted_at = UNIX_TIMESTAMP(), deleted_reason = ?
		WHERE fk_submission_id = ?`,
		deleteReason, sid)
	if err != nil {
		return err
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE comment SET deleted_at = UNIX_TIMESTAMP(), deleted_reason = ?
		WHERE fk_submission_id = ?`,
		deleteReason, sid)
	if err != nil {
		return err
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission SET deleted_at = UNIX_TIMESTAMP(), deleted_reason = ?
		WHERE id = ?`,
		deleteReason, sid)
	if err != nil {
		return err
	}

	err = d.UpdateSubmissionCacheTable(dbs, sid)
	if err != nil {
		return err
	}

	return nil
}

// SoftDeleteComment marks comment as deleted
func (d *mysqlDAL) SoftDeleteComment(dbs DBSession, cid int64, deleteReason string) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE comment SET deleted_at = UNIX_TIMESTAMP(), deleted_reason = ?
		WHERE id = ?`,
		deleteReason, cid)
	if err != nil {
		return err
	}

	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT fk_submission_id FROM comment
		WHERE id = ?`,
		cid)

	var sid int64
	err = row.Scan(&sid)
	if err != nil {
		return err
	}

	err = d.UpdateSubmissionCacheTable(dbs, sid)
	if err != nil {
		return err
	}

	return nil
}

// StoreNotificationSettings clears and stores new notification settings for user
func (d *mysqlDAL) StoreNotificationSettings(dbs DBSession, uid int64, actions []string) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		DELETE FROM notification_settings WHERE fk_user_id = ?`,
		uid)
	if err != nil {
		return err
	}

	if len(actions) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(actions)*2)
	for _, role := range actions {
		data = append(data, uid, role)
	}

	const valuePlaceholder = `(?, (SELECT id FROM action WHERE name = ?))`
	_, err = dbs.Tx().ExecContext(dbs.Ctx(),
		`INSERT INTO notification_settings (fk_user_id, fk_action_id) VALUES `+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(actions)-1),
		data...)
	return err
}

// GetNotificationSettingsByUserID returns actions on which user is notified on submissions he's subscribed to
func (d *mysqlDAL) GetNotificationSettingsByUserID(dbs DBSession, uid int64) ([]string, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT (SELECT name FROM action WHERE action.id = notification_settings.fk_action_id) AS action_name
		FROM notification_settings 
		WHERE fk_user_id = ?`,
		uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]string, 0)
	var action string

	for rows.Next() {
		if err := rows.Scan(&action); err != nil {
			return nil, err
		}
		result = append(result, action)
	}

	return result, nil
}

// SubscribeUserToSubmission stores subscription to a submission
func (d *mysqlDAL) SubscribeUserToSubmission(dbs DBSession, uid, sid int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		INSERT INTO submission_notification_subscription (fk_user_id, fk_submission_id, created_at)
		VALUES (?, ?, UNIX_TIMESTAMP())`,
		uid, sid)
	return err
}

// UnsubscribeUserFromSubmission deletes subscription to a submission
func (d *mysqlDAL) UnsubscribeUserFromSubmission(dbs DBSession, uid, sid int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		DELETE FROM submission_notification_subscription
		WHERE fk_user_id = ? AND fk_submission_id = ?`,
		uid, sid)
	return err
}

// IsUserSubscribedToSubmission returns true if the user is subscribed to a submission
func (d *mysqlDAL) IsUserSubscribedToSubmission(dbs DBSession, uid, sid int64) (bool, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*) FROM submission_notification_subscription
		WHERE fk_user_id = ? AND fk_submission_id = ?`,
		uid, sid)

	var count uint64

	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// StoreNotification stores a notification message in the database which acts as a queue for the notification service
func (d *mysqlDAL) StoreNotification(dbs DBSession, msg, notificationType string) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		INSERT INTO submission_notification (message, fk_submission_notification_type_id, created_at)
		VALUES(?, (SELECT id FROM submission_notification_type WHERE name = ?), UNIX_TIMESTAMP())`,
		msg, notificationType)

	return err
}

// GetUsersForNotification returns a list of users who should be notified by an event
func (d *mysqlDAL) GetUsersForNotification(dbs DBSession, authorID, sid int64, action string) ([]int64, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT DISTINCT notification_settings.fk_user_id
		FROM notification_settings
		LEFT JOIN submission_notification_subscription ON submission_notification_subscription.fk_user_id = notification_settings.fk_user_id
		WHERE submission_notification_subscription.fk_submission_id = ?
		AND notification_settings.fk_action_id = (SELECT id FROM action where name = ?)
		AND notification_settings.fk_user_id != ?`,
		sid, action, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]int64, 0)
	var uid int64

	for rows.Next() {
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		result = append(result, uid)
	}

	return result, nil
}

// GetUsersForUniversalNotification returns a list of users who should be notified by an event not dependent on a submission ID
func (d *mysqlDAL) GetUsersForUniversalNotification(dbs DBSession, authorID int64, action string) ([]int64, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT DISTINCT fk_user_id
		FROM notification_settings
		WHERE fk_action_id = (SELECT id FROM action where name = ?)
		AND fk_user_id != ?`,
		action, authorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]int64, 0)
	var uid int64

	for rows.Next() {
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		result = append(result, uid)
	}

	return result, nil
}

// GetOldestUnsentNotification returns oldest unsent notification
func (d *mysqlDAL) GetOldestUnsentNotification(dbs DBSession) (*types.Notification, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT id, (SELECT name FROM submission_notification_type WHERE id = fk_submission_notification_type_id), message, created_at, sent_at 
		FROM submission_notification
		WHERE sent_at IS NULL
		ORDER BY created_at LIMIT 1`)

	notification := &types.Notification{}
	var createdAt int64
	var sentAt *int64

	err := row.Scan(&notification.ID, &notification.Type, &notification.Message, &createdAt, &sentAt)
	if err != nil {
		return nil, err
	}

	notification.CreatedAt = time.Unix(createdAt, 0)
	if sentAt != nil {
		notification.SentAt = time.Unix(*sentAt, 0)
	}

	return notification, nil
}

// MarkNotificationAsSent returns oldest unsent notification
func (d *mysqlDAL) MarkNotificationAsSent(dbs DBSession, nid int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_notification SET sent_at = UNIX_TIMESTAMP() 
		WHERE id = ?`, nid)

	return err
}

// StoreCurationImage stores curation image
func (d *mysqlDAL) StoreCurationImage(dbs DBSession, c *types.CurationImage) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		INSERT INTO curation_image (fk_submission_file_id, fk_curation_image_type_id, filename) 
		VALUES (?, (SELECT id FROM curation_image_type WHERE name = ?), ?)`,
		c.SubmissionFileID, c.Type, c.Filename)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetCurationImagesBySubmissionFileID return images for a given submission file ID
func (d *mysqlDAL) GetCurationImagesBySubmissionFileID(dbs DBSession, sfid int64) ([]*types.CurationImage, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT id, (SELECT name FROM curation_image_type WHERE id = fk_curation_image_type_id), filename
		FROM curation_image
		WHERE fk_submission_file_id = ?`,
		sfid)
	if err != nil {
		return nil, err
	}

	var result = make([]*types.CurationImage, 0)
	for rows.Next() {
		c := &types.CurationImage{SubmissionFileID: sfid}
		err := rows.Scan(&c.ID, &c.Type, &c.Filename)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}

	return result, nil
}

// GetCurationImage returns curation image
func (d *mysqlDAL) GetCurationImage(dbs DBSession, ciid int64) (*types.CurationImage, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT fk_submission_file_id, (SELECT name FROM curation_image_type WHERE id = fk_curation_image_type_id), filename
		FROM curation_image
		WHERE id = ?`,
		ciid)

	ci := &types.CurationImage{ID: ciid}

	err := row.Scan(&ci.SubmissionFileID, &ci.Type, &ci.Filename)
	if err != nil {
		return nil, err
	}

	return ci, nil
}

// GetNextSubmission returns ID of next submission that's not deleted
func (d *mysqlDAL) GetNextSubmission(dbs DBSession, sid int64) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT id
		FROM submission
		WHERE id > ? AND deleted_at IS NULL
		ORDER BY id
		LIMIT 1`,
		sid)

	var nsid int64

	err := row.Scan(&nsid)
	if err != nil {
		return 0, err
	}

	return nsid, nil
}

// GetPreviousSubmission returns ID of previous submission that's not deleted
func (d *mysqlDAL) GetPreviousSubmission(dbs DBSession, sid int64) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT id
		FROM submission
		WHERE id < ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT 1`,
		sid)

	var psid int64

	err := row.Scan(&psid)
	if err != nil {
		return 0, err
	}

	return psid, nil
}

// ClearMasterDBGames clears the masterdb metadata table
func (d *mysqlDAL) ClearMasterDBGames(dbs DBSession) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `DELETE FROM masterdb_game`)
	return err
}

// StoreMasterDBGames stores games into the masterdb metadata table
func (d *mysqlDAL) StoreMasterDBGames(dbs DBSession, games []*types.MasterDatabaseGame) error {
	if len(games) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(games)*21)
	for _, g := range games {
		data = append(data, g.UUID, g.Title, g.AlternateTitles, g.Series, g.Developer, g.Publisher, g.Platform,
			g.Extreme, g.PlayMode, g.Status, g.GameNotes, g.Source, g.LaunchCommand, g.ReleaseDate,
			g.Version, g.OriginalDescription, g.Languages, g.Library, g.Tags, g.DateAdded.Unix(), g.DateModified.Unix())
	}

	const valuePlaceholder = `(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := dbs.Tx().ExecContext(dbs.Ctx(),
		`INSERT IGNORE INTO masterdb_game (uuid, title, alternate_titles, series, developer, publisher, platform, extreme, play_mode, status, game_notes, source, launch_command, release_date, version, original_description, languages, library, tags, date_added, date_modified) VALUES 
		`+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(games)-1),
		data...)
	return err
}

// GetAllSimilarityAttributes returns IDs, titles and, launch commands
func (d *mysqlDAL) GetAllSimilarityAttributes(dbs DBSession) ([]*types.SimilarityAttributes, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT CONCAT(submission.id), meta.title, meta.launch_command FROM submission
		LEFT JOIN submission_cache ON submission_cache.fk_submission_id = submission.id
		LEFT JOIN submission_file AS newest_file ON newest_file.id = submission_cache.fk_newest_file_id
		LEFT JOIN curation_meta meta ON meta.fk_submission_file_id = newest_file.id
		WHERE submission.deleted_at IS NULL
	UNION 
		SELECT uuid, title, launch_command from masterdb_game`)

	if err != nil {
		return nil, err
	}

	var result = make([]*types.SimilarityAttributes, 0, 100000)
	for rows.Next() {
		lc := &types.SimilarityAttributes{}
		err := rows.Scan(&lc.ID, &lc.Title, &lc.LaunchCommand)
		if err != nil {
			return nil, err
		}
		result = append(result, lc)
	}

	return result, nil
}

// StoreFlashfreezeRootFile stores flashfreeze root file
func (d *mysqlDAL) StoreFlashfreezeRootFile(dbs DBSession, s *types.FlashfreezeFile) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO flashfreeze_file (fk_user_id, original_filename, current_filename, size, created_at, md5sum, sha256sum) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.UserID, s.OriginalFilename, s.CurrentFilename, s.Size, s.UploadedAt.Unix(), s.MD5Sum, s.SHA256Sum)
	if err != nil {
		return 0, err
	}
	fid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return fid, nil
}

// StoreFlashfreezeDeepFile stores data about indexed flashfreeze uploads
func (d *mysqlDAL) StoreFlashfreezeDeepFile(dbs DBSession, fid int64, entries []*types.IndexedFileEntry) error {
	if len(entries) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(entries)*7)
	for _, ife := range entries {
		data = append(data, fid, ife.Name, ife.SizeCompressed, ife.SizeUncompressed, ife.MD5, ife.SHA256, ife.FileUtilOutput)
	}

	const valuePlaceholder = `(?, ?, ?, ?, ?, ?, ?)`
	_, err := dbs.Tx().ExecContext(dbs.Ctx(),
		`INSERT INTO flashfreeze_file_contents (fk_flashfreeze_file_id, filename, size_compressed, size_uncompressed, md5sum, sha256sum, description) VALUES 
		`+valuePlaceholder+strings.Repeat(`,`+valuePlaceholder, len(entries)-1),
		data...)
	return err
}

// UpdateFlashfreezeRootFileIndexedState marks submission file as deleted
func (d *mysqlDAL) UpdateFlashfreezeRootFileIndexedState(dbs DBSession, fid int64, indexedAt *time.Time, indexingErrors uint64) error {

	if indexedAt != nil {
		_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE flashfreeze_file SET indexed_at = ?, indexing_errors = ? WHERE id = ?`,
			indexedAt.Unix(), indexingErrors, fid)
		return err
	}

	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE flashfreeze_file SET indexed_at = NULL, indexing_errors = NULL WHERE id = ?`,
		fid)
	return err
}

// GetFlashfreezeRootFile returns flashfreeze root file
func (d *mysqlDAL) GetFlashfreezeRootFile(dbs DBSession, fid int64) (*types.FlashfreezeFile, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT fk_user_id, original_filename, current_filename, size, created_at, md5sum, sha256sum
		FROM flashfreeze_file
		WHERE id = ?`,
		fid)

	ff := &types.FlashfreezeFile{ID: fid}

	var uploadedAt int64

	err := row.Scan(&ff.UserID, &ff.OriginalFilename, &ff.CurrentFilename, &ff.Size, &uploadedAt, &ff.MD5Sum, &ff.SHA256Sum)
	if err != nil {
		return nil, err
	}

	ff.UploadedAt = time.Unix(uploadedAt, 0)

	return ff, nil
}

// GetAllFlashfreezeRootFiles returns all flashfreeze root files
func (d *mysqlDAL) GetAllFlashfreezeRootFiles(dbs DBSession) ([]*types.FlashfreezeFile, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT id, fk_user_id, original_filename, current_filename, size, created_at, md5sum, sha256sum
		FROM flashfreeze_file`)

	if err != nil {
		return nil, err
	}

	var result = make([]*types.FlashfreezeFile, 0, 100000)
	for rows.Next() {
		var uploadedAt int64
		ff := &types.FlashfreezeFile{}
		err := rows.Scan(&ff.ID, &ff.UserID, &ff.OriginalFilename, &ff.CurrentFilename, &ff.Size, &uploadedAt, &ff.MD5Sum, &ff.SHA256Sum)
		if err != nil {
			return nil, err
		}

		ff.UploadedAt = time.Unix(uploadedAt, 0)

		result = append(result, ff)
	}

	return result, nil
}

// GetAllUnindexedFlashfreezeRootFiles returns all flashfreeze root files which are not indexed
func (d *mysqlDAL) GetAllUnindexedFlashfreezeRootFiles(dbs DBSession) ([]*types.FlashfreezeFile, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT id, fk_user_id, original_filename, current_filename, size, created_at, md5sum, sha256sum
		FROM flashfreeze_file
		WHERE id NOT IN (
		    SELECT DISTINCT fk_flashfreeze_file_id 
		    FROM flashfreeze_file_contents
		    WHERE current_filename NOT LIKE "%.warc" AND current_filename NOT LIKE "%.warc.gz"
		    )`)

	if err != nil {
		return nil, err
	}

	var result = make([]*types.FlashfreezeFile, 0, 100000)
	for rows.Next() {
		var uploadedAt int64
		ff := &types.FlashfreezeFile{}
		err := rows.Scan(&ff.ID, &ff.UserID, &ff.OriginalFilename, &ff.CurrentFilename, &ff.Size, &uploadedAt, &ff.MD5Sum, &ff.SHA256Sum)
		if err != nil {
			return nil, err
		}

		ff.UploadedAt = time.Unix(uploadedAt, 0)

		result = append(result, ff)
	}

	return result, nil
}

// StoreFixFirstStep creates fix entry in DB with basic info
func (d *mysqlDAL) StoreFixFirstStep(dbs DBSession, uid int64, c *types.CreateFixFirstStep) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO fixes (fk_user_id, fk_fix_type_id, submit_finished, title, description, created_at) 
                           VALUES (?, (SELECT id FROM fix_type WHERE fix_type.name=?), false, ?, ?, UNIX_TIMESTAMP())`,
		uid, c.FixType, c.Title, c.Description)
	if err != nil {
		return 0, err
	}
	fid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return fid, nil
}

// GetFixByID returns a fix
func (d *mysqlDAL) GetFixByID(dbs DBSession, fid int64) (*types.Fix, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT fk_user_id, (SELECT name FROM fix_type WHERE id=fixes.fk_fix_type_id), submit_finished, title, description, created_at
		FROM fixes
		WHERE id = ?`,
		fid)

	f := &types.Fix{}
	var createdAt int64
	if err := row.Scan(&f.AuthorID, &f.FixType, &f.SubmitFinished, &f.Title, &f.Description, &createdAt); err != nil {
		return nil, err
	}

	f.CreatedAt = time.Unix(createdAt, 0)

	return f, nil
}

// StoreFixesFile stores fixes file
func (d *mysqlDAL) StoreFixesFile(dbs DBSession, s *types.FixesFile) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO fixes_file (fk_user_id, fk_fix_id, original_filename, current_filename, size, created_at, md5sum, sha256sum) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.UserID, s.FixID, s.OriginalFilename, s.CurrentFilename, s.Size, s.UploadedAt.Unix(), s.MD5Sum, s.SHA256Sum)
	if err != nil {
		return 0, err
	}
	fid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return fid, nil
}

// GetFilesForFix returns all files of a given fix
func (d *mysqlDAL) GetFilesForFix(dbs DBSession, fid int64) ([]*types.ExtendedFixesFile, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT fixes_file.id, fk_user_id, discord_user.username, fk_fix_id, original_filename, current_filename, size, created_at, md5sum, sha256sum
		FROM fixes_file 
		LEFT JOIN discord_user ON discord_user.id = fk_user_id
		WHERE fk_fix_id=?`,
		fid)
	if err != nil {
		return nil, err
	}

	var result = make([]*types.ExtendedFixesFile, 0, 2)
	for rows.Next() {
		var uploadedAt int64
		ff := &types.ExtendedFixesFile{}
		err := rows.Scan(&ff.ID, &ff.UserID, &ff.UploadedBy, &ff.FixID, &ff.OriginalFilename, &ff.CurrentFilename, &ff.Size, &uploadedAt, &ff.MD5Sum, &ff.SHA256Sum)
		if err != nil {
			return nil, err
		}

		ff.UploadedAt = time.Unix(uploadedAt, 0)

		result = append(result, ff)
	}

	return result, nil
}

// DeleteUserSessions deletes all sessions of a given user, including inactive sessions
func (d *mysqlDAL) DeleteUserSessions(dbs DBSession, uid int64) (int64, error) {
	r, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		DELETE FROM session WHERE uid=?`,
		uid)
	if err != nil {
		return 0, err
	}

	count, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetTotalCommentsCount returns a total number of comments in the system
func (d *mysqlDAL) GetTotalCommentsCount(dbs DBSession) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*) FROM comment`)

	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// GetTotalUserCount returns a total number of users in the system
func (d *mysqlDAL) GetTotalUserCount(dbs DBSession) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*) FROM discord_user`)

	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// GetTotalFlashfreezeCount returns a total number of indexed flashfreeze files in the system
func (d *mysqlDAL) GetTotalFlashfreezeCount(dbs DBSession) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*) FROM flashfreeze_file`)

	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// GetTotalFlashfreezeFileCount returns a total number of indexed flashfreeze files in the system
func (d *mysqlDAL) GetTotalFlashfreezeFileCount(dbs DBSession) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*) FROM flashfreeze_file_contents`)

	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// GetTotalSubmissionFilesize returns a total size of all uploaded submissions
func (d *mysqlDAL) GetTotalSubmissionFilesize(dbs DBSession) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT SUM(size) FROM submission_file`)

	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// GetTotalFlashfreezeFilesize returns a total size of all uploaded flashfreeze items
func (d *mysqlDAL) GetTotalFlashfreezeFilesize(dbs DBSession) (int64, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT SUM(size) FROM flashfreeze_file`)

	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

// GetFixesFiles gets fixes files, returns error if input len != output len
func (d *mysqlDAL) GetFixesFiles(dbs DBSession, ffids []int64) ([]*types.FixesFile, error) {
	if len(ffids) == 0 {
		return nil, nil
	}

	data := make([]interface{}, len(ffids))
	for i, d := range ffids {
		data[i] = d
	}

	q := `
		SELECT fk_user_id, fk_fix_id, original_filename, current_filename, size, created_at, md5sum, sha256sum 
		FROM fixes_file 
		WHERE id IN(?` + strings.Repeat(",?", len(ffids)-1) + `)
		AND deleted_at IS NULL
		ORDER BY created_at DESC`

	var rows *sql.Rows
	var err error
	rows, err = dbs.Tx().QueryContext(dbs.Ctx(), q, data...)
	if err != nil {
		return nil, err
	}

	var result = make([]*types.FixesFile, 0, len(ffids))
	for rows.Next() {
		sf := &types.FixesFile{}
		var uploadedAt int64
		err := rows.Scan(&sf.UserID, &sf.FixID, &sf.OriginalFilename, &sf.CurrentFilename, &sf.Size, &uploadedAt, &sf.MD5Sum, &sf.SHA256Sum)
		if err != nil {
			return nil, err
		}
		sf.UploadedAt = time.Unix(uploadedAt, 0)
		result = append(result, sf)
	}

	if len(result) != len(ffids) {
		return nil, fmt.Errorf("%d files were not found", len(result)-len(ffids))
	}

	return result, nil
}

// GetUsers returns all users
func (d *mysqlDAL) GetUsers(dbs DBSession) ([]*types.User, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `SELECT id, username FROM discord_user`)
	if err != nil {
		return nil, err
	}

	var result = make([]*types.User, 0)
	for rows.Next() {
		u := &types.User{}
		var uid int64
		err := rows.Scan(&uid, &u.Username)
		if err != nil {
			return nil, err
		}

		u.ID = fmt.Sprintf("%d", uid)

		result = append(result, u)
	}

	return result, nil
}

// GetCommentsByUserIDAndAction returns comments with an oddly specific filter
func (d *mysqlDAL) GetCommentsByUserIDAndAction(dbs DBSession, uid int64, action string) ([]*types.Comment, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT id, message, created_at FROM
		(
			SELECT id, message, (SELECT name FROM action WHERE id=comment.fk_action_id) as action, created_at 
			FROM comment 
			WHERE fk_user_id=? 
			AND comment.deleted_at IS NULL
		) as t
		WHERE action=?
		ORDER BY created_at DESC;`, uid, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*types.Comment, 0)

	var createdAt int64

	for rows.Next() {

		c := &types.Comment{AuthorID: uid, Action: action}
		if err := rows.Scan(&c.ID, &c.Message, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(createdAt, 0)
		result = append(result, c)
	}

	return result, nil
}
