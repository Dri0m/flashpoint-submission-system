package database

import (
	"database/sql"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"strings"
	"time"
)

func (d *mysqlDAL) UpdateSubmissionCacheTable(dbs DBSession, sid int64) error {
	l := utils.LogCtx(dbs.Ctx()).WithField("event", "cache-table-update").WithField("table", "submission_cache")
	l.Debug("updating submission cache table")
	start := time.Now()

	_, err := dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_cache
		SET fk_newest_file_id = (SELECT id FROM submission_file WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1),
		    fk_oldest_file_id = (SELECT id FROM submission_file WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
		    fk_newest_comment_id = (SELECT id FROM comment WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1)
		
		WHERE fk_submission_id = ?`,
		sid, sid, sid, sid)
	if err != nil {
		return err
	}

	assignedTestingIDseq, err := getUserCountWithEnabledAction(dbs, `= "assign-testing"`, `IN("unassign-testing")`, sid, false)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	assignedVerificationIDseq, err := getUserCountWithEnabledAction(dbs, `= "assign-verification"`, `IN("unassign-verification")`, sid, false)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	requestedChangesIDseq, err := getUserCountWithEnabledAction(dbs, `= "request-changes"`, `IN("approve", "verify")`, sid, false)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	approvedIDseq, err := getUserCountWithEnabledAction(dbs, `= "approve"`, `IN("request-changes")`, sid, true)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	verifiedIDseq, err := getUserCountWithEnabledAction(dbs, `= "verify"`, `IN("request-changes")`, sid, true)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	ofs, cfs, md5s, sha256s, err := getFileDataSequences(dbs, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	botAction, err := getBotAction(dbs, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	distinctActionsSeq, err := getDistinctActions(dbs, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if distinctActionsSeq != nil {
		for _, action := range strings.Split(*distinctActionsSeq, ",") {
			if action == constants.ActionReject {
				assignedTestingIDseq = nil
				assignedVerificationIDseq = nil
				requestedChangesIDseq = nil
				approvedIDseq = nil
				verifiedIDseq = nil

				reject := constants.ActionReject
				distinctActionsSeq = &reject

				break
			}
		}
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_cache
		SET active_assigned_testing_ids = ?,
		    active_assigned_verification_ids = ?,
		    active_requested_changes_ids = ?,
		    active_approved_ids = ?,
		    active_verified_ids = ?,
		    
		    original_filename_sequence = ?,
		    current_filename_sequence = ?,
		    md5sum_sequence = ?,
		    sha256sum_sequence = ?,
		    
		    bot_action = ?,
		    distinct_actions = ?
		
		WHERE fk_submission_id = ?`,
		assignedTestingIDseq, assignedVerificationIDseq, requestedChangesIDseq, approvedIDseq, verifiedIDseq,
		ofs, cfs, md5s, sha256s,
		botAction, distinctActionsSeq,
		sid)
	if err != nil {
		return err
	}

	duration := time.Since(start)
	l.WithField("duration_ns", duration.Nanoseconds()).Debug("submission cache table update finished successfully")
	return err
}

