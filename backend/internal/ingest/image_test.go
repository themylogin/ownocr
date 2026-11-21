package ingest

import (
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/filestore"
)

func dimensions(t *testing.T, path string) (int, int) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode config %s: %v", path, err)
	}
	return config.Width, config.Height
}

func TestDecodeJPEG(t *testing.T) {
	img, err := Decode("testdata/landscape/page1.jpg")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 3000 {
		t.Errorf("width = %d, want 3000", got)
	}
	if got := img.Bounds().Dy(); got != 2000 {
		t.Errorf("height = %d, want 2000", got)
	}
}

func TestDecodeRejectsCorrupt(t *testing.T) {
	if _, err := Decode("testdata/corrupt.jpg"); err == nil {
		t.Fatal("Decode accepted a corrupt JPEG")
	}
}

func TestDecodeRejectsNonImage(t *testing.T) {
	if _, err := Decode("testdata/notes.txt"); err == nil {
		t.Fatal("Decode accepted a text file")
	}
}

func TestResize(t *testing.T) {
	cases := []struct {
		name                  string
		width, height         int
		target                int
		wantWidth, wantHeight int
	}{
		{"landscape to thumb", 3000, 2000, filestore.ThumbSize, 480, 320},
		{"landscape to display", 3000, 2000, filestore.DisplaySize, 1280, 853},
		{"portrait to thumb", 2000, 3000, filestore.ThumbSize, 320, 480},
		{"portrait to display", 2000, 3000, filestore.DisplaySize, 853, 1280},
		{"square to thumb", 1000, 1000, filestore.ThumbSize, 480, 480},
		{"below thumb stays", 300, 200, filestore.ThumbSize, 300, 200},
		{"below display stays", 900, 600, filestore.DisplaySize, 900, 600},
		{"extreme to thumb", 3000, 500, filestore.ThumbSize, 480, 80},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := image.NewRGBA(image.Rect(0, 0, c.width, c.height))

			got := Resize(source, c.target)

			if got.Bounds().Dx() != c.wantWidth || got.Bounds().Dy() != c.wantHeight {
				t.Errorf("Resize = %dx%d, want %dx%d",
					got.Bounds().Dx(), got.Bounds().Dy(), c.wantWidth, c.wantHeight)
			}
		})
	}
}

func TestResizeNeverUpscales(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 300, 200))

	got := Resize(source, filestore.DisplaySize)

	if got != image.Image(source) {
		t.Error("Resize copied an image that was already small enough")
	}
}

func TestWriteRenditions(t *testing.T) {
	dir := t.TempDir()

	ext, width, height, err := WriteRenditions("testdata/landscape/page1.jpg", dir, "pagekey")
	if err != nil {
		t.Fatalf("WriteRenditions: %v", err)
	}

	if ext != "jpg" {
		t.Errorf("ext = %q, want jpg", ext)
	}
	if width != 3000 || height != 2000 {
		t.Errorf("dimensions = %dx%d, want 3000x2000", width, height)
	}

	original := filepath.Join(dir, "pagekey.orig.jpg")
	display := filepath.Join(dir, "pagekey.1280.jpg")
	thumb := filepath.Join(dir, "pagekey.480.jpg")

	if w, h := dimensions(t, display); w != 1280 || h != 853 {
		t.Errorf("display = %dx%d, want 1280x853", w, h)
	}
	if w, h := dimensions(t, thumb); w != 480 || h != 320 {
		t.Errorf("thumb = %dx%d, want 480x320", w, h)
	}

	source, err := os.ReadFile("testdata/landscape/page1.jpg")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	stored, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if string(source) != string(stored) {
		t.Error("the stored original does not match the uploaded bytes")
	}
}

func TestWriteRenditionsDoesNotUpscale(t *testing.T) {
	dir := t.TempDir()

	_, width, height, err := WriteRenditions("testdata/small/page1.jpg", dir, "tiny")
	if err != nil {
		t.Fatalf("WriteRenditions: %v", err)
	}
	if width != 300 || height != 200 {
		t.Fatalf("dimensions = %dx%d, want 300x200", width, height)
	}

	for _, name := range []string{"tiny.orig.jpg", "tiny.1280.jpg", "tiny.480.jpg"} {
		if w, h := dimensions(t, filepath.Join(dir, name)); w != 300 || h != 200 {
			t.Errorf("%s = %dx%d, want 300x200", name, w, h)
		}
	}
}

func TestWriteRenditionsDisplayGap(t *testing.T) {
	dir := t.TempDir()

	if _, _, _, err := WriteRenditions("testdata/display-gap/page1.jpg", dir, "gap"); err != nil {
		t.Fatalf("WriteRenditions: %v", err)
	}

	if w, h := dimensions(t, filepath.Join(dir, "gap.1280.jpg")); w != 900 || h != 600 {
		t.Errorf("display = %dx%d, want 900x600", w, h)
	}
	if w, h := dimensions(t, filepath.Join(dir, "gap.480.jpg")); w != 480 || h != 320 {
		t.Errorf("thumb = %dx%d, want 480x320", w, h)
	}
}
