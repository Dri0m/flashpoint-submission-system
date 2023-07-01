package database

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/constants"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type postgresDAL struct {
	db *pgxpool.Pool
}

func NewPostgresDAL(conn *pgxpool.Pool) *postgresDAL {
	return &postgresDAL{
		db: conn,
	}
}

// OpenPostgresDB opens DAL or panics
func OpenPostgresDB(l *logrus.Entry, conf *config.Config) *pgxpool.Pool {
	l.Infoln("connecting to the postgres database")

	user := conf.PostgresUser
	pass := conf.PostgresPassword
	ip := conf.PostgresHost
	port := conf.PostgresPort

	conn, err := pgxpool.New(context.Background(),
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", user, pass, ip, port, user))

	if err != nil {
		l.Fatal(err)
	}

	l.Infoln("postgres database connected")
	return conn
}

type PostgresSession struct {
	context     context.Context
	transaction pgx.Tx
}

// NewSession begins a transaction
func (d *postgresDAL) NewSession(ctx context.Context) (PGDBSession, error) {
	tx, err := d.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}

	return &PostgresSession{
		context:     ctx,
		transaction: tx,
	}, nil
}

func (dbs *PostgresSession) Commit() error {
	return dbs.transaction.Commit(dbs.context)
}

func (dbs *PostgresSession) Rollback() error {
	err := dbs.Tx().Rollback(dbs.context)
	if err != nil && err.Error() == "sql: transaction has already been committed or rolled back" {
		err = nil
	}
	if err != nil {
		utils.LogCtx(dbs.Ctx()).Error(err)
	}
	return err
}

func (dbs *PostgresSession) Tx() pgx.Tx {
	return dbs.transaction
}

func (dbs *PostgresSession) Ctx() context.Context {
	return dbs.context
}

func (d *postgresDAL) GetTagCategories(dbs PGDBSession) ([]*types.TagCategory, error) {
	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT id, name, color, coalesce(description, 'none') as description FROM tag_category`)
	if err != nil {
		return nil, err
	}

	result := make([]*types.TagCategory, 0)

	for rows.Next() {
		category := &types.TagCategory{}
		if err := rows.Scan(&category.ID, &category.Name, &category.Color, &category.Description); err != nil {
			return nil, err
		}

		result = append(result, category)
	}

	rows.Close()

	return result, nil
}

func (d *postgresDAL) SearchTags(dbs PGDBSession, modifiedAfter *string) ([]*types.Tag, error) {
	var rows pgx.Rows
	var err error
	if modifiedAfter != nil {
		rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description, tag_category.name, tag.date_modified, primary_alias, (SELECT string_agg(name, '; ') FROM tag_alias WHERE tag_id = tag.id) as alias_names, user_id 
			FROM tag 
				LEFT JOIN tag_category ON tag_category.id = tag.category_id 
			WHERE tag.date_modified >= $1 AND tag.deleted = FALSE
			ORDER BY tag_category.name, primary_alias`, modifiedAfter)
	} else {
		rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description, tag_category.name, tag.date_modified, primary_alias, (SELECT string_agg(name, '; ') FROM tag_alias WHERE tag_id = tag.id) as alias_names, user_id 
			FROM tag 
				LEFT JOIN tag_category ON tag_category.id = tag.category_id 
			ORDER BY tag_category.name, primary_alias`)
	}
	if err != nil {
		return nil, err
	}

	result := make([]*types.Tag, 0)

	for rows.Next() {
		tag := &types.Tag{}
		if err := rows.Scan(&tag.ID, &tag.Description, &tag.Category, &tag.DateModified, &tag.Name, &tag.Aliases, &tag.UserID); err != nil {
			return nil, err
		}

		result = append(result, tag)
	}

	rows.Close()

	return result, nil
}

func (d *postgresDAL) SearchPlatforms(dbs PGDBSession, modifiedAfter *string) ([]*types.Platform, error) {
	var rows pgx.Rows
	var err error
	if modifiedAfter != nil {
		rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT platform.id, coalesce(platform.description, 'none') as description,
       		platform.date_modified, primary_alias, (SELECT string_agg(name, '; ')
			FROM platform_alias WHERE platform_id = platform.id) as alias_names, user_id 
			FROM platform
			WHERE platform.date_modified >= $1 AND platform.deleted = FALSE
			ORDER BY primary_alias`, modifiedAfter)
	} else {
		rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT platform.id, coalesce(platform.description, 'none') as description, platform.date_modified, primary_alias, (SELECT string_agg(name, '; ') FROM platform_alias WHERE platform_id = platform.id) as alias_names, user_id 
			FROM platform
			ORDER BY primary_alias`)
	}
	if err != nil {
		return nil, err
	}

	result := make([]*types.Platform, 0)

	for rows.Next() {
		platform := &types.Platform{}
		if err := rows.Scan(&platform.ID, &platform.Description, &platform.DateModified, &platform.Name, &platform.Aliases, &platform.UserID); err != nil {
			return nil, err
		}

		result = append(result, platform)
	}

	rows.Close()

	return result, nil
}

func (d *postgresDAL) SearchDeletedGames(dbs PGDBSession, modifiedAfter *string) ([]*types.DeletedGame, error) {
	var rows pgx.Rows
	var err error
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT id, date_modified, reason FROM game
	WHERE game.date_modified >= $1 AND game.deleted = TRUE
	ORDER BY game.date_modified, game.id`, modifiedAfter)
	if err != nil {
		return nil, err
	}

	results := make([]*types.DeletedGame, 0)

	for rows.Next() {
		game := &types.DeletedGame{}
		err = rows.Scan(&game.ID, &game.DateModified, &game.Reason)
		if err != nil {
			return nil, err
		}
		results = append(results, game)
	}

	return results, nil
}