func getUserCountWithEnabledAction(dbs DBSession, enablerChunk, disablerChunk string, sid int64, onlyFromLastFileVersion bool) (*string, error) {

	lastFileJoinQuery := ` `
	lastFileVersionQuery := ` `

	if onlyFromLastFileVersion {
		lastFileJoinQuery = `LEFT JOIN submission_file AS last_file ON last_file.id = submission_cache.fk_newest_file_id`
		lastFileVersionQuery = `AND ranked_comment.created_at > last_file.created_at`
	}

	q := fmt.Sprintf(`
		SELECT GROUP_CONCAT(latest_enabler.author_id) AS user_ids_with_enabled_action
		FROM submission
			LEFT JOIN (
				WITH ranked_comment AS (
					SELECT c.*,
						ROW_NUMBER() OVER (
							PARTITION BY c.fk_submission_id,
							c.fk_user_id
							ORDER BY created_at DESC
						) AS rn
					FROM comment AS c
					WHERE c.fk_user_id != 810112564787675166
						AND c.fk_action_id IN (
							SELECT id
							FROM action
							WHERE name %s
						)
						AND c.deleted_at IS NULL
						AND c.fk_submission_id = %d
					ORDER BY created_at ASC
				)
				SELECT ranked_comment.fk_submission_id AS submission_id,
					ranked_comment.fk_user_id AS author_id,
					ranked_comment.created_at
				FROM ranked_comment
					LEFT JOIN (SELECT * FROM submission_cache WHERE fk_submission_id = %d) AS submission_cache ON submission_cache.fk_submission_id = ranked_comment.fk_submission_id
					`+lastFileJoinQuery+`
				WHERE rn = 1
					`+lastFileVersionQuery+`
			) AS latest_enabler ON latest_enabler.submission_id = submission.id
			LEFT JOIN (
				WITH ranked_comment AS (
					SELECT c.*,
						ROW_NUMBER() OVER (
							PARTITION BY c.fk_submission_id,
							c.fk_user_id
							ORDER BY created_at DESC
						) AS rn
					FROM comment AS c
					WHERE c.fk_user_id != 810112564787675166
						AND c.fk_action_id IN (
							SELECT id
							FROM action
							WHERE name %s
						)
						AND c.deleted_at IS NULL
				    	AND c.fk_submission_id = %d
				)
				SELECT ranked_comment.fk_submission_id AS submission_id,
					ranked_comment.fk_user_id AS author_id,
					ranked_comment.created_at
				FROM ranked_comment
					LEFT JOIN (SELECT * FROM submission_cache WHERE fk_submission_id = %d) AS submission_cache ON submission_cache.fk_submission_id = ranked_comment.fk_submission_id
					`+lastFileJoinQuery+`
				WHERE rn = 1
					`+lastFileVersionQuery+`
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
		GROUP BY submission.id`,
		enablerChunk, sid, sid, disablerChunk, sid, sid)

	row := dbs.Tx().QueryRowContext(dbs.Ctx(), q)

	var idSequence string
	err := row.Scan(&idSequence)
	if err != nil {
		return nil, err
	}

	return &idSequence, nil
}

func getFileDataSequences(dbs DBSession, sid int64) (ofs, cfs, md5s, sha256s *string, err error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		WITH ranked_file AS (
			SELECT s.*,
				ROW_NUMBER() OVER (
					PARTITION BY fk_submission_id
					ORDER BY created_at
				) AS rn,
				GROUP_CONCAT(original_filename) AS original_filename_sequence,
				GROUP_CONCAT(current_filename) AS current_filename_sequence,
				GROUP_CONCAT(md5sum) AS md5sum_sequence,
				GROUP_CONCAT(sha256sum) AS sha256sum_sequence
			FROM submission_file AS s
			WHERE s.deleted_at IS NULL
			GROUP BY s.fk_submission_id
		)
		SELECT original_filename_sequence,
			current_filename_sequence,
			md5sum_sequence,
			sha256sum_sequence
		FROM ranked_file
		WHERE rn = 1
		AND fk_submission_id = ?`,
		sid)

	err = row.Scan(&ofs, &cfs, &md5s, &sha256s)
	return
}

func getDistinctActions(dbs DBSession, sid int64) (result *string, err error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		SELECT GROUP_CONCAT(
					DISTINCT (
						SELECT name
						FROM action
						WHERE id = comment.fk_action_id
					)
				) AS actions
			FROM comment
				LEFT JOIN submission on submission.id = comment.fk_submission_id
			WHERE fk_submission_id = ?
				AND comment.deleted_at IS NULL
			GROUP BY submission.id`,
		sid)

	err = row.Scan(&result)
	return
}

func getBotAction(dbs DBSession, sid int64) (result *string, err error) {
	row := dbs.Tx().QueryRowContext(dbs.Ctx(), `
		WITH ranked_comment AS (
			SELECT c.*,
				ROW_NUMBER() OVER (
					PARTITION BY fk_submission_id
					ORDER BY created_at DESC
				) AS rn
			FROM comment AS c
			WHERE c.fk_user_id = 810112564787675166
				AND c.deleted_at IS NULL
		)
		SELECT (
				SELECT name
				FROM action
				WHERE action.id = ranked_comment.fk_action_id
			) AS action
		FROM ranked_comment
		WHERE rn = 1
		AND fk_submission_id = ?`,
		sid)

	err = row.Scan(&result)
	return
}
