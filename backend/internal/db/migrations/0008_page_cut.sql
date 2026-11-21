-- A page cut out of a spread records the side of the gutter it holds and the
-- column it was cut at. The column counts in the columns of the stored image of
-- the source page: the left page holds every column before it, the right page
-- holds it and every column after. One column says it all, because the other
-- edge of a cut page is an edge of the parent image.
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS cut_side TEXT;
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS cut_column INTEGER;

ALTER TABLE book_page DROP CONSTRAINT IF EXISTS book_page_cut_check;
ALTER TABLE book_page ADD CONSTRAINT book_page_cut_check CHECK (
    (cut_side IS NULL AND cut_column IS NULL)
    OR (cut_side IN ('left', 'right') AND cut_column IS NOT NULL)
);

-- Every split so far wrote the left page before the right page, and wrote each
-- half as a crop of the source original. A stored width is therefore a count of
-- source columns, and both cuts follow from the widths. A source page that made
-- one page was never cut, so the backfill passes it over.
WITH cut AS (
    SELECT page.id,
           row_number() OVER (PARTITION BY page.source_page_id
                              ORDER BY page.page_index) AS half,
           count(*) OVER (PARTITION BY page.source_page_id) AS halves,
           page.width,
           source.width AS source_width
    FROM book_page page
    JOIN book_page source ON source.id = page.source_page_id
)
UPDATE book_page target
SET cut_side = CASE WHEN cut.half = 1 THEN 'left' ELSE 'right' END,
    cut_column = CASE WHEN cut.half = 1 THEN cut.width
                      ELSE cut.source_width - cut.width END
FROM cut
WHERE target.id = cut.id AND cut.halves = 2;