func (d *postgresDAL) CountSinceDate(dbs PGDBSession, modifiedAfter *string) (int, error) {
	result := 0

	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) 
		FROM game 
		WHERE game.date_modified > $1 AND game.deleted = FALSE`, modifiedAfter).Scan(&result)

	if err != nil {
		return 0, err
	}

	return result, nil
}

func (d *postgresDAL) SearchGames(dbs PGDBSession, modifiedAfter *string, modifiedBefore *string, broad bool, afterId *string) ([]*types.Game, []*types.AdditionalApp, []*types.GameData, [][]string, [][]string, error) {
	var rows pgx.Rows
	var err error
	limit := 2500
	tagRelations := make([][]string, 0)
	platformRelations := make([][]string, 0)
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT game.id, game.parent_game_id, game.title, game.alternate_titles, game.series,
       		game.developer, game.publisher, game.date_added, game.date_modified, game.play_mode, game.status, game.notes, game.source,
       		game.application_path, game.launch_command, game.release_date, game.version, game.original_description, game.language,
       		game.library, game.active_data_id, game.tags_str, game.platforms_str, game.platform_name
			FROM game
			WHERE game.date_modified >= $1 AND game.date_modified <= $2 AND game.id > $3 AND game.deleted = FALSE
			ORDER BY game.id
			LIMIT $4`, modifiedAfter, modifiedBefore, afterId, limit)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	result := make([]*types.Game, 0)
	resultAddApps := make([]*types.AdditionalApp, 0)
	resultGameData := make([]*types.GameData, 0)

	for rows.Next() {
		game := &types.Game{}
		if err := rows.Scan(&game.ID, &game.ParentGameID, &game.Title, &game.AlternateTitles, &game.Series, &game.Developer,
			&game.Publisher, &game.DateAdded, &game.DateModified, &game.PlayMode, &game.Status, &game.Notes, &game.Source,
			&game.ApplicationPath, &game.LaunchCommand, &game.ReleaseDate, &game.Version, &game.OriginalDesc, &game.Language,
			&game.Library, &game.ActiveDataID, &game.TagsStr, &game.PlatformsStr, &game.PrimaryPlatform); err != nil {
			return nil, nil, nil, nil, nil, err
		}

		result = append(result, game)
	}

	// Store tag relations
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT gtt.game_id, gtt.tag_id 
	FROM game_tags_tag gtt 
	WHERE gtt.game_id IN (
	    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.date_modified <= $2  AND game.id > $3
	    AND game.deleted = FALSE ORDER BY game.id LIMIT $4
	)`, modifiedAfter, modifiedBefore, afterId, limit)
	for rows.Next() {
		relation := make([]string, 2)
		if err := rows.Scan(&relation[0], &relation[1]); err != nil {
			return nil, nil, nil, nil, nil, err
		}

		tagRelations = append(tagRelations, relation)
	}

	// Store platform relations
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT gpp.game_id, gpp.platform_id 
	FROM game_platforms_platform gpp 
	WHERE gpp.game_id IN (
	    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.date_modified <= $2 AND game.id > $3 
	    AND game.deleted = FALSE ORDER BY game.id LIMIT $4
	)`, modifiedAfter, modifiedBefore, afterId, limit)
	for rows.Next() {
		relation := make([]string, 2)
		if err := rows.Scan(&relation[0], &relation[1]); err != nil {
			return nil, nil, nil, nil, nil, err
		}

		platformRelations = append(platformRelations, relation)
	}

	// If broad include add apps and game data
	if broad {
		if modifiedAfter != nil {
			rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT aa.name, aa.application_path, aa.launch_command,
			aa.wait_for_exit, aa.auto_run_before, aa.parent_game_id
			FROM additional_app aa
			WHERE aa.parent_game_id IN (
			    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.date_modified <= $2 AND game.id > $3 
			    AND game.deleted = FALSE ORDER BY game.id LIMIT $4
			)`, modifiedAfter, modifiedBefore, afterId, limit)

			for rows.Next() {
				addApp := &types.AdditionalApp{}
				if err := rows.Scan(&addApp.Name, &addApp.ApplicationPath, &addApp.LaunchCommand,
					&addApp.WaitForExit, &addApp.AutoRunBefore, &addApp.ParentGameID); err != nil {
					return nil, nil, nil, nil, nil, err
				}

				resultAddApps = append(resultAddApps, addApp)
			}

			rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT gd.id, gd.game_id, gd.title, gd.date_added, gd.sha256,
       		gd.crc32, gd.size, gd.parameters, gd.application_path, gd.launch_command
			FROM game_data gd
			WHERE gd.game_id IN (
				SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.date_modified <= $2 AND game.id > $3 
				AND game.deleted = FALSE ORDER BY game.id LIMIT $4
			)`, modifiedAfter, modifiedBefore, afterId, limit)

			for rows.Next() {
				gameData := &types.GameData{}
				if err := rows.Scan(&gameData.ID, &gameData.GameID, &gameData.Title, &gameData.DateAdded, &gameData.SHA256,
					&gameData.CRC32, &gameData.Size, &gameData.Parameters, &gameData.ApplicationPath, &gameData.LaunchCommand); err != nil {
					return nil, nil, nil, nil, nil, err
				}

				resultGameData = append(resultGameData, gameData)
			}
		}
	}

	rows.Close()

	return result, resultAddApps, resultGameData, tagRelations, platformRelations, nil
}

func (d *postgresDAL) GetTag(dbs PGDBSession, tagId int64) (*types.Tag, error) {
	var tag types.Tag
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description,
       tag_category.name, tag.date_modified, primary_alias,
       (SELECT string_agg(name, '; ') FROM tag_alias WHERE tag_id = tag.id) as alias_names,
       action, reason, deleted, user_id 
		FROM tag 
			LEFT JOIN tag_category ON tag_category.id = tag.category_id 
		WHERE tag.id = $1`, tagId).
		Scan(&tag.ID, &tag.Description, &tag.Category, &tag.DateModified, &tag.Name, &tag.Aliases,
			&tag.Action, &tag.Reason, &tag.Deleted, &tag.UserID)
	if err != nil {
		return nil, err
	}

	return &tag, nil
}

func (d *postgresDAL) GetTagByName(dbs PGDBSession, tagName string) (*types.Tag, error) {
	var tag types.Tag
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description,
       tag_category.name, tag.date_modified, primary_alias,
       (SELECT string_agg(name, '; ') FROM tag_alias WHERE tag_id = tag.id) as alias_names,
       action, reason, deleted, user_id 
		FROM tag 
			LEFT JOIN tag_category ON tag_category.id = tag.category_id 
		WHERE tag.id = (
		    SELECT tag_alias.tag_id FROM tag_alias
		    WHERE tag_alias.name = $1 LIMIT 1
		)`, tagName).
		Scan(&tag.ID, &tag.Description, &tag.Category, &tag.DateModified, &tag.Name, &tag.Aliases,
			&tag.Action, &tag.Reason, &tag.Deleted, &tag.UserID)
	if err != nil {
		return nil, err
	}

	return &tag, nil
}

func (d *postgresDAL) GetPlatform(dbs PGDBSession, platformId int64) (*types.Platform, error) {
	var platform types.Platform
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT platform.id, coalesce(platform.description, 'none') as description,
       		platform.date_modified, primary_alias,
       		(SELECT string_agg(name, '; ') FROM platform_alias WHERE platform_id = platform.id) as alias_names,
       		action, reason, deleted, user_id 
		FROM platform
		WHERE platform.id = $1`, platformId).
		Scan(&platform.ID, &platform.Description, &platform.DateModified, &platform.Name, &platform.Aliases,
			&platform.Action, &platform.Reason, &platform.Deleted, &platform.UserID)
	if err != nil {
		return nil, err
	}

	return &platform, nil
}

func (d *postgresDAL) GetPlatformByName(dbs PGDBSession, platformName string) (*types.Platform, error) {
	var platform types.Platform
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT platform.id, coalesce(platform.description, 'none') as description,
       		platform.date_modified, primary_alias,
       		(SELECT string_agg(name, '; ') FROM platform_alias WHERE platform_id = platform.id) as alias_names,
       		action, reason, deleted, user_id 
		FROM platform
		WHERE platform.id = (
		    SELECT platform_alias.platform_id FROM platform_alias
		    WHERE platform_alias.name = $1 LIMIT 1
		)`, platformName).
		Scan(&platform.ID, &platform.Description, &platform.DateModified, &platform.Name, &platform.Aliases,
			&platform.Action, &platform.Reason, &platform.Deleted, &platform.UserID)
	if err != nil {
		return nil, err
	}

	return &platform, nil
}

