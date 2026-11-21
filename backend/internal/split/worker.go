package split

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"time"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

const (
	workTimeout = 6 * time.Hour
	// One batch holds this many pages. The sidecar decodes them one at a time,
	// and the copy reports its page count after every batch.
	batchSize = 20
)

type Worker struct {
	river.WorkerDefaults[Args]

	store   *books.Store
	files   *filestore.Store
	hub     *ws.Hub
	sidecar *Client
}

func NewWorker(store *books.Store, files *filestore.Store, hub *ws.Hub, sidecar *Client) *Worker {
	return &Worker{store: store, files: files, hub: hub, sidecar: sidecar}
}

func (w *Worker) Timeout(*river.Job[Args]) time.Duration {
	return workTimeout
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	copyBook, source, sourcePages, err := w.load(ctx, job.Args.BookID)
	if errors.Is(err, books.ErrNotFound) {
		// The book was rejected while the job waited. There is nothing to do.
		return nil
	}
	if err != nil {
		return w.reportFailure(ctx, job, err)
	}

	processing, err := w.store.SetProcessing(ctx, copyBook.ID, books.PhaseConverting)
	if err != nil {
		return err
	}
	w.publish(ctx, processing)

	written, err := w.rebuild(ctx, copyBook, source, sourcePages)
	if err != nil {
		return w.reportFailure(ctx, job, err)
	}

	complete, err := w.store.SetComplete(ctx, copyBook.ID, written)
	if err != nil {
		return err
	}
	w.publishFinal(ctx, complete)

	return nil
}

func (w *Worker) load(ctx context.Context, bookID int64) (books.Book, books.Book, []books.Page, error) {
	copyBook, err := w.store.Get(ctx, bookID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}
	if copyBook.SourceBookID == nil {
		return books.Book{}, books.Book{}, nil, errors.New("the book has no source book to split")
	}

	source, err := w.store.Get(ctx, *copyBook.SourceBookID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}

	pages, err := w.store.Pages(ctx, source.ID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}
	if len(pages) == 0 {
		return books.Book{}, books.Book{}, nil, errors.New("the source book has no pages")
	}

	return copyBook, source, pages, nil
}

// rebuild discards whatever an interrupted attempt left behind, then builds the
// whole copy again. A retry therefore cannot produce a duplicate page index or
// leave a stale file.
// The copy counts its own page indexes, because one source page makes several
// pages. It cannot borrow the index of its source page: two pages would claim
// one index, and the unique index on the book and the page index rejects that.
func (w *Worker) rebuild(ctx context.Context, copyBook, source books.Book,
	sourcePages []books.Page) (int, error) {
	if err := w.store.DeletePages(ctx, copyBook.ID); err != nil {
		return 0, err
	}

	pageDir := w.files.PageDir(copyBook.StorageKey)
	if err := os.RemoveAll(pageDir); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return 0, err
	}

	written := 0
	for start := 0; start < len(sourcePages); start += batchSize {
		end := min(start+batchSize, len(sourcePages))
		batch := sourcePages[start:end]

		originals := make([]string, len(batch))
		for index, page := range batch {
			originals[index] = w.files.PagePath(source.StorageKey, page.StorageKey,
				filestore.RenditionOriginal, page.OriginalExt)
		}

		cuts, err := w.sidecar.Cuts(ctx, originals)
		if err != nil {
			return 0, err
		}

		for index, page := range batch {
			count, err := w.storePages(ctx, copyBook, originals[index], pageDir, page,
				cuts[index], written)
			if err != nil {
				return 0, err
			}
			written += count
		}
		w.reportProgress(ctx, copyBook, written)
	}

	return written, nil
}

