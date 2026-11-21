package books

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatePending    = "pending"
	StateProcessing = "processing"
	StateComplete   = "complete"
	StateFailed     = "failed"
)

var ErrNotFound = errors.New("book not found")

// The state of the copy is derived on every read, so the mark a client shows
// can never drift from the copy it stands for.
const derivedJoin = ` LEFT JOIN LATERAL (
	SELECT state FROM book d WHERE d.source_book_id = book.id ORDER BY d.id LIMIT 1
) derived ON true`

const bookColumns = `book.id, book.name, book.storage_key, book.state, book.stage,
	book.page_count, book.error, book.created_at, book.updated_at, book.source_book_id,
	book.action, book.progress, book.ocr_profile, derived.state`

// The list order: a failure first, then work in flight, then books ready for an
// action, then books whose copy runs, then books whose copy is done.
const stageOrder = ` ORDER BY CASE
	WHEN book.state = 'failed' THEN 0
	WHEN book.state IN ('pending', 'processing') THEN 1
	WHEN derived.state IS NULL THEN 2
	WHEN derived.state = 'complete' THEN 4
	ELSE 3
END, book.created_at DESC, book.id DESC`

func selectBooks(where, order string) string {
	return `SELECT ` + bookColumns + ` FROM book` + derivedJoin + ` ` + where + order
}

// writeBooks wraps a write so its answer carries the same columns a read does.
// The statement must end without RETURNING.
func writeBooks(statement string) string {
	return `WITH written AS (` + statement + ` RETURNING *) SELECT ` + bookColumns +
		` FROM written book` + derivedJoin
}

type Book struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	StorageKey   string    `json:"storageKey"`
	State        string    `json:"state"`
	Stage        string    `json:"stage"`
	PageCount    int       `json:"pageCount"`
	Error        *string   `json:"error"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	SourceBookID *int64    `json:"sourceBookId"`
	Action       *string   `json:"action"`
	Progress     *string   `json:"progress"`
	OCRProfile   *string   `json:"ocrProfile"`
	DerivedState *string   `json:"derivedState"`
}

// CutSide and CutColumn describe the cut a page was made by, in the columns of
// the stored image of its source page. The left page holds every column before
// CutColumn; the right page holds that column and every column after it. A page
// that reached its book whole carries neither.
//
// ImageBookKey names the book whose directory holds the files of this page.
// Empty means the page owns its files, which is every page except one an OCR
// copy made: OCR reads a page without changing it, so its copy points at the
// image of the source page instead of writing its own.
type Page struct {
	ID           int64   `json:"id"`
	BookID       int64   `json:"bookId"`
	PageIndex    int     `json:"pageIndex"`
	StorageKey   string  `json:"storageKey"`
	OriginalExt  string  `json:"originalExt"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	SourcePageID *int64  `json:"sourcePageId"`
	RolledBack   bool    `json:"rolledBack"`
	CutSide      *string `json:"cutSide"`
	CutColumn    *int    `json:"cutColumn"`
	ImageBookKey *string `json:"imageBookKey"`
}

// ImageKey is the storage key of the book whose directory holds the files of
// this page. Every URL of a page rendition is built from it.
func (p Page) ImageKey(bookKey string) string {
	if p.ImageBookKey != nil && *p.ImageBookKey != "" {
		return *p.ImageBookKey
	}
	return bookKey
}

const (
	CutLeft  = "left"
	CutRight = "right"
)

const pageColumns = `id, book_id, page_index, storage_key, original_ext, width, height,
	source_page_id, rolled_back, cut_side, cut_column, image_book_key`

func NewStorageKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ListAll(ctx context.Context) ([]Book, error) {
	return s.list(ctx, selectBooks("", ` ORDER BY book.created_at DESC, book.id DESC`))
}

// ListByStage returns every book of a stage, whatever its state.
func (s *Store) ListByStage(ctx context.Context, stage string) ([]Book, error) {
	return s.list(ctx, selectBooks(`WHERE book.stage = $1`, stageOrder), stage)
}

