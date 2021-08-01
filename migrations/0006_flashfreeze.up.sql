CREATE TABLE IF NOT EXISTS flashfreeze_file
(
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id        BIGINT              NOT NULL,
    original_filename VARCHAR(255)        NOT NULL,
    current_filename  VARCHAR(255) UNIQUE NOT NULL,
    size              BIGINT              NOT NULL,
    created_at        BIGINT              NOT NULL,
    md5sum            CHAR(32) UNIQUE     NOT NULL,
    sha256sum         CHAR(64) UNIQUE     NOT NULL,
    indexed_at        BIGINT       DEFAULT NULL,
    deleted_at        BIGINT       DEFAULT NULL,
    deleted_reason    VARCHAR(255) DEFAULT NULL,
    FULLTEXT (original_filename) WITH PARSER NGRAM,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id)
);
CREATE INDEX idx_flashfreeze_file_created_at ON flashfreeze_file (created_at);
CREATE INDEX idx_flashfreeze_file_deleted_at ON flashfreeze_file (deleted_at);