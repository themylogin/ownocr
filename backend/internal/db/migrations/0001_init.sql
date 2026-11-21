CREATE TABLE IF NOT EXISTS jobs (
    id BIGSERIAL PRIMARY KEY,
    source_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    text TEXT,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS jobs_created_at_idx ON jobs (created_at DESC);