func (s *Store) CountByStage(ctx context.Context, stage string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM book WHERE stage = $1`, stage).Scan(&count)
	return count, err
}

func (s *Store) Get(ctx context.Context, id int64) (Book, error) {
	rows, err := s.pool.Query(ctx, selectBooks(`WHERE book.id = $1`, ""), id)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

func (s *Store) GetTx(ctx context.Context, tx pgx.Tx, id int64) (Book, error) {
	rows, err := tx.Query(ctx, selectBooks(`WHERE book.id = $1`, ""), id)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

func (s *Store) Pages(ctx context.Context, bookID int64) ([]Page, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+pageColumns+`
		FROM book_page WHERE book_id = $1 ORDER BY page_index`, bookID)
	if err != nil {
		return nil, err
	}

	pages, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Page])
	if err != nil {
		return nil, err
	}
	if pages == nil {
		pages = []Page{}
	}
	return pages, nil
}

// FirstPages returns the cover page of each requested book, keyed by book id.
// A book with no pages is absent from the map.
func (s *Store) FirstPages(ctx context.Context, bookIDs []int64) (map[int64]Page, error) {
	covers := make(map[int64]Page, len(bookIDs))
	if len(bookIDs) == 0 {
		return covers, nil
	}

	rows, err := s.pool.Query(ctx, `SELECT `+pageColumns+`
		FROM book_page WHERE book_id = ANY($1)
		AND page_index = (SELECT min(page_index) FROM book_page inner_page
			WHERE inner_page.book_id = book_page.book_id)`, bookIDs)
	if err != nil {
		return nil, err
	}

	pages, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Page])
	if err != nil {
		return nil, err
	}
	for _, page := range pages {
		covers[page.BookID] = page
	}
	return covers, nil
}

// Cover returns the first page of a book, or nil when it has none yet.
func (s *Store) Cover(ctx context.Context, bookID int64) *Page {
	covers, err := s.FirstPages(ctx, []int64{bookID})
	if err != nil {
		return nil
	}
	if cover, ok := covers[bookID]; ok {
		return &cover
	}
	return nil
}

func (s *Store) CreateTx(ctx context.Context, tx pgx.Tx, name, storageKey string) (Book, error) {
	rows, err := tx.Query(ctx, writeBooks(`INSERT INTO book (name, storage_key, state, stage, progress)
		VALUES ($1, $2, $3, $4, $5)`), name, storageKey, StatePending, FirstStage(), PhaseQueued)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

// CreateDerivedTx makes the copy that an action fills. The reference it carries
// is what marks the source book. The copy carries the stage of its action from
// this moment, so it is listed where it is going.
//
// ocrProfile is the argument the OCR action takes, and is empty for every other
// action. The copy holds it because a retry cannot derive it: the worker deletes
// the pages of the copy first, which removes their results with them.
func (s *Store) CreateDerivedTx(ctx context.Context, tx pgx.Tx, name, storageKey string,
	sourceID int64, action Action, ocrProfile string) (Book, error) {
	var profile *string
	if ocrProfile != "" {
		profile = &ocrProfile
	}

	rows, err := tx.Query(ctx, writeBooks(`INSERT INTO book
		(name, storage_key, state, stage, source_book_id, action, progress, ocr_profile)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`),
		name, storageKey, StatePending, action.Stage, sourceID, action.ID, PhaseQueued, profile)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

// RequeueTx returns a failed copy to the queue. The copy keeps its identity and
// its stage, so a retry never produces a third book and never leaves its tab.
func (s *Store) RequeueTx(ctx context.Context, tx pgx.Tx, id int64) (Book, error) {
	rows, err := tx.Query(ctx, writeBooks(`UPDATE book SET state = $2, error = NULL, page_count = 0,
		progress = $3, updated_at = now() WHERE id = $1`), id, StatePending, PhaseQueued)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

// LockTx serializes two clients that ask for the same book at once. The second
// waits, then sees the copy the first created.
func (s *Store) LockTx(ctx context.Context, tx pgx.Tx, id int64) error {
	var locked int64
	err := tx.QueryRow(ctx, `SELECT id FROM book WHERE id = $1 FOR UPDATE`, id).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Chain returns the book and every copy made from it, deepest first. Deleting
// them in that order never leaves a copy without its source.
func (s *Store) Chain(ctx context.Context, id int64) ([]Book, error) {
	rows, err := s.pool.Query(ctx, `WITH RECURSIVE chain AS (
		SELECT id, 0 AS depth FROM book WHERE id = $1
		UNION ALL
		SELECT child.id, chain.depth + 1
		FROM book child JOIN chain ON child.source_book_id = chain.id
	)
	SELECT `+bookColumns+` FROM chain
	JOIN book ON book.id = chain.id`+derivedJoin+`
	ORDER BY chain.depth DESC, book.id DESC`, id)
	if err != nil {
		return nil, err
	}

	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Book])
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list, nil
}

// Delete removes a book and its page rows. The caller removes its files.
func (s *Store) Delete(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM book WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SourceOf returns the book a copy came from. The second answer is false when
// the book is not a copy.
func (s *Store) SourceOf(ctx context.Context, book Book) (Book, bool) {
	if book.SourceBookID == nil {
		return Book{}, false
	}

	source, err := s.Get(ctx, *book.SourceBookID)
	if err != nil {
		return Book{}, false
	}
	return source, true
}

// DerivedTx returns the copy that references the given book, if one exists.
func (s *Store) DerivedTx(ctx context.Context, tx pgx.Tx, sourceID int64) (Book, error) {
	rows, err := tx.Query(ctx, selectBooks(`WHERE book.source_book_id = $1`,
		` ORDER BY book.id LIMIT 1`), sourceID)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

func (s *Store) SetProcessing(ctx context.Context, id int64, phase string) (Book, error) {
	return s.write(ctx, `UPDATE book SET state = $2, error = NULL, page_count = 0, progress = $3,
		updated_at = now() WHERE id = $1`, id, StateProcessing, phase)
}

func (s *Store) SetComplete(ctx context.Context, id int64, pageCount int) (Book, error) {
	return s.write(ctx, `UPDATE book SET state = $2, page_count = $3, error = NULL, progress = NULL,
		updated_at = now() WHERE id = $1`, id, StateComplete, pageCount)
}

func (s *Store) SetFailed(ctx context.Context, id int64, reason string) (Book, error) {
	return s.write(ctx, `UPDATE book SET state = $2, error = $3, progress = NULL, updated_at = now()
		WHERE id = $1`, id, StateFailed, reason)
}

// SetProcessingKeepingCountTx starts work that adds to a book instead of
// rebuilding it. SetProcessing zeroes page_count, which is right for a worker
// that deletes the pages first and wrong for one that leaves them standing.
//
// It takes a transaction, so the state of the book and the job that acts on it
// commit together or not at all.
func (s *Store) SetProcessingKeepingCountTx(ctx context.Context, tx pgx.Tx, id int64,
	phase string) (Book, error) {
	return s.writeTx(ctx, tx, `UPDATE book SET state = $2, error = NULL, progress = $3,
		updated_at = now() WHERE id = $1`, id, StateProcessing, phase)
}

// SetCompleteKeepingCount ends such work. The page count it leaves is the count
// the book already held.
func (s *Store) SetCompleteKeepingCount(ctx context.Context, id int64) (Book, error) {
	return s.write(ctx, `UPDATE book SET state = $2, error = NULL, progress = NULL,
		updated_at = now() WHERE id = $1`, id, StateComplete)
}

func (s *Store) SetProgress(ctx context.Context, id int64, phase string) (Book, error) {
	return s.write(ctx, `UPDATE book SET progress = $2, updated_at = now() WHERE id = $1`, id, phase)
}

func (s *Store) SetPageCount(ctx context.Context, id int64, pageCount int) (Book, error) {
	return s.write(ctx, `UPDATE book SET page_count = $2, updated_at = now() WHERE id = $1`,
		id, pageCount)
}

func (s *Store) DeletePages(ctx context.Context, bookID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM book_page WHERE book_id = $1`, bookID)
	return err
}

