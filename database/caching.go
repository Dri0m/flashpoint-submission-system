package database

import (
	"database/sql"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"time"
)

func updateSubmissionCacheTable(dbs DBSession, sid int64) error {
	l := utils.LogCtx(dbs.Ctx()).WithField("event", "cache-table-update").WithField("table", "submission_cache")
	l.Debug("updating submission cache table")
	start := time.Now()

	assignedTestingIDseq, err := getUserCountWithEnabledAction(dbs, `= "assign-testing"`, `= "unassign-testing"`, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	assignedVerificationIDseq, err := getUserCountWithEnabledAction(dbs, `= "assign-verification"`, `= "unassign-verification"`, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	requestedChangesIDseq, err := getUserCountWithEnabledAction(dbs, `= "request-changes"`, `IN("approve", "verify")`, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	approvedIDseq, err := getUserCountWithEnabledAction(dbs, `= "approve"`, `= "request-changes"`, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	verifiedIDseq, err := getUserCountWithEnabledAction(dbs, `= "verify"`, `= "request-changes"`, sid)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	_, err = dbs.Tx().ExecContext(dbs.Ctx(), `
		UPDATE submission_cache
		SET fk_newest_file_id = (SELECT id FROM submission_file WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1),
		    fk_oldest_file_id = (SELECT id FROM submission_file WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at LIMIT 1),
		    fk_newest_comment_id = (SELECT id FROM comment WHERE fk_submission_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1),
		    active_assigned_testing_ids = ?,
		    active_assigned_verification_ids = ?,
		    active_requested_changes_ids = ?,
		    active_approved_ids = ?,
		    active_verified_ids = ?
		WHERE fk_submission_id = ?`,
		sid, sid, sid, assignedTestingIDseq, assignedVerificationIDseq, requestedChangesIDseq, approvedIDseq, verifiedIDseq, sid)
	if err != nil {
		return err
	}

	duration := time.Since(start)
	l.WithField("duration", duration).Debug("submission cache table update finished successfully")
	return err
}

func getUserCountWithEnabledAction(dbs DBSession, enablerChunk, disablerChunk string, sid int64) (*string, error) {
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
					LEFT JOIN submission_file AS last_file ON last_file.id = submission_cache.fk_newest_file_id
				WHERE rn = 1
					AND ranked_comment.created_at > last_file.created_at
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
					LEFT JOIN submission_file AS last_file ON last_file.id = submission_cache.fk_newest_file_id
				WHERE rn = 1
					AND ranked_comment.created_at > last_file.created_at
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
