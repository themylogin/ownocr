package httpapi

import (
	"context"
	"log/slog"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/guttertrim"
)

type bookSummary struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	State        string  `json:"state"`
	Stage        string  `json:"stage"`
	PageCount    int     `json:"pageCount"`
	Error        *string `json:"error"`
	CoverURL     *string `json:"coverUrl"`
	CreatedAt    string  `json:"createdAt"`
	DerivedState *string `json:"derivedState"`
	Progress     *string `json:"progress"`
	Action       *string `json:"action"`
	OCRProfile   *string `json:"ocrProfile"`
	SourceBookID *int64  `json:"sourceBookId"`
}

type pageView struct {
	Index        int     `json:"index"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	OriginalURL  string  `json:"originalUrl"`
	DisplayURL   string  `json:"displayUrl"`
	ThumbnailURL string  `json:"thumbnailUrl"`
	SourcePageID *int64  `json:"sourcePageId"`
	RolledBack   bool    `json:"rolledBack"`
	CutSide      *string `json:"cutSide"`
}

// ocrLineView is one read line. The box is left, top, right and bottom, in the
// pixels of the stored page image, which are the width and the height the page
// view reports.
type ocrLineView struct {
	Box  [4]int `json:"box"`
	Text string `json:"text"`

	// FromLineReading marks a line of a merged reading whose text the merge took
	// from the line engine, because the reading engine and the line engine did not
	// read that line alike. It is absent for every line of an engine result, which
	// holds that engine's own text throughout.
	FromLineReading bool `json:"fromLineReading,omitempty"`
}

// Trim is the strip of the page that holds the ink of the page beside it, which
// a renderer covers. It is null for a page that carries none. The width and the
// height are the stored size of the page either way: a trim covers pixels and
// moves none, so every box counts in the same space it always did.
type pageOCRView struct {
	Model  string           `json:"model"`
	Width  int              `json:"width"`
	Height int              `json:"height"`
	Trim   *guttertrim.Trim `json:"trim"`
	Lines  []ocrLineView    `json:"lines"`

	// Readings are the readings this page can answer, best first, each named by
	// the profile that made it. A reading a renderer cannot draw is not among
	// them, because it can neither answer nor be chosen.
	Readings []readingView `json:"readings"`

	// Disagrees marks a page whose reading parts company with its line reading,
	// and Disagreement says why, in words and with the numbers that decided it.
	// Only the whole book answer reports them: the count belongs to the book, and
	// one reporting site cannot drift from another.
	Disagrees    bool     `json:"disagrees"`
	Disagreement []string `json:"disagreement,omitempty"`
}

// readingView is one reading a reader can choose. Profile is what the interface
// shows — "Normal", "Extra", "Ultra" — and Model is what a request names.
type readingView struct {
	Model   string `json:"model"`
	Profile string `json:"profile"`
	Engine  string `json:"engine"`
	Answers bool   `json:"answers"`
}

// bookOCRPageView embeds the page answer, so one entry of the book answer and
// the answer for that page alone hold the same fields under the same names.
type bookOCRPageView struct {
	Index int `json:"index"`
	pageOCRView
}

type bookOCRView struct {
	Pages []bookOCRPageView `json:"pages"`

	// Disagreeing counts the pages of the book that disagree. A book whose pages
	// hold one reading each answers zero: one engine cannot disagree with itself.
	Disagreeing int `json:"disagreeing"`
}

type bookDetail struct {
	bookSummary
	GridCell     books.GridCell `json:"gridCell"`
	PageRollback bool           `json:"pageRollback"`
	PageCut      bool           `json:"pageCut"`
	PageRescan   bool           `json:"pageRescan"`
	Pages        []pageView     `json:"pages"`
}

func pageViewOf(book books.Book, page books.Page) pageView {
	imageKey := page.ImageKey(book.StorageKey)
	return pageView{
		Index:        page.PageIndex,
		Width:        page.Width,
		Height:       page.Height,
		OriginalURL:  filestore.PageURL(imageKey, page.StorageKey, filestore.RenditionOriginal, page.OriginalExt),
		DisplayURL:   filestore.PageURL(imageKey, page.StorageKey, filestore.RenditionDisplay, "jpg"),
		ThumbnailURL: filestore.PageURL(imageKey, page.StorageKey, filestore.RenditionThumb, "jpg"),
		SourcePageID: page.SourcePageID,
		RolledBack:   page.RolledBack,
		CutSide:      page.CutSide,
	}
}

// offersPageRollback answers whether the pages of this book can return to the
// pages they were made from. The client reads this alone, so it needs no
// knowledge of the action list.
func offersPageRollback(book books.Book) bool {
	if book.Action == nil || book.State != books.StateComplete || book.DerivedState != nil {
		return false
	}

	action, known := books.ActionByID(*book.Action)
	return known && action.OffersRollback
}

// offersPageCut answers whether the pages of this book can be cut out of their
// source pages again. A page adds its own condition: it must record a cut.
func offersPageCut(book books.Book) bool {
	return book.State == books.StateComplete && book.DerivedState == nil
}

// offersPageRescan answers whether the pages of this book can be read again by
// the visual engine. A page adds its own condition: that engine must not have
// read it already.
func offersPageRescan(book books.Book) bool {
	return book.Stage == books.StageOCR && book.State == books.StateComplete
}

func summaryOf(book books.Book, cover *books.Page) bookSummary {
	view := bookSummary{
		ID:           book.ID,
		Name:         book.Name,
		State:        book.State,
		Stage:        book.Stage,
		PageCount:    book.PageCount,
		Error:        book.Error,
		CreatedAt:    book.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00"),
		DerivedState: book.DerivedState,
		Progress:     book.Progress,
		Action:       book.Action,
		OCRProfile:   book.OCRProfile,
		SourceBookID: book.SourceBookID,
	}

	if cover != nil {
		url := filestore.PageURL(cover.ImageKey(book.StorageKey), cover.StorageKey,
			filestore.RenditionThumb, "jpg")
		view.CoverURL = &url
	}

	return view
}

func (a *api) summary(ctx context.Context, book books.Book) bookSummary {
	covers, err := a.deps.Books.FirstPages(ctx, []int64{book.ID})
	if err != nil {
		slog.Error("load cover", "book", book.ID, "error", err)
		return summaryOf(book, nil)
	}

	if cover, ok := covers[book.ID]; ok {
		return summaryOf(book, &cover)
	}
	return summaryOf(book, nil)
}

func (a *api) summaries(ctx context.Context, list []books.Book) ([]bookSummary, error) {
	ids := make([]int64, 0, len(list))
	for _, book := range list {
		ids = append(ids, book.ID)
	}

	covers, err := a.deps.Books.FirstPages(ctx, ids)
	if err != nil {
		return nil, err
	}

	views := make([]bookSummary, 0, len(list))
	for _, book := range list {
		if cover, ok := covers[book.ID]; ok {
			views = append(views, summaryOf(book, &cover))
			continue
		}
		views = append(views, summaryOf(book, nil))
	}

	return views, nil
}

func detailOf(book books.Book, pages []books.Page) bookDetail {
	views := make([]pageView, 0, len(pages))
	for _, page := range pages {
		views = append(views, pageViewOf(book, page))
	}

	var cover *books.Page
	if len(pages) > 0 {
		cover = &pages[0]
	}

	return bookDetail{
		bookSummary:  summaryOf(book, cover),
		GridCell:     books.ComputeGridCell(pages),
		PageRollback: offersPageRollback(book),
		PageCut:      offersPageCut(book),
		PageRescan:   offersPageRescan(book),
		Pages:        views,
	}
}