// PageOCR is what one engine produced for one page. Text is what the engine
// wrote, unchanged: PP-OCRv6 writes plain lines, PaddleOCR-VL writes markdown.
// Data is the result of that engine in full, and holds what Text drops.
type PageOCR struct {
	ID     int64  `json:"id"`
	PageID int64  `json:"pageId"`
	Model  string `json:"model"`
	Text   string `json:"txt"`
	Data   []byte `json:"data"`
}

// InsertPageOCR stores one result, and clears the reading a reader chose for its
// page when the result outranks it. A second result for the same page and engine
// replaces the first, so a retry cannot leave two.
//
// ranked names the readings a renderer can draw, best first, which is also the
// order a better reading is judged by. A caller that passes none stores the result
// and changes no reading.
func (s *Store) InsertPageOCR(ctx context.Context, result PageOCR, ranked []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := insertPageOCRTx(ctx, tx, result, ranked); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func insertPageOCRTx(ctx context.Context, tx pgx.Tx, result PageOCR, ranked []string) error {
	_, err := tx.Exec(ctx, insertPageOCRSQL,
		result.PageID, result.Model, result.Text, result.Data)
	if err != nil {
		return err
	}

	if len(ranked) == 0 {
		return nil
	}

	_, err = tx.Exec(ctx, clearChosenReadingSQL, result.PageID, result.Model, ranked)
	return err
}

const insertPageOCRSQL = `INSERT INTO page_ocr (page_id, model, txt, data)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (page_id, model) DO UPDATE SET txt = EXCLUDED.txt, data = EXCLUDED.data`

// clearChosenReadingSQL drops the reading a reader chose for a page when the result
// just stored outranks it, so the page answers the best reading it holds as though
// nothing had been chosen. A lower array position is a better reading, so the
// comparison reads backwards.
//
// A page that names nothing needs no statement: it answers by rank already, so a
// better reading arriving becomes its answer with nothing written.
//
// A result a renderer cannot draw has no position and clears nothing. Nor does one
// that merely equals the chosen reading, which is the same reading stored again.
const clearChosenReadingSQL = `UPDATE book_page
	SET ocr_id = NULL
	WHERE id = $1
	AND ocr_id IS NOT NULL
	AND array_position($3::text[], $2) IS NOT NULL
	AND array_position($3::text[], $2) < (
		SELECT array_position($3::text[], named.model)
			FROM page_ocr named WHERE named.id = book_page.ocr_id)`

// ReplacePageOCR stores one result of a page and removes the named results of
// that page, in one transaction. A caller that replaces a reading the other
// results derive from names them here, so the page never holds a result derived
// from a reading it no longer holds.
//
// It stores before it removes. A crash between the two would otherwise leave the
// page with no reading at all, which is worse than the state it started in.
func (s *Store) ReplacePageOCR(ctx context.Context, result PageOCR, remove,
	ranked []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := insertPageOCRTx(ctx, tx, result, ranked); err != nil {
		return err
	}

	if len(remove) > 0 {
		// The result just stored survives a remove list that names it.
		_, err = tx.Exec(ctx, `DELETE FROM page_ocr
			WHERE page_id = $1 AND model = ANY($2) AND model <> $3`,
			result.PageID, remove, result.Model)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// PageReading answers the reading a page answers: the result it names, or the
// best ranked result it holds when it names none.
//
// A page answers the reading it names whatever ranked holds. The list is the order
// a page falls back to, not a filter on a choice already made.
func (s *Store) PageReading(ctx context.Context, pageID int64,
	ranked []string) (PageOCR, bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT result.id, result.page_id, result.model,
		result.txt, result.data
		FROM page_ocr result JOIN book_page page ON page.id = result.page_id
		WHERE result.page_id = $1
		AND (result.id = page.ocr_id OR array_position($2::text[], result.model) IS NOT NULL)
		ORDER BY (result.id = page.ocr_id) DESC,
			array_position($2::text[], result.model) LIMIT 1`, pageID, ranked)
	if err != nil {
		return PageOCR{}, false, err
	}

	result, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[PageOCR])
	if errors.Is(err, pgx.ErrNoRows) {
		return PageOCR{}, false, nil
	}
	if err != nil {
		return PageOCR{}, false, err
	}
	return result, true, nil
}

// BookReadings answers the reading every page of a book answers, keyed by page
// id. It picks per page by the same rule PageReading holds, so the answer for a
// page here and the answer for that page alone can never name two models.
func (s *Store) BookReadings(ctx context.Context, bookID int64,
	ranked []string) (map[int64]PageOCR, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (result.page_id)
		result.id, result.page_id, result.model, result.txt, result.data
		FROM page_ocr result JOIN book_page page ON page.id = result.page_id
		WHERE page.book_id = $1
		AND (result.id = page.ocr_id OR array_position($2::text[], result.model) IS NOT NULL)
		ORDER BY result.page_id, (result.id = page.ocr_id) DESC,
			array_position($2::text[], result.model)`, bookID, ranked)
	if err != nil {
		return nil, err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[PageOCR])
	if err != nil {
		return nil, err
	}

	byPage := make(map[int64]PageOCR, len(results))
	for _, result := range results {
		byPage[result.PageID] = result
	}
	return byPage, nil
}

// Reading is one result a page holds, without the weight of that result. Answers
// marks the one the page answers, which is the one it names or the best ranked it
// holds.
type Reading struct {
	ID      int64  `json:"id"`
	Model   string `json:"model"`
	Answers bool   `json:"answers"`
}

// PageReadings answers every result a page holds, best ranked first, and marks
// the one that answers. It leaves out a result whose model is not in ranked,
// which a renderer cannot draw and a reader therefore cannot choose.
func (s *Store) PageReadings(ctx context.Context, pageID int64,
	ranked []string) ([]Reading, error) {
	rows, err := s.pool.Query(ctx, `SELECT result.id, result.model,
		result.id = COALESCE(page.ocr_id, first_value(result.id) OVER (
			ORDER BY array_position($2::text[], result.model)))
		FROM page_ocr result JOIN book_page page ON page.id = result.page_id
		WHERE result.page_id = $1 AND array_position($2::text[], result.model) IS NOT NULL
		ORDER BY array_position($2::text[], result.model)`, pageID, ranked)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Reading])
}

