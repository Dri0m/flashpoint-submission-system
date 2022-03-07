CREATE TABLE fixes_file
(
    id                    BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id            BIGINT              NOT NULL,
    fk_fix_id             BIGINT              NOT NULL,
    original_filename     VARCHAR(255)        NOT NULL,
    current_filename      VARCHAR(255) UNIQUE NOT NULL,
    size                  BIGINT              NOT NULL,
    created_at            BIGINT              NOT NULL,
    md5sum                CHAR(32)            NOT NULL,
    sha256sum             CHAR(64)            NOT NULL,
    deleted_at            BIGINT       DEFAULT NULL,
    deleted_reason        VARCHAR(255) DEFAULT NULL,
    fk_deleted_by_user_id BIGINT       DEFAULT NULL,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_fix_id) REFERENCES fixes (id),
    FOREIGN KEY (fk_deleted_by_user_id) REFERENCES discord_user (id)
);
CREATE INDEX idx_fixes_file_created_at ON fixes (created_at);
CREATE INDEX idx_fixes_file_deleted_at ON fixes (deleted_at);
