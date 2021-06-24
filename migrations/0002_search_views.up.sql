create view view_active_assigned_testing as (
SELECT submission.id AS submission_id,
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
        WHERE c.fk_author_id != 810112564787675166
          AND c.fk_action_id = (
            SELECT id
            FROM action
            WHERE name = "assign-testing"
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
        WHERE c.fk_author_id != 810112564787675166
          AND c.fk_action_id = (
            SELECT id
            FROM action
            WHERE name = "unassign-testing"
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
    );
create view view_active_assigned_verification as (
SELECT submission.id AS submission_id,
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
        WHERE c.fk_author_id != 810112564787675166
          AND c.fk_action_id = (
            SELECT id
            FROM action
            WHERE name = "assign-verification"
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
        WHERE c.fk_author_id != 810112564787675166
          AND c.fk_action_id = (
            SELECT id
            FROM action
            WHERE name = "unassign-verification"
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
    );
create view view_active_requested_changes as (
SELECT submission.id AS submission_id,
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
        WHERE c.fk_author_id != 810112564787675166
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
        WHERE c.fk_author_id != 810112564787675166
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
    );
create view view_active_approved as (
SELECT submission.id AS submission_id,
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
        WHERE c.fk_author_id != 810112564787675166
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
        WHERE c.fk_author_id != 810112564787675166
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
    );
create view view_active_verified as (
SELECT submission.id AS submission_id,
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
        WHERE c.fk_author_id != 810112564787675166
          AND c.fk_action_id = (
            SELECT id
            FROM action
            WHERE name = "verify"
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
        WHERE c.fk_author_id != 810112564787675166
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
    );