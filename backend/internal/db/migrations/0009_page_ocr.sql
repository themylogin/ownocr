-- The OCR endpoint runs two engines. A page therefore holds one result per
-- engine that read it, not one markdown file. `txt` is what the engine wrote,
-- unchanged: PP-OCRv6 writes plain lines, PaddleOCR-VL writes markdown. `data`
-- is the pipeline result of that engine, which holds what `txt` drops.
CREATE TABLE IF NOT EXISTS page_ocr (
    id BIGSERIAL PRIMARY KEY,
    page_id BIGINT NOT NULL REFERENCES book_page (id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    txt TEXT NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS page_ocr_page_model_idx ON page_ocr (page_id, model);

-- The profile the user picked. It cannot be derived on a retry: the OCR worker
-- deletes the pages of the copy first, which removes the results through the
-- cascade above. It also says which engine holds the text of record, so no row
-- needs a flag of its own.
ALTER TABLE book ADD COLUMN IF NOT EXISTS ocr_profile TEXT;

-- The endpoint ran PaddleOCR-VL alone until now, so `vl` names what happened.
-- This runs before the constraint, which validates every existing row.
UPDATE book SET ocr_profile = 'vl' WHERE action = 'ocr';

-- A check passes when its expression is NULL, not only when it is true. Both
-- branches therefore test for NULL before they compare, so an OCR copy that
-- carries no profile is refused instead of admitted.
ALTER TABLE book DROP CONSTRAINT IF EXISTS book_ocr_profile_check;
ALTER TABLE book ADD CONSTRAINT book_ocr_profile_check CHECK (
    (action IS DISTINCT FROM 'ocr' AND ocr_profile IS NULL)
    OR (action = 'ocr' AND ocr_profile IS NOT NULL AND ocr_profile IN ('v6', 'vl'))
);

-- The book whose directory holds the files of this page. NULL means the page
-- owns its files, which is every page written before now and every page of
-- every action other than OCR. An OCR copy reads a page without changing it, so
-- it points at the image of its source page instead of writing its own.
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS image_book_key TEXT;

-- The `done` stage named the end of the pipeline rather than what a book in it
-- already is. Every other stage names what a book already is, and a book in the
-- last stage has been read. Only the OCR action ever made a book carrying
-- `done`, so every one of them belongs in `ocr`.
UPDATE book SET stage = 'ocr' WHERE stage = 'done';
