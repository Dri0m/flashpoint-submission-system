CREATE EXTENSION IF NOT EXISTS citext;
BEGIN TRANSACTION;

-- Tables

CREATE TABLE IF NOT EXISTS "tag_category" (
	"id"	serial PRIMARY KEY,
	"name"	citext NOT NULL,
	"color"	varchar NOT NULL,
	"description"	varchar
);
CREATE TABLE IF NOT EXISTS "tag_alias" (
	"name"	citext PRIMARY KEY,
	"tag_id"	integer NOT NULL
);
CREATE TABLE IF NOT EXISTS "tag" (
	"id"	serial PRIMARY KEY,
	"date_modified"	timestamp NOT NULL DEFAULT now(),
	"primary_alias"	citext NOT NULL,
	"category_id"	integer NOT NULL,
	"description"	varchar,
	"action" varchar NOT NULL,
	"reason" varchar NOT NULL,
	"deleted" bool DEFAULT FALSE,
	"user_id" bigint NOT NULL,
	CONSTRAINT "FK_tag_category_id" FOREIGN KEY("category_id") REFERENCES "tag_category"("id") ON DELETE NO ACTION ON UPDATE NO ACTION,
	CONSTRAINT "FK_tag_primary_alias" FOREIGN KEY("primary_alias") REFERENCES "tag_alias"("name") ON DELETE CASCADE ON UPDATE NO ACTION
);

