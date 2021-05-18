CREATE TABLE IF NOT EXISTS session
(
    secret     TEXT    NOT NULL,
    uid        TEXT    NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS discord_user
(
    id       TEXT NOT NULL,
    username TEXT NOT NULL,
    avatar   TEXT NOT NULL,
    discriminator TEXT NOT NULL,
    public_flags INTEGER NOT NULL,
    flags INTEGER NOT NULL,
    locale TEXT NOT NULL,
    mfa_enabled INTEGER NOT NULL
);