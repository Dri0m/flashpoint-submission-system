CREATE TABLE IF NOT EXISTS masterdb_game
(
    id                   BIGINT PRIMARY KEY AUTO_INCREMENT,
    uuid                 CHAR(36) NOT NULL,
    title                TEXT,
    alternate_titles     TEXT,
    series               TEXT,
    developer            TEXT,
    publisher            TEXT,
    platform             TEXT,
    extreme              TEXT,
    play_mode            TEXT,
    status               TEXT,
    game_notes           TEXT,
    source               TEXT,
    launch_command       TEXT,
    release_date         TEXT,
    version              TEXT,
    original_description TEXT,
    languages            TEXT,
    library              TEXT,
    tags                 TEXT,
    tag_categories       TEXT
);