-- A page of a copy names the page it was made from. Equal page_index no longer
-- means equal page: a delete leaves gaps, and a split makes two pages from one.
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS source_page_id BIGINT
    REFERENCES book_page (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS book_page_source_idx ON book_page (source_page_id);

-- Every existing copy was written index for index, so the old convention is an
-- exact match. This runs before any code can create a gap.
UPDATE book_page c SET source_page_id = p.id
FROM book b JOIN book_page p ON p.book_id = b.source_book_id
WHERE c.book_id = b.id AND c.source_page_id IS NULL
  AND p.page_index = c.page_index;
