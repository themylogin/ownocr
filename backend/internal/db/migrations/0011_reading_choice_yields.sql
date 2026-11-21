-- A reading better than the one a page answers now clears the choice a reader made
-- instead of standing aside for it, so the page answers the best reading it holds as
-- though nothing had been chosen.
--
-- That leaves nothing for this column to say. `ocr_id` is set by a reader and by
-- nobody else, so a page naming a reading is a page a reader chose for, and the flag
-- held the same fact twice.
ALTER TABLE book_page DROP COLUMN IF EXISTS ocr_chosen_by_reader;