// SetPageReading makes a page answer one of the results it holds. A page names a
// reading because a reader chose it and for no other reason, so this is the only
// writer of that name.
//
// It answers ErrNotFound when the result is not a result of that page, so a request
// naming another page's reading changes nothing.
func (s *Store) SetPageReading(ctx context.Context, pageID, resultID int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE book_page SET ocr_id = $2
		WHERE id = $1 AND EXISTS (
			SELECT 1 FROM page_ocr WHERE id = $2 AND page_id = $1)`, pageID, resultID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PageOCRResults returns every result of a page, the engine name in order.
func (s *Store) PageOCRResults(ctx context.Context, pageID int64) ([]PageOCR, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, page_id, model, txt, data
		FROM page_ocr WHERE page_id = $1 ORDER BY model`, pageID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[PageOCR])
}

// PageIndexesWithModel answers the index of every page of a book that holds a
// result of the given model. The caller names the model, so the store needs no
// knowledge of an engine.
func (s *Store) PageIndexesWithModel(ctx context.Context, bookID int64, model string) (map[int]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT page.page_index
		FROM book_page page JOIN page_ocr result ON result.page_id = page.id
		WHERE page.book_id = $1 AND result.model = $2`, bookID, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	held := make(map[int]bool)
	for rows.Next() {
		var index int
		if err := rows.Scan(&index); err != nil {
			return nil, err
		}
		held[index] = true
	}
	return held, rows.Err()
}

// PageOCRIn answers the result of a page whose model stands first in the list.
// The caller holds the preference, so the store needs no knowledge of an engine
// or a profile. The second answer is false when the page holds none of them.
//
// It reads the list and nothing else. A caller asking which reading a page
// answers wants PageReading, which honours the reading the page names; a caller
// asking for the result of one engine wants this.
func (s *Store) PageOCRIn(ctx context.Context, pageID int64, models []string) (PageOCR, bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, page_id, model, txt, data
		FROM page_ocr WHERE page_id = $1 AND model = ANY($2)
		ORDER BY array_position($2, model) LIMIT 1`, pageID, models)
	if err != nil {
		return PageOCR{}, false, err
	}

	result, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[PageOCR])
	if errors.Is(err, pgx.ErrNoRows) {
		return PageOCR{}, false, nil
	}
	if err != nil {
		return PageOCR{}, false, err
	}
	return result, true, nil
}

