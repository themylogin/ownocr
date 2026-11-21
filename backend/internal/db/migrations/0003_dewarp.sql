ALTER TABLE book ADD COLUMN IF NOT EXISTS source_book_id BIGINT
    REFERENCES book (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS book_source_idx ON book (source_book_id);