// A page the sidecar reports as no spread reaches the copy whole, through the
// writer that copies the source bytes. A spread is decoded once and cut twice.
func (w *Worker) storePages(ctx context.Context, copyBook books.Book, original, pageDir string,
	source books.Page, cut *Cut, written int) (int, error) {
	if cut == nil {
		return 1, w.storeWholePage(ctx, copyBook, original, pageDir, source, written+1)
	}

	img, err := ingest.Decode(original)
	if err != nil {
		return 0, fmt.Errorf("page %d: %w", source.PageIndex, err)
	}

	bounds := img.Bounds()
	halves := []struct {
		rect   image.Rectangle
		side   string
		column int
	}{
		{image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+cut.Left, bounds.Max.Y),
			books.CutLeft, cut.Left},
		{image.Rect(bounds.Min.X+cut.Right, bounds.Min.Y, bounds.Max.X, bounds.Max.Y),
			books.CutRight, cut.Right},
	}

	for index, half := range halves {
		if half.rect.Dx() <= 0 {
			return 0, fmt.Errorf("page %d: the sidecar named an empty half at %d",
				source.PageIndex, index)
		}
		if err := w.storeCutPage(ctx, copyBook, pageDir, source, ingest.Crop(img, half.rect),
			written+index+1, half.side, half.column); err != nil {
			return 0, err
		}
	}

	return len(halves), nil
}

// The copy gets its own storage key and its own three renditions, so removing
// the source book cannot take the pages of the copy with it.
func (w *Worker) storeWholePage(ctx context.Context, copyBook books.Book, original, pageDir string,
	source books.Page, index int) error {
	pageKey, err := books.NewStorageKey()
	if err != nil {
		return err
	}

	ext, width, height, err := ingest.WriteRenditions(original, pageDir, pageKey)
	if err != nil {
		return fmt.Errorf("page %d: %w", source.PageIndex, err)
	}

	return w.insertPage(ctx, copyBook, source, pageKey, ext, width, height, index, nil, nil)
}

// A cut page records the side of the gutter it holds and the column it was cut
// at, which is what lets a user move that cut afterwards.
func (w *Worker) storeCutPage(ctx context.Context, copyBook books.Book, pageDir string,
	source books.Page, half image.Image, index int, side string, column int) error {
	pageKey, err := books.NewStorageKey()
	if err != nil {
		return err
	}

	ext, width, height, err := ingest.WriteImageRenditions(half, pageDir, pageKey)
	if err != nil {
		return fmt.Errorf("page %d: %w", source.PageIndex, err)
	}

	return w.insertPage(ctx, copyBook, source, pageKey, ext, width, height, index, &side, &column)
}

func (w *Worker) insertPage(ctx context.Context, copyBook books.Book, source books.Page,
	pageKey, ext string, width, height, index int, side *string, column *int) error {
	return w.store.InsertPage(ctx, books.Page{
		BookID:       copyBook.ID,
		PageIndex:    index,
		StorageKey:   pageKey,
		OriginalExt:  ext,
		Width:        width,
		Height:       height,
		SourcePageID: &source.ID,
		CutSide:      side,
		CutColumn:    column,
	})
}

// The worker knows how many pages it has written, not how many it will write:
// a source page makes one page or two. SetComplete writes the final count, so
// this reports after every batch.
func (w *Worker) reportProgress(ctx context.Context, book books.Book, done int) {
	updated, err := w.store.SetPageCount(ctx, book.ID, done)
	if err != nil {
		return
	}
	w.publish(ctx, updated)
}

// River retries until MaxAttempts. Only the last attempt marks the copy failed,
// so the interface does not flip to "failed" while retries remain.
func (w *Worker) reportFailure(ctx context.Context, job *river.Job[Args], cause error) error {
	if job.Attempt < job.MaxAttempts {
		return cause
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	failed, err := w.store.SetFailed(writeCtx, job.Args.BookID, cause.Error())
	if err != nil {
		return errors.Join(cause, err)
	}
	w.publishFinal(writeCtx, failed)

	return cause
}

func (w *Worker) publish(ctx context.Context, book books.Book) {
	w.hub.Publish("book.updated", books.Event(book, w.store.Cover(ctx, book.ID)))
}

// publishFinal also announces the source book. Its mark follows the state of
// this copy, so a client that shows the source must hear that the copy ended.
func (w *Worker) publishFinal(ctx context.Context, book books.Book) {
	w.publish(ctx, book)

	if source, ok := w.store.SourceOf(ctx, book); ok {
		w.publish(ctx, source)
	}
}
