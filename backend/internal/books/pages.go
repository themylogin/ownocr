package books

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrPageNotFound = errors.New("page not found")

// PageOfBook carries a page with the storage key of the book that holds it. The
// file of a page lives under that key, so a caller that removes files needs
// both.
type PageOfBook struct {
	Page
	BookStorageKey string
	BookStage      string
	BookName       string
}

const pageOfBookColumns = `page.id, page.book_id, page.page_index, page.storage_key,
	page.original_ext, page.width, page.height, page.source_page_id, page.rolled_back,
	page.cut_side, page.cut_column, page.image_book_key,
	book.storage_key, book.stage, book.name`

// PageAt returns one page of a book by its index.
func (s *Store) PageAt(ctx context.Context, bookID int64, index int) (PageOfBook, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+pageOfBookColumns+`
		FROM book_page page JOIN book ON book.id = page.book_id
		WHERE page.book_id = $1 AND page.page_index = $2`, bookID, index)
	if err != nil {
		return PageOfBook{}, err
	}
	return collectOnePage(rows)
}

// PageByID returns one page by its own id.
func (s *Store) PageByID(ctx context.Context, id int64) (PageOfBook, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+pageOfBookColumns+`
		FROM book_page page JOIN book ON book.id = page.book_id
		WHERE page.id = $1`, id)
	if err != nil {
		return PageOfBook{}, err
	}
	return collectOnePage(rows)
}

// PageDescendants returns a page and every page made from it, deepest first.
// Removing them in that order never leaves a page without its source.
func (s *Store) PageDescendants(ctx context.Context, pageID int64) ([]PageOfBook, error) {
	rows, err := s.pool.Query(ctx, `WITH RECURSIVE descendants AS (
		SELECT id, 0 AS depth FROM book_page WHERE id = $1
		UNION ALL
		SELECT child.id, descendants.depth + 1
		FROM book_page child JOIN descendants ON child.source_page_id = descendants.id
	)
	SELECT `+pageOfBookColumns+` FROM descendants
	JOIN book_page page ON page.id = descendants.id
	JOIN book ON book.id = page.book_id
	ORDER BY descendants.depth DESC, page.id DESC`, pageID)
	if err != nil {
		return nil, err
	}

	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[PageOfBook])
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrPageNotFound
	}
	return list, nil
}

// SourcePage returns the page a page was made from. The second answer is false
// when the page names none, which is what a lost or absent source reads as.
func (s *Store) SourcePage(ctx context.Context, page PageOfBook) (PageOfBook, bool) {
	if page.SourcePageID == nil {
		return PageOfBook{}, false
	}

	source, err := s.PageByID(ctx, *page.SourcePageID)
	if err != nil {
		return PageOfBook{}, false
	}
	return source, true
}

// maxLineage bounds the walk up the source pages of a page. A page made from
// itself would otherwise never end the recursion. No real chain is longer than
// the stages of the pipeline.
const maxLineage = 16

// CutSideOf answers the side of the gutter a page was cut out of, taken from the
// nearest page of its lineage that carries one. A copy holds the pages of its
// source book, so a page of an OCR copy carries no cut side of its own and takes
// the side of the page it was read from.
//
// The second answer is false for a page that was never cut out of a spread.
func (s *Store) CutSideOf(ctx context.Context, pageID int64) (string, bool) {
	var side *string
	err := s.pool.QueryRow(ctx, `WITH RECURSIVE lineage AS (
		SELECT id, source_page_id, cut_side, 0 AS depth
		FROM book_page WHERE id = $1
		UNION ALL
		SELECT parent.id, parent.source_page_id, parent.cut_side, lineage.depth + 1
		FROM book_page parent JOIN lineage ON parent.id = lineage.source_page_id
		WHERE lineage.cut_side IS NULL AND lineage.depth < $2
	)
	SELECT cut_side FROM lineage
	WHERE cut_side IS NOT NULL ORDER BY depth LIMIT 1`, pageID, maxLineage).Scan(&side)
	if err != nil || side == nil {
		return "", false
	}
	return *side, true
}

