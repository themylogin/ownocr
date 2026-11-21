package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/testdb"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

type harness struct {
	store *books.Store
	files *filestore.Store
	work  *Worker
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	pool := testdb.New(t)

	files, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("filestore: %v", err)
	}

	store := books.NewStore(pool)
	hub := ws.NewHub([]string{"*"})

	return &harness{store: store, files: files, work: NewWorker(store, files, hub, 150)}
}

// seed creates a book row and lays its sources out the way the upload handler
// does, with a zero padded position prefix per file.
func (h *harness) seed(t *testing.T, name string, sources ...string) books.Book {
	t.Helper()
	ctx := context.Background()

	key, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	book, err := h.store.CreateTx(ctx, tx, name, key)
	if err != nil {
		t.Fatalf("CreateTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	dir := h.files.SourceDir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("source dir: %v", err)
	}
	for i, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		target := filepath.Join(dir, fmt.Sprintf("%04d_%s", i+1, filepath.Base(source)))
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", target, err)
		}
	}

	return book
}

func (h *harness) run(t *testing.T, bookID int64, attempt, maxAttempts int) error {
	t.Helper()

	job := &river.Job[Args]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts},
		Args:   Args{BookID: bookID},
	}
	return h.work.Work(context.Background(), job)
}

func TestWorkerIngestsPDFInOrder(t *testing.T) {
	h := newHarness(t)
	book := h.seed(t, "Ten Pager", "testdata/book10.pdf")

	if err := h.run(t, book.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	ctx := context.Background()
	updated, err := h.store.Get(ctx, book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.State != books.StateComplete {
		t.Errorf("State = %q, want complete", updated.State)
	}
	if updated.Stage != books.StageUploaded {
		t.Errorf("Stage = %q, want %q", updated.Stage, books.StageUploaded)
	}
	if updated.Progress != nil {
		t.Errorf("a complete book kept the phase %q", *updated.Progress)
	}
	if updated.PageCount != 10 {
		t.Errorf("PageCount = %d, want 10", updated.PageCount)
	}

	pages, err := h.store.Pages(ctx, book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 10 {
		t.Fatalf("got %d pages, want 10", len(pages))
	}
	for i, page := range pages {
		if page.PageIndex != i+1 {
			t.Fatalf("page %d has index %d", i, page.PageIndex)
		}
		if page.Width != 1000 || page.Height != 1500 {
			t.Errorf("page %d = %dx%d, want 1000x1500", page.PageIndex, page.Width, page.Height)
		}
	}

	entries, err := os.ReadDir(h.files.PageDir(book.StorageKey))
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	if len(entries) != 30 {
		t.Errorf("got %d files, want 30 (10 pages x 3 renditions)", len(entries))
	}
}

func TestWorkerKeepsImageUploadOrder(t *testing.T) {
	h := newHarness(t)
	book := h.seed(t, "Unordered",
		"testdata/unordered/b.jpg", "testdata/unordered/a.jpg", "testdata/unordered/c.jpg")

	if err := h.run(t, book.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	pages, err := h.store.Pages(context.Background(), book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}

	want := []string{"testdata/unordered/b.jpg", "testdata/unordered/a.jpg", "testdata/unordered/c.jpg"}
	for i, page := range pages {
		original := filepath.Join(h.files.PageDir(book.StorageKey),
			page.StorageKey+"."+filestore.RenditionOriginal+"."+page.OriginalExt)

		stored, err := os.ReadFile(original)
		if err != nil {
			t.Fatalf("read original: %v", err)
		}
		source, err := os.ReadFile(want[i])
		if err != nil {
			t.Fatalf("read source: %v", err)
		}
		if string(stored) != string(source) {
			t.Errorf("page %d does not hold the bytes of %s", page.PageIndex, want[i])
		}
	}
}

func TestWorkerRebuildDiscardsPreviousAttempt(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	book := h.seed(t, "Retried", "testdata/second.pdf")

	// Stand in for an attempt that died after three pages.
	for index := 1; index <= 3; index++ {
		err := h.store.InsertPage(ctx, books.Page{
			BookID: book.ID, PageIndex: index, StorageKey: fmt.Sprintf("stale%d", index),
			OriginalExt: "jpg", Width: 1, Height: 1,
		})
		if err != nil {
			t.Fatalf("seed page: %v", err)
		}
	}
	pageDir := h.files.PageDir(book.StorageKey)
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pageDir, "stale1.480.jpg"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("stale file: %v", err)
	}

	if err := h.run(t, book.ID, 2, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	pages, err := h.store.Pages(ctx, book.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}

	seen := map[int]bool{}
	for _, page := range pages {
		if seen[page.PageIndex] {
			t.Fatalf("page index %d appears twice", page.PageIndex)
		}
		seen[page.PageIndex] = true
		if strings.HasPrefix(page.StorageKey, "stale") {
			t.Errorf("a stale page row survived: %s", page.StorageKey)
		}
	}

	entries, err := os.ReadDir(pageDir)
	if err != nil {
		t.Fatalf("read page dir: %v", err)
	}
	if len(entries) != 6 {
		t.Errorf("got %d files, want 6", len(entries))
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "stale") {
			t.Errorf("a stale file survived: %s", entry.Name())
		}
	}
}

func TestWorkerFailsOnLastAttemptOnly(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	book := h.seed(t, "Corrupt", "testdata/corrupt.jpg")

	if err := h.run(t, book.ID, 1, 3); err == nil {
		t.Fatal("Work accepted a corrupt image")
	}

	middle, err := h.store.Get(ctx, book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if middle.State != books.StateProcessing {
		t.Errorf("State after attempt 1 = %q, want processing", middle.State)
	}
	if middle.Progress == nil || *middle.Progress != books.PhaseConverting {
		t.Errorf("Progress = %v, want %q", middle.Progress, books.PhaseConverting)
	}

	if err := h.run(t, book.ID, 3, 3); err == nil {
		t.Fatal("Work accepted a corrupt image on the final attempt")
	}

	final, err := h.store.Get(ctx, book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if final.State != books.StateFailed {
		t.Errorf("State after the final attempt = %q, want failed", final.State)
	}
	if final.Error == nil || !strings.Contains(*final.Error, "corrupt.jpg") {
		t.Errorf("reason = %v, want it to name corrupt.jpg", final.Error)
	}
	if final.Stage != books.StageUploaded {
		t.Errorf("a failed book left its stage: %q", final.Stage)
	}
	if final.Progress != nil {
		t.Errorf("a failed book kept the phase %q", *final.Progress)
	}
}

func TestWorkerFailsOnUnreadablePDF(t *testing.T) {
	h := newHarness(t)
	book := h.seed(t, "Truncated", "testdata/truncated.pdf")

	if err := h.run(t, book.ID, 3, 3); err == nil {
		t.Fatal("Work accepted a truncated PDF")
	}

	final, err := h.store.Get(context.Background(), book.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if final.State != books.StateFailed {
		t.Errorf("State = %q, want failed", final.State)
	}
}

func TestWorkerKeepsTheSource(t *testing.T) {
	h := newHarness(t)
	book := h.seed(t, "Kept", "testdata/second.pdf")

	if err := h.run(t, book.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	entries, err := os.ReadDir(h.files.SourceDir(book.StorageKey))
	if err != nil {
		t.Fatalf("read source dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d source files, want 1", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), "second.pdf") {
		t.Errorf("source file = %s", entries[0].Name())
	}
}
