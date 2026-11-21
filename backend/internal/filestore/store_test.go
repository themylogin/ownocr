package filestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCreatesRoots(t *testing.T) {
	root := t.TempDir()

	store, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, dir := range []string{store.PagesRoot(), store.SourcesRoot(), store.TempRoot()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestPaths(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got, want := store.PageDir("bk"), filepath.Join(store.PagesRoot(), "bk"); got != want {
		t.Errorf("PageDir = %q, want %q", got, want)
	}
	if got, want := store.SourceDir("bk"), filepath.Join(store.SourcesRoot(), "bk"); got != want {
		t.Errorf("SourceDir = %q, want %q", got, want)
	}

	got := store.PagePath("bk", "pg", RenditionThumb, "jpg")
	want := filepath.Join(store.PagesRoot(), "bk", "pg.480.jpg")
	if got != want {
		t.Errorf("PagePath = %q, want %q", got, want)
	}
}

func TestPageURL(t *testing.T) {
	cases := []struct {
		rendition string
		ext       string
		want      string
	}{
		{RenditionOriginal, "jpg", "/files/bk/pg.orig.jpg"},
		{RenditionDisplay, "jpg", "/files/bk/pg.1280.jpg"},
		{RenditionThumb, "jpg", "/files/bk/pg.480.jpg"},
		{RenditionOriginal, "png", "/files/bk/pg.orig.png"},
	}

	for _, c := range cases {
		if got := PageURL("bk", "pg", c.rendition, c.ext); got != c.want {
			t.Errorf("PageURL(%q, %q) = %q, want %q", c.rendition, c.ext, got, c.want)
		}
	}
}

func TestNewTempDir(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	dir, err := store.NewTempDir("upload1")
	if err != nil {
		t.Fatalf("NewTempDir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("temp dir missing: %v", err)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"scan.jpg", "scan.jpg"},
		{"../../etc/passwd", "passwd"},
		{"..\\..\\windows\\system32", "system32"},
		{"/absolute/path.pdf", "path.pdf"},
		{"..", "file"},
		{"", "file"},
		{"   ", "file"},
		{"a\x00b.jpg", "ab.jpg"},
		{"line\nbreak.jpg", "linebreak.jpg"},
		{"Moby Dick (1851).pdf", "Moby Dick (1851).pdf"},
	}

	for _, c := range cases {
		if got := SanitizeName(c.in); got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeNameCapsLength(t *testing.T) {
	long := strings.Repeat("a", 300) + ".jpg"

	got := SanitizeName(long)
	if len(got) > maxNameBytes {
		t.Errorf("length = %d, want at most %d", len(got), maxNameBytes)
	}

	atCap := strings.Repeat("b", maxNameBytes)
	if got := SanitizeName(atCap); got != atCap {
		t.Errorf("a name at the cap was altered: %d bytes", len(got))
	}
}

func TestSanitizeNameNeverEscapes(t *testing.T) {
	for _, name := range []string{"../../etc/passwd", "..", "/", "\\", "a/../../b"} {
		got := SanitizeName(name)
		if strings.Contains(got, "/") || strings.Contains(got, "\\") || got == ".." {
			t.Errorf("SanitizeName(%q) = %q still traverses", name, got)
		}
	}
}
