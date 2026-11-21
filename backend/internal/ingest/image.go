package ingest

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	"github.com/themylogin/ownocr/backend/internal/filestore"
)

const jpegQuality = 85

func Decode(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// Resize scales img so its longest side equals longestSide. An image already
// within that bound is returned unchanged: a page is never enlarged.
func Resize(img image.Image, longestSide int) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	if width <= longestSide && height <= longestSide {
		return img
	}

	targetWidth, targetHeight := width, height
	if width >= height {
		targetWidth = longestSide
		targetHeight = scaleDimension(height, longestSide, width)
	} else {
		targetHeight = longestSide
		targetWidth = scaleDimension(width, longestSide, height)
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	return dst
}

func scaleDimension(value, target, reference int) int {
	scaled := (value*target + reference/2) / reference
	if scaled < 1 {
		return 1
	}
	return scaled
}

// Crop answers the part of img inside rect. The answer shares the pixels of img
// when the decoder gives a sub image, so a caller must not write into either.
func Crop(img image.Image, rect image.Rectangle) image.Image {
	rect = rect.Intersect(img.Bounds())

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if source, ok := img.(subImager); ok {
		return source.SubImage(rect)
	}

	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Copy(dst, image.Point{}, img, rect, draw.Src, nil)
	return dst
}

// WriteImageRenditions writes the three renditions of an image the caller holds
// in memory. A cut half has no file of its own, so its original is encoded here
// rather than copied.
func WriteImageRenditions(img image.Image, pageDir, pageKey string) (ext string,
	width, height int, err error) {
	bounds := img.Bounds()
	width, height = bounds.Dx(), bounds.Dy()

	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return "", 0, 0, err
	}

	original := filepath.Join(pageDir, pageKey+"."+filestore.RenditionOriginal+".jpg")
	if err := writeJPEG(img, original); err != nil {
		return "", 0, 0, err
	}

	display := filepath.Join(pageDir, pageKey+"."+filestore.RenditionDisplay+".jpg")
	if err := writeJPEG(Resize(img, filestore.DisplaySize), display); err != nil {
		return "", 0, 0, err
	}

	thumb := filepath.Join(pageDir, pageKey+"."+filestore.RenditionThumb+".jpg")
	if err := writeJPEG(Resize(img, filestore.ThumbSize), thumb); err != nil {
		return "", 0, 0, err
	}

	return "jpg", width, height, nil
}

// WriteRenditions decodes the source once and writes the original, the display
// rendition, and the thumbnail into the book's page directory.
func WriteRenditions(sourcePath, pageDir, pageKey string) (ext string, width, height int, err error) {
	img, err := Decode(sourcePath)
	if err != nil {
		return "", 0, 0, err
	}

	bounds := img.Bounds()
	width, height = bounds.Dx(), bounds.Dy()

	ext = originalExt(sourcePath)
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return "", 0, 0, err
	}

	original := filepath.Join(pageDir, pageKey+"."+filestore.RenditionOriginal+"."+ext)
	if err := copyFile(sourcePath, original); err != nil {
		return "", 0, 0, err
	}

	display := filepath.Join(pageDir, pageKey+"."+filestore.RenditionDisplay+".jpg")
	if err := writeJPEG(Resize(img, filestore.DisplaySize), display); err != nil {
		return "", 0, 0, err
	}

	thumb := filepath.Join(pageDir, pageKey+"."+filestore.RenditionThumb+".jpg")
	if err := writeJPEG(Resize(img, filestore.ThumbSize), thumb); err != nil {
		return "", 0, 0, err
	}

	return ext, width, height, nil
}

func originalExt(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "jpeg" {
		return "jpg"
	}
	if ext == "" {
		return "jpg"
	}
	return ext
}

func writeJPEG(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return err
	}
	return file.Close()
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
