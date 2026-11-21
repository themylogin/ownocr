DROP TABLE IF EXISTS jobs;

CREATE TABLE IF NOT EXISTS book (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL DEFAULT 'pending',
    stage TEXT,
    page_count INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS book_stage_idx ON book (stage, created_at DESC);
CREATE INDEX IF NOT EXISTS book_state_idx ON book (state, created_at DESC);

CREATE TABLE IF NOT EXISTS book_page (
    id BIGSERIAL PRIMARY KEY,
    book_id BIGINT NOT NULL REFERENCES book (id) ON DELETE CASCADE,
    page_index INTEGER NOT NULL,
    storage_key TEXT NOT NULL,
    original_ext TEXT NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS book_page_order_idx ON book_page (book_id, page_index);
