package ingest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

const (
	workTimeout      = 6 * time.Hour
	progressInterval = 10
)

type Worker struct {
	river.WorkerDefaults[Args]

	store *books.Store
	files *filestore.Store
	hub   *ws.Hub
	dpi   int
}

func NewWorker(store *books.Store, files *filestore.Store, hub *ws.Hub, dpi int) *Worker {
	return &Worker{store: store, files: files, hub: hub, dpi: dpi}
}

func (w *Worker) Timeout(*river.Job[Args]) time.Duration {
	return workTimeout
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	book, err := w.store.Get(ctx, job.Args.BookID)
	if errors.Is(err, books.ErrNotFound) {
		// The book was rejected while the job waited. There is nothing to do.
		return nil
	}
	if err != nil {
		return err
	}

	processing, err := w.store.SetProcessing(ctx, book.ID, books.PhaseConverting)
	if err != nil {
		return err
	}
	w.publish(ctx, processing)

	pageCount, err := w.rebuild(ctx, book)
	if err != nil {
		return w.reportFailure(ctx, job, err)
	}

	complete, err := w.store.SetComplete(ctx, book.ID, pageCount)
	if err != nil {
		return err
	}
	w.publish(ctx, complete)

	return nil
}

// rebuild discards whatever an interrupted attempt left behind, then builds the
// whole book again. A retry therefore cannot produce a duplicate page index or
// leave a stale file.
func (w *Worker) rebuild(ctx context.Context, book books.Book) (int, error) {
	if err := w.store.DeletePages(ctx, book.ID); err != nil {
		return 0, err
	}

	pageDir := w.files.PageDir(book.StorageKey)
	if err := os.RemoveAll(pageDir); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return 0, err
	}

	sources, err := sourceFiles(w.files.SourceDir(book.StorageKey))
	if err != nil {
		return 0, err
	}
	if len(sources) == 0 {
		return 0, errors.New("the upload has no files")
	}

	if len(sources) == 1 && isPDF(sources[0]) {
		return w.ingestPDF(ctx, book, sources[0], pageDir)
	}
	return w.ingestImages(ctx, book, sources, pageDir)
}

func (w *Worker) ingestPDF(ctx context.Context, book books.Book, pdfPath, pageDir string) (int, error) {
	total, err := PageCount(ctx, pdfPath)
	if err != nil {
		return 0, err
	}

	renderDir, err := os.MkdirTemp(w.files.TempRoot(), "render-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(renderDir)

	for number := 1; number <= total; number++ {
		rendered, err := RenderPage(ctx, pdfPath, number, w.dpi, filepath.Join(renderDir, "page"))
		if err != nil {
			return 0, err
		}

		if err := w.storePage(ctx, book, rendered, pageDir, number); err != nil {
			return 0, err
		}
		if err := os.Remove(rendered); err != nil {
			return 0, err
		}

		w.reportProgress(ctx, book, number, total)
	}

	return total, nil
}

func (w *Worker) ingestImages(ctx context.Context, book books.Book, sources []string, pageDir string) (int, error) {
	for index, source := range sources {
		if err := w.storePage(ctx, book, source, pageDir, index+1); err != nil {
			return 0, err
		}
		w.reportProgress(ctx, book, index+1, len(sources))
	}

	return len(sources), nil
}

func (w *Worker) storePage(ctx context.Context, book books.Book, source, pageDir string, index int) error {
	pageKey, err := books.NewStorageKey()
	if err != nil {
		return err
	}

	ext, width, height, err := WriteRenditions(source, pageDir, pageKey)
	if err != nil {
		return fmt.Errorf("page %d (%s): %w", index, uploadedName(source), err)
	}

	return w.store.InsertPage(ctx, books.Page{
		BookID:      book.ID,
		PageIndex:   index,
		StorageKey:  pageKey,
		OriginalExt: ext,
		Width:       width,
		Height:      height,
	})
}

func (w *Worker) reportProgress(ctx context.Context, book books.Book, done, total int) {
	if done%progressInterval != 0 && done != total {
		return
	}

	updated, err := w.store.SetPageCount(ctx, book.ID, done)
	if err != nil {
		return
	}
	w.publish(ctx, updated)
}

// River retries until MaxAttempts. Only the last attempt marks the book failed,
// so the interface does not flip to "failed" while retries remain.
func (w *Worker) reportFailure(ctx context.Context, job *river.Job[Args], cause error) error {
	if job.Attempt < job.MaxAttempts {
		return cause
	}

	// A timeout or a shutdown cancels ctx, and the row still needs the reason.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	failed, err := w.store.SetFailed(writeCtx, job.Args.BookID, cause.Error())
	if err != nil {
		return errors.Join(cause, err)
	}
	w.publish(writeCtx, failed)

	return cause
}

func (w *Worker) publish(ctx context.Context, book books.Book) {
	w.hub.Publish("book.updated", books.Event(book, w.store.Cover(ctx, book.ID)))
}

// sourceFiles lists the upload in the order it arrived. The zero padded
// position prefix that the upload handler wrote makes a lexical sort exact.
func sourceFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths, nil
}

// uploadedName drops the position prefix the upload handler added, so a failure
// names the file the way the user does.
func uploadedName(path string) string {
	base := filepath.Base(path)
	if len(base) > 5 && base[4] == '_' {
		if _, err := strconv.Atoi(base[:4]); err == nil {
			return base[5:]
		}
	}
	return base
}

func isPDF(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 5)
	if _, err := file.Read(header); err != nil {
		return false
	}
	return strings.HasPrefix(string(header), "%PDF-")
}
