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

	result := make([]*types.TagCategory, 100)

	for rows.Next() {
		category := &types.TagCategory{}
		if err := rows.Scan(&category.ID, &category.Name, &category.Color, &category.Description); err != nil {
			return nil, err
		}

		result[category.ID] = category
	}

	rows.Close()

	return result, nil
}

func (d *postgresDAL) SearchTags(dbs PGDBSession) ([]*types.Tag, error) {
	rows, err := dbs.Tx().Query(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description, tag_category.name, tag.date_modified, primary_alias FROM tag LEFT JOIN tag_category ON tag_category.id = tag.category_id ORDER BY tag_category.name, primary_alias`)
	if err != nil {
		return nil, err
	}

	result := make([]*types.Tag, 0)

	for rows.Next() {
		tag := &types.Tag{}
		if err := rows.Scan(&tag.ID, &tag.Description, &tag.Category, &tag.DateModified, &tag.Name); err != nil {
			return nil, err
		}

		result = append(result, tag)
	}

	rows.Close()

	return result, nil
}

func (d *postgresDAL) GetTag(dbs PGDBSession, tagId int64) (types.Tag, error) {
	var tag types.Tag
	err := dbs.Tx().QueryRow(dbs.Ctx(), `SELECT tag.id, coalesce(tag.description, 'none') as description, tag_category.name, tag.date_modified, primary_alias FROM tag LEFT JOIN tag_category ON tag_category.id = tag.category_id WHERE tag.id = $1`, tagId).
		Scan(&tag.ID, &tag.Description, &tag.Category, &tag.DateModified, &tag.Name)
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
	rows, err = dbs.Tx().Query(dbs.Ctx(), `SELECT * FROM tag WHERE id IN (
    	SELECT tag_id FROM game_tags_tag WHERE game_id = $1)`, gameId)
	if err != nil {
		return game, err
	}
	defer rows.Close()
	for rows.Next() {
		var data types.Tag
		err = rows.Scan(&data.ID, &data.DateModified, &data.Name, &data.Category, &data.Description, &data.UserID)
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
    	SELECT platform_id FROM game_platforms_platform WHERE game_id = $1)`, gameId)
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
