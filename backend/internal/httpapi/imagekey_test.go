package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/books"
)

// An OCR copy owns no files. Its pages name the book that holds the image, so
// every URL of a page and every cover must follow that key, not the key of the
// copy.
func TestPageURLsFollowTheImageBookKey(t *testing.T) {
	h := newPageHarness(t)
	ctx := context.Background()

	source := h.createBook(t, "Read me")
	first := h.addPage(t, source, 1, nil, 400, 600, 10)
	source = h.complete(t, source, 1)

	read := h.deriveBook(t, source, books.ActionOCR)
	if err := h.store.InsertPage(ctx, books.Page{
		BookID: read.ID, PageIndex: 1, StorageKey: first.StorageKey,
		OriginalExt: first.OriginalExt, Width: first.Width, Height: first.Height,
		SourcePageID: &first.ID, ImageBookKey: &source.StorageKey,
	}); err != nil {
		t.Fatalf("InsertPage: %v", err)
	}
	read = h.complete(t, read, 1)

	detail := h.detail(t, read.ID)
	if len(detail.Pages) != 1 {
		t.Fatalf("the copy has %d pages, want 1", len(detail.Pages))
	}

	page := detail.Pages[0]
	for name, url := range map[string]string{
		"originalUrl":  page.OriginalURL,
		"displayUrl":   page.DisplayURL,
		"thumbnailUrl": page.ThumbnailURL,
	} {
		if !strings.HasPrefix(url, "/files/"+source.StorageKey+"/") {
			t.Errorf("%s = %q, want it under the source book %q", name, url, source.StorageKey)
		}
		if strings.Contains(url, read.StorageKey) {
			t.Errorf("%s = %q, names the copy that holds no files", name, url)
		}
	}

	if detail.CoverURL == nil ||
		!strings.HasPrefix(*detail.CoverURL, "/files/"+source.StorageKey+"/") {
		t.Errorf("coverUrl = %v, want it under the source book", detail.CoverURL)
	}
}

// A page that owns its files names no image book key, and its URLs stay under
// its own book.
func TestPageURLsKeepTheirOwnBookWithoutAnImageBookKey(t *testing.T) {
	h := newPageHarness(t)

	source := h.createBook(t, "Ordinary")
	h.addPage(t, source, 1, nil, 400, 600, 10)
	source = h.complete(t, source, 1)

	page := h.detail(t, source.ID).Pages[0]
	if !strings.HasPrefix(page.DisplayURL, "/files/"+source.StorageKey+"/") {
		t.Errorf("displayUrl = %q, want it under its own book", page.DisplayURL)
	}
}

// The image of a source page must not change while a copy points at it. Two
// guards hold that: a book with a copy offers no page rollback and no re-cut.
// Both were written for other reasons, and the OCR copy now depends on them.
// See openspec design: "A page of an OCR copy points at the image of its source
// page".
func TestABookWithACopyOffersNoPageRollbackAndNoReCut(t *testing.T) {
	h := newPageHarness(t)
	_, split := h.seedSplit(t)

	before := h.detail(t, split.ID)
	if !before.PageCut {
		t.Fatal("a complete split book with no copy offers no re-cut")
	}

	read := h.deriveBook(t, split, books.ActionOCR)
	h.complete(t, read, 0)

	after := h.detail(t, split.ID)
	if after.PageCut {
		t.Error("a book whose pages a copy reads still offers a re-cut")
	}
	if after.PageRollback {
		t.Error("a book whose pages a copy reads still offers a page rollback")
	}
}
