CREATE TABLE IF NOT EXISTS session
(
    id         BIGINT PRIMARY KEY AUTO_INCREMENT,
    secret     CHAR(36) NOT NULL,
    uid        BIGINT   NOT NULL,
    expires_at BIGINT   NOT NULL
);

CREATE TABLE IF NOT EXISTS discord_user
(
    id            BIGINT PRIMARY KEY,
    username      VARCHAR(127) NOT NULL,
    avatar        VARCHAR(127) NOT NULL,
    discriminator VARCHAR(127) NOT NULL,
    public_flags  BIGINT       NOT NULL,
    flags         BIGINT       NOT NULL,
    locale        VARCHAR(127) NOT NULL,
    mfa_enabled   BIGINT       NOT NULL
);
CREATE INDEX idx_discord_user_username ON discord_user (username);

INSERT INTO discord_user (id, username, avatar, discriminator, public_flags, flags, locale, mfa_enabled)
VALUES (810112564787675166, 'RedMinima', '156dd40e0c72ed8e84034b53aad32af4', '1337', 0, 0, 'en_US', 0);

CREATE TABLE IF NOT EXISTS discord_role
(
    id    BIGINT PRIMARY KEY,
    name  VARCHAR(63) NOT NULL,
    color VARCHAR(10) NOT NULL
);

CREATE TABLE IF NOT EXISTS discord_user_role
(
    id     BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_uid BIGINT NOT NULL,
    fk_rid BIGINT NOT NULL,
    CONSTRAINT discord_user_role_fk_uid FOREIGN KEY (fk_uid) REFERENCES discord_user (id) ON DELETE CASCADE,
    CONSTRAINT discord_user_role_fk_rid FOREIGN KEY (fk_rid) REFERENCES discord_role (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS submission_level
(
    id   BIGINT PRIMARY KEY,
    name VARCHAR(63) UNIQUE
);

INSERT IGNORE INTO submission_level (id, name)
VALUES (1, 'audition'),
       (2, 'trial'),
       (3, 'staff');

CREATE TABLE IF NOT EXISTS submission
(
    id                     BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_submission_level_id BIGINT NOT NULL,
    deleted_at             BIGINT       DEFAULT NULL,
    deleted_reason         VARCHAR(255) DEFAULT NULL,
    FOREIGN KEY (fk_submission_level_id) REFERENCES submission_level (id)
);
CREATE INDEX idx_submission_deleted_at ON submission (deleted_at);

CREATE TABLE IF NOT EXISTS submission_file
(
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id        BIGINT              NOT NULL,
    fk_submission_id  BIGINT              NOT NULL,
    original_filename VARCHAR(255)        NOT NULL,
    current_filename  VARCHAR(255) UNIQUE NOT NULL,
    size              BIGINT              NOT NULL,
    created_at        BIGINT              NOT NULL,
    md5sum            CHAR(32) UNIQUE     NOT NULL,
    sha256sum         CHAR(64) UNIQUE     NOT NULL,
    deleted_at        BIGINT       DEFAULT NULL,
    deleted_reason    VARCHAR(255) DEFAULT NULL,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_submission_id) REFERENCES submission (id)
);
CREATE INDEX idx_submission_file_created_at ON submission_file (created_at);
CREATE INDEX idx_submission_file_deleted_at ON submission_file (deleted_at);

CREATE TABLE IF NOT EXISTS curation_meta
(
    id                    BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_submission_file_id BIGINT NOT NULL,
    application_path      TEXT,
    developer             TEXT,
    extreme               VARCHAR(7),
    game_notes            TEXT,
    languages             TEXT,
    launch_command        TEXT,
    original_description  TEXT,
    play_mode             TEXT,
    platform              VARCHAR(63),
    publisher             TEXT,
    release_date          TEXT,
    series                TEXT,
    source                TEXT,
    status                TEXT,
    tags                  TEXT,
    tag_categories        TEXT,
    title                 TEXT,
    alternate_titles      TEXT,
    library               VARCHAR(31),
    version               TEXT,
    curation_notes        TEXT,
    mount_parameters      TEXT,
    FOREIGN KEY (fk_submission_file_id) REFERENCES submission_file (id)
);
CREATE INDEX idx_curation_meta_extreme ON curation_meta (extreme);
CREATE INDEX idx_curation_meta_library ON curation_meta (library);

CREATE TABLE IF NOT EXISTS action
(
    id   BIGINT PRIMARY KEY,
    name VARCHAR(63) UNIQUE
);

INSERT IGNORE INTO action (id, name)
VALUES (1, 'comment'),
       (2, 'approve'),
       (3, 'request-changes'),
       (4, 'mark-added'),
       (5, 'upload-file'),
       (6, 'verify'),
       (7, 'assign-testing'),
       (8, 'unassign-testing'),
       (9, 'assign-verification'),
       (10, 'unassign-verification');

CREATE TABLE IF NOT EXISTS comment
(
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id       BIGINT NOT NULL,
    fk_submission_id BIGINT NOT NULL,
    message          TEXT,
    fk_action_id     BIGINT,
    created_at       BIGINT NOT NULL,
    deleted_at       BIGINT       DEFAULT NULL,
    deleted_reason   VARCHAR(255) DEFAULT NULL,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_submission_id) REFERENCES submission (id),
    FOREIGN KEY (fk_action_id) REFERENCES action (id)
);
CREATE INDEX idx_comment_created_at ON comment (created_at);
CREATE INDEX idx_comment_deleted_at ON comment (deleted_at);

