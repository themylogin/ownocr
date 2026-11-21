// Command fixturegen writes the deterministic image and PDF fixtures that the
// Go tests and the Playwright suite share. It uses only the standard library:
// JPEGs come from image/jpeg, and each PDF embeds those JPEGs as /DCTDecode
// image XObjects, which needs no PDF library and renders in any viewer.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", "fixtures", "directory to write fixtures into")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "fixturegen:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	sets := []struct {
		dir    string
		width  int
		height int
		count  int
	}{
		{"landscape", 3000, 2000, 3},
		{"portrait", 2000, 3000, 3},
		{"square", 1000, 1000, 2},
		{"extreme", 3000, 500, 3},
		{"small", 300, 200, 1},
		{"display-gap", 900, 600, 1},
	}

	for _, set := range sets {
		dir := filepath.Join(out, set.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		for i := 1; i <= set.count; i++ {
			name := filepath.Join(dir, fmt.Sprintf("page%d.jpg", i))
			if err := os.WriteFile(name, mustJPEG(set.width, set.height, i), 0o644); err != nil {
				return err
			}
		}
	}

	spread := filepath.Join(out, "spread")
	if err := os.MkdirAll(spread, 0o755); err != nil {
		return err
	}
	for i := 1; i <= 3; i++ {
		name := filepath.Join(spread, fmt.Sprintf("page%d.jpg", i))
		if err := os.WriteFile(name, mustSpreadJPEG(1600, 1200, i), 0o644); err != nil {
			return err
		}
	}

	mixed := filepath.Join(out, "mixed")
	if err := os.MkdirAll(mixed, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(mixed, "page1.jpg"), mustJPEG(3000, 2000, 1), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(mixed, "page2.jpg"), mustJPEG(2000, 3000, 2), 0o644); err != nil {
		return err
	}

	unordered := filepath.Join(out, "unordered")
	if err := os.MkdirAll(unordered, 0o755); err != nil {
		return err
	}
	for i, name := range []string{"b.jpg", "a.jpg", "c.jpg"} {
		if err := os.WriteFile(filepath.Join(unordered, name), mustJPEG(1200, 1600, i+1), 0o644); err != nil {
			return err
		}
	}

	// MediaBox is in points, so the rendered pixel size is mediaBox * dpi / 72.
	// 480x720 pt at the default 300 DPI gives a 2000x3000 px page.
	if err := writePDF(filepath.Join(out, "book10.pdf"), 10, 480, 720, 1000, 1500); err != nil {
		return err
	}
	if err := writePDF(filepath.Join(out, "book1.pdf"), 1, 480, 720, 1000, 1500); err != nil {
		return err
	}
	if err := writePDF(filepath.Join(out, "second.pdf"), 2, 480, 720, 1000, 1500); err != nil {
		return err
	}
	// Deliberately small: 60 pages render at 1000x1333 so the suite stays fast.
	if err := writePDF(filepath.Join(out, "book60.pdf"), 60, 240, 320, 500, 667); err != nil {
		return err
	}
	if err := writePDF(filepath.Join(out, "landscape.pdf"), 3, 720, 480, 1500, 1000); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(out, "corrupt.jpg"), corruptJPEG(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "truncated.pdf"), truncatedPDF(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "notes.txt"), []byte("not an image\n"), 0o644); err != nil {
		return err
	}

	return nil
}

// Each page carries a distinct hue and a solid marker band whose position
// encodes the page number, so a test can tell rendered pages apart.
func mustJPEG(width, height, seed int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	base := color.RGBA{R: uint8(40 + seed*17%180), G: uint8(90 + seed*53%140), B: uint8(150 + seed*29%100), A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, base)
		}
	}

	bandTop := (height / 12) * ((seed - 1) % 10)
	bandHeight := height / 14
	if bandHeight < 1 {
		bandHeight = 1
	}
	for y := bandTop; y < bandTop+bandHeight && y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// A synthetic book spread: two blocks of text rows either side of a vertical
// gutter band. The gutter is the only structure that runs the whole height, and
// that is exactly what the splitter looks for. The text rows are short and
// horizontal, so the splitter's vertical opening erases them.
func mustSpreadJPEG(width, height, seed int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	paper := color.RGBA{R: uint8(225 + seed*7%25), G: uint8(224 + seed*11%25), B: uint8(214 + seed*13%30), A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, paper)
		}
	}

	ink := color.RGBA{R: 40, G: 40, B: 45, A: 255}
	pitch := max(height/34, 3)
	thickness := max(pitch/3, 1)
	for _, block := range [][2]int{
		{width * 7 / 100, width * 43 / 100},
		{width * 57 / 100, width * 93 / 100},
	} {
		for y := height / 8; y < height-height/8; y += pitch {
			for dy := 0; dy < thickness && y+dy < height; dy++ {
				for x := block[0]; x < block[1]; x++ {
					img.Set(x, y+dy, ink)
				}
			}
		}
	}

	gutter := color.RGBA{R: 95, G: 92, B: 90, A: 255}
	centre := width / 2
	for y := 0; y < height; y++ {
		for x := max(centre-width/120, 0); x <= min(centre+width/120, width-1); x++ {
			img.Set(x, y, gutter)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func corruptJPEG() []byte {
	valid := mustJPEG(400, 300, 1)
	broken := append([]byte(nil), valid[:len(valid)/2]...)
	for i := 20; i < 60 && i < len(broken); i++ {
		broken[i] = 0xFF
	}
	return broken
}

func truncatedPDF() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n")
}

// writePDF builds a minimal PDF where every page is one full-bleed JPEG. The
// JPEG bytes go in untouched under /DCTDecode, so no encoder is needed here.
func writePDF(path string, pages, mediaWidth, mediaHeight, imageWidth, imageHeight int) error {
	var buf bytes.Buffer
	offsets := []int{0}

	object := func(body []byte) {
		offsets = append(offsets, buf.Len())
		buf.Write(body)
	}

	buf.WriteString("%PDF-1.4\n")

	// Object numbering: 1 catalog, 2 pages tree, then per page a page object,
	// a content stream, and an image XObject.
	kids := &bytes.Buffer{}
	for i := 0; i < pages; i++ {
		fmt.Fprintf(kids, "%d 0 R ", 3+i*3)
	}

	object([]byte("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"))
	object([]byte(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Count %d /Kids [%s] >>\nendobj\n",
		pages, kids.String())))

	for i := 0; i < pages; i++ {
		pageObj := 3 + i*3
		contentObj := pageObj + 1
		imageObj := pageObj + 2

		object([]byte(fmt.Sprintf(
			"%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] "+
				"/Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R >>\nendobj\n",
			pageObj, mediaWidth, mediaHeight, imageObj, contentObj)))

		content := fmt.Sprintf("q %d 0 0 %d 0 0 cm /Im0 Do Q", mediaWidth, mediaHeight)
		object([]byte(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			contentObj, len(content), content)))

		data := mustJPEG(imageWidth, imageHeight, i+1)
		header := fmt.Sprintf(
			"%d 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d "+
				"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
			imageObj, imageWidth, imageHeight, len(data))

		offsets = append(offsets, buf.Len())
		buf.WriteString(header)
		buf.Write(data)
		buf.WriteString("\nendstream\nendobj\n")
	}

	xrefStart := buf.Len()
	count := len(offsets)
	fmt.Fprintf(&buf, "xref\n0 %d\n", count)
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		count, xrefStart)

	return os.WriteFile(path, buf.Bytes(), 0o644)
}
