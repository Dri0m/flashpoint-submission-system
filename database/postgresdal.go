package database

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/types"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"log"
	"time"
)

type postgresDAL struct {
	db *pgx.Conn
}

func NewPostgresDAL(conn *pgx.Conn) *postgresDAL {
	return &postgresDAL{
		db: conn,
	}
}

// OpenPostgresDB opens DAL or panics
func OpenPostgresDB(l *logrus.Entry, conf *config.Config) *pgx.Conn {
	l.Infoln("connecting to the postgres database")

	user := conf.PostgresUser
	pass := conf.PostgresPassword
	ip := conf.PostgresHost
	port := conf.PostgresPort

	conn, err := pgx.Connect(context.Background(),
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
			WHERE tag.date_modified >= $1 
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
		rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT platform.id, coalesce(platform.description, 'none') as description, platform.date_modified, primary_alias, (SELECT string_agg(name, '; ') FROM platform_alias WHERE platform_id = platform.id) as alias_names, user_id 
			FROM platform
			WHERE platform.date_modified >= $1 
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

func (d *postgresDAL) SearchGames(dbs PGDBSession, modifiedAfter *string, broad bool, afterId *string) ([]*types.Game, []*types.AdditionalApp, []*types.GameData, [][]string, [][]string, error) {
	var rows pgx.Rows
	var err error
	limit := 2500
	tagRelations := make([][]string, 0)
	platformRelations := make([][]string, 0)
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT game.id, game.parent_game_id, game.title, game.alternate_titles, game.series,
       		game.developer, game.publisher, game.date_added, game.date_modified, game.play_mode, game.status, game.notes, game.source,
       		game.application_path, game.launch_command, game.release_date, game.version, game.original_description, game.language,
       		game.library, game.active_data_id, game.tags_str, game.platforms_str, game.user_id
			FROM game
			WHERE game.date_modified >= $1 AND game.id > $2
			ORDER BY game.date_modified, game.id
			LIMIT $3`, modifiedAfter, afterId, limit)
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
			&game.Library, &game.ActiveDataID, &game.TagsStr, &game.PlatformsStr, &game.UserID); err != nil {
			return nil, nil, nil, nil, nil, err
		}

		result = append(result, game)
	}

	// Store tag relations
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT gtt.game_id, gtt.tag_id 
	FROM game_tags_tag gtt 
	WHERE gtt.game_id IN (
	    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.id > $2 LIMIT $3
	)`, modifiedAfter, afterId, limit)
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
	    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.id > $2 LIMIT $3
	)`, modifiedAfter, afterId, limit)
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
			    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.id > $2 LIMIT $3
			)`, modifiedAfter, afterId, limit)

			for rows.Next() {
				addApp := &types.AdditionalApp{}
				if err := rows.Scan(&addApp.Name, &addApp.ApplicationPath, &addApp.LaunchCommand,
					&addApp.WaitForExit, &addApp.AutoRunBefore, &addApp.ParentGameID); err != nil {
					return nil, nil, nil, nil, nil, err
				}

				resultAddApps = append(resultAddApps, addApp)
			}

			rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT gd.id, gd.game_id, gd.title, gd.date_added, gd.sha256,
       		gd.crc32, gd.size, gd.parameters
			FROM game_data gd
			WHERE gd.game_id IN (
			    SELECT game.id FROM game WHERE game.date_modified >= $1 AND game.id > $2 LIMIT $3
			)`, modifiedAfter, afterId, limit)

			for rows.Next() {
				gameData := &types.GameData{}
				if err := rows.Scan(&gameData.ID, &gameData.GameID, &gameData.Title, &gameData.DateAdded, &gameData.SHA256,
					&gameData.CRC32, &gameData.Size, &gameData.Parameters); err != nil {
					return nil, nil, nil, nil, nil, err
				}

				resultGameData = append(resultGameData, gameData)
			}
		}
	}

	rows.Close()

	return result, resultAddApps, resultGameData, tagRelations, platformRelations, nil
}

func (d *postgresDAL) GetTag(dbs PGDBSession, tagId int64) (types.Tag, error) {
	var tag types.Tag
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description, tag_category.name, tag.date_modified, primary_alias, (SELECT string_agg(name, '; ') FROM tag_alias WHERE tag_id = tag.id) as alias_names, user_id 
		FROM tag 
			LEFT JOIN tag_category ON tag_category.id = tag.category_id 
		WHERE tag.id = $1`, tagId).
		Scan(&tag.ID, &tag.Description, &tag.Category, &tag.DateModified, &tag.Name, &tag.Aliases, &tag.UserID)
	if err != nil {
		return tag, err
	}

	return tag, nil
}

