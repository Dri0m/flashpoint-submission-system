package database

import (
	"fmt"
	"strconv"
	"strings"
)

func chooseSubmissions(dbs DBSession) (*string, int, error) {
	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), `
	SELECT submission.id
	FROM submission
		LEFT JOIN submission_cache ON submission_cache.fk_submission_id = submission.id
		LEFT JOIN comment AS newest_comment ON newest_comment.id = submission_cache.fk_newest_comment_id
	WHERE submission.deleted_at IS NULL
    ORDER BY newest_comment.created_at DESC
	`)

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	sids := make([]string, 0)

	for rows.Next() {
		var sid int64
		if err := rows.Scan(&sid); err != nil {
			return nil, 0, err
		}
		sids = append(sids, strconv.FormatInt(sid, 10))
	}

	result := "(" + strings.Join(sids, ",") + ")"

	return &result, len(sids), nil
}

func commentPair(enabler, disabler, choosenSubmissions string) string {
	return fmt.Sprintf(`
		(SELECT submission.id AS submission_id,
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
						AND c.fk_submission_id IN %s
					ORDER BY created_at ASC
				)
				SELECT ranked_comment.fk_submission_id AS submission_id,
					ranked_comment.fk_user_id AS author_id,
					ranked_comment.created_at
				FROM ranked_comment
					LEFT JOIN (SELECT * FROM submission_cache WHERE fk_submission_id IN %s) AS submission_cache ON submission_cache.fk_submission_id = ranked_comment.fk_submission_id
					LEFT JOIN submission_file AS last_file ON last_file.id = submission_cache.fk_submission_id
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
				    	AND c.fk_submission_id IN %s
				)
				SELECT ranked_comment.fk_submission_id AS submission_id,
					ranked_comment.fk_user_id AS author_id,
					ranked_comment.created_at
				FROM ranked_comment
					LEFT JOIN (SELECT * FROM submission_cache WHERE fk_submission_id IN %s) AS submission_cache ON submission_cache.fk_submission_id = ranked_comment.fk_submission_id
					LEFT JOIN submission_file AS last_file ON last_file.id = submission_cache.fk_submission_id
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
			) AND submission.id IN %s
		GROUP BY submission.id)`,
		enabler, choosenSubmissions, choosenSubmissions,
		disabler, choosenSubmissions, choosenSubmissions, choosenSubmissions)
}
