package database

import (
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SearchSubmissions returns extended submissions based on given filter
func (d *mysqlDAL) SearchSubmissions(dbs DBSession, filter *types.SubmissionsFilter) ([]*types.ExtendedSubmission, int64, error) {
	uid := utils.UserID(dbs.Ctx()) // TODO this should be passed as param

	filters := make([]string, 0)
	masterFilters := make([]string, 0)
	data := make([]interface{}, 0)
	masterData := make([]interface{}, 0)

	const defaultLimit int64 = 100
	const defaultOffset int64 = 0
	const defaultOrderBy string = "updated_at"
	const defaultSortOrder string = "DESC"

	currentLimit := defaultLimit
	currentOffset := defaultOffset
	currentOrderBy := defaultOrderBy
	currentSortOrder := defaultSortOrder

	if filter != nil {
		if len(filter.SubmissionIDs) > 0 {
			filters = append(filters, `(submission.id IN(?`+strings.Repeat(`,?`, len(filter.SubmissionIDs)-1)+`))`)
			for _, sid := range filter.SubmissionIDs {
				data = append(data, sid)
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.SubmitterID != nil {
			filters = append(filters, "(uploader.id = ?)")
			data = append(data, *filter.SubmitterID)
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.TitlePartial != nil {
			filters = append(filters, "(meta.title LIKE ? OR meta.alternate_titles LIKE ?)")
			data = append(data, utils.FormatLike(*filter.TitlePartial), utils.FormatLike(*filter.TitlePartial))
			masterFilters = append(masterFilters, "(title LIKE ? OR alternate_titles LIKE ?)")
			masterData = append(masterData, utils.FormatLike(*filter.TitlePartial), utils.FormatLike(*filter.TitlePartial))
		}
		if filter.SubmitterUsernamePartial != nil {
			tableName := `uploader.username`
			filters, masterFilters, data, masterData = addMultifilter(
				tableName, nil, *filter.SubmitterUsernamePartial, filters, masterFilters, data, masterData)
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.PlatformPartial != nil {
			tableName := `meta.platform`
			masterTableName := `platform`
			filters, masterFilters, data, masterData = addMultifilter(
				tableName, &masterTableName, *filter.PlatformPartial, filters, masterFilters, data, masterData)
		}
		if filter.LibraryPartial != nil {
			filters = append(filters, "(meta.library LIKE ?)")
			data = append(data, utils.FormatLike(*filter.LibraryPartial))
			masterFilters = append(masterFilters, "(library LIKE ?)")
			masterData = append(masterData, utils.FormatLike(*filter.LibraryPartial))
		}
		if filter.OriginalFilenamePartialAny != nil {
			filters = append(filters, "(submission_cache.original_filename_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.OriginalFilenamePartialAny))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.CurrentFilenamePartialAny != nil {
			filters = append(filters, "(submission_cache.current_filename_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.CurrentFilenamePartialAny))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.MD5SumPartialAny != nil {
			filters = append(filters, "(submission_cache.md5sum_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.MD5SumPartialAny))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.SHA256SumPartialAny != nil {
			filters = append(filters, "(submission_cache.sha256sum_sequence LIKE ?)")
			data = append(data, utils.FormatLike(*filter.SHA256SumPartialAny))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if len(filter.BotActions) != 0 {
			filters = append(filters, `(submission_cache.bot_action IN(?`+strings.Repeat(",?", len(filter.BotActions)-1)+`))`)
			for _, ba := range filter.BotActions {
				data = append(data, ba)
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if len(filter.SubmissionLevels) != 0 {
			filters = append(filters, `((SELECT name FROM submission_level WHERE id = submission.fk_submission_level_id) IN(?`+strings.Repeat(",?", len(filter.SubmissionLevels)-1)+`))`)
			for _, ba := range filter.SubmissionLevels {
				data = append(data, ba)
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
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
				filters = append(filters, "(submission_cache.active_assigned_testing_ids IS NULL)")
			} else if *filter.AssignedStatusTesting == "assigned" {
				filters = append(filters, "(submission_cache.active_assigned_testing_ids IS NOT NULL)")
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.AssignedStatusVerification != nil {
			if *filter.AssignedStatusVerification == "unassigned" {
				filters = append(filters, "(submission_cache.active_assigned_verification_ids IS NULL)")
			} else if *filter.AssignedStatusVerification == "assigned" {
				filters = append(filters, "(submission_cache.active_assigned_verification_ids IS NOT NULL)")
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.RequestedChangedStatus != nil {
			if *filter.RequestedChangedStatus == "none" {
				filters = append(filters, "(submission_cache.active_requested_changes_ids IS NULL)")
			} else if *filter.RequestedChangedStatus == "ongoing" {
				filters = append(filters, "(submission_cache.active_requested_changes_ids IS NOT NULL)")
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.ApprovalsStatus != nil {
			if *filter.ApprovalsStatus == "none" {
				filters = append(filters, "(submission_cache.active_approved_ids IS NULL)")
			} else if *filter.ApprovalsStatus == "approved" {
				filters = append(filters, "(submission_cache.active_approved_ids IS NOT NULL)")
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.VerificationStatus != nil {
			if *filter.VerificationStatus == "none" {
				filters = append(filters, "(submission_cache.active_verified_ids IS NULL)")
			} else if *filter.VerificationStatus == "verified" {
				filters = append(filters, "(submission_cache.active_verified_ids IS NOT NULL)")
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.AssignedStatusTestingMe != nil {
			if *filter.AssignedStatusTestingMe == "unassigned" {
				filters = append(filters, "(submission_cache.active_assigned_testing_ids NOT LIKE ? OR submission_cache.active_assigned_testing_ids IS NULL)")
			} else if *filter.AssignedStatusTestingMe == "assigned" {
				filters = append(filters, "(submission_cache.active_assigned_testing_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.AssignedStatusVerificationMe != nil {
			if *filter.AssignedStatusVerificationMe == "unassigned" {
				filters = append(filters, "(submission_cache.active_assigned_verification_ids NOT LIKE ? OR submission_cache.active_assigned_verification_ids IS NULL)")
			} else if *filter.AssignedStatusVerificationMe == "assigned" {
				filters = append(filters, "(submission_cache.active_assigned_verification_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.RequestedChangedStatusMe != nil {
			if *filter.RequestedChangedStatusMe == "none" {
				filters = append(filters, "(submission_cache.active_requested_changes_ids NOT LIKE ? OR submission_cache.active_requested_changes_ids IS NULL)")
			} else if *filter.RequestedChangedStatusMe == "ongoing" {
				filters = append(filters, "(submission_cache.active_requested_changes_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.ApprovalsStatusMe != nil {
			if *filter.ApprovalsStatusMe == "no" {
				filters = append(filters, "(submission_cache.active_approved_ids NOT LIKE ? OR submission_cache.active_approved_ids IS NULL)")
			} else if *filter.ApprovalsStatusMe == "yes" {
				filters = append(filters, "(submission_cache.active_approved_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.VerificationStatusMe != nil {
			if *filter.VerificationStatusMe == "no" {
				filters = append(filters, "(submission_cache.active_verified_ids NOT LIKE ? OR submission_cache.active_verified_ids IS NULL)")
			} else if *filter.VerificationStatusMe == "yes" {
				filters = append(filters, "(submission_cache.active_verified_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", uid)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}

		if filter.AssignedStatusTestingUser != nil {
			if *filter.AssignedStatusTestingUser == "unassigned" {
				filters = append(filters, "(submission_cache.active_assigned_testing_ids NOT LIKE ? OR submission_cache.active_assigned_testing_ids IS NULL)")
			} else if *filter.AssignedStatusTestingUser == "assigned" {
				filters = append(filters, "(submission_cache.active_assigned_testing_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", *filter.AssignedStatusUserID)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.AssignedStatusVerificationUser != nil {
			if *filter.AssignedStatusVerificationUser == "unassigned" {
				filters = append(filters, "(submission_cache.active_assigned_verification_ids NOT LIKE ? OR submission_cache.active_assigned_verification_ids IS NULL)")
			} else if *filter.AssignedStatusVerificationUser == "assigned" {
				filters = append(filters, "(submission_cache.active_assigned_verification_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", *filter.AssignedStatusUserID)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.RequestedChangedStatusUser != nil {
			if *filter.RequestedChangedStatusUser == "none" {
				filters = append(filters, "(submission_cache.active_requested_changes_ids NOT LIKE ? OR submission_cache.active_requested_changes_ids IS NULL)")
			} else if *filter.RequestedChangedStatusUser == "ongoing" {
				filters = append(filters, "(submission_cache.active_requested_changes_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", *filter.AssignedStatusUserID)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.ApprovalsStatusUser != nil {
			if *filter.ApprovalsStatusUser == "no" {
				filters = append(filters, "(submission_cache.active_approved_ids NOT LIKE ? OR submission_cache.active_approved_ids IS NULL)")
			} else if *filter.ApprovalsStatusUser == "yes" {
				filters = append(filters, "(submission_cache.active_approved_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", *filter.AssignedStatusUserID)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.VerificationStatusUser != nil {
			if *filter.VerificationStatusUser == "no" {
				filters = append(filters, "(submission_cache.active_verified_ids NOT LIKE ? OR submission_cache.active_verified_ids IS NULL)")
			} else if *filter.VerificationStatusUser == "yes" {
				filters = append(filters, "(submission_cache.active_verified_ids LIKE ?)")
			}
			data = append(data, utils.FormatLike(fmt.Sprintf("%d", *filter.AssignedStatusUserID)))
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}

		if filter.IsExtreme != nil {
			filters = append(filters, "(meta.extreme = ?)")
			data = append(data, *filter.IsExtreme)
			masterFilters = append(masterFilters, "(extreme = ?)")
			masterData = append(masterData, *filter.IsExtreme)
		}
		if len(filter.DistinctActions) != 0 {
			filters = append(filters, `(REGEXP_LIKE (submission_cache.distinct_actions, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.DistinctActions)-1)+`)))`)
			for _, da := range filter.DistinctActions {
				data = append(data, da)
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if len(filter.DistinctActionsNot) != 0 {
			filters = append(filters, `(NOT REGEXP_LIKE (submission_cache.distinct_actions, CONCAT(CONCAT(?)`+strings.Repeat(", '|', CONCAT(?)", len(filter.DistinctActionsNot)-1)+`)))`)
			for _, da := range filter.DistinctActionsNot {
				data = append(data, da)
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.LaunchCommandFuzzy != nil { // TODO not really fuzzy is it
			filters = append(filters, "(meta.launch_command LIKE ?)")
			data = append(data, utils.FormatLike(*filter.LaunchCommandFuzzy))
			masterFilters = append(masterFilters, "(launch_command LIKE ?)")
			masterData = append(masterData, utils.FormatLike(*filter.LaunchCommandFuzzy))
		}
		if filter.LastUploaderNotMe != nil {
			if *filter.LastUploaderNotMe == "yes" {
				filters = append(filters, "(uploader.id != ?)")
				data = append(data, uid)
			}
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.OrderBy != nil {
			if *filter.OrderBy == "uploaded" {
				currentOrderBy = "created_at"
			} else if *filter.OrderBy == "updated" {
				currentOrderBy = "updated_at"
			} else if *filter.OrderBy == "size" {
				currentOrderBy = "newest_file_size"
			}
		}
		if filter.AscDesc != nil {
			if *filter.AscDesc == "asc" {
				currentSortOrder = "ASC"
			} else if *filter.AscDesc == "desc" {
				currentSortOrder = "DESC"
			}
		}
		if filter.SubscribedMe != nil {
			if *filter.SubscribedMe == "yes" {
				filters = append(filters, "(sns.fk_user_id = ?)")
			}
			data = append(data, uid)
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.ExcludeLegacy {
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.UpdatedByID != nil {
			filters = append(filters, "(updater.id = ?)")
			data = append(data, *filter.UpdatedByID)
			masterFilters = append(masterFilters, "(1 = 0)") // exclude legacy results
		}
		if filter.IsContentChange != nil {
			if *filter.IsContentChange == "yes" {
				filters = append(filters, "(meta.game_exists = true)")
			} else {
				filters = append(filters, "(meta.game_exists = false)")
			}

		}
	}

	and := ""
	if len(filters) > 0 {
		and = " AND "
	}

	masterAnd := ""
	if len(masterFilters) > 0 {
		masterAnd = " AND "
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
		newest_file.id AS newest_file_id,
		newest_file.original_filename AS newest_file_original_filename,
		newest_file.current_filename AS newest_file_current_filename,
		newest_file.size AS newest_file_size,
		oldest_file.created_at AS created_at,
		newest_comment.created_at AS updated_at,
		newest_file.fk_user_id AS newest_file_user_id,
		meta.title AS meta_title,
		meta.alternate_titles AS meta_alternate_titles,
		meta.platform AS meta_platform,
		meta.launch_command AS meta_launch_command,
		meta.library AS meta_library,
		meta.extreme AS meta_extreme,
		submission_cache.bot_action AS bot_action,
		submission_file_count.count AS file_count,
		submission_cache.active_assigned_testing_ids AS active_assigned_testing_ids,
		submission_cache.active_assigned_verification_ids AS active_assigned_verification_ids,
		submission_cache.active_requested_changes_ids AS active_requested_changes_ids,
		submission_cache.active_approved_ids AS active_approved_ids,
		submission_cache.active_verified_ids AS active_verified_ids,
		submission_cache.distinct_actions AS distinct_actions,
		meta.game_exists AS meta_game_exists
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
		LEFT JOIN curation_meta meta ON meta.fk_submission_file_id = newest_file.id`

	rest := ` LEFT JOIN submission_notification_subscription AS sns ON sns.fk_submission_id = submission.id
		WHERE submission.deleted_at IS NULL` + and + strings.Join(filters, " AND ") + `
		GROUP BY submission.id
		UNION
			SELECT -1 AS submission_id,
			(SELECT "legacy") AS submission_level,
			(SELECT -1) AS uploader_id,
			(SELECT "legacy") AS uploader_username,
			(SELECT "legacy") AS uploader_avatar,
			(SELECT -1) AS updater_id,
			(SELECT "legacy") AS updater_username,
			(SELECT "legacy") AS updater_avatar,
			(SELECT -1) AS newest_file_id,
			(SELECT "legacy") AS newest_file_original_filename,
			(SELECT "legacy") AS newest_file_current_filename,
			(SELECT 42) AS newest_file_size,
			date_added AS created_at,
			date_modified AS updated_at,
			(SELECT -1) AS newest_file_user_id,
			title AS meta_title,
			alternate_titles AS meta_alternate_titles,
			platform AS meta_platform,
			launch_command  AS meta_launch_command,
			library  AS meta_library,
			extreme AS meta_extreme,
			(SELECT "legacy") AS bot_action,
			(SELECT 0) AS file_count,
			(SELECT "") AS active_assigned_testing_ids,
			(SELECT "") AS active_assigned_verification_ids,
			(SELECT "") AS active_requested_changes_ids,
			(SELECT "") AS active_approved_ids,
			(SELECT "") AS active_verified_ids,
			(SELECT "mark-added") AS distinct_actions,
			(SELECT TRUE) as meta_game_exists
			FROM masterdb_game
			WHERE (SELECT 1) ` + masterAnd + strings.Join(masterFilters, " AND ") + `
		ORDER BY ` + currentOrderBy + ` ` + currentSortOrder + `
		`
	unlimitedQuery := finalQuery + rest
	finalQuery = unlimitedQuery + ` LIMIT ? OFFSET ?`

	finalData := make([]interface{}, 0)
	finalData = append(finalData, data...)
	unlimitedData := append(finalData, masterData...)
	finalData = append(unlimitedData, currentLimit, currentOffset)

	countingQuery := `SELECT COUNT(*) FROM ( ` + unlimitedQuery + ` ) AS counterino`
	var counter int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		row := d.db.QueryRowContext(dbs.Ctx(), countingQuery, unlimitedData...)
		if err := row.Scan(&counter); err != nil {
			counter = -1
			return
		}
	}()

	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), finalQuery, finalData...)
	if err != nil {
		return nil, 0, err
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
			&distinctActions, &s.GameExists); err != nil {
			return nil, 0, err
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
					return nil, 0, err
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
					return nil, 0, err
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
					return nil, 0, err
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
					return nil, 0, err
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
					return nil, 0, err
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

	rows.Close()
	wg.Wait()

	return result, counter, nil
}

func addMultifilter(tableName string, masterTableName *string, filterContents string, filters, masterFilters []string, data, masterData []interface{}) ([]string, []string, []interface{}, []interface{}) {
	substrings := strings.Split(filterContents, ",")
	trimmed := make([]string, 0, len(substrings))
	for _, ss := range substrings {
		trimmed = append(trimmed, strings.TrimSpace(ss))
	}
	include := make([]string, 0, len(substrings))
	exclude := make([]string, 0, len(substrings))
	for _, s := range trimmed {
		if len(s) == 0 {
			continue
		}
		if s[0] == '!' {
			exclude = append(exclude, s[1:])
		} else {
			include = append(include, s)
		}
	}

	if len(include) > 0 {
		includePlaceholder := `(` + tableName + ` LIKE ?)`
		filters = append(filters, `(`+includePlaceholder+strings.Repeat(` OR `+includePlaceholder, len(include)-1)+`)`)

		if masterTableName != nil {
			masterIncludePlaceholder := `(` + *masterTableName + ` LIKE ?)`
			masterFilters = append(masterFilters, `(`+masterIncludePlaceholder+strings.Repeat(` OR `+masterIncludePlaceholder, len(include)-1)+`)`)
		}
	}

	if len(exclude) > 0 {
		excludePlaceholder := `(` + tableName + ` NOT LIKE ?)`
		filters = append(filters, `(`+excludePlaceholder+strings.Repeat(` AND `+excludePlaceholder, len(exclude)-1)+`)`)

		if masterTableName != nil {
			masterExcludePlaceholder := `(` + *masterTableName + ` NOT LIKE ?)`
			masterFilters = append(masterFilters, `(`+masterExcludePlaceholder+strings.Repeat(` AND `+masterExcludePlaceholder, len(exclude)-1)+`)`)
		}
	}

	for _, s := range include {
		data = append(data, utils.FormatLike(s))
		if masterTableName != nil {
			masterData = append(masterData, utils.FormatLike(s))
		}
	}
	for _, s := range exclude {
		data = append(data, utils.FormatLike(s))
		if masterTableName != nil {
			masterData = append(masterData, utils.FormatLike(s))
		}
	}

	return filters, masterFilters, data, masterData
}

func magicAnd(a []string) string {
	if len(a) > 0 {
		return " AND "
	}
	return ""
}

// SearchFlashfreezeFiles returns extended flashfreeze files based on given filter
func (d *mysqlDAL) SearchFlashfreezeFiles(dbs DBSession, filter *types.FlashfreezeFilter) ([]*types.ExtendedFlashfreezeItem, int64, error) {
	filters := make([]string, 0)
	data := make([]interface{}, 0)
	entryFilters := make([]string, 0)
	entryData := make([]interface{}, 0)

	const defaultLimit int64 = 100
	const defaultOffset int64 = 0

	currentLimit := defaultLimit
	currentOffset := defaultOffset

	filtersFulltext := make([]string, 0)
	dataFulltext := make([]interface{}, 0)
	entryFiltersFulltext := make([]string, 0)
	entryDataFulltext := make([]interface{}, 0)

	if filter != nil {
		if len(filter.FileIDs) > 0 {
			filters = append(filters, `(file_id IN(?`+strings.Repeat(`,?`, len(filter.FileIDs)-1)+`))`)
			for _, sid := range filter.FileIDs {
				data = append(data, sid)
			}

			entryFilters = append(entryFilters, `(file_id IN(?`+strings.Repeat(`,?`, len(filter.FileIDs)-1)+`))`)
			for _, sid := range filter.FileIDs {
				entryData = append(entryData, sid)
			}
		}
		if filter.SubmitterID != nil {
			filters = append(filters, "(uploader_id = ?)")
			data = append(data, *filter.SubmitterID)
			entryFilters = append(entryFilters, "(uploader_id= ?)")
			entryData = append(entryData, *filter.SubmitterID)
		}
		if filter.SubmitterUsernamePartial != nil {
			tableName := `uploader_username`
			entryTableName := `uploader_username`
			filters, entryFilters, data, entryData = addMultifilter(
				tableName, &entryTableName, *filter.SubmitterUsernamePartial, filters, entryFilters, data, entryData)
		}
		if filter.MD5SumPartial != nil {
			filters = append(filters, "(md5sum LIKE ?)")
			data = append(data, utils.FormatLike(*filter.MD5SumPartial))
			entryFilters = append(entryFilters, "(md5sum LIKE ?)")
			entryData = append(entryData, utils.FormatLike(*filter.MD5SumPartial))
		}
		if filter.SHA256SumPartial != nil {
			filters = append(filters, "(sha256sum LIKE ?)")
			data = append(data, utils.FormatLike(*filter.SHA256SumPartial))
			entryFilters = append(entryFilters, "(sha256sum LIKE ?)")
			entryData = append(entryData, utils.FormatLike(*filter.SHA256SumPartial))
		}

		if filter.NamePrefix != nil {
			filters = append(filters, "(original_filename LIKE ? || '%')")
			data = append(data, utils.FormatLike(*filter.NamePrefix))
			entryFilters = append(entryFilters, "(original_filename LIKE ? || '%')")
			entryData = append(entryData, utils.FormatLike(*filter.NamePrefix))
		}
		if filter.DescriptionPrefix != nil {
			filters = append(filters, "(1 = 0)") // exclude root files

			entryFilters = append(entryFilters, "(description LIKE ? || '%')")
			entryData = append(entryData, utils.FormatLike(*filter.DescriptionPrefix))
		}

		// fulltext filters are inside the nested selects, so they are separate from all the other filters
		if filter.NameFulltext != nil {
			filtersFulltext = append(filtersFulltext, "(MATCH(file.original_filename) AGAINST(? IN BOOLEAN MODE))")
			dataFulltext = append(dataFulltext, utils.FormatLike(*filter.NameFulltext))
			entryFiltersFulltext = append(entryFiltersFulltext, "(MATCH(entry.filename) AGAINST(? IN BOOLEAN MODE))")
			entryDataFulltext = append(entryDataFulltext, utils.FormatLike(*filter.NameFulltext))
		}
		if filter.DescriptionFulltext != nil {
			filters = append(filters, "(1 = 0)") // exclude root files

			entryFiltersFulltext = append(entryFiltersFulltext, "(MATCH(entry.description) AGAINST(? IN BOOLEAN MODE))")
			entryDataFulltext = append(entryDataFulltext, utils.FormatLike(*filter.DescriptionFulltext))
		}

		if filter.SizeMin != nil {
			filters = append(filters, "(size >= ?)")
			data = append(data, *filter.SizeMin)
			entryFilters = append(entryFilters, "(size >= ?)")
			entryData = append(entryData, *filter.SizeMin)
		}
		if filter.SizeMax != nil {
			filters = append(filters, "(size <= ?)")
			data = append(data, *filter.SizeMax)
			entryFilters = append(entryFilters, "(size <= ?)")
			entryData = append(entryData, *filter.SizeMax)
		}
		if filter.SearchFiles != nil || filter.SearchFilesRecursively != nil {
			if !(filter.SearchFiles != nil && filter.SearchFilesRecursively != nil) {
				searchRoot := false
				searchDeep := false

				if filter.SearchFiles != nil {
					searchRoot = *filter.SearchFiles
				}
				if filter.SearchFilesRecursively != nil {
					searchDeep = *filter.SearchFilesRecursively
				}

				filters = append(filters, "(is_root_file = ?)")
				data = append(data, searchRoot)
				entryFilters = append(entryFilters, "(is_deep_file = ?)")
				entryData = append(entryData, searchDeep)
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

	finalData := make([]interface{}, 0)
	finalQuery := `
		SELECT 
			file_id,
		    uploader_id,
		    uploader_username,
		    original_filename,
		    md5sum,
		    sha256sum,
		    size,
		    uploaded_at,
		    description,
		    is_root_file,
		    is_deep_file,
		    indexing_time_seconds,
		    file_count,
			indexing_errors
		FROM (
		SELECT 
       		file.id AS file_id,
			file.fk_user_id AS uploader_id,
			uploader.username AS uploader_username,
			file.original_filename AS original_filename,
			file.md5sum AS md5sum,
			file.sha256sum AS sha256sum,
			file.size AS size,
			file.created_at AS uploaded_at,
			NULL AS description,
			True AS is_root_file,
			False AS is_deep_file,
		    (CASE WHEN file.indexed_at IS NOT NULL THEN (file.indexed_at - file.created_at) END) AS indexing_time_seconds,
			(SELECT COUNT(*) FROM flashfreeze_file_contents WHERE fk_flashfreeze_file_id = file.id) AS file_count,
		    file.indexing_errors as indexing_errors
		FROM flashfreeze_file file
			LEFT JOIN discord_user AS uploader ON uploader.id = file.fk_user_id `

	fulltextQuery := `WHERE file.deleted_at IS NULL ` + magicAnd(filtersFulltext) + strings.Join(filtersFulltext, " AND ") + ` ORDER BY uploaded_at DESC) AS root`
	finalQuery += fulltextQuery
	finalData = append(finalData, dataFulltext...)

	selector := ` WHERE 1=1 ` + magicAnd(filters) + strings.Join(filters, " AND ") + ` GROUP BY file_id `
	finalQuery += selector

	entryQuery := `
		UNION
			SELECT
				file_id,
				uploader_id,
				uploader_username,
				original_filename,
				md5sum,
				sha256sum,
				size,
				uploaded_at,
				description,
				is_root_file,
				is_deep_file,
				indexing_time_seconds,
				file_count,
				indexing_errors
			FROM (
			SELECT
			entry.fk_flashfreeze_file_id AS file_id,
				(SELECT file.fk_user_id FROM flashfreeze_file file WHERE file.id = entry.fk_flashfreeze_file_id) AS uploader_id,
				(SELECT uploader.username FROM flashfreeze_file file LEFT JOIN discord_user AS uploader ON uploader.id = file.fk_user_id WHERE file.id = entry.fk_flashfreeze_file_id) AS uploader_username,
				entry.filename AS original_filename,
				entry.md5sum AS md5sum,
				entry.sha256sum AS sha256sum,
				entry.size_uncompressed AS size,
				NULL AS uploaded_at,
				entry.description as description,
				False as is_root_file,
				True as is_deep_file,
				NULL AS indexing_time_seconds,
				NULL AS file_count,
				NULL AS indexing_errors
			FROM flashfreeze_file_contents entry `
	finalQuery += entryQuery

	entryFulltextQuery := ` WHERE 1=1 ` + magicAnd(entryFiltersFulltext) + strings.Join(entryFiltersFulltext, " AND ") + `) AS deep `
	finalQuery += entryFulltextQuery
	finalData = append(finalData, entryDataFulltext...)

	entrySelector := ` WHERE 1=1 ` + magicAnd(entryFilters) + strings.Join(entryFilters, " AND ")
	finalQuery += entrySelector

	rest := `
		LIMIT ? OFFSET ?
		`

	unlimitedQuery := finalQuery
	finalQuery += rest

	finalData = append(finalData, data...)
	unlimitedData := append(finalData, entryData...)
	finalData = append(unlimitedData, currentLimit, currentOffset)

	countingQuery := `SELECT COUNT(*) FROM ( ` + unlimitedQuery + ` ) AS counterino`
	var counter int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		row := d.db.QueryRowContext(dbs.Ctx(), countingQuery, unlimitedData...)
		if err := row.Scan(&counter); err != nil {
			counter = -1
			return
		}
	}()

	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), finalQuery, finalData...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]*types.ExtendedFlashfreezeItem, 0)

	var uploadedAt *int64
	var indexingTime *int64

	for rows.Next() {
		f := &types.ExtendedFlashfreezeItem{}
		if err := rows.Scan(&f.FileID, &f.SubmitterID, &f.SubmitterUsername,
			&f.OriginalFilename, &f.MD5Sum, &f.SHA256Sum, &f.Size,
			&uploadedAt, &f.Description, &f.IsRootFile, &f.IsDeepFile, &indexingTime, &f.FileCount, &f.IndexingErrors); err != nil {
			return nil, 0, err
		}

		if uploadedAt != nil {
			t := time.Unix(*uploadedAt, 0)
			f.UploadedAt = &t
		}
		if indexingTime != nil {
			t := time.Duration(*indexingTime) * time.Second
			f.IndexingTime = &t
		}

		result = append(result, f)
	}

	rows.Close()
	wg.Wait()

	return result, counter, nil
}

// SearchFixes returns extended fixes based on given filter
func (d *mysqlDAL) SearchFixes(dbs DBSession, filter *types.FixesFilter) ([]*types.ExtendedFixesItem, int64, error) {
	filters := make([]string, 0)
	data := make([]interface{}, 0)

	const defaultLimit int64 = 100
	const defaultOffset int64 = 0

	currentLimit := defaultLimit
	currentOffset := defaultOffset

	filtersFulltext := make([]string, 0)
	dataFulltext := make([]interface{}, 0)

	if filter != nil {
		if len(filter.FixIDs) > 0 {
			filters = append(filters, `(fix_id IN(?`+strings.Repeat(`,?`, len(filter.FixIDs)-1)+`))`)
			for _, sid := range filter.FixIDs {
				data = append(data, sid)
			}
		}
		if filter.SubmitterID != nil {
			filters = append(filters, "(uploader_id = ?)")
			data = append(data, *filter.SubmitterID)
		}
		if filter.SubmitterUsernamePartial != nil {
			tableName := `uploader_username`
			entryTableName := `uploader_username`
			filters, _, data, _ = addMultifilter(
				tableName, &entryTableName, *filter.SubmitterUsernamePartial, filters, nil, data, nil)
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

	finalData := make([]interface{}, 0)
	finalQuery := `
		SELECT 
			fix_id,
		    title,
		    description,
		    uploader_id,
		    uploader_username,
		    uploaded_at
		FROM (
		SELECT 
       		fixes.id AS fix_id,
		    fixes.title AS title,
		    fixes.description AS description,
			fixes.fk_user_id AS uploader_id,
			uploader.username AS uploader_username,
			fixes.created_at AS uploaded_at
		FROM fixes
			LEFT JOIN discord_user AS uploader ON uploader.id = fixes.fk_user_id `

	fulltextQuery := `WHERE fixes.deleted_at IS NULL ` + magicAnd(filtersFulltext) + strings.Join(filtersFulltext, " AND ") + ` ORDER BY uploaded_at DESC) AS root`
	finalQuery += fulltextQuery
	finalData = append(finalData, dataFulltext...)

	selector := ` WHERE 1=1 ` + magicAnd(filters) + strings.Join(filters, " AND ") + ` GROUP BY fix_id `
	finalQuery += selector

	rest := `
		LIMIT ? OFFSET ?
		`

	unlimitedQuery := finalQuery
	finalQuery += rest

	finalData = append(finalData, data...)
	unlimitedData := finalData
	finalData = append(finalData, currentLimit, currentOffset)

	countingQuery := `SELECT COUNT(*) FROM ( ` + unlimitedQuery + ` ) AS counterino`
	var counter int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		row := d.db.QueryRowContext(dbs.Ctx(), countingQuery, unlimitedData...)
		if err := row.Scan(&counter); err != nil {
			counter = -1
			return
		}
	}()

	rows, err := dbs.Tx().QueryContext(dbs.Ctx(), finalQuery, finalData...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]*types.ExtendedFixesItem, 0)

	var uploadedAt *int64

	for rows.Next() {
		f := &types.ExtendedFixesItem{}
		if err := rows.Scan(&f.FixID, &f.Title, &f.Description, &f.SubmitterID, &f.SubmitterUsername, &uploadedAt); err != nil {
			return nil, 0, err
		}

		if uploadedAt != nil {
			t := time.Unix(*uploadedAt, 0)
			f.UploadedAt = &t
		}

		result = append(result, f)
	}

	rows.Close()
	wg.Wait()

	return result, counter, nil
}
