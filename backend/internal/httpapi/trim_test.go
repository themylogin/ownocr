package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/ocr"
)

// trimPageWidth is the width every page of these tests is stored at. The boxes
// below are the boxes of page 45 of book 17 scaled to it, so the numbers of
// design.md hold here too.
const trimPageWidth = 2083

// stripData writes a v6 result of a page carrying a strip: fragments against the
// left edge, a body beginning at 182, a running head and a folio. It is page 45
// of book 17, which the trim must cut at 122.
func stripData() string {
	lines := []string{
		`{"bbox": [0, 273, 55, 331], "text": "ar-", "score": 0.4}`,
		`{"bbox": [0, 343, 58, 406], "text": "on", "score": 0.4}`,
		`{"bbox": [0, 409, 61, 485], "text": "ón", "score": 0.4}`,
		`{"bbox": [784, 154, 1309, 214], "text": "7. LA DIFÍCIL KATESTRAMMÉNE", "score": 0.9}`,
		`{"bbox": [1738, 142, 1829, 214], "text": "45", "score": 0.9}`,
	}
	for i := 0; i < 20; i++ {
		top := 285 + i*76
		lines = append(lines, fmt.Sprintf(
			`{"bbox": [182, %d, 1805, %d], "text": "a line of the page itself", "score": 0.9}`,
			top, top+62))
	}
	// A one word last line of a paragraph, which begins at the margin and ends
	// short. It must survive.
	lines = append(lines,
		`{"bbox": [182, 2000, 430, 2062], "text": "inocente.", "score": 0.9}`)

	return `{"raw": {}, "lines": [` + strings.Join(lines, ",\n") + `]}`
}

// cleanData writes a v6 result of a page that carries no strip.
func cleanData() string {
	lines := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		top := 285 + i*76
		lines = append(lines, fmt.Sprintf(
			`{"bbox": [182, %d, 1805, %d], "text": "a line of the page itself", "score": 0.9}`,
			top, top+62))
	}
	return `{"raw": {}, "lines": [` + strings.Join(lines, ",\n") + `]}`
}

// seedReadSplit builds a split copy of one spread and an OCR copy of that split,
// and answers the OCR copy with the page of each half. The pages of the OCR copy
// carry no cut side of their own, which is what the lineage walk is for.
func (h *pageHarness) seedReadSplit(t *testing.T) (books.Book, books.PageOfBook, books.PageOfBook) {
	t.Helper()

	source := h.createBook(t, "Spread")
	spread := h.addPage(t, source, 1, nil, 400, 600, 90)
	source = h.complete(t, source, 1)

	split := h.deriveBook(t, source, books.ActionSplit)
	left := h.addCutPage(t, split, 1, &spread.ID, books.CutLeft, 220, trimPageWidth, 60)
	right := h.addCutPage(t, split, 2, &spread.ID, books.CutRight, 180, trimPageWidth, 120)
	split = h.complete(t, split, 2)

	read := h.deriveBook(t, split, books.ActionOCR)
	readLeft := h.addPage(t, read, 1, &left.ID, trimPageWidth, 600, 60)
	readRight := h.addPage(t, read, 2, &right.ID, trimPageWidth, 600, 120)

	return h.complete(t, read, 2), readLeft, readRight
}

func TestPageOCRTrimsTheStripOfTheNeighbouringPage(t *testing.T) {
	h := newPageHarness(t)
	read, _, right := h.seedReadSplit(t)

	h.addResult(t, right.ID, ocr.ModeV6, "the strip and the page", stripData())

	view := h.pageOCRView(t, read.ID, 2)

	if view.Trim == nil {
		t.Fatal("the page reports no trim, want one against the left edge")
	}
	if view.Trim.Side != "left" || view.Trim.Column != 122 {
		t.Errorf("the trim is %d against the %s edge, want 122 against the left edge",
			view.Trim.Column, view.Trim.Side)
	}

	// Three fragments of the 26 lines are dropped.
	if len(view.Lines) != 23 {
		t.Fatalf("the page answered %d lines, want 23", len(view.Lines))
	}
	for _, line := range view.Lines {
		if line.Box[2] <= view.Trim.Column {
			t.Errorf("the answer holds %q, which lies inside the trim", line.Text)
		}
	}

	// The trim covers pixels and moves none.
	if view.Width != trimPageWidth {
		t.Errorf("the page reports the width %d, want %d", view.Width, trimPageWidth)
	}
	if view.Lines[0].Box[0] != 182 && view.Lines[0].Box[0] != 784 && view.Lines[0].Box[0] != 1738 {
		t.Errorf("a kept box begins at %d, want the stored box unchanged", view.Lines[0].Box[0])
	}
}