// CutSidesOfBook answers the cut side of every page of one book that reaches
// one, by page id. It walks the same lineage as CutSideOf, for every page at
// once, so a book of five hundred pages costs one query and not five hundred.
func (s *Store) CutSidesOfBook(ctx context.Context, bookID int64) (map[int64]string, error) {
	rows, err := s.pool.Query(ctx, `WITH RECURSIVE lineage AS (
		SELECT id AS page_id, source_page_id, cut_side, 0 AS depth
		FROM book_page WHERE book_id = $1
		UNION ALL
		SELECT lineage.page_id, parent.source_page_id, parent.cut_side, lineage.depth + 1
		FROM book_page parent JOIN lineage ON parent.id = lineage.source_page_id
		WHERE lineage.cut_side IS NULL AND lineage.depth < $2
	)
	SELECT DISTINCT ON (page_id) page_id, cut_side FROM lineage
	WHERE cut_side IS NOT NULL ORDER BY page_id, depth`, bookID, maxLineage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sides := map[int64]string{}
	for rows.Next() {
		var pageID int64
		var side string
		if err := rows.Scan(&pageID, &side); err != nil {
			return nil, err
		}
		sides[pageID] = side
	}
	return sides, rows.Err()
}

func (s *Store) DeletePage(ctx context.Context, id int64) error {
	return deletePage(ctx, s.pool.Exec, id)
}

// DeletePageTx removes a page inside the transaction that holds the chain lock,
// so a chain of pages goes whole or not at all.
func (s *Store) DeletePageTx(ctx context.Context, tx pgx.Tx, id int64) error {
	return deletePage(ctx, tx.Exec, id)
}

// RollBackPage points a page at a new set of renditions, made from the page it
// came from. The page keeps its index and the source page it names, so a
// rollback changes the bytes alone. It marks the page, which is what takes the
// rollback out of its menu.
func (s *Store) RollBackPage(ctx context.Context, id int64, storageKey, ext string,
	width, height int) (PageOfBook, error) {
	rows, err := s.pool.Query(ctx, rollBackPage, id, storageKey, ext, width, height)
	if err != nil {
		return PageOfBook{}, err
	}
	return collectOnePage(rows)
}

func (s *Store) RollBackPageTx(ctx context.Context, tx pgx.Tx, id int64, storageKey,
	ext string, width, height int) (PageOfBook, error) {
	rows, err := tx.Query(ctx, rollBackPage, id, storageKey, ext, width, height)
	if err != nil {
		return PageOfBook{}, err
	}
	return collectOnePage(rows)
}

const rollBackPage = `WITH written AS (
	UPDATE book_page SET storage_key = $2, original_ext = $3, width = $4, height = $5,
		rolled_back = true
	WHERE id = $1 RETURNING *
)
SELECT ` + pageOfBookColumns + ` FROM written page JOIN book ON book.id = page.book_id`

// ReCutPageTx points a page at renditions cut out of its source page at another
// column. The page keeps its index, the source page it names and the side of
// the gutter it holds: a re-cut moves one edge and nothing else.
func (s *Store) ReCutPageTx(ctx context.Context, tx pgx.Tx, id int64, storageKey,
	ext string, width, height, column int) (PageOfBook, error) {
	rows, err := tx.Query(ctx, reCutPage, id, storageKey, ext, width, height, column)
	if err != nil {
		return PageOfBook{}, err
	}
	return collectOnePage(rows)
}

const reCutPage = `WITH written AS (
	UPDATE book_page SET storage_key = $2, original_ext = $3, width = $4, height = $5,
		cut_column = $6
	WHERE id = $1 RETURNING *
)
SELECT ` + pageOfBookColumns + ` FROM written page JOIN book ON book.id = page.book_id`

type execFunc func(context.Context, string, ...any) (pgconn.CommandTag, error)

func deletePage(ctx context.Context, exec execFunc, id int64) error {
	tag, err := exec(ctx, `DELETE FROM book_page WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPageNotFound
	}
	return nil
}

// CountPages answers how many pages a book holds now. A delete leaves gaps in
// the index, so the count cannot be read from the last index.
func (s *Store) CountPages(ctx context.Context, bookID int64) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM book_page WHERE book_id = $1`,
		bookID).Scan(&count)
	return count, err
}

// LockChainTx locks the book and every book made from it, in ascending id
// order, so two writers of one chain take turns instead of interleaving. The
// order is what keeps two chains that overlap from deadlocking.
func (s *Store) LockChainTx(ctx context.Context, tx pgx.Tx, bookID int64) error {
	rows, err := tx.Query(ctx, `WITH RECURSIVE chain AS (
		SELECT id FROM book WHERE id = $1
		UNION ALL
		SELECT child.id FROM book child JOIN chain ON child.source_book_id = chain.id
	)
	SELECT book.id FROM book JOIN chain ON chain.id = book.id
	ORDER BY book.id FOR UPDATE`, bookID)
	if err != nil {
		return err
	}

	locked, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	if len(locked) == 0 {
		return ErrNotFound
	}
	return nil
}

func collectOnePage(rows pgx.Rows) (PageOfBook, error) {
	page, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[PageOfBook])
	if errors.Is(err, pgx.ErrNoRows) {
		return PageOfBook{}, ErrPageNotFound
	}
	return page, err
}