func (d *postgresDAL) GetGamesUsingTagTotal(dbs PGDBSession, tagId int64) (int64, error) {
	var count int64
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT COUNT(*) FROM game_tags_tag WHERE tag_id = $1`, tagId).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (d *postgresDAL) GetGame(dbs PGDBSession, gameId string) (types.Game, error) {
	// Get game
	var game types.Game
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT * FROM game WHERE id = $1`, gameId).
		Scan(&game.ID, &game.ParentGameID, &game.Title, &game.AlternateTitles, &game.Series, &game.Developer,
			&game.Publisher, &game.DateAdded, &game.DateModified, &game.PlayMode, &game.Status, &game.Notes,
			&game.Source, &game.ApplicationPath, &game.LaunchCommand, &game.ReleaseDate, &game.Version,
			&game.OriginalDesc, &game.Language, &game.Library, &game.ActiveDataID, &game.TagsStr, &game.PlatformsStr, &game.UserID)
	if err != nil {
		return game, err
	}

	// Get add apps
	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT * FROM additional_app WHERE parent_game_id = $1`, gameId)
	if err != nil {
		return game, err
	}
	defer rows.Close()
	for rows.Next() {
		var addApp types.AdditionalApp
		err = rows.Scan(&addApp.ID, &addApp.ApplicationPath, &addApp.AutoRunBefore, &addApp.LaunchCommand, &addApp.Name, &addApp.WaitForExit, &addApp.ParentGameID)
		if err != nil {
			return game, err
		}
		game.AddApps = append(game.AddApps, &addApp)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Get game data
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT * FROM game_data WHERE game_id = $1`, gameId)
	if err != nil {
		return game, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.GameData
		err = rows.Scan(&data.ID, &data.GameID, &data.Title, &data.DateAdded, &data.SHA256, &data.CRC32, &data.Size, &data.Parameters)
		if err != nil {
			return game, err
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
		return game, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.Tag
		err = rows.Scan(&data.ID, &data.Description, &data.Category, &data.DateModified, &data.Name, &data.UserID)
		if err != nil {
			return game, err
		}
		game.Tags = append(game.Tags, &data)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// Get platforms
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT * FROM platform WHERE id IN (
    	SELECT platform_id FROM game_platforms_platform WHERE game_id = $1) ORDER BY primary_alias`, gameId)
	if err != nil {
		return game, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.Platform
		err = rows.Scan(&data.ID, &data.DateModified, &data.Name, &data.Description, &data.UserID)
		if err != nil {
			return game, err
		}
		game.Platforms = append(game.Platforms, &data)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	return game, nil
}

func (d *postgresDAL) SaveGame(dbs PGDBSession, game *types.Game, uid int64) error {
	// Save tag relations
	_, err := dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_tags_tag WHERE game_id = $1`, game.ID)
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_tags_tag"},
		[]string{"game_id", "tag_id"},
		pgx.CopyFromSlice(len(game.Tags), func(i int) ([]interface{}, error) {
			return []interface{}{
				game.ID,
				game.Tags[i].ID,
			}, nil
		}),
	)
	if err != nil {
		return err
	}

	// Save platform relations
	_, err = dbs.Tx().Exec(dbs.Ctx(), `DELETE FROM game_platforms_platform WHERE game_id = $1`, game.ID)
	_, err = dbs.Tx().CopyFrom(
		dbs.Ctx(),
		pgx.Identifier{"game_platforms_platform"},
		[]string{"game_id", "platform_id"},
		pgx.CopyFromSlice(len(game.Platforms), func(i int) ([]interface{}, error) {
			return []interface{}{
				game.ID,
				game.Platforms[i].ID,
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
	query := "UPDATE game SET parent_game_id=$1, title=$2, alternate_titles=$3, series=$4, developer=$5, publisher=$6, platforms_str=$7, play_mode=$8, status=$9, notes=$10, tags_str=$11, source=$12, application_path=$13, launch_command=$14, release_date=$15, version=$16, original_description=$17, language=$18, library=$19, active_data_id=$20, user_id=$21 WHERE id=$22"
	_, err = dbs.Tx().Exec(dbs.Ctx(), query, game.ParentGameID, game.Title, game.AlternateTitles, game.Series, game.Developer, game.Publisher, game.PlatformsStr, game.PlayMode, game.Status, game.Notes, game.TagsStr, game.Source, game.ApplicationPath, game.LaunchCommand, game.ReleaseDate, game.Version, game.OriginalDesc, game.Language, game.Library, game.ActiveDataID, uid, game.ID)
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
		[]string{"id", "category_id", "description", "primary_alias", "user_id"},
		pgx.CopyFromSlice(len(dump.Tags.Tags), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Tags.Tags[i].ID,
				dump.Tags.Tags[i].CategoryID,
				dump.Tags.Tags[i].Description,
				dump.Tags.Tags[i].PrimaryAlias,
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
		[]string{"id", "description", "primary_alias", "user_id"},
		pgx.CopyFromSlice(len(dump.Platforms.Platforms), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Platforms.Platforms[i].ID,
				dump.Platforms.Platforms[i].Description,
				dump.Platforms.Platforms[i].PrimaryAlias,
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
		[]string{"game_id", "title", "date_added", "sha256", "crc32", "size", "parameters"},
		pgx.CopyFromSlice(len(dump.Games.GameData), func(i int) ([]interface{}, error) {
			return []interface{}{
				dump.Games.GameData[i].GameID,
				dump.Games.GameData[i].Title,
				dump.Games.GameData[i].DateAdded,
				dump.Games.GameData[i].SHA256,
				dump.Games.GameData[i].CRC32,
				dump.Games.GameData[i].Size,
				dump.Games.GameData[i].Parameters,
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
			"tags_str", "platforms_str", "user_id"},
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
				conf.SystemUid,
			}, nil
		}),
	)

	utils.LogCtx(dbs.Ctx()).Debug("manual logging")

	// Manual game logging
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_additional_app (application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id, date_modified) 
		SELECT application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id, $1
		FROM additional_app`, t)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game_data (game_id, title, date_added, sha256, crc32, size, parameters, date_modified)
		SELECT game_id, title, date_added, sha256, crc32, size, parameters, $1
		FROM game_data`, t)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game_tags_tag ("game_id", "tag_id", "date_modified")
   		SELECT game_id, tag_id, $1 FROM game_tags_tag`, t)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game_platforms_platform ("game_id", "platform_id", "date_modified")
   		SELECT game_id, platform_id, $1 FROM game_platforms_platform`, t)
	_, err = dbs.Tx().Exec(dbs.Ctx(), `INSERT INTO changelog_game (id, parent_game_id, title, alternate_titles, series, developer, publisher, date_added, date_modified, play_mode, status, notes, source, application_path, launch_command, release_date, version, original_description, language, library, active_data_id, tags_str, platforms_str, user_id)
		SELECT id, parent_game_id, title, alternate_titles, series, developer, publisher, date_added, date_modified, play_mode, status, notes, source, application_path, launch_command, release_date, version, original_description, language, library, active_data_id, tags_str, platforms_str, user_id
		FROM game`)

	utils.LogCtx(dbs.Ctx()).Debug("done database import")

	return nil
}