func (d *postgresDAL) GetGamesUsingTagTotal(dbs PGDBSession, tagId int64) (int64, error) {
	var count int64
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) FROM game_tags_tag WHERE tag_id = $1
	AND game_id IN (SELECT id FROM game WHERE deleted = FALSE)`, tagId).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (d *postgresDAL) GetGame(dbs PGDBSession, gameId string) (*types.Game, error) {
	// Get game
	var game types.Game
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT id, parent_game_id, title, alternate_titles, series, developer,
       		publisher, date_added, date_modified, play_mode, status, notes,
       		source, application_path, launch_command, release_Date, version,
       		original_description, language, library, active_data_id, tags_str, platforms_str,
       		action, reason, deleted, user_id, platform_name
			FROM game WHERE id = $1`, gameId).
		Scan(&game.ID, &game.ParentGameID, &game.Title, &game.AlternateTitles, &game.Series, &game.Developer,
			&game.Publisher, &game.DateAdded, &game.DateModified, &game.PlayMode, &game.Status, &game.Notes,
			&game.Source, &game.ApplicationPath, &game.LaunchCommand, &game.ReleaseDate, &game.Version,
			&game.OriginalDesc, &game.Language, &game.Library, &game.ActiveDataID, &game.TagsStr, &game.PlatformsStr,
			&game.Action, &game.Reason, &game.Deleted, &game.UserID, &game.PrimaryPlatform)
	if err != nil {
		return nil, err
	}

	// Get add apps
	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT * FROM additional_app WHERE parent_game_id = $1`, gameId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var addApp types.AdditionalApp
		err = rows.Scan(&addApp.ID, &addApp.ApplicationPath, &addApp.AutoRunBefore, &addApp.LaunchCommand, &addApp.Name, &addApp.WaitForExit, &addApp.ParentGameID)
		if err != nil {
			return nil, err
		}
		game.AddApps = append(game.AddApps, &addApp)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Get game data
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT id, game_id, title, date_added, sha256,
       crc32, size, parameters, application_path, launch_command, indexed FROM game_data WHERE game_id = $1
       ORDER BY game_data.date_added DESC`, gameId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.GameData
		err = rows.Scan(&data.ID, &data.GameID, &data.Title, &data.DateAdded, &data.SHA256,
			&data.CRC32, &data.Size, &data.Parameters, &data.ApplicationPath, &data.LaunchCommand, &data.Indexed)
		if err != nil {
			return nil, err
		}
		game.Data = append(game.Data, &data)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Get tags
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description, tag_category.name, tag.date_modified, primary_alias, user_id FROM tag LEFT JOIN tag_category ON tag_category.id = tag.category_id WHERE tag.id IN (
    	SELECT tag_id FROM game_tags_tag WHERE game_id = $1) ORDER BY primary_alias`, gameId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.Tag
		err = rows.Scan(&data.ID, &data.Description, &data.Category, &data.DateModified, &data.Name, &data.UserID)
		if err != nil {
			return nil, err
		}
		game.Tags = append(game.Tags, &data)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Get platforms
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT id, date_modified, primary_alias, description, user_id FROM platform WHERE id IN (
    	SELECT platform_id FROM game_platforms_platform WHERE game_id = $1) ORDER BY primary_alias`, gameId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.Platform
		err = rows.Scan(&data.ID, &data.DateModified, &data.Name, &data.Description, &data.UserID)
		if err != nil {
			return nil, err
		}
		game.Platforms = append(game.Platforms, &data)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	return &game, nil
}

func (d *postgresDAL) GetGameDataIndex(dbs PGDBSession, gameID string, date int64) (*types.GameDataIndex, error) {
	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT path, size, 
        encode(crc32, 'hex'), 
        encode(md5, 'hex'),
        encode(sha1, 'hex'),
        encode(sha256, 'hex'),
        zip_date
		FROM game_data_index WHERE game_id = $1`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var index types.GameDataIndex
	index.GameID = gameID
	index.Date = date
	index.Data = make([]types.GameDataIndexFile, 0)

	// Save results with matching date to row (This saves a lot of space over using an index)
	for rows.Next() {
		var r types.GameDataIndexFile
		var rDate time.Time
		err = rows.Scan(&r.Path, &r.Size, &r.CRC32, &r.MD5, &r.SHA1, &r.SHA256, &rDate)
		if err != nil {
			return nil, err
		}
		if rDate.UnixMilli() == date {
			// Matching date, add to list
			index.Data = append(index.Data, r)
		}
	}

	return &index, nil
}

func (d *postgresDAL) GetIndexMatchesHash(dbs PGDBSession, hashType string, hashStr string) ([]*types.IndexMatchData, error) {
	data := make([]*types.IndexMatchData, 0)

	rows, err := dbs.Tx().Query(dbs.Ctx(), fmt.Sprintf(`SELECT 
		encode(crc32, 'hex'), 
        encode(md5, 'hex'),
        encode(sha1, 'hex'),
        encode(sha256, 'hex'),
        size, path, game_id, zip_date 
		FROM game_data_index WHERE %s = decode($1, 'hex')`, hashType), hashStr)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var r types.IndexMatchData
		var d time.Time
		err = rows.Scan(&r.CRC32, &r.MD5, &r.SHA1, &r.SHA256, &r.Size, &r.Path, &r.GameID, &d)
		if err != nil {
			return nil, err
		}
		r.Date = d.UnixMilli()
		data = append(data, &r)
	}

	return data, nil
}

func (d *postgresDAL) SaveTag(dbs PGDBSession, tag *types.Tag, uid int64) error {
	// Store existing primary alias, update redundant game fields if changes later
	existingTag, err := d.GetTag(dbs, tag.ID)
	if err != nil {
		return err
	}
	aliasChanged := strings.ToLower(tag.Name) != strings.ToLower(existingTag.Name)
	descUpdated := strings.ToLower(tag.Description) != strings.ToLower(existingTag.Description)
	aliasesUpdated := strings.ToLower(*tag.Aliases) != strings.ToLower(*existingTag.Aliases)
	categoryUpdated := strings.ToLower(tag.Category) != strings.ToLower(existingTag.Category)
	categories, err := d.GetTagCategories(dbs)
	if err != nil {
		return err
	}

	// Generate tag update reason
	reasons := make([]string, 0)
	if descUpdated {
		reasons = append(reasons, "Description Changed")
	}
	if aliasChanged {
		reasons = append(reasons, "Primary Alias Changed")
	}
	if aliasesUpdated {
		reasons = append(reasons, "Aliases Changed")
	}
	if categoryUpdated {
		reasons = append(reasons, "Category Changed")

		// Make sure category is valid
		if !containsCategory(categories, tag.Category) {
			return types.InvalidTagUpdate{}
		}
	}

	if len(reasons) == 0 {
		return types.InvalidTagUpdate{}
	}

	// Make sure all alias changes are valid
	aliases := strings.Split(*tag.Aliases, ";")
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		existingTagWithAlias, _ := d.GetTagByName(dbs, alias)
		if existingTagWithAlias != nil && existingTagWithAlias.ID != tag.ID {
			// Alias exists on another tag, cannot save
			return types.InvalidTagUpdate{}
		}
	}

	// Make sure at least one alias is present
	if len(aliases) == 0 {
		return types.InvalidTagUpdate{}
	}
	if aliases[0] == "" {
		return types.InvalidTagUpdate{}
	}

	// Remove old aliases
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM tag_alias WHERE tag_id = $1`, tag.ID)
	if err != nil {
		return err
	}

	// Add new aliases
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		_, err := dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO tag_alias (name, tag_id) VALUES ($1, $2)`,
			alias, tag.ID)
		if err != nil {
			return err
		}
	}

	// Update tag
	var catId int64
	if categoryUpdated {
		catId = getCategoryID(categories, tag.Category)
	} else {
		catId = getCategoryID(categories, existingTag.Category)
	}
	_, err = dbs.Tx().Exec(dbs.Ctx(), `UPDATE tag SET description = $1, primary_alias = $2, category_id = $3,
               reason = $4, action = 'update', user_id = $5 WHERE id = $6`,
		tag.Description, tag.Name, catId, strings.Join(reasons, ", "), uid, tag.ID)
	if err != nil {
		return err
	}

	if aliasChanged {
		// Update redundant game fields
		_, err := dbs.Tx().Exec(dbs.Ctx(), `UPDATE game
			SET reason = 'Updating Redundant Tag String',
			action = 'update',
			user_id = $1,
			tags_str = coalesce(
				(
					SELECT string_agg(
								   (SELECT primary_alias FROM tag WHERE id = t.tag_id), '; '
							   )
					FROM game_tags_tag t
					WHERE t.game_id = game.id
				), ''
			) WHERE game.id IN (
			    SELECT game_tags_tag.game_id FROM game_tags_tag
				WHERE game_tags_tag.tag_id = $2
			)`, constants.SystemID, tag.ID)
		if err != nil {
			return err
		}
	}

	err = dbs.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (d *postgresDAL) SaveGame(dbs PGDBSession, game *types.Game, uid int64) error {
	newTags := make([]*types.Tag, 0)
	newPlats := make([]*types.Platform, 0)

	// Clear tag relations
	_, err := dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_tags_tag WHERE game_id = $1`, game.ID)
	// Match tags to existing ones by name
	for _, tag := range game.Tags {
		t, err := d.GetOrCreateTag(dbs, tag.Name, "default", fmt.Sprintf("Game Metadata Update - ID %s", game.ID), uid)
		if err != nil {
			return err
		}
		newTags = append(newTags, t)
	}
	// Save tag relations
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_tags_tag"},
		[]string{"game_id", "tag_id"},
		pgx.CopyFromSlice(len(newTags), func(i int) ([]interface{}, error) {
			return []interface{}{
				game.ID,
				newTags[i].ID,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Clear platform relations
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_platforms_platform WHERE game_id = $1`, game.ID)
	// Create new platforms where applicable
	for _, plat := range game.Platforms {
		p, err := d.GetOrCreatePlatform(dbs, plat.Name, fmt.Sprintf("Game Metadata Update - ID %s", game.ID), uid)
		if err != nil {
			return err
		}
		newPlats = append(newPlats, p)
	}
	// Save platform relations
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_platforms_platform"},
		[]string{"game_id", "platform_id"},
		pgx.CopyFromSlice(len(newPlats), func(i int) ([]interface{}, error) {
			return []interface{}{
				game.ID,
				newPlats[i].ID,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Save add apps
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM additional_app WHERE parent_game_id = $1`, game.ID)
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"additional_app"},
		[]string{"application_path", "auto_run_before", "launch_command", "name", "wait_for_exit", "parent_game_id"},
		pgx.CopyFromSlice(len(game.AddApps), func(i int) ([]interface{}, error) {
			return []interface{}{
				game.AddApps[i].ApplicationPath,
				game.AddApps[i].AutoRunBefore,
				game.AddApps[i].LaunchCommand,
				game.AddApps[i].Name,
				game.AddApps[i].WaitForExit,
				game.ID,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Save game
	query := `UPDATE game SET parent_game_id=$1, title=$2, alternate_titles=$3, series=$4, developer=$5,
                publisher=$6, play_mode=$7, status=$8, notes=$9, source=$10,
                application_path=$11, launch_command=$12, release_date=$13, version=$14, original_description=$15,
                language=$16, library=$17, active_data_id=$18, user_id=$19, action=$20, reason=$21,
                tags_str = coalesce(
					(
						SELECT string_agg(
										(SELECT primary_alias FROM tag WHERE id = t.tag_id), '; '
									)
						FROM game_tags_tag t
						WHERE t.game_id = game.id
					 ), ''
				),
                platforms_str = (
					SELECT string_agg(
					   (SELECT primary_alias FROM platform WHERE id = p.platform_id), '; '
				    )
					FROM game_platforms_platform p
					WHERE p.game_id = game.id
				), 
				platform_name=$22
            	WHERE id=$23`
	_, err = dbs.Tx().Exec(dbs.Ctx(), query, game.ParentGameID, game.Title, game.AlternateTitles, game.Series, game.Developer,
		game.Publisher, game.PlayMode, game.Status, game.Notes, game.Source,
		game.ApplicationPath, game.LaunchCommand, game.ReleaseDate, game.Version, game.OriginalDesc,
		game.Language, game.Library, game.ActiveDataID, uid, "update", "User changed metadata", game.PrimaryPlatform, game.ID)
	if err != nil {
		return err
	}

	err = dbs.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (d *postgresDAL) DeveloperImportDatabaseJson(dbs PGDBSession, dump *types.LauncherDump) error {
	// Delete all existing entries
	_, err := dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM tag WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM tag_alias WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM tag_category WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM platform WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM platform_alias WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM additional_app WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_data WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_tags_tag WHERE 1=1`)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_platforms_platform WHERE 1=1`)
	if err != nil {
		return err
	}
	conf := config.GetConfig(nil)

	// --- TAGS ---

	// Copy tag categories
	utils.LogCtx(dbs.Ctx()).Debug("copying tag categories")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"tag_category"},
		[]string{"id", "name", "color", "description"},
		pgx.CopyFromSlice(len(dump.Tags.Categories), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Tags.Categories[i].ID,
				dump.Tags.Categories[i].Name,
				dump.Tags.Categories[i].Color,
				dump.Tags.Categories[i].Description,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copy tag aliases
	utils.LogCtx(dbs.Ctx()).Debug("copying tag aliases")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"tag_alias"},
		[]string{"tag_id", "name"},
		pgx.CopyFromSlice(len(dump.Tags.Aliases), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Tags.Aliases[i].TagID,
				dump.Tags.Aliases[i].Name,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copy tags
	utils.LogCtx(dbs.Ctx()).Debug("copying tags")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"tag"},
		[]string{"id", "category_id", "description", "primary_alias", "action", "reason", "user_id"},
		pgx.CopyFromSlice(len(dump.Tags.Tags), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Tags.Tags[i].ID,
				dump.Tags.Tags[i].CategoryID,
				dump.Tags.Tags[i].Description,
				dump.Tags.Tags[i].PrimaryAlias,
				"create",
				"Database Import",
				conf.SystemUid,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// --- PLATFORMS ---

	// Copy platform aliases
	utils.LogCtx(dbs.Ctx()).Debug("copying platform aliases")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"platform_alias"},
		[]string{"platform_id", "name"},
		pgx.CopyFromSlice(len(dump.Platforms.Aliases), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Platforms.Aliases[i].PlatformID,
				dump.Platforms.Aliases[i].Name,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copy platforms
	utils.LogCtx(dbs.Ctx()).Debug("copying platforms")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"platform"},
		[]string{"id", "description", "primary_alias", "action", "reason", "user_id"},
		pgx.CopyFromSlice(len(dump.Platforms.Platforms), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Platforms.Platforms[i].ID,
				dump.Platforms.Platforms[i].Description,
				dump.Platforms.Platforms[i].PrimaryAlias,
				"create",
				"Database Import",
				conf.SystemUid,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// --- GAMES ---

	// Copy add apps
	utils.LogCtx(dbs.Ctx()).Debug("copying add apps")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"additional_app"},
		[]string{"application_path", "auto_run_before", "launch_command", "name", "wait_for_exit", "parent_game_id"},
		pgx.CopyFromSlice(len(dump.Games.AddApps), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Games.AddApps[i].ApplicationPath,
				dump.Games.AddApps[i].AutoRunBefore,
				dump.Games.AddApps[i].LaunchCommand,
				dump.Games.AddApps[i].Name,
				dump.Games.AddApps[i].WaitForExit,
				dump.Games.AddApps[i].ParentGameID,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copy game data
	utils.LogCtx(dbs.Ctx()).Debug("copying game data")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_data"},
		[]string{"game_id", "title", "date_added", "sha256", "crc32", "size", "parameters", "application_path", "launch_command"},
		pgx.CopyFromSlice(len(dump.Games.GameData), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Games.GameData[i].GameID,
				dump.Games.GameData[i].Title,
				dump.Games.GameData[i].DateAdded,
				dump.Games.GameData[i].SHA256,
				dump.Games.GameData[i].CRC32,
				dump.Games.GameData[i].Size,
				dump.Games.GameData[i].Parameters,
				dump.Games.GameData[i].ApplicationPath,
				dump.Games.GameData[i].LaunchCommand,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copy game tag relations
	utils.LogCtx(dbs.Ctx()).Debug("copying game tag relations")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_tags_tag"},
		[]string{"game_id", "tag_id"},
		pgx.CopyFromSlice(len(dump.TagRelations), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.TagRelations[i].GameID,
				dump.TagRelations[i].Value,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copy game platform relations
	utils.LogCtx(dbs.Ctx()).Debug("copying game platform relations")
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_platforms_platform"},
		[]string{"game_id", "platform_id"},
		pgx.CopyFromSlice(len(dump.PlatformRelations), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.PlatformRelations[i].GameID,
				dump.PlatformRelations[i].Value,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Copying games
	utils.LogCtx(dbs.Ctx()).Debug("copying games")
	t := time.Now()
	// Disable triggers
	_, err = dbs.Tx().Exec(dbs.Ctx(), `SET session_replication_role = replica`)
	if err != nil {
		return err
	}
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game"},
		[]string{"id", "parent_game_id", "title", "alternate_titles", "series", "developer", "publisher",
			"date_added", "date_modified", "play_mode", "status", "notes", "source", "application_path",
			"launch_command", "release_date", "version", "original_description", "language", "library",
			"tags_str", "platforms_str", "action", "reason", "user_id", "platform_name"},
		pgx.CopyFromSlice(len(dump.Games.Games), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Games.Games[i].ID,
				dump.Games.Games[i].ParentGameID,
				dump.Games.Games[i].Title,
				dump.Games.Games[i].AlternateTitles,
				dump.Games.Games[i].Series,
				dump.Games.Games[i].Developer,
				dump.Games.Games[i].Publisher,
				dump.Games.Games[i].DateAdded,
				t,
				dump.Games.Games[i].PlayMode,
				dump.Games.Games[i].Status,
				dump.Games.Games[i].Notes,
				dump.Games.Games[i].Source,
				dump.Games.Games[i].ApplicationPath,
				dump.Games.Games[i].LaunchCommand,
				dump.Games.Games[i].ReleaseDate,
				dump.Games.Games[i].Version,
				dump.Games.Games[i].OriginalDesc,
				dump.Games.Games[i].Language,
				dump.Games.Games[i].Library,
				dump.Games.Games[i].TagsStr,
				dump.Games.Games[i].PlatformsStr,
				"create",
				"Database Import",
				conf.SystemUid,
				dump.Games.Games[i].PrimaryPlatform,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	utils.LogCtx(dbs.Ctx()).Debug("manual logging")

	// Manual game logging
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_additional_app (application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id, date_modified) 
		SELECT application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id, $1
		FROM additional_app`, t)
	if err != nil {
		return err
	}
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game_data (game_id, title, date_added, sha256, crc32, size, parameters, date_modified,
                                 application_path, launch_command)
		SELECT game_id, title, date_added, sha256, crc32, size, parameters, $1, application_path, launch_command
		FROM game_data`, t)
	if err != nil {
		return err
	}
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game_tags_tag ("game_id", "tag_id", "date_modified")
   		SELECT game_id, tag_id, $1 FROM game_tags_tag`, t)
	if err != nil {
		return err
	}
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game_platforms_platform ("game_id", "platform_id", "date_modified")
   		SELECT game_id, platform_id, $1 FROM game_platforms_platform`, t)
	if err != nil {
		return err
	}
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game (id, parent_game_id, title, alternate_titles, series,
                            developer, publisher, date_added, date_modified, play_mode, status, notes, source,
                            application_path, launch_command, release_date, version, original_description, language,
                            library, active_data_id, tags_str, platforms_str, action, reason, user_id, platform_name)
		SELECT id, parent_game_id, title, alternate_titles, series,
		       developer, publisher, date_added, date_modified, play_mode, status, notes, source,
		       application_path, launch_command, release_date, version, original_description, language,
		       library, active_data_id, tags_str, platforms_str, action, reason, user_id, platform_name
		FROM game`)
	if err != nil {
		return err
	}

	utils.LogCtx(dbs.Ctx()).Debug("done database import")

	return nil
}

func (d *postgresDAL) GetTagCategory(dbs PGDBSession, categoryId int64) (*types.TagCategory, error) {
	var category types.TagCategory
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT "id", "name", "color", "description" FROM tag_category WHERE "id" = $1`,
		categoryId).Scan(&category.ID, &category.Name, &category.Color, &category.Description)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (d *postgresDAL) GetOrCreateTagCategory(dbs PGDBSession, categoryName string) (*types.TagCategory, error) {
	categoryId, err := GetCategoryID(dbs, categoryName)
	if err != nil {
		return nil, err
	}
	if categoryId != -1 {
		// Category found, return
		return d.GetTagCategory(dbs, categoryId)
	} else {
		// Create new tag category
		var newCategoryId int64
		err = dbs.Tx().QueryRow(dbs.Ctx(), `INSERT INTO tag_category ("name", "color", "description")
			VALUES ($1, $2, $3) RETURNING id`,
			categoryName, "#FFFFFF", "").Scan(&newCategoryId)
		if err != nil {
			return nil, err
		}

		// Return tag category
		return d.GetTagCategory(dbs, newCategoryId)
	}
}

func (d *postgresDAL) GetOrCreatePlatform(dbs PGDBSession, platformName string, reason string, uid int64) (*types.Platform, error) {
	platformId, err := GetPlatformID(dbs, platformName)
	if err != nil {
		return nil, err
	}
	if platformId != -1 {
		// Platform exists, return
		return d.GetPlatform(dbs, platformId)
	} else {
		// Create new alias
		_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO platform_alias ("name", "platform_id")
			VALUES ($1, $2)`,
			platformName, -1)
		if err != nil {
			return nil, err
		}

		// Create new platform
		var newPlatformId int64
		err = dbs.Tx().QueryRow(dbs.Ctx(), `INSERT INTO platform ("primary_alias", "description", "action", "reason", "user_id")
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			platformName, "", "create", reason, uid).
			Scan(&newPlatformId)
		if err != nil {
			return nil, err
		}

		// Set Platform Alias ID
		_, err = dbs.Tx().Exec(dbs.Ctx(), `UPDATE platform_alias SET platform_id = $1 WHERE name = $2`,
			newPlatformId, platformName)
		if err != nil {
			return nil, err
		}

		return d.GetPlatform(dbs, newPlatformId)
	}
}

func (d *postgresDAL) GetOrCreateTag(dbs PGDBSession, tagName string, tagCategory string, reason string, uid int64) (*types.Tag, error) {
	// Find tag id if already exists
	utils.LogCtx(dbs.Ctx()).Debug("Getting tag id if exists")
	tagId, err := GetTagID(dbs, tagName)
	if err != nil {
		return nil, err
	}
	if tagId != -1 {
		// Found tag, return early
		return d.GetTag(dbs, tagId)
	} else {
		// Tag not found create new tag
		// Get category ID
		category, err := d.GetOrCreateTagCategory(dbs, tagCategory)
		if err != nil {
			return nil, err
		}

		// Create Tag Alias
		_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO tag_alias ("name", "tag_id")
			VALUES ($1, $2)`,
			tagName, -1)
		if err != nil {
			return nil, err
		}

		// Create Tag
		var newTagId int64
		err = dbs.Tx().QueryRow(dbs.Ctx(), `INSERT INTO tag ("primary_alias", "category_id", "description",
                 "action", "reason", "user_id")
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			tagName, category.ID, "", "create", reason, uid).
			Scan(&newTagId)
		if err != nil {
			return nil, err
		}

		// Set Tag Alias ID
		_, err = dbs.Tx().Exec(dbs.Ctx(), `UPDATE tag_alias SET tag_id = $1 WHERE name = $2`,
			newTagId, tagName)
		if err != nil {
			return nil, err
		}

		// Return new tag
		return d.GetTag(dbs, newTagId)
	}
}

func (d *postgresDAL) ApplyGamePatch(dbs PGDBSession, uid int64, game *types.Game, patch *types.GameContentPatch, addApps []*types.CurationAdditionalApp) error {
	if addApps != nil {
		// Clear existing add apps
		_, err := dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM additional_app
			WHERE parent_game_id = $1`, game.ID)
		if err != nil {
			return err
		}

		// Add new add apps
		for _, addApp := range addApps {
			_, err := dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO additional_app
				(application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				addApp.ApplicationPath, false, addApp.LaunchCommand, addApp.Heading, false, game.ID)
			if err != nil {
				return err
			}
		}
	}

	if patch != nil {
		if patch.Title != nil {
			game.Title = *patch.Title
		}
		if patch.AlternateTitles != nil {
			game.AlternateTitles = *patch.AlternateTitles
		}
		if patch.Series != nil {
			game.Series = *patch.Series
		}
		if patch.Developer != nil {
			game.Developer = *patch.Developer
		}
		if patch.Publisher != nil {
			game.Publisher = *patch.Publisher
		}
		if patch.PlayMode != nil {
			game.PlayMode = *patch.PlayMode
		}
		if patch.Status != nil {
			game.Status = *patch.Status
		}
		if patch.Notes != nil {
			game.Notes = *patch.Notes
		}
		if patch.Source != nil {
			game.Source = *patch.Source
		}
		if patch.ReleaseDate != nil {
			game.ReleaseDate = *patch.ReleaseDate
		}
		if patch.Version != nil {
			game.Version = *patch.Version
		}
		if patch.OriginalDesc != nil {
			game.OriginalDesc = *patch.OriginalDesc
		}
		if patch.Language != nil {
			game.Language = *patch.Language
		}
		if patch.Library != nil {
			game.Library = *patch.Library
		}
	}

	_, err := dbs.Tx().Exec(dbs.Ctx(), `UPDATE game 
		SET title = $1, alternate_titles = $2, series = $3, developer = $4, publisher = $5,
		    play_mode = $6, status = $7, notes = $8, source = $9, release_date = $10,
		    version = $11, original_description = $12, language = $13, library = $14,
		    action = 'update', reason = 'Content Patch Metadata', user_id = $15
		    WHERE id = $16`,
		game.Title, game.AlternateTitles, game.Series, game.Developer, game.Publisher,
		game.PlayMode, game.Status, game.Notes, game.Source, game.ReleaseDate,
		game.Version, game.OriginalDesc, game.Language, game.Library, uid, game.ID)
	if err != nil {
		return err
	}

	return nil
}

func (d *postgresDAL) IndexerGetNext(ctx context.Context) (*types.GameData, error) {
	var gameData types.GameData
	err := d.db.QueryRow(ctx, `SELECT id, game_id, date_added
		FROM game_data WHERE indexed = FALSE AND index_error = FALSE LIMIT 1`).Scan(
		&gameData.ID, &gameData.GameID, &gameData.DateAdded)
	if err != nil {
		return nil, err
	}
	return &gameData, nil
}

func (d *postgresDAL) IndexerInsert(ctx context.Context, crc32sum []byte, md5sum []byte, sha256sum []byte, sha1sum []byte,
	size uint64, path string, gameId string, zipDate time.Time) error {
	_, err := d.db.Exec(ctx, `INSERT INTO game_data_index (crc32, md5, sha256, sha1, size, path, game_id, zip_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		crc32sum, md5sum, sha256sum, sha1sum, size, path, gameId, zipDate)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(ctx, `UPDATE game_data SET indexed = TRUE, index_error = FALSE WHERE game_id = $1 AND date_added = $2`,
		gameId, zipDate)
	return err
}

func (d *postgresDAL) IndexerMarkFailure(ctx context.Context, gameId string, zipDate time.Time) error {
	_, err := d.db.Exec(ctx, `UPDATE game_data SET index_error = TRUE WHERE game_id = $1 AND date_added = $2`,
		gameId, zipDate)
	return err
}

func (d *postgresDAL) AddGameData(dbs PGDBSession, uid int64, gameId string, vr *types.ValidatorRepackResponse) (*types.GameData, error) {
	// DO EXPENSIVE OPERATIONS FIRST

	// Game Data - Get SHA256
	file, err := os.Open(*vr.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	hashStr := fmt.Sprintf("%x", hash.Sum(nil))

	// Game Data - Get Size
	fileInfo, err := os.Stat(*vr.FilePath)
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// Save Game Data

	var gameData types.GameData
	gameData.GameID = gameId
	gameData.Title = "Data Pack"
	gameData.Size = fileSize
	gameData.SHA256 = hashStr
	gameData.CRC32 = 0
	gameData.Parameters = vr.Meta.MountParameters
	gameData.ApplicationPath = *vr.Meta.ApplicationPath
	gameData.LaunchCommand = *vr.Meta.LaunchCommand
	err = dbs.Tx().QueryRow(dbs.Ctx(), `INSERT INTO game_data ("game_id", "title", "sha256", "crc32", "size", "parameters",
                       "application_path", "launch_command")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, date_added`,
		&gameData.GameID, &gameData.Title, &gameData.SHA256, &gameData.CRC32, &gameData.Size, &gameData.Parameters,
		&gameData.ApplicationPath, &gameData.LaunchCommand).
		Scan(&gameData.ID, &gameData.DateAdded)
	if err != nil {
		return nil, err
	}
	// Update active data id
	_, err = dbs.Tx().Exec(dbs.Ctx(), `UPDATE game 
	SET active_data_id = (SELECT id FROM game_data WHERE game_data.game_id = game.id ORDER BY date_modified DESC LIMIT 1),
	    user_id = $1, action = 'update', reason = 'Content Patch Game Data'
	WHERE game.id = $2`, uid, gameId)
	if err != nil {
		return nil, err
	}

	return &gameData, nil
}

func (d *postgresDAL) AddSubmissionFromValidator(dbs PGDBSession, uid int64, vr *types.ValidatorRepackResponse) (*types.Game, error) {
	// DO EXPENSIVE OPERATIONS FIRST

	// Game Data - Get SHA256
	file, err := os.Open(*vr.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	hashStr := fmt.Sprintf("%x", hash.Sum(nil))

	// Game Data - Get Size
	fileInfo, err := os.Stat(*vr.FilePath)
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// Map metadata to a new game
	game := types.Game{}

	game.ID = uuid.New().String()
	EnsureString(vr.Meta.Title, &game.Title)
	EnsureString(vr.Meta.AlternateTitles, &game.AlternateTitles)
	EnsureString(vr.Meta.Series, &game.Series)
	EnsureString(vr.Meta.Developer, &game.Developer)
	EnsureString(vr.Meta.Publisher, &game.Publisher)
	EnsureString(vr.Meta.PlayMode, &game.PlayMode)
	EnsureString(vr.Meta.Status, &game.Status)
	EnsureString(vr.Meta.GameNotes, &game.Notes)
	EnsureString(vr.Meta.Source, &game.Source)
	EnsureString(vr.Meta.ApplicationPath, &game.ApplicationPath)
	EnsureString(vr.Meta.LaunchCommand, &game.LaunchCommand)
	EnsureString(vr.Meta.ReleaseDate, &game.ReleaseDate)
	EnsureString(vr.Meta.Version, &game.Version)
	EnsureString(vr.Meta.OriginalDescription, &game.OriginalDesc)
	EnsureString(vr.Meta.Languages, &game.Language)
	EnsureString(vr.Meta.Library, &game.Library)
	EnsureString(vr.Meta.PrimaryPlatform, &game.PrimaryPlatform)
	game.UserID = uid
	game.AddApps = make([]*types.AdditionalApp, 0)

	// Tags
	rawTags := strings.Split(*vr.Meta.Tags, ";")
	tags := make([]*types.Tag, 0)
	for _, tagName := range rawTags {
		tag, err := d.GetOrCreateTag(dbs, strings.TrimSpace(tagName), "default", "Submission Import", uid)
		if err != nil {
			return nil, err
		}

		tags = append(tags, tag)
	}

	// Platforms
	rawPlatforms := strings.Split(*vr.Meta.Platform, ";")
	platforms := make([]*types.Platform, 0)
	for _, platformName := range rawPlatforms {
		platform, err := d.GetOrCreatePlatform(dbs, strings.TrimSpace(platformName), "Submission Import", uid)
		if err != nil {
			return nil, err
		}

		platforms = append(platforms, platform)
	}

	// Save Game Data

	if vr.Meta.ApplicationPath == nil || vr.Meta.LaunchCommand == nil {
		return nil, types.MissingLaunchParams{}
	}

	var gameData types.GameData
	gameData.GameID = game.ID
	gameData.Title = "Data Pack"
	gameData.Size = fileSize
	gameData.SHA256 = hashStr
	gameData.CRC32 = 0
	gameData.Parameters = vr.Meta.MountParameters
	gameData.ApplicationPath = *vr.Meta.ApplicationPath
	gameData.LaunchCommand = *vr.Meta.LaunchCommand
	err = dbs.Tx().QueryRow(dbs.Ctx(), `INSERT INTO game_data ("game_id", "title", "sha256", "crc32", "size", "parameters",
                       "application_path", "launch_command")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, date_added`,
		&gameData.GameID, &gameData.Title, &gameData.SHA256, &gameData.CRC32, &gameData.Size, &gameData.Parameters,
		&gameData.ApplicationPath, &gameData.LaunchCommand).
		Scan(&gameData.ID, &gameData.DateAdded)
	if err != nil {
		return nil, err
	}
	game.Data = []*types.GameData{&gameData}

	// Save Additional Apps
	if vr.Meta.Extras != nil {
		_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO additional_app 
    	("application_path", "auto_run_before", "launch_command", "name", "wait_for_exit", "parent_game_id") 
		VALUES (':extras:', FALSE, $1, 'Extras', FALSE, $2)`,
			vr.Meta.Extras, game.ID)
		if err != nil {
			return nil, err
		}
	}
	if vr.Meta.Message != nil {
		_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO additional_app 
    	("application_path", "auto_run_before", "launch_command", "name", "wait_for_exit", "parent_game_id") 
		VALUES (':message:', FALSE, $1, 'Message', FALSE, $2)`,
			vr.Meta.Message, game.ID)
		if err != nil {
			return nil, err
		}
	}
	for _, addApp := range vr.Meta.AdditionalApps {
		if addApp.ApplicationPath != nil && addApp.LaunchCommand != nil && addApp.Heading != nil {
			_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO additional_app 
    		("application_path", "auto_run_before", "launch_command", "name", "wait_for_exit", "parent_game_id") 
			VALUES ($1, FALSE, $2, $3, FALSE, $4)`,
				addApp.ApplicationPath, addApp.LaunchCommand, addApp.Heading, game.ID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, types.InvalidAddApps{}
		}
	}

	// Save Tag Relations
	for _, tag := range tags {
		_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO game_tags_tag ("game_id", "tag_id") VALUES ($1, $2)`, game.ID, tag.ID)
		if err != nil {
			return nil, err
		}
	}
	tagsStrArr := make([]string, 0)
	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT tag.primary_alias FROM tag WHERE tag.id IN (
		SELECT DISTINCT tag_id FROM game_tags_tag WHERE game_tags_tag.game_id = $1
	)`, game.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tagName string
		err = rows.Scan(&tagName)
		if err != nil {
			return nil, err
		}
		tagsStrArr = append(tagsStrArr, tagName)
	}

	// Save Platform Relations
	for _, platform := range platforms {
		_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO game_platforms_platform ("game_id", "platform_id") VALUES ($1, $2)`,
			game.ID, platform.ID)
		if err != nil {
			return nil, err
		}
	}
	platformsStrArr := make([]string, 0)
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT platform.primary_alias FROM platform WHERE platform.id IN (
		SELECT DISTINCT platform_id FROM game_platforms_platform WHERE game_platforms_platform.game_id = $1
	)`, game.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var platformName string
		err = rows.Scan(&platformName)
		if err != nil {
			return nil, err
		}
		platformsStrArr = append(platformsStrArr, platformName)
	}

	// Save Game
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO game
	(id, parent_game_id, title, alternate_titles, series, developer, publisher, play_mode, status, notes,
	 source, application_path, launch_command, release_date, version, original_description, language, library,
	 active_data_id, tags_str, platforms_str, action, reason, user_id, platform_name) 
	 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)`,
		game.ID, "", game.Title, game.AlternateTitles, game.Series, game.Developer, game.Publisher, game.PlayMode,
		game.Status, game.Notes, game.Source, game.ApplicationPath, game.LaunchCommand, game.ReleaseDate, game.Version,
		game.OriginalDesc, game.Language, game.Library, gameData.ID, strings.Join(tagsStrArr, "; "),
		strings.Join(platformsStrArr, "; "), "create", "Submission Import", uid, game.PrimaryPlatform)

	if err != nil {
		return nil, err
	}

	return &game, nil
}

func (d *postgresDAL) GetGameRevisionInfo(dbs PGDBSession, gameId string) ([]*types.RevisionInfo, error) {
	revisions := make([]*types.RevisionInfo, 0)

	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT date_modified, action, reason, user_id FROM changelog_game WHERE id = $1`, gameId)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		revision := &types.RevisionInfo{}
		err = rows.Scan(&revision.CreatedAt, &revision.Action, &revision.Reason, &revision.AuthorID)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}

	return revisions, nil
}

func (d *postgresDAL) GetTagRevisionInfo(dbs PGDBSession, tagId int64) ([]*types.RevisionInfo, error) {
	revisions := make([]*types.RevisionInfo, 0)

	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT date_modified, action, reason, user_id FROM changelog_tag WHERE id = $1`, tagId)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		revision := &types.RevisionInfo{}
		err = rows.Scan(&revision.CreatedAt, &revision.Action, &revision.Reason, &revision.AuthorID)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}

	return revisions, nil
}

