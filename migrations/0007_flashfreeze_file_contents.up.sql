CREATE TABLE IF NOT EXISTS flashfreeze_file_contents
(
    id                     BIGINT PRIMARY KEY AUTO_INCREMENT,
    fk_flashfreeze_file_id BIGINT   NOT NULL,
    filename               TEXT     NOT NULL,
    size_compressed        BIGINT   NOT NULL,
    size_uncompressed      BIGINT   NOT NULL,
    md5sum                 CHAR(32) NOT NULL,
    sha256sum              CHAR(64) NOT NULL,
    description            TEXT     NOT NULL,
    FULLTEXT (filename) WITH PARSER NGRAM,
    FULLTEXT (description) WITH PARSER NGRAM,
    FOREIGN KEY (fk_flashfreeze_file_id) REFERENCES flashfreeze_file (id)
);
CREATE INDEX idx_flashfreeze_file_contents_size_compressed ON flashfreeze_file_contents (size_compressed);
CREATE INDEX idx_flashfreeze_file_contents_size_uncompressed ON flashfreeze_file_contents (size_uncompressed);
CREATE INDEX idx_flashfreeze_file_contents_size_md5sum ON flashfreeze_file_contents (md5sum);
CREATE INDEX idx_flashfreeze_file_contents_size_sha256sum ON flashfreeze_file_contents (sha256sum);