// BookOCRIn answers the result of every page of a book whose model stands first
// in the list, keyed by page id. A page that holds none of them is absent from
// the map. It reads the list and nothing else, as PageOCRIn does.
func (s *Store) BookOCRIn(ctx context.Context, bookID int64, models []string) (map[int64]PageOCR, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (page_ocr.page_id)
		page_ocr.id, page_ocr.page_id, page_ocr.model, page_ocr.txt, page_ocr.data
		FROM page_ocr JOIN book_page ON book_page.id = page_ocr.page_id
		WHERE book_page.book_id = $1 AND page_ocr.model = ANY($2)
		ORDER BY page_ocr.page_id, array_position($2, page_ocr.model)`, bookID, models)
	if err != nil {
		return nil, err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[PageOCR])
	if err != nil {
		return nil, err
	}

	byPage := make(map[int64]PageOCR, len(results))
	for _, result := range results {
		byPage[result.PageID] = result
	}
	return byPage, nil
}

// InsertPageID stores a page and answers its id, which its results reference.
func (s *Store) InsertPageID(ctx context.Context, page Page) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO book_page
		(book_id, page_index, storage_key, original_ext, width, height, source_page_id,
		cut_side, cut_column, image_book_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		page.BookID, page.PageIndex, page.StorageKey, page.OriginalExt, page.Width, page.Height,
		page.SourcePageID, page.CutSide, page.CutColumn, page.ImageBookKey).Scan(&id)
	return id, err
}

func (s *Store) InsertPage(ctx context.Context, page Page) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO book_page
		(book_id, page_index, storage_key, original_ext, width, height, source_page_id,
		cut_side, cut_column, image_book_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		page.BookID, page.PageIndex, page.StorageKey, page.OriginalExt, page.Width, page.Height,
		page.SourcePageID, page.CutSide, page.CutColumn, page.ImageBookKey)
	return err
}

func (s *Store) list(ctx context.Context, sql string, args ...any) ([]Book, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Book])
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []Book{}
	}
	return list, nil
}

func (s *Store) write(ctx context.Context, statement string, args ...any) (Book, error) {
	rows, err := s.pool.Query(ctx, writeBooks(statement), args...)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

func (s *Store) writeTx(ctx context.Context, tx pgx.Tx, statement string, args ...any) (Book, error) {
	rows, err := tx.Query(ctx, writeBooks(statement), args...)
	if err != nil {
		return Book{}, err
	}
	return collectOne(rows)
}

func collectOne(rows pgx.Rows) (Book, error) {
	book, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[Book])
	if errors.Is(err, pgx.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return book, err
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }
