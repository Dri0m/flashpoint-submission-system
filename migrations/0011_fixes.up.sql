CREATE TABLE fix_type
(
    id   BIGINT PRIMARY KEY,
    name VARCHAR(63) UNIQUE
);

INSERT INTO fix_type (id, name)
VALUES (1, 'generic');

CREATE TABLE fixes
(
    id                    BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_user_id            BIGINT NOT NULL,
    fk_fix_type_id        BIGINT NOT NULL,
    submit_finished       BOOL,
    title                 TEXT   NOT NULL,
    description           TEXT   NOT NULL,
    created_at            BIGINT       DEFAULT NULL,
    deleted_at            BIGINT       DEFAULT NULL,
    deleted_reason        VARCHAR(255) DEFAULT NULL,
    fk_deleted_by_user_id BIGINT       DEFAULT NULL,
    FOREIGN KEY (fk_user_id) REFERENCES discord_user (id),
    FOREIGN KEY (fk_fix_type_id) REFERENCES fix_type (id),
    FOREIGN KEY (fk_deleted_by_user_id) REFERENCES discord_user (id)
);
CREATE INDEX idx_fixes_created_at ON fixes (created_at);
CREATE INDEX idx_fixes_deleted_at ON fixes (deleted_at);
