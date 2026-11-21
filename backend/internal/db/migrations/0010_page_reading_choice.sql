-- A page can be read by more than one engine, and one of its results answers.
-- Until now the answer came from a fixed preference in the code, so a reader
-- could not disagree with it and a better reading could not take over.
--
-- NULL means the page names no reading and answers by that preference, which is
-- how every page written before now reads. `ON DELETE SET NULL` is what returns
-- a page to the preference when the reading it named is removed: a normal rescan
-- deletes the visual results of a page, and the page answers by preference again
-- with no application code involved.
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS ocr_id BIGINT
    REFERENCES page_ocr (id) ON DELETE SET NULL;

-- Whether a reader set the name or the system did. Without it the system cannot
-- tell its own assignment from a choice a reader made, and the rule that a better
-- model never overrules a reader has nothing to test.
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS ocr_chosen_by_reader BOOLEAN NOT NULL DEFAULT FALSE;
