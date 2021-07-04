CREATE TABLE IF NOT EXISTS masterdb_game
(
    id                   BIGINT PRIMARY KEY AUTO_INCREMENT,
    uuid                 CHAR(36) UNIQUE NOT NULL,
    title                TEXT,
    alternate_titles     TEXT,
    series               TEXT,
    developer            TEXT,
    publisher            TEXT,
    platform             VARCHAR(63),
    extreme              VARCHAR(7),
    play_mode            TEXT,
    status               TEXT,
    game_notes           TEXT,
    source               TEXT,
    launch_command       TEXT,
    release_date         TEXT,
    version              TEXT,
    original_description TEXT,
    languages            TEXT,
    library              VARCHAR(31),
    tags                 TEXT,
    date_added           BIGINT,
    date_modified        BIGINT
);
CREATE INDEX idx_masterdb_game_extreme ON masterdb_game (extreme);
CREATE INDEX idx_masterdb_game_date_added ON masterdb_game (date_added);
CREATE INDEX idx_masterdb_game_date_modified ON masterdb_game (date_modified);