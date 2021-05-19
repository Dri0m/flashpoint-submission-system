CREATE TABLE IF NOT EXISTS session
(
    secret     TEXT    NOT NULL,
    uid        INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS discord_user
(
    uid           INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL,
    avatar        TEXT    NOT NULL,
    discriminator TEXT    NOT NULL,
    public_flags  INTEGER NOT NULL,
    flags         INTEGER NOT NULL,
    locale        TEXT    NOT NULL,
    mfa_enabled   INTEGER NOT NULL
);
