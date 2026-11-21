-- A page that returned to the page it came from already shows the original, so
-- it offers the rollback no more. A worker that rebuilds a copy writes new page
-- rows, which start false again.
ALTER TABLE book_page ADD COLUMN IF NOT EXISTS rolled_back BOOLEAN NOT NULL DEFAULT false;
