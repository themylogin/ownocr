package ingest

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPageCount(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"testdata/book10.pdf", 10},
		{"testdata/book1.pdf", 1},
		{"testdata/second.pdf", 2},
		{"testdata/book60.pdf", 60},
	}

	for _, c := range cases {
		got, err := PageCount(context.Background(), c.path)
		if err != nil {
			t.Errorf("PageCount(%s): %v", c.path, err)
			continue
		}
		if got != c.want {
			t.Errorf("PageCount(%s) = %d, want %d", c.path, got, c.want)
		}
	}
}

func TestPageCountRejectsNonPDF(t *testing.T) {
	for _, path := range []string{"testdata/notes.txt", "testdata/truncated.pdf", "testdata/landscape/page1.jpg"} {
		if _, err := PageCount(context.Background(), path); err == nil {
			t.Errorf("PageCount accepted %s", path)
		}
	}
}

func TestRenderPage(t *testing.T) {
	dir := t.TempDir()

	path, err := RenderPage(context.Background(), "testdata/book10.pdf", 1, 300, filepath.Join(dir, "page"))
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}

	if w, h := dimensions(t, path); w != 2000 || h != 3000 {
		t.Errorf("rendered page = %dx%d, want 2000x3000", w, h)
	}
}

func TestRenderPageHonoursDPI(t *testing.T) {
	dir := t.TempDir()

	path, err := RenderPage(context.Background(), "testdata/book10.pdf", 2, 150, filepath.Join(dir, "page"))
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}

	if w, h := dimensions(t, path); w != 1000 || h != 1500 {
		t.Errorf("rendered page = %dx%d, want 1000x1500", w, h)
	}
}

func TestRenderPageRejectsMissingPage(t *testing.T) {
	dir := t.TempDir()

	if _, err := RenderPage(context.Background(), "testdata/book1.pdf", 5, 72, filepath.Join(dir, "page")); err == nil {
		t.Fatal("RenderPage accepted a page beyond the document")
	}
}

func TestRenderPageStopsOnCancel(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if _, err := RenderPage(ctx, "testdata/book60.pdf", 1, 300, filepath.Join(dir, "page")); err == nil {
		t.Fatal("RenderPage ignored a cancelled context")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("RenderPage took %s to notice cancellation", elapsed)
	}
}