func (d *postgresDAL) DeleteGame(dbs PGDBSession, gameId string, uid int64, reason string, imagesPath string, gamesPath string, deletedImagesPath string, deletedGamesPath string) error {
	// Get Game Data
	game, err := d.GetGame(dbs, gameId)
	if err != nil {
		return err
	}

	// Disable Database Entry
	_, err = dbs.Tx().Exec(dbs.Ctx(), `UPDATE game SET action = 'delete', deleted = TRUE, user_id = $1, reason = $2 WHERE id = $3`,
		uid, reason, gameId)
	if err != nil {
		return err
	}

	// Disable Game Files
	for _, data := range game.Data {
		base := fmt.Sprintf("%s-%d%s", game.ID, data.DateAdded.UnixMilli(), ".zip")
		existingFileName := filepath.Join(gamesPath, base)
		if _, err := os.Stat(existingFileName); err == nil {
			newFilename := filepath.Join(deletedGamesPath, base)
			err = os.MkdirAll(filepath.Dir(newFilename), 0755)
			if err != nil {
				return err
			}
			err = os.Rename(existingFileName, newFilename)
			if err != nil {
				return err
			}
		}
	}

	// Disable Image Files
	logoPath := fmt.Sprintf("%s/Logos/%s/%s/%s.png", imagesPath, gameId[:2], gameId[2:4], gameId)
	ssPath := fmt.Sprintf("%s/Screenshots/%s/%s/%s.png", imagesPath, gameId[:2], gameId[2:4], gameId)

	if _, err := os.Stat(logoPath); err == nil {
		newLogoPath := fmt.Sprintf("%s/Logos/%s/%s/%s.png", deletedImagesPath, gameId[:2], gameId[2:4], gameId)
		err = os.MkdirAll(filepath.Dir(newLogoPath), 0755)
		if err != nil {
			return err
		}
		err = os.Rename(logoPath, newLogoPath)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(ssPath); err == nil {
		newSSPath := fmt.Sprintf("%s/Screenshots/%s/%s/%s.png", deletedImagesPath, gameId[:2], gameId[2:4], gameId)
		err = os.MkdirAll(filepath.Dir(newSSPath), 0755)
		if err != nil {
			return err
		}
		err = os.Rename(ssPath, newSSPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *postgresDAL) RestoreGame(dbs PGDBSession, gameId string, uid int64, reason string, imagesPath string, gamesPath string, deletedImagesPath string, deletedGamesPath string) error {
	// Get Game Data
	game, err := d.GetGame(dbs, gameId)
	if err != nil {
		return err
	}

	_, err = dbs.Tx().Exec(dbs.Ctx(), `UPDATE game SET action = 'restore', deleted = FALSE, user_id = $1, reason = $2 WHERE id = $3`,
		uid, reason, gameId)
	if err != nil {
		return err
	}

	// Disable Game Files
	for _, data := range game.Data {
		base := fmt.Sprintf("%s-%d%s", game.ID, data.DateAdded.UnixMilli(), ".zip")
		existingFileName := filepath.Join(deletedGamesPath, base)
		if _, err := os.Stat(existingFileName); err == nil {
			newFilename := filepath.Join(gamesPath, base)
			err = os.MkdirAll(filepath.Dir(newFilename), 0755)
			if err != nil {
				return err
			}
			err = os.Rename(existingFileName, newFilename)
			if err != nil {
				return err
			}
		}
	}

	// Enable Image Files
	logoPath := fmt.Sprintf("%s/Logos/%s/%s/%s.png", deletedImagesPath, gameId[:2], gameId[2:4], gameId)
	ssPath := fmt.Sprintf("%s/Screenshots/%s/%s/%s.png", deletedImagesPath, gameId[:2], gameId[2:4], gameId)

	if _, err := os.Stat(logoPath); err == nil {
		newLogoPath := fmt.Sprintf("%s/Logos/%s/%s/%s.png", imagesPath, gameId[:2], gameId[2:4], gameId)
		err = os.MkdirAll(filepath.Dir(newLogoPath), 0755)
		if err != nil {
			return err
		}
		err = os.Rename(logoPath, newLogoPath)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(ssPath); err == nil {
		newSSPath := fmt.Sprintf("%s/Screenshots/%s/%s/%s.png", imagesPath, gameId[:2], gameId[2:4], gameId)
		err = os.MkdirAll(filepath.Dir(newSSPath), 0755)
		if err != nil {
			return err
		}
		err = os.Rename(ssPath, newSSPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *postgresDAL) UpdateTagsFromTagsList(dbs PGDBSession, tagsList []types.Tag) error {
	conf := config.GetConfig(nil)
	for _, tag := range tagsList {
		_, err := dbs.Tx().Exec(dbs.Ctx(), `UPDATE tag
			SET description = $1, action = $2, reason = $3, user_id = $4
			WHERE tag.id IN (SELECT DISTINCT tag_alias.tag_id FROM tag_alias WHERE tag_alias.name = $5)`,
			tag.Description, "update", "Scraped Description from Wiki", conf.SystemUid, tag.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *postgresDAL) GetMetadataStats(dbs PGDBSession) (*types.MetadataStatsPageDataBare, error) {
	var totalGames int64
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) FROM game WHERE deleted = false AND library = 'arcade'`).Scan(&totalGames)
	if err != nil {
		return nil, err
	}

	var totalAnims int64
	err = dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) FROM game WHERE deleted = false AND library = 'theatre'`).Scan(&totalAnims)
	if err != nil {
		return nil, err
	}

	var totalLegacy int64
	err = dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*)
		FROM game
		LEFT JOIN game_data ON game.id = game_data.game_id
		WHERE game_data.game_id IS NULL AND game.deleted = false`).Scan(&totalLegacy)
	if err != nil {
		return nil, err
	}

	var totalPlatforms int64
	err = dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) FROM platform WHERE deleted = false`).Scan(&totalPlatforms)
	if err != nil {
		return nil, err
	}

	var totalTags int64
	err = dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) FROM tag WHERE deleted = false`).Scan(&totalTags)
	if err != nil {
		return nil, err
	}

	data := types.MetadataStatsPageDataBare{
		TotalGames:      totalGames,
		TotalAnimations: totalAnims,
		TotalLegacy:     totalLegacy,
		TotalPlatforms:  totalPlatforms,
		TotalTags:       totalTags,
	}

	return &data, nil
}

func GetPlatformID(dbs PGDBSession, name string) (int64, error) {
	query := `SELECT platform_id FROM platform_alias WHERE platform_alias.name = $1`
	var id int64
	err := dbs.Tx().QueryRow(dbs.Ctx(), query, name).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Platform not found, return -1 to indicate
			return -1, nil
		}
		return 0, err
	}
	return id, nil
}

func GetTagID(dbs PGDBSession, name string) (int64, error) {
	query := `SELECT tag_id FROM tag_alias WHERE tag_alias.name = $1`
	var id int64
	err := dbs.Tx().QueryRow(dbs.Ctx(), query, name).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Tag not found, return -1 to indicate
			return -1, nil
		}
		return 0, err
	}
	return id, nil
}

func GetCategoryID(dbs PGDBSession, name string) (int64, error) {
	query := `SELECT id FROM tag_category WHERE tag_category.name = $1`
	var id int64
	err := dbs.Tx().QueryRow(dbs.Ctx(), query, name).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Tag category not found, return -1 to indicate
			return -1, nil
		}
		return 0, err
	}
	return id, nil
}

func EnsureString(src *string, dest *string) {
	if src == nil {
		*dest = ""
	} else {
		*dest = *src
	}
}

func containsCategory(arr []*types.TagCategory, target string) bool {
	for _, cat := range arr {
		if cat.Name == target {
			return true
		}
	}
	return false
}

func getCategoryID(arr []*types.TagCategory, target string) int64 {
	for _, cat := range arr {
		if cat.Name == target {
			return cat.ID
		}
	}
	return -1
}