CREATE TABLE IF NOT EXISTS "changelog_tag" (
     "row" serial PRIMARY KEY,
	 "id"	serial,
	 "date_modified"	timestamp NOT NULL,
	 "primary_alias"	citext NOT NULL,
	 "category_id"	integer NOT NULL,
	 "description"	varchar,
	 "action" varchar NOT NULL,
	 "reason" varchar NOT NULL,
	 "user_id" bigint NOT NULL
);
CREATE TABLE IF NOT EXISTS "changelog_tag_alias" (
    "row" serial PRIMARY KEY,
	"name"	citext NOT NULL,
	"tag_id"	integer NOT NULL,
	"date_modified"	timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "game_tags_tag" (
	"game_id"	varchar(36) NOT NULL,
	"tag_id"	integer NOT NULL,
	PRIMARY KEY("game_id","tag_id")
);
CREATE TABLE IF NOT EXISTS "changelog_game_tags_tag" (
    row serial PRIMARY KEY,
	"game_id"	varchar(36) NOT NULL,
	"tag_id"	integer NOT NULL,
	"date_modified" timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "game" (
	"id"	varchar(36) PRIMARY KEY,
	"parent_game_id"	varchar(36),
	"title"	varchar NOT NULL,
	"alternate_titles"	varchar NOT NULL,
	"series"	varchar NOT NULL,
	"developer"	varchar NOT NULL,
	"publisher"	varchar NOT NULL,
	"date_added"	timestamp NOT NULL DEFAULT now(),
	"date_modified"	timestamp NOT NULL DEFAULT now(),
	"play_mode"	varchar NOT NULL,
	"status"	varchar NOT NULL,
	"notes"	varchar NOT NULL,
	"source"	varchar NOT NULL,
	"application_path"	varchar NOT NULL,
	"launch_command"	varchar NOT NULL,
	"release_date"	varchar NOT NULL,
	"version"	varchar NOT NULL,
	"original_description"	varchar NOT NULL,
	"language"	varchar NOT NULL,
	"library"	varchar NOT NULL,
	"active_data_id"	integer,
	"tags_str"	citext NOT NULL DEFAULT (''),
	"platforms_str"	varchar NOT NULL DEFAULT (''),
	"action" varchar NOT NULL,
	"reason" varchar NOT NULL,
	"deleted" bool DEFAULT FALSE,
	"user_id" bigint NOT NULL
);
CREATE TABLE IF NOT EXISTS "changelog_game" (
	"row" serial PRIMARY KEY,
	"id"	varchar(36),
	"parent_game_id"	varchar(36),
	"title"	varchar NOT NULL,
	"alternate_titles"	varchar NOT NULL,
	"series"	varchar NOT NULL,
	"developer"	varchar NOT NULL,
	"publisher"	varchar NOT NULL,
	"date_added"	timestamp NOT NULL,
	"date_modified"	timestamp NOT NULL DEFAULT now(),
	"play_mode"	varchar NOT NULL,
	"status"	varchar NOT NULL,
	"notes"	varchar NOT NULL,
	"source"	varchar NOT NULL,
	"application_path"	varchar NOT NULL,
	"launch_command"	varchar NOT NULL,
	"release_date"	varchar NOT NULL,
	"version"	varchar NOT NULL,
	"original_description"	varchar NOT NULL,
	"language"	varchar NOT NULL,
	"library"	varchar NOT NULL,
	"active_data_id"	integer,
	"tags_str"	citext NOT NULL DEFAULT (''),
	"platforms_str"	varchar NOT NULL DEFAULT (''),
	"action" varchar NOT NULL,
	"reason" varchar NOT NULL,
	"user_id" bigint NOT NULL
);

CREATE TABLE IF NOT EXISTS "additional_app" (
	"id"	serial PRIMARY KEY,
	"application_path"	varchar NOT NULL,
	"auto_run_before"	boolean NOT NULL,
	"launch_command"	varchar NOT NULL,
	"name"	citext NOT NULL,
	"wait_for_exit"	boolean NOT NULL,
	"parent_game_id"	varchar(36) NOT NULL
);
CREATE TABLE IF NOT EXISTS "changelog_additional_app" (
	"row"	serial PRIMARY KEY,
	"application_path"	varchar NOT NULL,
	"auto_run_before"	boolean NOT NULL,
	"launch_command"	varchar NOT NULL,
	"name"	citext NOT NULL,
	"wait_for_exit"	boolean NOT NULL,
	"parent_game_id"	varchar(36) NOT NULL,
	"date_modified" timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "game_data" (
	"id"	serial PRIMARY KEY,
	"game_id"	varchar(36),
	"title"	varchar NOT NULL,
	"date_added"	timestamp NOT NULL DEFAULT now(),
	"sha256"	varchar NOT NULL,
	"crc32"	integer NOT NULL,
	"size"	bigint NOT NULL,
	"application_path" varchar NOT NULL,
	"launch_command" varchar NOT NULL,
	"parameters"	varchar
);
CREATE TABLE IF NOT EXISTS "changelog_game_data" (
	 "row"	serial PRIMARY KEY,
	 "game_id"	varchar(36),
	 "title"	varchar NOT NULL,
	 "date_added"	timestamp NOT NULL,
	 "sha256"	varchar NOT NULL,
	 "crc32"	integer NOT NULL,
	 "size"	bigint NOT NULL,
	 "parameters"	varchar,
	 "application_path" varchar NOT NULL,
	 "launch_command" varchar NOT NULL,
	 "date_modified" timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "platform_alias" (
	"name"	citext PRIMARY KEY,
	"platform_id"	integer NOT NULL
);
CREATE TABLE IF NOT EXISTS "changelog_platform_alias" (
    "row" serial PRIMARY KEY,
	"name" citext NOT NULL,
    "platform_id" integer NOT NULL,
    "date_modified" timestamp NOT NULL
);

CREATE TABLE IF NOT EXISTS "platform" (
	"id"	serial PRIMARY KEY,
	"date_modified"	timestamp NOT NULL DEFAULT now(),
	"primary_alias"	citext NOT NULL,
	"description"	varchar,
	"action" varchar NOT NULL,
	"reason" varchar NOT NULL,
	"deleted" bool DEFAULT FALSE,
	"user_id" bigint NOT NULL,
	CONSTRAINT "FK_platform_primary_alias" FOREIGN KEY("primary_alias") REFERENCES "platform_alias"("name") ON DELETE CASCADE ON UPDATE NO ACTION
);
CREATE TABLE IF NOT EXISTS "changelog_platform" (
    row serial PRIMARY KEY,
	"id"	serial,
	"date_modified"	timestamp NOT NULL DEFAULT now(),
	"primary_alias"	citext NOT NULL,
	"description"	varchar,
	"action" varchar NOT NULL,
	"reason" varchar NOT NULL,
	"user_id" bigint NOT NULL
);

CREATE TABLE IF NOT EXISTS "game_platforms_platform" (
	"game_id"	varchar(36) NOT NULL,
	"platform_id"	integer NOT NULL,
	PRIMARY KEY("game_id","platform_id")
);
CREATE TABLE IF NOT EXISTS "changelog_game_platforms_platform" (
    "row" serial PRIMARY KEY,
	"game_id"	varchar(36) NOT NULL,
	"platform_id"	integer NOT NULL,
	"date_modified" timestamp
);

CREATE INDEX IF NOT EXISTS "IDX_platform_alive" ON "platform" (
	 "deleted"
);

CREATE INDEX IF NOT EXISTS "IDX_tag_alive" ON "tag" (
	"deleted"
);

CREATE INDEX IF NOT EXISTS "IDX_34d6ff6807129b3b193aea2678" ON "tag_alias" (
	"name"
);
CREATE INDEX IF NOT EXISTS "IDX_6366e7093c3571f85f1b5ffd4f" ON "game_tags_tag" (
	"game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_d12253f0cbce01f030a9ced11d" ON "game_tags_tag" (
	"tag_id"
);
CREATE INDEX IF NOT EXISTS "IDX_6366e7093c3571f85f1b5ffd4f2" ON "game_platforms_platform" (
	"game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_d12253f0cbce01f030a9ced11d2" ON "game_platforms_platform" (
	"platform_id"
);
CREATE INDEX IF NOT EXISTS "IDX_game_alive" ON "game" (
	"deleted"
);
CREATE INDEX IF NOT EXISTS "IDX_gameTitle" ON "game" (
	"title"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_gameDateModified_id" ON "game" (
	"date_modified",
	"id"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_total" ON "game" (
	"library"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_lookup_series" ON "game" (
	"library",
	"series"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_lookup_publisher" ON "game" (
	"library",
	"publisher"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_lookup_developer" ON "game" (
	"library",
	"developer"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_lookup_dateModified" ON "game" (
	"library",
	"date_modified"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_lookup_dateAdded" ON "game" (
	"library",
	"date_added"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_lookup_title" ON "game" (
	"library",
	"title"
) WHERE deleted = FALSE;
CREATE INDEX IF NOT EXISTS "IDX_add_app_parent_game_id" ON "additional_app"(
	"parent_game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_game_data_ game_id" ON "game_data"(
	"game_id"
);

-- Changelog Indexing

CREATE INDEX IF NOT EXISTS "IDX_changelog_tag_id" ON "changelog_tag" (
	"id"
);
CREATE INDEX IF NOT EXISTS "IDX_changelog_platform_id" ON "changelog_platform" (
	"id"
);
CREATE INDEX IF NOT EXISTS "IDX_changelog_tag_alias_tag_id" ON "changelog_tag_alias" (
	"tag_id"
);
CREATE INDEX IF NOT EXISTS "IDX_changelog_platform_alias_platform_id" ON "changelog_platform_alias" (
	"platform_id"
);
CREATE INDEX IF NOT EXISTS "IDX_changelog_add_app_game_id" ON "changelog_additional_app" (
	"parent_game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_changelog_game_data_game_id" ON "changelog_game_data" (
	"game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_6366e7093c3571f85f1b5ffd4fc" ON "changelog_game_tags_tag" (
	"game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_d12253f0cbce01f030a9ced11dc" ON "changelog_game_tags_tag" (
	"tag_id"
);
CREATE INDEX IF NOT EXISTS "IDX_6366e7093c3571f85f1b5ffd4f2c" ON "changelog_game_platforms_platform" (
   "game_id"
);
CREATE INDEX IF NOT EXISTS "IDX_d12253f0cbce01f030a9ced11d2c" ON "changelog_game_platforms_platform" (
   "platform_id"
);

-- Procedures and Triggers

CREATE OR REPLACE FUNCTION set_date_modified()
	RETURNS TRIGGER AS $$
BEGIN
	NEW.date_modified = NOW();
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Log Trigger Functions

CREATE OR REPLACE FUNCTION log_tag()
	RETURNS TRIGGER AS $$
BEGIN
	INSERT INTO changelog_tag (id, date_modified, primary_alias, category_id, description, action, reason, user_id)
	VALUES (NEW.id, NEW.date_modified, NEW.primary_alias, NEW.category_id, NEW.description, NEW.action, NEW.reason, NEW.user_id);

	-- loop through the aliases and log their updates
	INSERT INTO changelog_tag_alias (tag_id, name, date_modified)
	SELECT tag_alias.tag_id, tag_alias.name, NEW.date_modified
	FROM tag_alias
	WHERE tag_id = NEW.id;

	CALL log_tag_relations('tag_id', NEW.id, NEW.date_modified);

	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION log_platform()
	RETURNS TRIGGER AS $$
BEGIN
	INSERT INTO changelog_platform (id, date_modified, primary_alias, description, action, reason, user_id)
	VALUES (NEW.id, NEW.date_modified, NEW.primary_alias, NEW.description, NEW.action, NEW.reason, NEW.user_id);

	-- loop through the aliases and log their updates
	INSERT INTO changelog_platform_alias (platform_id, name, date_modified)
	SELECT platform_alias.platform_id, platform_alias.name, NEW.date_modified
	FROM platform_alias
	WHERE platform_id = NEW.id;

	CALL log_platform_relations('platform_id', NEW.id, NEW.date_modified);

	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION log_game()
	RETURNS TRIGGER AS $$
BEGIN
	CALL log_game_data_for_game(NEW.id, NEW.date_modified);
	CALL log_add_app_for_game(NEW.id, NEW.date_modified);
	CALL log_platform_relations('game_id', NEW.id, NEW.date_modified);
	CALL log_tag_relations('game_id', NEW.id, NEW.date_modified);

	INSERT INTO changelog_game (id, parent_game_id, title, alternate_titles, series, developer,
	                            publisher, date_added, date_modified, play_mode, status, notes, source,
	                            application_path, launch_command, release_date, version, original_description,
	                            language, library, active_data_id, tags_str, platforms_str, action, reason,
	                            user_id)
	VALUES (NEW.id, NEW.parent_game_id, NEW.title, NEW.alternate_titles, NEW.series, NEW.developer,
	        NEW.publisher, NEW.date_added, NEW.date_modified, NEW.play_mode, NEW.status, NEW.notes, NEW.source,
	        NEW.application_path, NEW.launch_command, NEW.release_date, NEW.version, NEW.original_description,
	        NEW.language, NEW.library, NEW.active_data_id, NEW.tags_str, NEW.platforms_str, NEW.action, NEW.reason,
	        NEW.user_id);

	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Log Procedures

CREATE OR REPLACE PROCEDURE log_tag_relations(id_column VARCHAR(40), value anyelement, date_modified timestamp)
AS $$
BEGIN
	EXECUTE format('INSERT INTO changelog_game_tags_tag ("game_id", "tag_id", "date_modified") ' ||
				   'SELECT game_id, tag_id, $2 FROM game_tags_tag ' ||
				   'WHERE %I = $1 LIMIT 1', id_column) USING value, date_modified;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE PROCEDURE log_platform_relations(id_column VARCHAR(40), value anyelement, date_modified timestamp)
AS $$
BEGIN
	EXECUTE format('INSERT INTO changelog_game_platforms_platform ("game_id", "platform_id", "date_modified") ' ||
				   'SELECT game_id, platform_id, $2 FROM game_platforms_platform ' ||
				   'WHERE %I = $1 LIMIT 1', id_column) USING value, date_modified;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE PROCEDURE log_game_data_for_game(value varchar, date_modified timestamp)
AS $$
BEGIN
	INSERT INTO changelog_game_data (game_id, title, date_added, sha256, crc32, size, parameters, date_modified,
	                                application_path, launch_command)
	SELECT game_id, title, date_added, sha256, crc32, size, parameters, date_modified, application_path, launch_command
	FROM game_data
	WHERE game_data.game_id = value;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE PROCEDURE log_add_app_for_game(value varchar, date_modified timestamp)
AS $$
BEGIN
	INSERT INTO changelog_additional_app (application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id, date_modified)
	SELECT application_path, auto_run_before, launch_command, name, wait_for_exit, parent_game_id, date_modified
	FROM additional_app
	WHERE additional_app.parent_game_id = value;
END;
$$ LANGUAGE plpgsql;

-- Date Modified Triggers

CREATE TRIGGER game_set_date_modified
	BEFORE UPDATE ON game
	FOR EACH ROW
EXECUTE PROCEDURE set_date_modified();

CREATE TRIGGER tag_set_date_modified
	BEFORE UPDATE ON tag
	FOR EACH ROW
EXECUTE PROCEDURE set_date_modified();

CREATE TRIGGER platform_set_date_modified
	BEFORE UPDATE ON platform
	FOR EACH ROW
EXECUTE PROCEDURE set_date_modified();

-- Update Triggers

CREATE TRIGGER tag_update_trigger
	AFTER UPDATE OR INSERT ON tag
	FOR EACH ROW
EXECUTE PROCEDURE log_tag();

CREATE TRIGGER platform_update_trigger
	AFTER UPDATE OR INSERT ON platform
	FOR EACH ROW
EXECUTE PROCEDURE log_platform();

CREATE TRIGGER game_update_trigger
	AFTER UPDATE OR INSERT ON game
	FOR EACH ROW
EXECUTE PROCEDURE log_game();

COMMIT;
