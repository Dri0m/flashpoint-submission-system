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

	err = updateSubmissionCacheTable(dbs, s.SubmissionID)
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
		return nil, fmt.Errorf("%s files were not found", len(result)-len(sfids))
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

// SearchSubmissions returns extended submissions based on given filter
func (d *mysqlDAL) SearchSubmissions(dbs DBSession, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, error) {
	filters := make([]string, 0)
	data := make([]interface{}, 0)

	uid := utils.UserIDFromContext(dbs.Ctx()) // TODO this should be passed as param
	data = append(data, uid, uid)

	const defaultLimit int64 = 100
	const defaultOffset int64 = 0

	currentLimit := defaultLimit
	currentOffset := defaultOffset

	if filter != nil {
		if len(filter.SubmissionIDs) > 0 {
			filters = append(filters, `(submission.id IN(?`+strings.Repeat(`,?`, len(filter.SubmissionIDs)-1)+`))`)
			for _, sid := range filter.SubmissionIDs {
				data = append(data, sid)
			}
		}
		if filter.SubmitterID != nil {
			filters = append(filters, "(uploader.id = ?)")
			data = append(data, *filter.SubmitterID)
		}
		if filter.TitlePartial != nil {
			filters = append(filters, "(meta.title LIKE ? OR meta.alternate_titles LIKE ?)")
			data = append(data, utils.FormatLike(*filter.TitlePartial), utils.FormatLike(*filter.TitlePartial))
		}
		if filter.SubmitterUsernamePartial != nil {
			filters = append(filters, "(uploader.username LIKE ?)")
			data = append(data, utils.FormatLike(*filter.SubmitterUsernamePartial))
		}
		if filter.PlatformPartial != nil {
			filters = append(filters, "(meta.platform LIKE ?)")
			data = append(data, utils.FormatLike(*filter.PlatformPartial))
		}
		if filter.LibraryPartial != nil {
			filters = append(filters, "(meta.library LIKE ?)")
			data = append(data, utils.FormatLike(*filter.LibraryPartial))
		}
		if filter.OriginalFilenamePartialAny != nil {
			filters = append(filters, "(filenames.original_filename_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.OriginalFilenamePartialAny))
		}
		if filter.CurrentFilenamePartialAny != nil {
			filters = append(filters, "(filenames.current_filename_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.CurrentFilenamePartialAny))
		}
		if filter.MD5SumPartialAny != nil {
			filters = append(filters, "(filenames.md5sum_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.MD5SumPartialAny))
		}
		if filter.SHA256SumPartialAny != nil {
			filters = append(filters, "(filenames.sha256sum_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.SHA256SumPartialAny))
		}
		if len(filter.BotActions) != 0 {
			filters = append(filters, `(bot_comment.action IN(?`+strings.Repeat(",?", len(filter.BotActions)-1)+`))`)
			for _, ba := range filter.BotActions {
				data = append(data, ba)
			}
		}
		if len(filter.SubmissionLevels) != 0 {
			filters = append(filters, `((SELECT name FROM submission_level WHERE id = submission.fk_submission_level_id) IN(?`+strings.Repeat(",?", len(filter.SubmissionLevels)-1)+`))`)
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
				filters = append(filters, `(actions_after_my_last_comment.user_action_string IS NOT NULL)`)
			} else {
				filters = append(filters, `(REGEXP_LIKE (actions_after_my_last_comment.user_action_string, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.ActionsAfterMyLastComment)-1)+`)))`)
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
		if filter.AssignedStatusTesting != nil {
			if *filter.AssignedStatusTesting == "unassigned" {
				filters = append(filters, "(active_assigned_testing.user_count_with_enabled_action = 0 OR active_assigned_testing.user_count_with_enabled_action IS NULL)")
			} else if *filter.AssignedStatusTesting == "assigned" {
				filters = append(filters, "(active_assigned_testing.user_count_with_enabled_action > 0)")
			}
		}
		if filter.AssignedStatusVerification != nil {
			if *filter.AssignedStatusVerification == "unassigned" {
				filters = append(filters, "(active_assigned_verification.user_count_with_enabled_action = 0 OR active_assigned_verification.user_count_with_enabled_action IS NULL)")
			} else if *filter.AssignedStatusVerification == "assigned" {
				filters = append(filters, "(active_assigned_verification.user_count_with_enabled_action > 0)")
			}
		}
		if filter.RequestedChangedStatus != nil {
			if *filter.RequestedChangedStatus == "none" {
				filters = append(filters, "(active_requested_changes.user_count_with_enabled_action = 0 OR active_requested_changes.user_count_with_enabled_action IS NULL)")
			} else if *filter.RequestedChangedStatus == "ongoing" {
				filters = append(filters, "(active_requested_changes.user_count_with_enabled_action > 0)")
			}
		}
		if filter.ApprovalsStatus != nil {
			if *filter.ApprovalsStatus == "none" {
				filters = append(filters, "(active_approved.user_count_with_enabled_action = 0 OR active_approved.user_count_with_enabled_action IS NULL)")
			} else if *filter.ApprovalsStatus == "approved" {
				filters = append(filters, "(active_approved.user_count_with_enabled_action > 0)")
			}
		}
		if filter.VerificationStatus != nil {
			if *filter.VerificationStatus == "none" {
				filters = append(filters, "(active_verified.user_count_with_enabled_action = 0 OR active_verified.user_count_with_enabled_action IS NULL)")
			} else if *filter.VerificationStatus == "verified" {
				filters = append(filters, "(active_verified.user_count_with_enabled_action > 0)")
			}
		}
		if filter.AssignedStatusTestingMe != nil {
			if *filter.AssignedStatusTestingMe == "unassigned" {
				filters = append(filters, "(active_assigned_testing.user_ids_with_enabled_action NOT LIKE ? OR active_assigned_testing.user_ids_with_enabled_action IS NULL)")
			} else if *filter.AssignedStatusTestingMe == "assigned" {
				filters = append(filters, "(active_assigned_testing.user_ids_with_enabled_action LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
		}
		if filter.AssignedStatusVerificationMe != nil {
			if *filter.AssignedStatusVerificationMe == "unassigned" {
				filters = append(filters, "(active_assigned_verification.user_ids_with_enabled_action NOT LIKE ? OR active_assigned_verification.user_ids_with_enabled_action IS NULL)")
			} else if *filter.AssignedStatusVerificationMe == "assigned" {
				filters = append(filters, "(active_assigned_verification.user_ids_with_enabled_action LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
		}
		if filter.RequestedChangedStatusMe != nil {
			if *filter.RequestedChangedStatusMe == "none" {
				filters = append(filters, "(active_requested_changes.user_ids_with_enabled_action NOT LIKE ? OR active_requested_changes.user_ids_with_enabled_action IS NULL)")
			} else if *filter.RequestedChangedStatusMe == "ongoing" {
				filters = append(filters, "(active_requested_changes.user_ids_with_enabled_action LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
		}
		if filter.ApprovalsStatusMe != nil {
			if *filter.ApprovalsStatusMe == "no" {
				filters = append(filters, "(active_approved.user_ids_with_enabled_action NOT LIKE ? OR active_approved.user_ids_with_enabled_action IS NULL)")
			} else if *filter.ApprovalsStatusMe == "yes" {
				filters = append(filters, "(active_approved.user_ids_with_enabled_action LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
		}
		if filter.VerificationStatusMe != nil {
			if *filter.VerificationStatusMe == "no" {
				filters = append(filters, "(active_verified.user_ids_with_enabled_action NOT LIKE ? OR active_verified.user_ids_with_enabled_action IS NULL)")
			} else if *filter.VerificationStatusMe == "yes" {
				filters = append(filters, "(active_verified.user_ids_with_enabled_action LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
		}

		if filter.IsExtreme != nil {
			filters = append(filters, "(meta.extreme = ?)")
			data = append(data, *filter.IsExtreme)
		}
		if len(filter.DistinctActions) != 0 {
			filters = append(filters, `(REGEXP_LIKE (distinct_actions.actions, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.DistinctActions)-1)+`)))`)
			for _, da := range filter.DistinctActions {
				data = append(data, da)
			}
		}
		if len(filter.DistinctActionsNot) != 0 {
			filters = append(filters, `(NOT REGEXP_LIKE (distinct_actions.actions, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.DistinctActionsNot)-1)+`)))`)
			for _, da := range filter.DistinctActionsNot {
				data = append(data, da)
			}
		}
	}

	data = append(data, currentLimit, currentOffset)

	and := ""
	if len(filters) > 0 {
		and = " AND "
	}

	subs, subCount, err := chooseSubmissions(dbs)
	if err != nil {
		return nil, err
	}
	chosenSubmissions := *subs

	fmt.Println(subCount)

	if subCount == 0 {
		return []*types.ExtendedSubmission{}, nil
	}

	chosenSubmissions = "(SELECT id from submission)"

	finalQuery := `
			SELECT submission.id AS submission_id,
		(
			SELECT name
			FROM submission_level
			WHERE id = submission.fk_submission_level_id
		) AS submission_level,
		uploader.id AS uploader_id,
		uploader.username AS uploader_username,
		uploader.avatar AS uploader_avatar,
		updater.id AS updater_id,
		updater.username AS updater_username,
		updater.avatar AS updater_avatar,
		newest_file.id,
		newest_file.original_filename,
		newest_file.current_filename,
		newest_file.size,
		file_data.created_at,
		newest_comment.created_at,
		newest_file.fk_user_id,
		meta.title,
		meta.alternate_titles,
		meta.platform,
		meta.launch_command,
		meta.library,
		meta.extreme,
		bot_comment.action AS bot_action,
		submission_file_count.count,
		active_assigned_testing.user_ids_with_enabled_action AS assigned_testing_user_ids,
		active_assigned_verification.user_ids_with_enabled_action AS assigned_verification_user_ids,
		active_requested_changes.user_ids_with_enabled_action AS requested_changes_user_ids,
		active_approved.user_ids_with_enabled_action AS approved_user_ids,
		active_verified.user_ids_with_enabled_action AS verified_user_ids,
		distinct_actions.actions
		FROM submission
		LEFT JOIN submission_cache ON submission_cache.fk_submission_id = submission.id
		LEFT JOIN submission_file AS oldest_file ON oldest_file.id = submission_cache.fk_oldest_file_id
		LEFT JOIN submission_file AS newest_file ON newest_file.id = submission_cache.fk_newest_file_id
		LEFT JOIN comment AS newest_comment ON newest_comment.id = submission_cache.fk_newest_comment_id
		LEFT JOIN (
			SELECT fk_submission_id, COUNT(*) AS count 
			FROM submission_file 
			WHERE deleted_at IS NULL 
			GROUP BY fk_submission_id
		) AS submission_file_count ON submission_file_count.fk_submission_id = submission.id
		LEFT JOIN (
			WITH ranked_file AS (
				SELECT s.*,
					ROW_NUMBER() OVER (
						PARTITION BY fk_submission_id
						ORDER BY created_at ASC
					) AS rn,
					GROUP_CONCAT(original_filename) AS original_filename_sequence,
					GROUP_CONCAT(current_filename) AS current_filename_sequence,
					GROUP_CONCAT(md5sum) AS md5sum_sequence,
					GROUP_CONCAT(sha256sum) AS sha256sum_sequence
				FROM submission_file AS s
				WHERE s.deleted_at IS NULL
				AND s.fk_submission_id IN ` + chosenSubmissions + `
				GROUP BY s.fk_submission_id
			)
			SELECT fk_user_id AS uploader_id,
				fk_submission_id,
				created_at AS created_at,
				original_filename_sequence,
				current_filename_sequence,
				md5sum_sequence,
				sha256sum_sequence
			FROM ranked_file
			WHERE rn = 1
		) AS file_data ON file_data.fk_submission_id = submission.id
		LEFT JOIN discord_user uploader ON oldest_file.fk_user_id = uploader.id
		LEFT JOIN discord_user updater ON newest_comment.fk_user_id = updater.id
		LEFT JOIN curation_meta meta ON meta.fk_submission_file_id = newest_file.id
		LEFT JOIN (
			WITH ranked_comment AS (
				SELECT c.*,
					ROW_NUMBER() OVER (
						PARTITION BY fk_submission_id
						ORDER BY created_at DESC
					) AS rn
				FROM comment AS c
				WHERE c.fk_user_id = 810112564787675166
					AND c.deleted_at IS NULL
					AND c.fk_submission_id IN ` + chosenSubmissions + `
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
								CHAR_LENGTH(comment_sequence) - LOCATE(
									REVERSE(CONCAT(810112564787675166)),
									REVERSE(comment_sequence)
								) - CHAR_LENGTH(CONCAT(?)) + 2 AS comment_sequence_substring_start
							FROM (
									SELECT MAX(id) AS comment_id,
										fk_submission_id,
										GROUP_CONCAT(
											CONCAT(
												fk_user_id,
												'-',
												(
													SELECT name
													FROM action
													WHERE action.id = fk_action_id
												)
											)
										) AS comment_sequence
									FROM comment
									WHERE fk_user_id != 810112564787675166
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
								) AS a
						) AS b
				) AS c
			WHERE REGEXP_LIKE(
					SUBSTRING(
						comment_sequence
						FROM comment_sequence_substring_start
					),
					CONCAT(CONCAT(?), '-\\S+,\\d+-\\S+')
				)
			AND fk_submission_id IN ` + chosenSubmissions + `
		) AS actions_after_my_last_comment ON actions_after_my_last_comment.fk_submission_id = submission.id
		LEFT JOIN ` + commentPair(`= "assign-testing"`, `= "unassign-testing"`, chosenSubmissions) + ` AS active_assigned_testing ON active_assigned_testing.submission_id = submission.id
		LEFT JOIN ` + commentPair(`= "assign-verification"`, `= "unassign-verification"`, chosenSubmissions) + ` AS active_assigned_verification ON active_assigned_verification.submission_id = submission.id
		LEFT JOIN ` + commentPair(`= "request-changes"`, `IN("approve", "verify")`, chosenSubmissions) + ` AS active_requested_changes ON active_requested_changes.submission_id = submission.id
		LEFT JOIN ` + commentPair(`= "approve"`, `= "request-changes"`, chosenSubmissions) + ` AS active_approved ON active_approved.submission_id = submission.id
		LEFT JOIN ` + commentPair(`= "verify"`, `= "request-changes"`, chosenSubmissions) + ` AS active_verified ON active_verified.submission_id = submission.id
		LEFT JOIN (
			SELECT submission.id AS submission_id,
				GROUP_CONCAT(
					DISTINCT (
						SELECT name
						FROM action
						WHERE id = comment.fk_action_id
					)
				) AS actions
			FROM comment
				LEFT JOIN submission on submission.id = comment.fk_submission_id
			WHERE submission.id IN ` + chosenSubmissions + `
			GROUP BY submission.id
		) AS distinct_actions ON distinct_actions.submission_id = submission.id
		WHERE submission.deleted_at IS NULL` + and + strings.Join(filters, " AND ") + `
		AND submission.id IN ` + chosenSubmissions + `
		GROUP BY submission.id
		ORDER BY newest_comment.created_at DESC
		LIMIT ? OFFSET ?
		`

	// fmt.Println(finalQuery)

	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), finalQuery, data...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*types.ExtendedSubmission, 0)

	var uploadedAt int64
	var updatedAt int64
	var submitterAvatar string
	var updaterAvatar string
	var assignedTestingUserIDs *string
	var assignedVerificationUserIDs *string
	var requestedChangesUserIDs *string
	var approvedUserIDs *string
	var verifiedUserIDs *string
	var distinctActions *string

	for rows.Next() {
		s := &types.ExtendedSubmission{}
		if err := rows.Scan(
			&s.SubmissionID,
			&s.SubmissionLevel,
			&s.SubmitterID, &s.SubmitterUsername, &submitterAvatar,
			&s.UpdaterID, &s.UpdaterUsername, &updaterAvatar,
			&s.FileID, &s.OriginalFilename, &s.CurrentFilename, &s.Size,
			&uploadedAt, &updatedAt, &s.LastUploaderID,
			&s.CurationTitle, &s.CurationAlternateTitles, &s.CurationPlatform, &s.CurationLaunchCommand, &s.CurationLibrary, &s.CurationExtreme,
			&s.BotAction,
			&s.FileCount,
			&assignedTestingUserIDs, &assignedVerificationUserIDs, &requestedChangesUserIDs, &approvedUserIDs, &verifiedUserIDs,
			&distinctActions); err != nil {
			return nil, err
		}
		s.SubmitterAvatarURL = utils.FormatAvatarURL(s.SubmitterID, submitterAvatar)
		s.UpdaterAvatarURL = utils.FormatAvatarURL(s.UpdaterID, updaterAvatar)
		s.UploadedAt = time.Unix(uploadedAt, 0)
		s.UpdatedAt = time.Unix(updatedAt, 0)

		s.AssignedTestingUserIDs = []int64{}
		if assignedTestingUserIDs != nil && len(*assignedTestingUserIDs) > 0 {
			userIDs := strings.Split(*assignedTestingUserIDs, ",")
			for _, userID := range userIDs {
				uid, err := strconv.ParseInt(userID, 10, 64)
				if err != nil {
					return nil, err
				}
				s.AssignedTestingUserIDs = append(s.AssignedTestingUserIDs, uid)
			}
		}

		s.AssignedVerificationUserIDs = []int64{}
		if assignedVerificationUserIDs != nil && len(*assignedVerificationUserIDs) > 0 {
			userIDs := strings.Split(*assignedVerificationUserIDs, ",")
			for _, userID := range userIDs {
				uid, err := strconv.ParseInt(userID, 10, 64)
				if err != nil {
					return nil, err
				}
				s.AssignedVerificationUserIDs = append(s.AssignedVerificationUserIDs, uid)
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

		s.VerifiedUserIDs = []int64{}
		if verifiedUserIDs != nil && len(*verifiedUserIDs) > 0 {
			userIDs := strings.Split(*verifiedUserIDs, ",")
			for _, userID := range userIDs {
				uid, err := strconv.ParseInt(userID, 10, 64)
				if err != nil {
					return nil, err
				}
				s.VerifiedUserIDs = append(s.VerifiedUserIDs, uid)
			}
		}

		s.DistinctActions = []string{}
		if distinctActions != nil && len(*distinctActions) > 0 {
			s.DistinctActions = append(s.DistinctActions, strings.Split(*distinctActions, ",")...)
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
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		INSERT INTO comment (fk_user_id, fk_submission_id, message, fk_action_id, created_at) 
        VALUES (?, ?, ?, (SELECT id FROM action WHERE name=?), ?)`,
		c.AuthorID, c.SubmissionID, msg, c.Action, c.CreatedAt.Unix())
	if err != nil {
		return err
	}

	err = updateSubmissionCacheTable(dbs, c.SubmissionID)
	if err != nil {
		return err
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

// SoftDeleteSubmissionFile marks submission file as deleted
func (d *mysqlDAL) SoftDeleteSubmissionFile(dbs DBSession, sfid int64, deleteReason string) error {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT COUNT(*), fk_submission_id FROM submission_file
		WHERE fk_submission_id = (SELECT fk_submission_id FROM submission_file WHERE id = ?)
        AND submission_file.deleted_at IS NULL
		GROUP BY fk_submission_id`,
		sfid, deleteReason)

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

	err = updateSubmissionCacheTable(dbs, sid)
	if err != nil {
		return err
	}

	return nil
}

func updateSubmissionCacheTable(dbs DBSession, sid int64) error {
	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_cache
		SET fk_newest_file_id = (SELECT id FROM submission_file WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1),
		    fk_oldest_file_id = (SELECT id FROM submission_file WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
		    fk_newest_comment_id = (SELECT id FROM comment WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1)
		WHERE fk_submission_id = ?`,
		sid, sid, sid, sid)
	return err
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

	err = updateSubmissionCacheTable(dbs, sid)
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

	err = updateSubmissionCacheTable(dbs, sid)
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

	return count == 1, nil
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
