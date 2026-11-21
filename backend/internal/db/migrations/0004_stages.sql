ALTER TABLE book ADD COLUMN IF NOT EXISTS progress TEXT;
ALTER TABLE book ADD COLUMN IF NOT EXISTS action TEXT;

UPDATE book SET action = 'dewarp' WHERE source_book_id IS NOT NULL;

UPDATE book SET stage = 'uploaded' WHERE stage = 'dewarp';
UPDATE book SET stage = 'dewarped' WHERE stage = 'split';
UPDATE book SET stage = 'dewarped' WHERE stage IS NULL AND source_book_id IS NOT NULL;
UPDATE book SET stage = 'uploaded' WHERE stage IS NULL;

ALTER TABLE book ALTER COLUMN stage SET NOT NULL;
ALTER TABLE book ALTER COLUMN stage SET DEFAULT 'uploaded';

DROP INDEX IF EXISTS book_state_idx;
DROP INDEX IF EXISTS book_stage_idx;
CREATE INDEX IF NOT EXISTS book_stage_order_idx ON book (stage, created_at DESC);
