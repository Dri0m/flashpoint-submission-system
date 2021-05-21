CREATE TABLE IF NOT EXISTS session
(
    id         INTEGER PRIMARY KEY,
    secret     TEXT    NOT NULL,
    uid        INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS discord_user
(
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL,
    avatar        TEXT    NOT NULL,
    discriminator TEXT    NOT NULL,
    public_flags  INTEGER NOT NULL,
    flags         INTEGER NOT NULL,
    locale        TEXT    NOT NULL,
    mfa_enabled   INTEGER NOT NULL
);

INSERT OR IGNORE INTO discord_user (id, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled)
VALUES (810112564787675166, 'RedMinima', '156dd40e0c72ed8e84034b53aad32af4', '1337', 0, 0, 'en_US', 0);

CREATE TABLE IF NOT EXISTS authorization
(
    id         INTEGER PRIMARY KEY,
    fk_uid     INTEGER NOT NULL,
    authorized INTEGER NOT NULL,
    FOREIGN KEY (fk_uid) REFERENCES discord_user (id)
);

CREATE TABLE IF NOT EXISTS submission
(
    id                INTEGER PRIMARY KEY,
    fk_uploader_id    INTEGER     NOT NULL,
    original_filename TEXT        NOT NULL,
    current_filename  TEXT UNIQUE NOT NULL,
    size              INTEGER     NOT NULL,
    uploaded_at       INTEGER     NOT NULL,
    FOREIGN KEY (fk_uploader_id) REFERENCES discord_user (id)
);

CREATE TABLE IF NOT EXISTS curation_meta
(
    id                   INTEGER PRIMARY KEY,
    fk_submission_id     INTEGER NOT NULL,
    application_path     TEXT,
    developer            TEXT,
    extreme              TEXT,
    game_notes           TEXT,
    languages            TEXT,
    launch_command       TEXT,
    original_description TEXT,
    play_mode            TEXT,
    platform             TEXT,
    publisher            TEXT,
    release_date         TEXT,
    series               TEXT,
    source               TEXT,
    status               TEXT,
    tags                 TEXT,
    tag_categories       TEXT,
    title                TEXT,
    alternate_titles     TEXT,
    library              TEXT,
    version              TEXT,
    curation_notes       TEXT,
    mount_parameters     TEXT,
    FOREIGN KEY (fk_submission_id) REFERENCES submission (id)
);

CREATE TABLE IF NOT EXISTS comment
(
    id               INTEGER PRIMARY KEY,
    fk_author_id     INTEGER NOT NULL,
    fk_submission_id INTEGER NOT NULL,
    message          TEXT,
    is_approving     INTEGER,
    created_at       INTEGER NOT NULL,
    FOREIGN KEY (fk_author_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_submission_id) REFERENCES submission (id)
);