CREATE TABLE IF NOT EXISTS notification_settings
(
    id           BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id   BIGINT NOT NULL,
    fk_action_id BIGINT NOT NULL,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_action_id) REFERENCES action (id)
);

CREATE TABLE IF NOT EXISTS submission_notification_subscription
(
    id               BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id       BIGINT NOT NULL,
    fk_submission_id BIGINT NOT NULL,
    created_at       BIGINT NOT NULL,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_submission_id) REFERENCES submission (id)
);
CREATE INDEX idx_submission_notification_subscription_created_at ON submission_notification_subscription (created_at);

CREATE TABLE IF NOT EXISTS submission_notification_type
(
    id   BIGINT PRIMARY KEY,
    name VARCHAR(63) UNIQUE
);

INSERT IGNORE INTO submission_notification_type (id, name)
VALUES (1, 'notification'),
       (2, 'curation-feed');

CREATE TABLE IF NOT EXISTS submission_notification
(
    id                                 BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_submission_notification_type_id BIGINT NOT NULL,
    message                            TEXT   NOT NULL,
    created_at                         BIGINT NOT NULL,
    sent_at                            BIGINT DEFAULT NULL,
    FOREIGN KEY (fk_submission_notification_type_id) REFERENCES submission_notification_type (id)
);
CREATE INDEX idx_submission_notification_created_at ON submission_notification (created_at);
CREATE INDEX idx_submission_notification_sent_at ON submission_notification (sent_at);

CREATE TABLE IF NOT EXISTS curation_image_type
(
    id   BIGINT PRIMARY KEY,
    name VARCHAR(63) UNIQUE
);

INSERT IGNORE INTO curation_image_type (id, name)
VALUES (1, 'logo'),
       (2, 'screenshot');

CREATE TABLE IF NOT EXISTS curation_image
(
    id                        BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_submission_file_id     BIGINT              NOT NULL,
    fk_curation_image_type_id BIGINT              NOT NULL,
    filename                  VARCHAR(255) UNIQUE NOT NULL,
    FOREIGN KEY (fk_submission_file_id) REFERENCES submission_file (id),
    FOREIGN KEY (fk_curation_image_type_id) REFERENCES curation_image_type (id)
);

CREATE TABLE IF NOT EXISTS submission_cache
(
    fk_submission_id                 BIGINT,
    fk_oldest_file_id                BIGINT,
    fk_newest_file_id                BIGINT,
    fk_newest_comment_id             BIGINT,

    active_assigned_testing_ids      TEXT,
    active_assigned_verification_ids TEXT,
    active_requested_changes_ids     TEXT,
    active_approved_ids              TEXT,

    original_filename_sequence       TEXT,
    current_filename_sequence        TEXT,
    md5sum_sequence                  TEXT,
    sha256sum_sequence               TEXT,
    active_verified_ids              TEXT,

    bot_action                       TEXT,
    distinct_actions                 TEXT,

    FOREIGN KEY (fk_submission_id) REFERENCES submission (id),
    FOREIGN KEY (fk_oldest_file_id) REFERENCES submission_file (id),
    FOREIGN KEY (fk_newest_file_id) REFERENCES submission_file (id),
    FOREIGN KEY (fk_newest_comment_id) REFERENCES comment (id)
);