// A folio, a running head and a one word last line all lie outside the trim, and
// every one of them is shorter than a body line.
func TestPageOCRKeepsTheShortLinesOutsideTheTrim(t *testing.T) {
	h := newPageHarness(t)
	read, _, right := h.seedReadSplit(t)

	h.addResult(t, right.ID, ocr.ModeV6, "the strip and the page", stripData())

	view := h.pageOCRView(t, read.ID, 2)

	want := map[string]bool{
		"7. LA DIFÍCIL KATESTRAMMÉNE": false,
		"45":                          false,
		"inocente.":                   false,
	}
	for _, line := range view.Lines {
		if _, named := want[line.Text]; named {
			want[line.Text] = true
		}
	}
	for text, held := range want {
		if !held {
			t.Errorf("the answer dropped %q, which lies outside the trim", text)
		}
	}
}

func TestPageOCRReportsNoTrimForAPageThatCarriesNoStrip(t *testing.T) {
	h := newPageHarness(t)
	read, left, _ := h.seedReadSplit(t)

	h.addResult(t, left.ID, ocr.ModeV6, "the page alone", cleanData())

	view := h.pageOCRView(t, read.ID, 1)

	if view.Trim != nil {
		t.Errorf("the page reports a trim of %d against the %s edge, want none",
			view.Trim.Column, view.Trim.Side)
	}
	if len(view.Lines) != 20 {
		t.Errorf("the page answered %d lines, want every stored line", len(view.Lines))
	}
}

// A page whose lineage holds no cut side is no half of a spread. Its strip-like
// lines, whatever they are, belong to it.
func TestPageOCRReportsNoTrimForAPageThatIsNoHalfOfASpread(t *testing.T) {
	h := newPageHarness(t)
	read, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, "the strip and the page", stripData())

	view := h.pageOCRView(t, read.ID, 1)

	if view.Trim != nil {
		t.Errorf("a page that was never cut reports a trim of %d", view.Trim.Column)
	}
	if len(view.Lines) != 26 {
		t.Errorf("the page answered %d lines, want every stored line", len(view.Lines))
	}
}

// The reader asks for the whole book in one request, so the book answer is what
// it draws. It must trim exactly as the page answer does.
func TestBookOCRTrimsTheSameWayThePageAnswerDoes(t *testing.T) {
	h := newPageHarness(t)
	read, left, right := h.seedReadSplit(t)

	h.addResult(t, right.ID, ocr.ModeV6, "the strip and the page", stripData())
	h.addResult(t, left.ID, ocr.ModeV6, "the page alone", cleanData())

	book := h.bookOCRView(t, read.ID)
	if len(book.Pages) != 2 {
		t.Fatalf("the book answered %d pages, want 2", len(book.Pages))
	}

	for _, page := range book.Pages {
		alone := h.pageOCRView(t, read.ID, page.Index)

		if (page.Trim == nil) != (alone.Trim == nil) {
			t.Fatalf("page %d: the book reports the trim %v and the page %v",
				page.Index, page.Trim, alone.Trim)
		}
		if page.Trim != nil && *page.Trim != *alone.Trim {
			t.Errorf("page %d: the book trims at %d and the page at %d",
				page.Index, page.Trim.Column, alone.Trim.Column)
		}
		if len(page.Lines) != len(alone.Lines) {
			t.Errorf("page %d: the book answered %d lines and the page %d",
				page.Index, len(page.Lines), len(alone.Lines))
		}
	}

	if book.Pages[1].Trim == nil || book.Pages[1].Trim.Column != 122 {
		t.Errorf("the right half of the book answer reports the trim %v, want 122",
			book.Pages[1].Trim)
	}
	if book.Pages[0].Trim != nil {
		t.Error("the left half of the book answer reports a trim, want none")
	}
}
