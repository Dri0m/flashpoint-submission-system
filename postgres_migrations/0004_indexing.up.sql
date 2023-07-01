ALTER TABLE game_data ADD COLUMN indexed boolean DEFAULT FALSE;
ALTER TABLE game_data ADD COLUMN index_error boolean DEFAULT FALSE;
CREATE TABLE "game_data_index" (
    "id" serial PRIMARY KEY,
    "crc32" bytea,
    "md5" bytea,
    "sha256" bytea,
    "sha1" bytea,
    "size" bigint,
    "path" citext,
    "game_id" uuid,
    "zip_date" timestamp
)