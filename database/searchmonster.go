package database

import (
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"strconv"
	"strings"
	"time"
)

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
			filters = append(filters, "(submission_cache.original_filename_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.OriginalFilenamePartialAny))
		}
		if filter.CurrentFilenamePartialAny != nil {
			filters = append(filters, "(submission_cache.current_filename_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.CurrentFilenamePartialAny))
		}
		if filter.MD5SumPartialAny != nil {
			filters = append(filters, "(submission_cache.md5sum_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.MD5SumPartialAny))
		}
		if filter.SHA256SumPartialAny != nil {
			filters = append(filters, "(submission_cache.sha256sum_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.SHA256SumPartialAny))
		}
		if len(filter.BotActions) != 0 {
			filters = append(filters, `(submission_cache.bot_action IN(?`+strings.Repeat(",?", len(filter.BotActions)-1)+`))`)
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
			filters = append(filters, `(REGEXP_LIKE (submission_cache.distinct_actions, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.DistinctActions)-1)+`)))`)
			for _, da := range filter.DistinctActions {
				data = append(data, da)
			}
		}
		if len(filter.DistinctActionsNot) != 0 {
			filters = append(filters, `(NOT REGEXP_LIKE (submission_cache.distinct_actions, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.DistinctActionsNot)-1)+`)))`)
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
		oldest_file.created_at,
		newest_comment.created_at,
		newest_file.fk_user_id,
		meta.title,
		meta.alternate_titles,
		meta.platform,
		meta.launch_command,
		meta.library,
		meta.extreme,
		submission_cache.bot_action,
		submission_file_count.count,
		submission_cache.active_assigned_testing_ids,
		submission_cache.active_assigned_verification_ids,
		submission_cache.active_requested_changes_ids,
		submission_cache.active_approved_ids,
		submission_cache.active_verified_ids,
		submission_cache.distinct_actions
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
		LEFT JOIN discord_user uploader ON oldest_file.fk_user_id = uploader.id
		LEFT JOIN discord_user updater ON newest_comment.fk_user_id = updater.id
		LEFT JOIN curation_meta meta ON meta.fk_submission_file_id = newest_file.id
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
		) AS actions_after_my_last_comment ON actions_after_my_last_comment.fk_submission_id = submission.id
		WHERE submission.deleted_at IS NULL` + and + strings.Join(filters, " AND ") + `
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
