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
	"strconv"
	"strings"
	"time"
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
		utils.LogIfErr(dbs.Ctx(), err)
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
	var row *sql.Row
	row = dbs.Tx().QueryRowContext(dbs.Ctx(), `SELECT uid, expires_at FROM session WHERE secret=?`, key)

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
	if len(roles) == 0 {
		return nil
	}
	data := make([]interface{}, 0, len(roles)*3)
	for _, role := range roles {
		data = append(data, uid, role)
	}

	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `DELETE FROM discord_user_role WHERE fk_uid = ?`, uid)
	if err != nil {
		return err
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
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// StoreSubmissionFile stores submission file
func (d *mysqlDAL) StoreSubmissionFile(dbs DBSession, s *types.SubmissionFile) (int64, error) {
	res, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO submission_file (fk_uploader_id, fk_submission_id, original_filename, current_filename, size, uploaded_at, md5sum, sha256sum) 
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
func (d *mysqlDAL) GetSubmissionFiles(dbs DBSession, sfids []int64) ([]*types.SubmissionFile, error) {
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
		return nil, fmt.Errorf("%s files were not found", len(result)-len(sfids))
	}

	return result, nil
}

// GetExtendedSubmissionFilesBySubmissionID returns all extended submission files for a given submission
func (d *mysqlDAL) GetExtendedSubmissionFilesBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedSubmissionFile, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
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
func (d *mysqlDAL) SearchSubmissions(dbs DBSession, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	filters := make([]string, 0)
	data := make([]interface{}, 0)

	uid := utils.UserIDFromContext(dbs.Ctx()) // TODO this should be passed as param
	data = append(data, constants.ValidatorID, constants.ValidatorID, uid, uid, constants.ValidatorID, uid,
		constants.ValidatorID, constants.ValidatorID, constants.ValidatorID, constants.ValidatorID)

	const defaultLimit int64 = 100
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
		if filter.LibraryPartial != nil {
			filters = append(filters, "meta.library LIKE ?")
			data = append(data, utils.FormatLike(*filter.LibraryPartial))
		}
		if filter.OriginalFilenamePartialAny != nil {
			filters = append(filters, "filenames.original_filename_sequence LIKE ?")
			data = append(data, utils.FormatLike(*filter.OriginalFilenamePartialAny))
		}
		if filter.CurrentFilenamePartialAny != nil {
			filters = append(filters, "filenames.current_filename_sequence LIKE ?")
			data = append(data, utils.FormatLike(*filter.CurrentFilenamePartialAny))
		}
		if filter.MD5SumPartialAny != nil {
			filters = append(filters, "filenames.md5sum_sequence LIKE ?")
			data = append(data, utils.FormatLike(*filter.MD5SumPartialAny))
		}
		if filter.SHA256SumPartialAny != nil {
			filters = append(filters, "filenames.sha256sum_sequence LIKE ?")
			data = append(data, utils.FormatLike(*filter.SHA256SumPartialAny))
		}
		if len(filter.BotActions) != 0 {
			filters = append(filters, `bot_comment.action IN(?`+strings.Repeat(",?", len(filter.BotActions)-1)+`)`)
			for _, ba := range filter.BotActions {
				data = append(data, ba)
			}
		}
		if len(filter.SubmissionLevels) != 0 {
			filters = append(filters, `(SELECT name FROM submission_level WHERE id = submission.fk_submission_level_id) IN(?`+strings.Repeat(",?", len(filter.SubmissionLevels)-1)+`)`)
			for _, ba := range filter.SubmissionLevels {
				data = append(data, ba)
			}
		}
		if len(filter.ActionsAfterMyLastComment) != 0 {
			foundAny := false
			for _, aamlc := range filter.ActionsAfterMyLastComment {
				if aamlc == "any" {
					foundAny = true
				}
			}
			if foundAny {
				filters = append(filters, `actions_after_my_last_comment.user_action_string IS NOT NULL`)
			} else {
				filters = append(filters, `REGEXP_LIKE (actions_after_my_last_comment.user_action_string, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.ActionsAfterMyLastComment)-1)+`))`)
				for _, aamlc := range filter.ActionsAfterMyLastComment {
					data = append(data, aamlc)
				}
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
		if filter.AssignedStatus != nil {
			if *filter.AssignedStatus == "unassigned" {
				filters = append(filters, "(active_assigned.user_count_with_enabled_action = 0 OR active_assigned.user_count_with_enabled_action IS NULL)")
			} else if *filter.AssignedStatus == "assigned" {
				filters = append(filters, "active_assigned.user_count_with_enabled_action > 0")
			}
		}
		if filter.RequestedChangedStatus != nil {
			if *filter.RequestedChangedStatus == "none" {
				filters = append(filters, "(active_requested_changes.user_count_with_enabled_action = 0 OR active_requested_changes.user_count_with_enabled_action IS NULL)")
			} else if *filter.RequestedChangedStatus == "ongoing" {
				filters = append(filters, "active_requested_changes.user_count_with_enabled_action > 0")
			}
		}
		if filter.ApprovalsStatus != nil {
			if *filter.ApprovalsStatus == "no" {
				filters = append(filters, "(active_approved.user_count_with_enabled_action = 0 OR active_approved.user_count_with_enabled_action IS NULL)")
			} else if *filter.ApprovalsStatus == "yes" {
				filters = append(filters, "active_approved.user_count_with_enabled_action > 0")
			}
		}
	}

	data = append(data, currentLimit, currentOffset)

	and := ""
	if len(filters) > 0 {
		and = " AND "
	}

	finalQuery := `
		SELECT submission.id AS submission_id,
			(SELECT name FROM submission_level WHERE id = submission.fk_submission_level_id) AS submission_level,
			uploader.id AS uploader_id,
			uploader.username AS uploader_username,
			uploader.avatar AS uploader_avatar,
			updater.id AS updater_id,
			updater.username AS updater_username,
			updater.avatar AS updater_avatar,
			newest.id AS submission_file_id,
			newest.original_filename,
			newest.current_filename,
			newest.size,
			oldest.uploaded_at,
			newest.updated_at,
			meta.title,
			meta.alternate_titles,
			meta.platform,
			meta.launch_command,
			meta.library,
			bot_comment.action AS bot_action,
			latest_action.action AS latest_action,
			file_counter.file_count,
			active_assigned.user_ids_with_enabled_action AS assigned_user_ids,
			active_requested_changes.user_ids_with_enabled_action AS requested_changes_user_ids,
			active_approved.user_ids_with_enabled_action AS approved_user_ids
		FROM submission
			LEFT JOIN (
				WITH ranked_file AS (
					SELECT s.*,
						ROW_NUMBER() OVER (
							PARTITION BY fk_submission_id
							ORDER BY uploaded_at ASC
						) AS rn
					FROM submission_file AS s
					WHERE s.deleted_at IS NULL
				)
				SELECT fk_uploader_id AS uploader_id,
					fk_submission_id,
					uploaded_at AS uploaded_at
				FROM ranked_file
				WHERE rn = 1
			) AS oldest ON oldest.fk_submission_id = submission.id
			LEFT JOIN (
				WITH ranked_file AS (
					SELECT s.*,
						ROW_NUMBER() OVER (
							PARTITION BY fk_submission_id
							ORDER BY uploaded_at DESC
						) AS rn
					FROM submission_file AS s
					WHERE s.deleted_at IS NULL
				)
				SELECT id,
					fk_uploader_id AS updater_id,
					fk_submission_id,
					original_filename,
					current_filename,
					size,
					uploaded_at AS updated_at
				FROM ranked_file
				WHERE rn = 1
			) AS newest ON newest.fk_submission_id = submission.id
			LEFT JOIN discord_user uploader ON oldest.uploader_id = uploader.id
			LEFT JOIN discord_user updater ON newest.updater_id = updater.id
			LEFT JOIN curation_meta meta ON meta.fk_submission_file_id = newest.id
			LEFT JOIN (
				WITH ranked_comment AS (
					SELECT c.*,
						ROW_NUMBER() OVER (
							PARTITION BY fk_submission_id
							ORDER BY created_at DESC
						) AS rn
					FROM comment AS c
					WHERE c.fk_author_id = ?
						AND c.deleted_at IS NULL
				)
				SELECT ranked_comment.fk_submission_id AS submission_id,
		(
						SELECT name
						FROM action
						WHERE action.id = ranked_comment.fk_action_id
					) AS action
				FROM ranked_comment
				WHERE rn = 1
			) AS bot_comment ON bot_comment.submission_id = submission.id
			LEFT JOIN (
				WITH ranked_comment AS (
					SELECT c.*,
						ROW_NUMBER() OVER (
							PARTITION BY fk_submission_id
							ORDER BY created_at DESC
						) AS rn
					FROM comment AS c
					WHERE c.fk_author_id != ?
						AND c.fk_action_id != (
							SELECT id
							FROM action
							WHERE name = "comment"
						)
						AND c.deleted_at IS NULL
				)
				SELECT ranked_comment.fk_submission_id AS submission_id,
					ranked_comment.created_at,
		(
						SELECT name
						FROM action
						WHERE action.id = ranked_comment.fk_action_id
					) AS action
				FROM ranked_comment
				WHERE rn = 1
			) AS latest_action ON latest_action.submission_id = submission.id
			LEFT JOIN (
				SELECT fk_submission_id,
					COUNT(id) AS file_count
				FROM submission_file
				WHERE submission_file.deleted_at IS NULL
				GROUP BY fk_submission_id
			) AS file_counter ON file_counter.fk_submission_id = submission.id
			LEFT JOIN (
				SELECT *,
					SUBSTRING(
						full_substring
						FROM POSITION(',' IN full_substring)
					) AS user_action_string
				FROM (
						SELECT *,
							SUBSTRING(
								comment_sequence
								FROM comment_sequence_substring_start
							) AS full_substring
						FROM (
								SELECT *,
									(
										SELECT fk_action_id
										FROM comment
										WHERE id = comment_id
									) AS fk_action_id,
									CHAR_LENGTH(comment_sequence) - LOCATE(REVERSE(CONCAT(?)), REVERSE(comment_sequence)) - CHAR_LENGTH(CONCAT(?)) + 2 as comment_sequence_substring_start
								FROM (
										SELECT MAX(id) AS comment_id,
											fk_submission_id,
											GROUP_CONCAT(
												CONCAT(
													fk_author_id,
													'-',
													(
														SELECT name
														FROM action
														WHERE action.id = fk_action_id
													)
												)
											) AS comment_sequence
										FROM comment
										WHERE fk_author_id != ?
											AND deleted_at IS NULL
											AND fk_action_id != (
												SELECT id
												FROM action
												WHERE name = "assign"
											)
											AND fk_action_id != (
												SELECT id
												FROM action
												WHERE name = "unassign"
											)
										GROUP BY fk_submission_id
										ORDER BY created_at DESC
									) as a
							) as b
					) as c
				WHERE REGEXP_LIKE(
						SUBSTRING(
							comment_sequence
							FROM comment_sequence_substring_start
						),
						CONCAT(CONCAT(?), '-\\S+,\\d+-\\S+')
					)
			) AS actions_after_my_last_comment ON actions_after_my_last_comment.fk_submission_id = submission.id
			LEFT JOIN (
				SELECT fk_submission_id,
					GROUP_CONCAT(original_filename) AS original_filename_sequence,
					GROUP_CONCAT(current_filename) AS current_filename_sequence,
					GROUP_CONCAT(md5sum) AS md5sum_sequence,
					GROUP_CONCAT(sha256sum) AS sha256sum_sequence
				FROM submission_file
				WHERE deleted_at IS NULL
				GROUP BY fk_submission_id
			) AS filenames ON filenames.fk_submission_id = submission.id
			LEFT JOIN (
				SELECT submission.id AS submission,
					COUNT(latest_enabler.author_id) AS user_count_with_enabled_action,
					GROUP_CONCAT(latest_enabler.author_id) AS user_ids_with_enabled_action,
					GROUP_CONCAT(
						(
							SELECT username
							FROM discord_user
							WHERE id = latest_enabler.author_id
						)
					) AS usernames_with_enabled_action
				FROM submission
					LEFT JOIN (
						WITH ranked_comment AS (
							SELECT c.*,
								ROW_NUMBER() OVER (
									PARTITION BY c.fk_submission_id,
									c.fk_author_id
									ORDER BY created_at DESC
								) AS rn
							FROM comment AS c
							WHERE c.fk_action_id = (
									SELECT id
									FROM action
									WHERE name = "assign"
								)
								AND c.deleted_at IS NULL
							ORDER BY created_at ASC
						)
						SELECT ranked_comment.fk_submission_id AS submission_id,
							ranked_comment.fk_author_id AS author_id,
							ranked_comment.created_at
						FROM ranked_comment
						WHERE rn = 1
					) AS latest_enabler ON latest_enabler.submission_id = submission.id
					LEFT JOIN (
						WITH ranked_comment AS (
							SELECT c.*,
								ROW_NUMBER() OVER (
									PARTITION BY c.fk_submission_id,
									c.fk_author_id
									ORDER BY created_at DESC
								) AS rn
							FROM comment AS c
							WHERE c.fk_action_id = (
									SELECT id
									FROM action
									WHERE name = "unassign"
								)
								AND c.deleted_at IS NULL
						)
						SELECT ranked_comment.fk_submission_id AS submission_id,
							ranked_comment.fk_author_id AS author_id,
							ranked_comment.created_at
						FROM ranked_comment
						WHERE rn = 1
					) AS latest_disabler ON latest_disabler.submission_id = submission.id
					AND latest_disabler.author_id = latest_enabler.author_id
				WHERE (
						(
							latest_enabler.created_at IS NOT NULL
							AND latest_disabler.created_at IS NOT NULL
							AND latest_enabler.created_at > latest_disabler.created_at
						)
						OR (
							latest_disabler.created_at IS NULL
							AND latest_enabler.created_at IS NOT NULL
						)
					)
				GROUP BY submission.id
			) AS active_assigned ON active_assigned.submission = submission.id
			LEFT JOIN (
				SELECT submission.id AS submission,
					COUNT(latest_enabler.author_id) AS user_count_with_enabled_action,
					GROUP_CONCAT(latest_enabler.author_id) AS user_ids_with_enabled_action,
					GROUP_CONCAT(
						(
							SELECT username
							FROM discord_user
							WHERE id = latest_enabler.author_id
						)
					) AS usernames_with_enabled_action
				FROM submission
					LEFT JOIN (
						WITH ranked_comment AS (
							SELECT c.*,
								ROW_NUMBER() OVER (
									PARTITION BY c.fk_submission_id,
									c.fk_author_id
									ORDER BY created_at DESC
								) AS rn
							FROM comment AS c
							WHERE c.fk_author_id != ?
								AND c.fk_action_id = (
									SELECT id
									FROM action
									WHERE name = "request-changes"
								)
								AND c.deleted_at IS NULL
							ORDER BY created_at ASC
						)
						SELECT ranked_comment.fk_submission_id AS submission_id,
							ranked_comment.fk_author_id AS author_id,
							ranked_comment.created_at
						FROM ranked_comment
						WHERE rn = 1
					) AS latest_enabler ON latest_enabler.submission_id = submission.id
					LEFT JOIN (
						WITH ranked_comment AS (
							SELECT c.*,
								ROW_NUMBER() OVER (
									PARTITION BY c.fk_submission_id,
									c.fk_author_id
									ORDER BY created_at DESC
								) AS rn
							FROM comment AS c
							WHERE c.fk_author_id != ?
								AND c.fk_action_id = (
									SELECT id
									FROM action
									WHERE name = "approve"
								)
								AND c.deleted_at IS NULL
						)
						SELECT ranked_comment.fk_submission_id AS submission_id,
							ranked_comment.fk_author_id AS author_id,
							ranked_comment.created_at
						FROM ranked_comment
						WHERE rn = 1
					) AS latest_disabler ON latest_disabler.submission_id = submission.id
					AND latest_disabler.author_id = latest_enabler.author_id
				WHERE (
						(
							latest_enabler.created_at IS NOT NULL
							AND latest_disabler.created_at IS NOT NULL
							AND latest_enabler.created_at > latest_disabler.created_at
						)
						OR (
							latest_disabler.created_at IS NULL
							AND latest_enabler.created_at IS NOT NULL
						)
					)
				GROUP BY submission.id
			) AS active_requested_changes ON active_requested_changes.submission = submission.id
			LEFT JOIN (
				SELECT submission.id AS submission,
					COUNT(latest_enabler.author_id) AS user_count_with_enabled_action,
					GROUP_CONCAT(latest_enabler.author_id) AS user_ids_with_enabled_action,
					GROUP_CONCAT(
						(
							SELECT username
							FROM discord_user
							WHERE id = latest_enabler.author_id
						)
					) AS usernames_with_enabled_action
				FROM submission
					LEFT JOIN (
						WITH ranked_comment AS (
							SELECT c.*,
								ROW_NUMBER() OVER (
									PARTITION BY c.fk_submission_id,
									c.fk_author_id
									ORDER BY created_at DESC
								) AS rn
							FROM comment AS c
							WHERE c.fk_author_id != ?
								AND c.fk_action_id = (
									SELECT id
									FROM action
									WHERE name = "approve"
								)
								AND c.deleted_at IS NULL
							ORDER BY created_at ASC
						)
						SELECT ranked_comment.fk_submission_id AS submission_id,
							ranked_comment.fk_author_id AS author_id,
							ranked_comment.created_at
						FROM ranked_comment
						WHERE rn = 1
					) AS latest_enabler ON latest_enabler.submission_id = submission.id
					LEFT JOIN (
						WITH ranked_comment AS (
							SELECT c.*,
								ROW_NUMBER() OVER (
									PARTITION BY c.fk_submission_id,
									c.fk_author_id
									ORDER BY created_at DESC
								) AS rn
							FROM comment AS c
							WHERE c.fk_author_id != ?
								AND c.fk_action_id = (
									SELECT id
									FROM action
									WHERE name = "request-changes"
								)
								AND c.deleted_at IS NULL
						)
						SELECT ranked_comment.fk_submission_id AS submission_id,
							ranked_comment.fk_author_id AS author_id,
							ranked_comment.created_at
						FROM ranked_comment
						WHERE rn = 1
					) AS latest_disabler ON latest_disabler.submission_id = submission.id
					AND latest_disabler.author_id = latest_enabler.author_id
				WHERE (
						(
							latest_enabler.created_at IS NOT NULL
							AND latest_disabler.created_at IS NOT NULL
							AND latest_enabler.created_at > latest_disabler.created_at
						)
						OR (
							latest_disabler.created_at IS NULL
							AND latest_enabler.created_at IS NOT NULL
						)
					)
				GROUP BY submission.id
			) AS active_approved ON active_approved.submission = submission.id
		WHERE submission.deleted_at IS NULL` + and + strings.Join(filters, " AND ") + `
		GROUP BY submission.id
		ORDER BY newest.updated_at DESC
		` + limit + ` ` + offset

	//fmt.Printf(finalQuery)

	var rows *sql.Rows
	var err error
	rows, err = dbs.Tx().QueryContext(dbs.Ctx(), finalQuery, data...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*types.ExtendedSubmission, 0)

	var uploadedAt int64
	var updatedAt int64
	var submitterAvatar string
	var updaterAvatar string
	var assignedUserIDs *string
	var requestedChangesUserIDs *string
	var approvedUserIDs *string

	for rows.Next() {
		s := &types.ExtendedSubmission{}
		if err := rows.Scan(
			&s.SubmissionID,
			&s.SubmissionLevel,
			&s.SubmitterID, &s.SubmitterUsername, &submitterAvatar,
			&s.UpdaterID, &s.UpdaterUsername, &updaterAvatar,
			&s.FileID, &s.OriginalFilename, &s.CurrentFilename, &s.Size,
			&uploadedAt, &updatedAt,
			&s.CurationTitle, &s.CurationAlternateTitles, &s.CurationPlatform, &s.CurationLaunchCommand, &s.CurationLibrary,
			&s.BotAction,
			&s.LatestAction,
			&s.FileCount,
			&assignedUserIDs, &requestedChangesUserIDs, &approvedUserIDs); err != nil {
			return nil, err
		}
		s.SubmitterAvatarURL = utils.FormatAvatarURL(s.SubmitterID, submitterAvatar)
		s.UpdaterAvatarURL = utils.FormatAvatarURL(s.UpdaterID, updaterAvatar)
		s.UploadedAt = time.Unix(uploadedAt, 0)
		s.UpdatedAt = time.Unix(updatedAt, 0)

		s.AssignedUserIDs = []int64{}
		if assignedUserIDs != nil && len(*assignedUserIDs) > 0 {
			userIDs := strings.Split(*assignedUserIDs, ",")
			for _, userID := range userIDs {
				uid, err := strconv.ParseInt(userID, 10, 64)
				if err != nil {
					return nil, err
				}
				s.AssignedUserIDs = append(s.AssignedUserIDs, uid)
			}
		}

		s.RequestedChangesUserIDs = []int64{}
		if requestedChangesUserIDs != nil && len(*requestedChangesUserIDs) > 0 {
			userIDs := strings.Split(*requestedChangesUserIDs, ",")
			for _, userID := range userIDs {
				uid, err := strconv.ParseInt(userID, 10, 64)
				if err != nil {
					return nil, err
				}
				s.RequestedChangesUserIDs = append(s.RequestedChangesUserIDs, uid)
			}
		}

		s.ApprovedUserIDs = []int64{}
		if approvedUserIDs != nil && len(*approvedUserIDs) > 0 {
			userIDs := strings.Split(*approvedUserIDs, ",")
			for _, userID := range userIDs {
				uid, err := strconv.ParseInt(userID, 10, 64)
				if err != nil {
					return nil, err
				}
				s.ApprovedUserIDs = append(s.ApprovedUserIDs, uid)
			}
		}

		result = append(result, s)
	}

	return result, nil
}

// StoreCurationMeta stores curation meta
func (d *mysqlDAL) StoreCurationMeta(dbs DBSession, cm *types.CurationMeta) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO curation_meta (fk_submission_file_id, application_path, developer, extreme, game_notes, languages,
                           launch_command, original_description, play_mode, platform, publisher, release_date, series, source, status,
                           tags, tag_categories, title, alternate_titles, library, version, curation_notes, mount_parameters) 
                           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cm.SubmissionFileID, cm.ApplicationPath, cm.Developer, cm.Extreme, cm.GameNotes, cm.Languages,
		cm.LaunchCommand, cm.OriginalDescription, cm.PlayMode, cm.Platform, cm.Publisher, cm.ReleaseDate, cm.Series, cm.Source, cm.Status,
		cm.Tags, cm.TagCategories, cm.Title, cm.AlternateTitles, cm.Library, cm.Version, cm.CurationNotes, cm.MountParameters)
	return err
}

// GetCurationMetaBySubmissionFileID returns curation meta for given submission file
func (d *mysqlDAL) GetCurationMetaBySubmissionFileID(dbs DBSession, sfid int64) (*types.CurationMeta, error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `SELECT submission_file.fk_submission_id, application_path, developer, extreme, game_notes, languages,
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
func (d *mysqlDAL) StoreComment(dbs DBSession, c *types.Comment) error {
	var msg *string
	if c.Message != nil {
		s := strings.TrimSpace(*c.Message)
		msg = &s
	}
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `INSERT INTO comment (fk_author_id, fk_submission_id, message, fk_action_id, created_at) 
                           VALUES (?, ?, ?, (SELECT id FROM action WHERE name=?), ?)`,
		c.AuthorID, c.SubmissionID, msg, c.Action, c.CreatedAt.Unix())
	return err
}

// GetExtendedCommentsBySubmissionID returns all comments with author data for a given submission
func (d *mysqlDAL) GetExtendedCommentsBySubmissionID(dbs DBSession, sid int64) ([]*types.ExtendedComment, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
		SELECT comment.id, discord_user.id, username, avatar, message, (SELECT name FROM action WHERE id=comment.fk_action_id) as action, created_at 
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

// SoftDeleteSubmissionFile marks submission file as deleted
func (d *mysqlDAL) SoftDeleteSubmissionFile(dbs DBSession, sfid int64) error {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
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

	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_file SET deleted_at = UNIX_TIMESTAMP() 
		WHERE id  = ?`,
		sfid)
	return err
}

// SoftDeleteSubmission marks submission and its files as deleted
func (d *mysqlDAL) SoftDeleteSubmission(dbs DBSession, sid int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_file SET deleted_at = UNIX_TIMESTAMP() 
		WHERE fk_submission_id = ?`,
		sid)
	if err != nil {
		return err
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE comment SET deleted_at = UNIX_TIMESTAMP() 
		WHERE fk_submission_id = ?`,
		sid)
	if err != nil {
		return err
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission SET deleted_at = UNIX_TIMESTAMP() 
		WHERE id = ?`,
		sid)
	return err
}

// SoftDeleteComment marks comment as deleted
func (d *mysqlDAL) SoftDeleteComment(dbs DBSession, cid int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE comment SET deleted_at = UNIX_TIMESTAMP() 
		WHERE id = ?`,
		cid)
	return err
}
