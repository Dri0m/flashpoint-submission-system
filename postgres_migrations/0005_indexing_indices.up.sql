CREATE INDEX game_data_index_sha256 ON game_data_index USING hash(sha256);
CREATE INDEX game_data_index_sha1 ON game_data_index USING hash(sha1);
CREATE INDEX game_data_index_crc32 ON game_data_index USING hash(crc32);
CREATE INDEX game_data_index_md5 ON game_data_index USING hash(md5);
CREATE INDEX game_data_index_game_id ON game_data_index USING hash(game_id);
CREATE INDEX game_data_index_path ON game_data_index USING btree(path);