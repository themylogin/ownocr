package httpapi

import (
	"context"
	"errors"
	"image"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/guttertrim"
	"github.com/themylogin/ownocr/backend/internal/health"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

type pageDeletionView struct {
	Books   []bookSummary `json:"books"`
	Allowed bool          `json:"allowed"`
	Reason  string        `json:"reason"`
}

type comparedPage struct {
	Stage      string `json:"stage"`
	StageTitle string `json:"stageTitle"`
	DisplayURL string `json:"displayUrl"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type pageRollbackView struct {
	Source  comparedPage `json:"source"`
	Current comparedPage `json:"current"`
}

// cutSource carries the page a re-cut cuts out of. The width is the width the
// column counts in, so a client can place the column on the display rendition.
type cutSource struct {
	DisplayURL string `json:"displayUrl"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type pageCutView struct {
	Source cutSource `json:"source"`
	Side   string    `json:"side"`
	Column int       `json:"column"`
}

type pageCutRequest struct {
	Column *int `json:"column"`
}

// pageTarget names the page a request acts on, after the book and the index
// both resolve.
type pageTarget struct {
	book books.Book
	page books.PageOfBook
}

func (a *api) resolvePage(w http.ResponseWriter, r *http.Request) (pageTarget, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such book")
		return pageTarget{}, false
	}

	index, err := strconv.Atoi(chi.URLParam(r, "index"))
	if err != nil {
		writeError(w, http.StatusNotFound, "no such page")
		return pageTarget{}, false
	}

	book, err := a.deps.Books.Get(r.Context(), id)
	if errors.Is(err, books.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such book")
		return pageTarget{}, false
	}
	if err != nil {
		slog.Error("get book", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return pageTarget{}, false
	}

	page, err := a.deps.Books.PageAt(r.Context(), id, index)
	if errors.Is(err, books.ErrPageNotFound) {
		writeError(w, http.StatusNotFound, "no such page")
		return pageTarget{}, false
	}
	if err != nil {
		slog.Error("get page", "book", id, "page", index, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return pageTarget{}, false
	}

	return pageTarget{book: book, page: page}, true
}

// inFlight names the book of a chain that still runs, if one does. A running
// job reads the pages of a source book and writes the pages of a copy, so
// changing either set underneath it would leave the copy disagreeing with its
// source.
func (a *api) inFlight(ctx context.Context, bookID int64) (string, error) {
	chain, err := a.deps.Books.Chain(ctx, bookID)
	if err != nil {
		return "", err
	}

	for _, book := range chain {
		if book.State == books.StatePending || book.State == books.StateProcessing {
			return "the book " + book.Name + " still runs", nil
		}
	}
	return "", nil
}

func (a *api) pageDeletion(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	descendants, err := a.deps.Books.PageDescendants(r.Context(), target.page.ID)
	if err != nil {
		slog.Error("page descendants", "page", target.page.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return
	}

	// The walk runs deepest first, and the confirmation reads down the chain. One
	// book may lose several pages, because one source page can make several. It
	// is still one book, so it is named once.
	affected := make([]books.Book, 0, len(descendants))
	named := map[int64]bool{}
	for index := len(descendants) - 1; index >= 0; index-- {
		bookID := descendants[index].BookID
		if named[bookID] {
			continue
		}
		named[bookID] = true

		book, err := a.deps.Books.Get(r.Context(), bookID)
		if err != nil {
			slog.Error("page deletion book", "book", bookID, "error", err)
			writeError(w, http.StatusInternalServerError, "cannot read the page")
			return
		}
		affected = append(affected, book)
	}

	views, err := a.summaries(r.Context(), affected)
	if err != nil {
		slog.Error("page deletion covers", "page", target.page.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return
	}

	reason, err := a.inFlight(r.Context(), target.book.ID)
	if err != nil {
		slog.Error("page deletion state", "book", target.book.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return
	}

	writeJSON(w, http.StatusOK, pageDeletionView{
		Books:   views,
		Allowed: reason == "",
		Reason:  reason,
	})
}

func (a *api) deletePage(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	removed, err := a.removePage(r.Context(), target)
	if err != nil {
		var invalid invalidUpload
		switch {
		case errors.As(err, &invalid):
			writeError(w, invalid.status, invalid.message)
		case errors.Is(err, books.ErrPageNotFound):
			writeError(w, http.StatusNotFound, "no such page")
		default:
			slog.Error("delete page", "book", target.book.ID, "page", target.page.PageIndex,
				"error", err)
			writeError(w, http.StatusInternalServerError, "cannot delete the page")
		}
		return
	}

	slog.Info("page deleted", "book", target.book.ID, "page", target.page.PageIndex,
		"books", removed)

	for _, bookID := range removed {
		if book, err := a.deps.Books.Get(r.Context(), bookID); err == nil {
			a.deps.Hub.Publish("book.updated", a.summary(r.Context(), book))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"books": removed})
}

// removePage locks the chain, checks it again inside the lock, then removes the
// page and every page made from it. It answers the books it changed.
func (a *api) removePage(ctx context.Context, target pageTarget) ([]int64, error) {
	tx, err := a.deps.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := a.deps.Books.LockChainTx(ctx, tx, target.book.ID); err != nil {
		return nil, err
	}

	reason, err := a.inFlight(ctx, target.book.ID)
	if err != nil {
		return nil, err
	}
	if reason != "" {
		return nil, reject(http.StatusConflict, "%s", reason)
	}

	descendants, err := a.deps.Books.PageDescendants(ctx, target.page.ID)
	if err != nil {
		return nil, err
	}

	// A book that lost several pages is named once, so the caller recounts it
	// once and hears about it once.
	changed := make([]int64, 0, len(descendants))
	named := map[int64]bool{}
	for _, page := range descendants {
		if err := a.deps.Books.DeletePageTx(ctx, tx, page.ID); err != nil {
			return nil, err
		}
		if !named[page.BookID] {
			named[page.BookID] = true
			changed = append(changed, page.BookID)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// The rows go before the files. A file left behind is waste; a row that
	// names missing files is a broken page.
	for _, page := range descendants {
		a.removePageFiles(page)
	}

	for _, bookID := range changed {
		count, err := a.deps.Books.CountPages(ctx, bookID)
		if err != nil {
			slog.Error("count pages", "book", bookID, "error", err)
			continue
		}
		if _, err := a.deps.Books.SetPageCount(ctx, bookID, count); err != nil {
			slog.Error("set page count", "book", bookID, "error", err)
		}
	}

	return changed, nil
}

func (a *api) removePageFiles(page books.PageOfBook) {
	renditions := []struct{ rendition, ext string }{
		{filestore.RenditionOriginal, page.OriginalExt},
		{filestore.RenditionDisplay, "jpg"},
		{filestore.RenditionThumb, "jpg"},
	}

	for _, entry := range renditions {
		path := a.deps.Files.PagePath(page.BookStorageKey, page.StorageKey,
			entry.rendition, entry.ext)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Error("remove page file", "page", page.ID, "path", path, "error", err)
		}
	}
}

// rollbackSource answers the page a rollback would install, and refuses when
// any condition of the flow does not hold.
func (a *api) rollbackSource(ctx context.Context, target pageTarget) (books.PageOfBook, error) {
	if target.book.Action == nil {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"the book took no action to roll back")
	}

	action, known := books.ActionByID(*target.book.Action)
	if !known {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"the book names the unknown action %s", *target.book.Action)
	}
	if !action.OffersRollback {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"a page of a %s copy cannot return to the page it was made from", action.Title)
	}

	if target.book.DerivedState != nil {
		return books.PageOfBook{}, reject(http.StatusConflict,
			"the book already has a copy")
	}

	if target.page.RolledBack {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"the page already shows the original")
	}

	source, ok := a.deps.Books.SourcePage(ctx, target.page)
	if !ok {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"the page it was made from is gone")
	}

	return source, nil
}

func (a *api) pageRollback(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	source, err := a.rollbackSource(r.Context(), target)
	if err != nil {
		writeRejection(w, err, "cannot read the page")
		return
	}

	writeJSON(w, http.StatusOK, pageRollbackView{
		Source:  comparedPageOf(source),
		Current: comparedPageOf(target.page),
	})
}

func (a *api) applyPageRollback(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	page, err := a.rollBack(r.Context(), target)
	if err != nil {
		writeRejection(w, err, "cannot roll the page back")
		return
	}

	slog.Info("page rolled back", "book", target.book.ID, "page", target.page.PageIndex)

	if book, err := a.deps.Books.Get(r.Context(), target.book.ID); err == nil {
		a.deps.Hub.Publish("book.updated", a.summary(r.Context(), book))
	}

	writeJSON(w, http.StatusOK, pageViewOf(target.book, page.Page))
}

// rollBack builds the renditions again from the original bytes of the source
// page, under a storage key of this book. The copy owns its bytes, so losing
// the source book later cannot take the page with it.
func (a *api) rollBack(ctx context.Context, target pageTarget) (books.PageOfBook, error) {
	tx, err := a.deps.Pool.Begin(ctx)
	if err != nil {
		return books.PageOfBook{}, err
	}
	defer tx.Rollback(ctx)

	if err := a.deps.Books.LockChainTx(ctx, tx, target.book.ID); err != nil {
		return books.PageOfBook{}, err
	}

	reason, err := a.inFlight(ctx, target.book.ID)
	if err != nil {
		return books.PageOfBook{}, err
	}
	if reason != "" {
		return books.PageOfBook{}, reject(http.StatusConflict, "%s", reason)
	}

	source, err := a.rollbackSource(ctx, target)
	if err != nil {
		return books.PageOfBook{}, err
	}

	original := a.deps.Files.PagePath(source.BookStorageKey, source.StorageKey,
		filestore.RenditionOriginal, source.OriginalExt)

	pageKey, err := books.NewStorageKey()
	if err != nil {
		return books.PageOfBook{}, err
	}

	pageDir := a.deps.Files.PageDir(target.page.BookStorageKey)
	ext, width, height, err := ingest.WriteRenditions(original, pageDir, pageKey)
	if err != nil {
		return books.PageOfBook{}, err
	}

	replaced, err := a.deps.Books.RollBackPageTx(ctx, tx, target.page.ID, pageKey, ext, width, height)
	if err != nil {
		a.removeRenditions(pageDir, pageKey, ext)
		return books.PageOfBook{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		a.removeRenditions(pageDir, pageKey, ext)
		return books.PageOfBook{}, err
	}

	a.removePageFiles(target.page)

	return replaced, nil
}

// cutTarget answers the page a re-cut cuts out of, and refuses when any
// condition of the flow does not hold.
func (a *api) cutTarget(ctx context.Context, target pageTarget) (books.PageOfBook, error) {
	if target.page.CutSide == nil || target.page.CutColumn == nil {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"the page was not cut out of another page")
	}

	if target.book.DerivedState != nil {
		return books.PageOfBook{}, reject(http.StatusConflict,
			"the book already has a copy")
	}

	source, ok := a.deps.Books.SourcePage(ctx, target.page)
	if !ok {
		return books.PageOfBook{}, reject(http.StatusUnprocessableEntity,
			"the page it was cut out of is gone")
	}

	return source, nil
}

func (a *api) pageCut(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	source, err := a.cutTarget(r.Context(), target)
	if err != nil {
		writeRejection(w, err, "cannot read the page")
		return
	}

	writeJSON(w, http.StatusOK, pageCutView{
		Source: cutSource{
			DisplayURL: filestore.PageURL(source.ImageKey(source.BookStorageKey),
				source.StorageKey, filestore.RenditionDisplay, "jpg"),
			Width:  source.Width,
			Height: source.Height,
		},
		Side:   *target.page.CutSide,
		Column: *target.page.CutColumn,
	})
}

// pageOCR answers the lines a renderer draws over the page: the merged result
// when the page holds one, and the PP-OCRv6 result when it does not. The boxes
// count in the stored page image, which is what the page view reports.
func (a *api) pageOCR(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	result, held, err := a.deps.Books.PageReading(r.Context(), target.page.ID, ocr.RenderModels)
	if err != nil {
		slog.Error("page ocr", "book", target.book.ID, "page", target.page.PageIndex,
			"error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return
	}
	if !held {
		writeError(w, http.StatusNotFound, "this page holds no text")
		return
	}

	lines, err := ocr.LinesOf(result.Data)
	if err != nil {
		slog.Error("page ocr lines", "book", target.book.ID, "page", target.page.PageIndex,
			"model", result.Model, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the text of the page")
		return
	}

	trim, views := a.trimmed(r.Context(), target.page.ID, result.Model,
		target.page.Width, lines)

	readings, err := a.readingsOf(r.Context(), target.page.ID)
	if err != nil {
		slog.Error("page readings", "book", target.book.ID, "page", target.page.PageIndex,
			"error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return
	}

	writeJSON(w, http.StatusOK, pageOCRView{
		Model:    result.Model,
		Width:    target.page.Width,
		Height:   target.page.Height,
		Trim:     trim,
		Lines:    views,
		Readings: readings,
	})
}

// readingsOf answers the readings a page can answer, best first, each named by
// the profile that made it. A reading a renderer cannot draw is left out, so a
// reader is offered nothing it cannot be shown.
func (a *api) readingsOf(ctx context.Context, pageID int64) ([]readingView, error) {
	// The rank and not the order of the menu: the rank is what decides which
	// reading answers, and the menu order is applied below.
	held, err := a.deps.Books.PageReadings(ctx, pageID, ocr.RenderModels)
	if err != nil {
		return nil, err
	}

	byModel := make(map[string]books.Reading, len(held))
	for _, reading := range held {
		byModel[reading.Model] = reading
	}

	readings := make([]readingView, 0, len(held))
	for _, model := range ocr.ReadingOrder() {
		reading, found := byModel[model]
		if !found {
			continue
		}
		profile, engine, drawable := ocr.ReadingNamesOf(reading.Model)
		if !drawable {
			continue
		}
		readings = append(readings, readingView{
			Model:   reading.Model,
			Profile: profile,
			Engine:  engine,
			Answers: reading.Answers,
		})
	}
	return readings, nil
}

// trimmed answers the trim of a page and the lines that survive it. The lines it
// drops hold the ink of the page beside this one, which this page never held.
//
// The cut side comes from the lineage: a page of an OCR copy carries none of its
// own. A page that reaches no cut side gets no trim and keeps every line.
func (a *api) trimmed(ctx context.Context, pageID int64, model string, width int,
	lines []ocr.StoredLine) (*guttertrim.Trim, []ocrLineView) {
	cutSide, ok := a.deps.Books.CutSideOf(ctx, pageID)
	if !ok {
		cutSide = ""
	}
	return trimLines(cutSide, model, width, lines)
}

// trimLines is what both answers share, so the page and the book cannot disagree
// about what a page says. An empty cut side means the page is no half of a
// spread, and every line survives.
func trimLines(cutSide, model string, width int,
	lines []ocr.StoredLine) (*guttertrim.Trim, []ocrLineView) {
	boxes := make([]ocrmerge.Box, len(lines))
	for i, line := range lines {
		boxes[i] = line.Box
	}

	trim, found := guttertrim.Of(boxes, width, cutSide)

	views := make([]ocrLineView, 0, len(lines))
	for _, line := range lines {
		if found && trim.Covers(line.Box, width) {
			continue
		}
		views = append(views, ocrLineView{
			Box:             line.Box,
			Text:            line.Text,
			FromLineReading: ocr.SubstitutedLine(model, line.Source),
		})
	}

	if !found {
		return nil, views
	}
	return &trim, views
}

// bookOCR answers the lines of every page of a book at once. The reader draws
// the whole book, so it asks once instead of once per page. An unread page
// carries no lines rather than failing the request: a copy that is still running
// holds unread pages, and the reader draws the pages it has.
func (a *api) bookOCR(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}

	book, err := a.deps.Books.Get(r.Context(), id)
	if errors.Is(err, books.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such book")
		return
	}
	if err != nil {
		slog.Error("get book", "book", id, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	pages, err := a.deps.Books.Pages(r.Context(), book.ID)
	if err != nil {
		slog.Error("book ocr pages", "book", book.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	results, err := a.deps.Books.BookReadings(r.Context(), book.ID, ocr.RenderModels)
	if err != nil {
		slog.Error("book ocr", "book", book.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	// One query for the whole book. The reader asks for every page at once, so a
	// lineage walk per page would be one query per page.
	cutSides, err := a.deps.Books.CutSidesOfBook(r.Context(), book.ID)
	if err != nil {
		slog.Error("book cut sides", "book", book.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	verdicts, err := a.judge(r.Context(), book.ID, pages, results)
	if err != nil {
		slog.Error("book health", "book", book.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the book")
		return
	}

	views := make([]bookOCRPageView, 0, len(pages))
	for _, page := range pages {
		view := bookOCRPageView{
			Index: page.PageIndex,
			pageOCRView: pageOCRView{
				Width:     page.Width,
				Height:    page.Height,
				Lines:     []ocrLineView{},
				Readings:     []readingView{},
				Disagrees:    verdicts[page.ID].Disagrees,
				Disagreement: verdicts[page.ID].Reasons(),
			},
		}

		if result, held := results[page.ID]; held {
			lines, err := ocr.LinesOf(result.Data)
			if err != nil {
				slog.Error("book ocr lines", "book", book.ID, "page", page.PageIndex,
					"model", result.Model, "error", err)
				writeError(w, http.StatusInternalServerError, "cannot read the text of the book")
				return
			}

			view.Model = result.Model
			view.Trim, view.Lines = trimLines(cutSides[page.ID], result.Model,
				page.Width, lines)

			view.Readings, err = a.readingsOf(r.Context(), page.ID)
			if err != nil {
				slog.Error("book readings", "book", book.ID, "page", page.PageIndex,
					"error", err)
				writeError(w, http.StatusInternalServerError, "cannot read the book")
				return
			}
		}

		views = append(views, view)
	}

	writeJSON(w, http.StatusOK, bookOCRView{
		Pages:       views,
		Disagreeing: health.Disagreeing(verdicts),
	})
}

// judge answers, per page of a book, whether the reading that page answers
// disagrees with its line reading.
//
// It judges the text the engine behind that reading wrote, not the merged reading
// built from it: a merged reading carries line text wherever the reading engine's
// text did not align, so measuring it against the line reading would compare the
// line engine with itself, and a reading engine that abandoned the page would
// pass because the merge already replaced what it abandoned.
func (a *api) judge(ctx context.Context, bookID int64, pages []books.Page,
	answering map[int64]books.PageOCR) (map[int64]health.Verdict, error) {
	lines, err := a.deps.Books.BookOCRIn(ctx, bookID, []string{ocr.ModeV6})
	if err != nil {
		return nil, err
	}

	engineTexts := make(map[string]map[int64]books.PageOCR)
	for _, model := range ocr.RenderModels {
		engine, merged := ocr.TextEngineOf(model)
		if !merged {
			continue
		}
		held, err := a.deps.Books.BookOCRIn(ctx, bookID, []string{engine})
		if err != nil {
			return nil, err
		}
		engineTexts[engine] = held
	}

	toJudge := make(map[int64]health.Page, len(pages))
	for _, page := range pages {
		answered, held := answering[page.ID]
		if !held {
			continue
		}

		text := answered.Text
		if engine, merged := ocr.TextEngineOf(answered.Model); merged {
			raw, wrote := engineTexts[engine][page.ID]
			if !wrote {
				continue
			}
			text = ocr.ReadingText(engine, raw.Text, raw.Data)
		}

		toJudge[page.ID] = health.Page{
			Answered:   text,
			Line:       lines[page.ID].Text,
			SameEngine: answered.Model == ocr.ModeV6,
		}
	}

	return health.Judgements(toJudge), nil
}

func (a *api) applyPageCut(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	var request pageCutRequest
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "the request must hold a column")
		return
	}
	if request.Column == nil {
		writeError(w, http.StatusUnprocessableEntity, "name the column to cut at")
		return
	}

	page, err := a.reCut(r.Context(), target, *request.Column)
	if err != nil {
		writeRejection(w, err, "cannot cut the page again")
		return
	}

	slog.Info("page cut again", "book", target.book.ID, "page", target.page.PageIndex,
		"column", *request.Column)

	if book, err := a.deps.Books.Get(r.Context(), target.book.ID); err == nil {
		a.deps.Hub.Publish("book.updated", a.summary(r.Context(), book))
	}

	writeJSON(w, http.StatusOK, pageViewOf(target.book, page.Page))
}

// reCut cuts the page out of the original bytes of its source page again, under
// a storage key of this book. It reads the source page, never the page it
// replaces, so a user who cut too far can cut back out.
func (a *api) reCut(ctx context.Context, target pageTarget, column int) (books.PageOfBook, error) {
	tx, err := a.deps.Pool.Begin(ctx)
	if err != nil {
		return books.PageOfBook{}, err
	}
	defer tx.Rollback(ctx)

	if err := a.deps.Books.LockChainTx(ctx, tx, target.book.ID); err != nil {
		return books.PageOfBook{}, err
	}

	reason, err := a.inFlight(ctx, target.book.ID)
	if err != nil {
		return books.PageOfBook{}, err
	}
	if reason != "" {
		return books.PageOfBook{}, reject(http.StatusConflict, "%s", reason)
	}

	source, err := a.cutTarget(ctx, target)
	if err != nil {
		return books.PageOfBook{}, err
	}
	if err := checkColumn(*target.page.CutSide, column, source.Width); err != nil {
		return books.PageOfBook{}, err
	}

	original := a.deps.Files.PagePath(source.BookStorageKey, source.StorageKey,
		filestore.RenditionOriginal, source.OriginalExt)

	img, err := ingest.Decode(original)
	if err != nil {
		return books.PageOfBook{}, err
	}

	pageKey, err := books.NewStorageKey()
	if err != nil {
		return books.PageOfBook{}, err
	}

	pageDir := a.deps.Files.PageDir(target.page.BookStorageKey)
	half := ingest.Crop(img, cutRect(*target.page.CutSide, column, img.Bounds()))

	ext, width, height, err := ingest.WriteImageRenditions(half, pageDir, pageKey)
	if err != nil {
		return books.PageOfBook{}, err
	}

	replaced, err := a.deps.Books.ReCutPageTx(ctx, tx, target.page.ID, pageKey, ext,
		width, height, column)
	if err != nil {
		a.removeRenditions(pageDir, pageKey, ext)
		return books.PageOfBook{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		a.removeRenditions(pageDir, pageKey, ext)
		return books.PageOfBook{}, err
	}

	a.removePageFiles(target.page)

	return replaced, nil
}

// checkColumn refuses a column outside the source page, and a column that
// leaves this page with no column of its own.
func checkColumn(side string, column, sourceWidth int) error {
	if column < 0 || column > sourceWidth {
		return reject(http.StatusUnprocessableEntity,
			"the column %d lies outside the page it was cut out of, which is %d wide",
			column, sourceWidth)
	}

	empty := column == 0
	if side == books.CutRight {
		empty = column == sourceWidth
	}
	if empty {
		return reject(http.StatusUnprocessableEntity, "the cut leaves the page with no column")
	}

	return nil
}

// cutRect answers the part of the source image this page takes. The left page
// holds every column before the cut. The right page holds that column and every
// column after it.
func cutRect(side string, column int, bounds image.Rectangle) image.Rectangle {
	if side == books.CutLeft {
		return image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+column, bounds.Max.Y)
	}
	return image.Rect(bounds.Min.X+column, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
}

func (a *api) removeRenditions(pageDir, pageKey, ext string) {
	for _, name := range []string{
		pageKey + "." + filestore.RenditionOriginal + "." + ext,
		pageKey + "." + filestore.RenditionDisplay + ".jpg",
		pageKey + "." + filestore.RenditionThumb + ".jpg",
	} {
		if err := os.Remove(filepath.Join(pageDir, name)); err != nil && !os.IsNotExist(err) {
			slog.Error("remove rendition", "path", filepath.Join(pageDir, name), "error", err)
		}
	}
}

func comparedPageOf(page books.PageOfBook) comparedPage {
	return comparedPage{
		Stage:      page.BookStage,
		StageTitle: books.StageTitle(page.BookStage),
		DisplayURL: filestore.PageURL(page.ImageKey(page.BookStorageKey), page.StorageKey,
			filestore.RenditionDisplay, "jpg"),
		Width:  page.Width,
		Height: page.Height,
	}
}

// writeRejection answers a refusal with the status it carries, and anything
// else as a server error.
func writeRejection(w http.ResponseWriter, err error, fallback string) {
	var invalid invalidUpload
	if errors.As(err, &invalid) {
		writeError(w, invalid.status, invalid.message)
		return
	}
	if errors.Is(err, books.ErrPageNotFound) {
		writeError(w, http.StatusNotFound, "no such page")
		return
	}

	slog.Error("page request", "error", err)
	writeError(w, http.StatusInternalServerError, fallback)
}

type pageReadingRequest struct {
	Model string `json:"model"`
}

// setPageReading makes a page answer one of the readings it holds, and records
// that a reader chose it, so no later reading overrules them.
//
// It refuses a reading the page does not hold and one a renderer cannot draw,
// and names the reason in both cases.
func (a *api) setPageReading(w http.ResponseWriter, r *http.Request) {
	target, ok := a.resolvePage(w, r)
	if !ok {
		return
	}

	var request pageReadingRequest
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "the request must name a reading")
		return
	}
	if request.Model == "" {
		writeError(w, http.StatusUnprocessableEntity, "name the reading the page answers")
		return
	}
	if _, _, drawable := ocr.ReadingNamesOf(request.Model); !drawable {
		writeError(w, http.StatusUnprocessableEntity,
			"the reading "+request.Model+" holds no lines, so nothing can draw it")
		return
	}

	held, err := a.deps.Books.PageReadings(r.Context(), target.page.ID, ocr.RenderModels)
	if err != nil {
		slog.Error("page readings", "book", target.book.ID, "page", target.page.PageIndex,
			"error", err)
		writeError(w, http.StatusInternalServerError, "cannot read the page")
		return
	}

	chosen := int64(0)
	for _, reading := range held {
		if reading.Model == request.Model {
			chosen = reading.ID
		}
	}
	if chosen == 0 {
		writeError(w, http.StatusUnprocessableEntity,
			"this page holds no "+request.Model+" reading")
		return
	}

	if err := a.deps.Books.SetPageReading(r.Context(), target.page.ID, chosen); err != nil {
		slog.Error("set page reading", "book", target.book.ID, "page", target.page.PageIndex,
			"model", request.Model, "error", err)
		writeError(w, http.StatusInternalServerError, "cannot set the reading of the page")
		return
	}

	a.pageOCR(w, r)
}
