package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/runpod"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

const (
	workTimeout      = 6 * time.Hour
	progressInterval = 10
)

// Settings carries the bucket the worker and the endpoint share, plus the
// endpoint itself.
type Settings struct {
	objectstore.Settings

	APIURL   string
	APIToken string

	// ClaudeAPIURL and ClaudeAPIToken name the metered API the Ultra rescan
	// reads on. The rescan worker alone uses them, and an empty URL leaves the
	// client on the API it names for itself.
	ClaudeAPIURL   string
	ClaudeAPIToken string
}

// Objects is the part of the object store the worker uses. A test supplies its
// own, so the whole job runs without a bucket.
type Objects interface {
	PutFile(ctx context.Context, key, path string) error
	GetFile(ctx context.Context, key, path string) error
	DeletePrefix(ctx context.Context, prefix string) error
}

type Worker struct {
	river.WorkerDefaults[Args]

	store    *books.Store
	files    *filestore.Store
	hub      *ws.Hub
	objects  Objects
	client   *runpod.Client
	settings Settings
}

func NewWorker(store *books.Store, files *filestore.Store, hub *ws.Hub,
	objects Objects, settings Settings) *Worker {
	return &Worker{
		store:    store,
		files:    files,
		hub:      hub,
		objects:  objects,
		client:   runpod.NewClient("OCR", settings.APIURL, settings.APIToken),
		settings: settings,
	}
}

func (w *Worker) Timeout(*river.Job[Args]) time.Duration {
	return workTimeout
}

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	started := time.Now()

	copyBook, source, sourcePages, err := w.load(ctx, job.Args.BookID)
	if errors.Is(err, books.ErrNotFound) {
		// The book was rejected while the job waited. There is nothing to do.
		return nil
	}
	if err != nil {
		return w.reportFailure(ctx, job, err)
	}

	// The profile comes from the copy, never from the job. A retry therefore
	// runs the engines the user picked, on every attempt.
	profile := ""
	if copyBook.OCRProfile != nil {
		profile = *copyBook.OCRProfile
	}
	modes, known := ModesOf(profile)
	if !known {
		return w.reportFailure(ctx, job,
			fmt.Errorf("the copy names the unknown OCR profile %q", profile))
	}

	log := slog.With("book", copyBook.ID, "source", source.ID, "key", copyBook.StorageKey)
	log.Info("ocr started", "pages", len(sourcePages), "profile", profile,
		"engines", modes, "attempt", job.Attempt)

	processing, err := w.store.SetProcessing(ctx, copyBook.ID, books.PhaseUploading)
	if err != nil {
		return err
	}
	w.publish(ctx, processing)

	pageCount, err := w.rebuild(ctx, log, copyBook, source, sourcePages, modes)
	if err != nil {
		log.Error("ocr failed", "attempt", job.Attempt, "error", err)
		return w.reportFailure(ctx, job, err)
	}

	// The prefix goes before the book completes, so a complete copy never
	// leaves objects behind.
	if err := w.objects.DeletePrefix(ctx, Prefix(copyBook.StorageKey)); err != nil {
		log.Error("cleanup failed", "prefix", Prefix(copyBook.StorageKey), "error", err)
		return w.reportFailure(ctx, job, err)
	}
	log.Info("exchange objects deleted", "prefix", Prefix(copyBook.StorageKey))

	complete, err := w.store.SetComplete(ctx, copyBook.ID, pageCount)
	if err != nil {
		return err
	}
	w.publishFinal(ctx, complete)

	log.Info("ocr finished", "pages", pageCount, "took", time.Since(started).Round(time.Second))

	return nil
}

func (w *Worker) load(ctx context.Context, bookID int64) (books.Book, books.Book, []books.Page, error) {
	copyBook, err := w.store.Get(ctx, bookID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}
	if copyBook.SourceBookID == nil {
		return books.Book{}, books.Book{}, nil, errors.New("the book has no source book to read")
	}

	source, err := w.store.Get(ctx, *copyBook.SourceBookID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}

	pages, err := w.store.Pages(ctx, source.ID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}
	if len(pages) == 0 {
		return books.Book{}, books.Book{}, nil, errors.New("the source book has no pages")
	}

	return copyBook, source, pages, nil
}

// rebuild discards whatever an interrupted attempt left behind, in the bucket
// and in the rows of the copy, then reads the whole book again.
//
// The copy owns no files: its pages show the images of the source pages, so
// there is no page directory to clear. Deleting the pages removes their stored
// results through the cascade.
func (w *Worker) rebuild(ctx context.Context, log *slog.Logger, copyBook, source books.Book,
	sourcePages []books.Page, modes []string) (int, error) {
	if err := w.store.DeletePages(ctx, copyBook.ID); err != nil {
		return 0, err
	}

	if err := w.objects.DeletePrefix(ctx, Prefix(copyBook.StorageKey)); err != nil {
		return 0, err
	}

	if err := w.upload(ctx, log, copyBook, source, sourcePages); err != nil {
		return 0, err
	}

	output, err := w.run(ctx, log, copyBook, modes)
	if err != nil {
		return 0, err
	}
	if err := checkOutput(output, sourcePages, modes, copyBook.StorageKey); err != nil {
		return 0, err
	}

	return len(sourcePages), w.download(ctx, log, copyBook, source, sourcePages, output, modes)
}

func (w *Worker) upload(ctx context.Context, log *slog.Logger, copyBook, source books.Book,
	sourcePages []books.Page) error {
	started := time.Now()
	log.Info("uploading pages", "pages", len(sourcePages), "prefix", InputPrefix(copyBook.StorageKey))

	var bytes int64
	for _, page := range sourcePages {
		local := w.files.PagePath(source.StorageKey, page.StorageKey,
			filestore.RenditionOriginal, page.OriginalExt)
		key := InputKey(copyBook.StorageKey, page.PageIndex, page.OriginalExt)

		pageStart := time.Now()
		if err := w.objects.PutFile(ctx, key, local); err != nil {
			log.Error("upload failed", "page", page.PageIndex, "key", key, "error", err)
			return fmt.Errorf("page %d: %w", page.PageIndex, err)
		}

		if info, err := os.Stat(local); err == nil {
			bytes += info.Size()
		}
		log.Debug("page uploaded", "page", page.PageIndex, "key", key,
			"took", time.Since(pageStart).Round(time.Millisecond))

		if page.PageIndex%progressInterval == 0 {
			log.Info("uploading", "done", page.PageIndex, "of", len(sourcePages))
		}
	}

	log.Info("pages uploaded", "pages", len(sourcePages), "megabytes", bytes>>20,
		"took", time.Since(started).Round(time.Millisecond))
	return nil
}

func (w *Worker) run(ctx context.Context, log *slog.Logger, copyBook books.Book,
	modes []string) (Output, error) {
	input := JobInput{
		Bucket:          w.settings.Bucket,
		EndpointURL:     w.settings.Endpoint,
		Region:          w.settings.Region,
		AccessKeyID:     w.settings.AccessKeyID,
		SecretAccessKey: w.settings.SecretAccessKey,
		InputPrefix:     InputPrefix(copyBook.StorageKey),
		OutputPrefix:    OutputPrefix(copyBook.StorageKey),
		Modes:           modes,
	}

	started := time.Now()
	log.Info("starting endpoint job", "url", w.settings.APIURL, "engines", modes,
		"input", input.InputPrefix, "output", input.OutputPrefix)

	w.progress(ctx, copyBook, books.PhaseWaitingAI)

	jobID, err := w.client.Run(ctx, input)
	if err != nil {
		log.Error("endpoint job did not start", "url", w.settings.APIURL, "error", err)
		return Output{}, err
	}
	log.Info("endpoint job queued", "job", jobID)

	raw, err := w.client.Wait(ctx, jobID, log, func(status string) {
		if phase, ok := runpod.PhaseOfStatus(status); ok {
			w.progress(ctx, copyBook, phase)
		}
	})
	if err != nil {
		log.Error("endpoint job did not finish", "job", jobID,
			"took", time.Since(started).Round(time.Second), "error", err)
		return Output{}, err
	}

	output, err := decodeOutput(raw)
	if err != nil {
		log.Error("endpoint job returned an unreadable result", "job", jobID, "error", err)
		return Output{}, err
	}

	log.Info("endpoint job done", "job", jobID, "pages", len(output.Pages),
		"failed", len(output.Failed), "chars", charsOf(output),
		"took", time.Since(started).Round(time.Second))
	return output, nil
}

// checkOutput refuses a partial result. A copy with fewer pages than its source
// is worse than no copy at all.
//
// The endpoint answers one entry per engine per page, so the count is taken per
// engine. A profile naming two engines returns twice the page count and is
// correct.
func checkOutput(output Output, sourcePages []books.Page, modes []string, bookKey string) error {
	if len(output.Failed) > 0 {
		pages := make([]string, 0, len(output.Failed))
		for _, failure := range output.Failed {
			pages = append(pages, fmt.Sprintf("page %s (%s)",
				pageNumberOf(failure.InputKey), failure.Error))
		}
		sort.Strings(pages)
		return fmt.Errorf("the endpoint could not read %s", strings.Join(pages, ", "))
	}

	returned := make(map[string]map[string]bool, len(modes))
	for _, mode := range modes {
		returned[mode] = make(map[string]bool, len(sourcePages))
	}
	for _, page := range output.Pages {
		if seen, ok := returned[page.Mode()]; ok {
			seen[page.InputKey] = true
		}
	}

	for _, mode := range modes {
		missing := make([]string, 0)
		for _, source := range sourcePages {
			key := InputKey(bookKey, source.PageIndex, source.OriginalExt)
			if !returned[mode][key] {
				missing = append(missing, fmt.Sprintf("page %d", source.PageIndex))
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("the %s engine returned %d of %d pages, missing %s",
				mode, len(sourcePages)-len(missing), len(sourcePages),
				strings.Join(missing, ", "))
		}
	}

	return nil
}

// download fetches every markdown file first, then builds the pages. The two
// passes keep the reported phase honest: a page by page loop would flip between
// "Downloading" and "Converting" once per page.
func (w *Worker) download(ctx context.Context, log *slog.Logger, copyBook, source books.Book,
	sourcePages []books.Page, output Output, modes []string) error {
	workDir, err := os.MkdirTemp(w.files.TempRoot(), "ocr-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	fetched, err := w.fetchPages(ctx, log, copyBook, sourcePages, output, modes, workDir)
	if err != nil {
		return err
	}

	return w.buildPages(ctx, log, copyBook, source, sourcePages, fetched)
}

// engineResult is what one engine produced for one page, on local disk.
type engineResult struct {
	mode string
	text string
	data string
}

// fetchPages keys a result by its input key and its engine together. Two
// entries share one input key when the profile names two engines, so the input
// key alone would drop one of them.
func (w *Worker) fetchPages(ctx context.Context, log *slog.Logger, copyBook books.Book,
	sourcePages []books.Page, output Output, modes []string,
	workDir string) ([][]engineResult, error) {
	started := time.Now()
	log.Info("downloading pages", "results", len(output.Pages), "engines", modes,
		"prefix", OutputPrefix(copyBook.StorageKey))

	w.progress(ctx, copyBook, books.PhaseDownloading)

	type pageKey struct{ input, mode string }
	byPage := make(map[pageKey]OutputPage, len(output.Pages))
	for _, page := range output.Pages {
		byPage[pageKey{page.InputKey, page.Mode()}] = page
	}

	fetched := make([][]engineResult, len(sourcePages))

	for index, source := range sourcePages {
		inputKey := InputKey(copyBook.StorageKey, source.PageIndex, source.OriginalExt)

		for _, mode := range modes {
			result, ok := byPage[pageKey{inputKey, mode}]
			if !ok {
				return nil, fmt.Errorf("the endpoint returned no %s result for page %d",
					mode, source.PageIndex)
			}

			textKey, hasText := result.TextKey()
			jsonKey, hasJSON := result.JSONKey()
			if !hasText || !hasJSON {
				return nil, fmt.Errorf("the %s result for page %d names %v, "+
					"which is not a text and a result", mode, source.PageIndex, result.OutputKeys)
			}

			text := filepath.Join(workDir, fmt.Sprintf("%04d.%s.txt", source.PageIndex, mode))
			data := filepath.Join(workDir, fmt.Sprintf("%04d.%s.json", source.PageIndex, mode))
			for key, local := range map[string]string{textKey: text, jsonKey: data} {
				if err := w.objects.GetFile(ctx, key, local); err != nil {
					log.Error("download failed", "page", source.PageIndex, "engine", mode,
						"key", key, "error", err)
					return nil, fmt.Errorf("page %d: %w", source.PageIndex, err)
				}
			}

			fetched[index] = append(fetched[index], engineResult{mode: mode, text: text, data: data})

			log.Debug("page downloaded", "page", source.PageIndex, "engine", mode,
				"chars", result.Chars, "seconds", result.Seconds)
		}

		if source.PageIndex%progressInterval == 0 {
			log.Info("downloading", "done", source.PageIndex, "of", len(sourcePages))
		}
	}

	log.Info("pages downloaded", "pages", len(sourcePages), "engines", len(modes),
		"took", time.Since(started).Round(time.Millisecond))
	return fetched, nil
}

// buildPages gives the copy its own page row for every source page. OCR reads a
// page, it does not change it, so the page shows the image of its source page
// and writes no image of its own. What the endpoint returned becomes one stored
// result per engine.
func (w *Worker) buildPages(ctx context.Context, log *slog.Logger, copyBook, source books.Book,
	sourcePages []books.Page, fetched [][]engineResult) error {
	started := time.Now()
	log.Info("storing pages", "pages", len(sourcePages))

	w.progress(ctx, copyBook, books.PhaseConverting)

	for index, sourcePage := range sourcePages {
		if err := w.storePage(ctx, copyBook, source, sourcePage, fetched[index]); err != nil {
			return err
		}
		if sourcePage.PageIndex%progressInterval == 0 {
			log.Info("storing", "done", sourcePage.PageIndex, "of", len(sourcePages))
		}

		w.reportProgress(ctx, copyBook, sourcePage.PageIndex, len(sourcePages))
	}

	log.Info("pages stored", "pages", len(sourcePages),
		"took", time.Since(started).Round(time.Millisecond))
	return nil
}

// storePage writes the page row and every result of that page. The page keeps
// the storage key and the size of its source page, and names the book that
// holds the image, so no rendition is written and no byte is copied.
//
// A page both engines read gains a third result, derived from the two. The engine
// results go in unchanged, and the merged one is written from them afterwards.
func (w *Worker) storePage(ctx context.Context, copyBook, source books.Book,
	sourcePage books.Page, results []engineResult) error {
	pageID, err := w.store.InsertPageID(ctx, books.Page{
		BookID:       copyBook.ID,
		PageIndex:    sourcePage.PageIndex,
		StorageKey:   sourcePage.StorageKey,
		OriginalExt:  sourcePage.OriginalExt,
		Width:        sourcePage.Width,
		Height:       sourcePage.Height,
		SourcePageID: &sourcePage.ID,
		ImageBookKey: &source.StorageKey,
	})
	if err != nil {
		return fmt.Errorf("page %d: %w", sourcePage.PageIndex, err)
	}

	dataByMode := make(map[string][]byte, len(results))
	for _, result := range results {
		text, err := os.ReadFile(result.text)
		if err != nil {
			return fmt.Errorf("page %d, %s text: %w", sourcePage.PageIndex, result.mode, err)
		}
		data, err := os.ReadFile(result.data)
		if err != nil {
			return fmt.Errorf("page %d, %s result: %w", sourcePage.PageIndex, result.mode, err)
		}

		if err := w.store.InsertPageOCR(ctx, books.PageOCR{
			PageID: pageID,
			Model:  result.mode,
			Text:   string(text),
			Data:   data,
		}, RenderModels); err != nil {
			return fmt.Errorf("page %d, %s: %w", sourcePage.PageIndex, result.mode, err)
		}
		dataByMode[result.mode] = data
	}

	return w.storeMerged(ctx, pageID, sourcePage.PageIndex, dataByMode)
}

// storeMerged writes the merged result of a page both engines read. A page one
// engine read has nothing to join, so it gains no row.
func (w *Worker) storeMerged(ctx context.Context, pageID int64, pageIndex int,
	dataByMode map[string][]byte) error {
	boxData, readBoxes := dataByMode[ModeV6]
	textData, readText := dataByMode[ModeVL]
	if !readBoxes || !readText {
		return nil
	}

	merged, err := MergeOf(boxData, textData)
	if err != nil {
		return fmt.Errorf("page %d, %s: %w", pageIndex, ModelMerged, err)
	}

	data, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("page %d, %s: %w", pageIndex, ModelMerged, err)
	}

	if err := w.store.InsertPageOCR(ctx, books.PageOCR{
		PageID: pageID,
		Model:  ModelMerged,
		Text:   MergedText(merged),
		Data:   data,
	}, RenderModels); err != nil {
		return fmt.Errorf("page %d, %s: %w", pageIndex, ModelMerged, err)
	}

	return nil
}

func (w *Worker) progress(ctx context.Context, book books.Book, phase string) {
	updated, err := w.store.SetProgress(ctx, book.ID, phase)
	if err != nil {
		return
	}
	w.publish(ctx, updated)
}

func (w *Worker) reportProgress(ctx context.Context, book books.Book, done, total int) {
	if done%progressInterval != 0 && done != total {
		return
	}

	updated, err := w.store.SetPageCount(ctx, book.ID, done)
	if err != nil {
		return
	}
	w.publish(ctx, updated)
}

// River retries until MaxAttempts. Only the last attempt marks the copy failed,
// so the interface does not flip to "failed" while retries remain.
func (w *Worker) reportFailure(ctx context.Context, job *river.Job[Args], cause error) error {
	if job.Attempt < job.MaxAttempts {
		return cause
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	failed, err := w.store.SetFailed(writeCtx, job.Args.BookID, cause.Error())
	if err != nil {
		return errors.Join(cause, err)
	}
	w.publishFinal(writeCtx, failed)

	return cause
}

func (w *Worker) publish(ctx context.Context, book books.Book) {
	w.hub.Publish("book.updated", books.Event(book, w.store.Cover(ctx, book.ID)))
}

// publishFinal also announces the source book. Its mark follows the state of
// this copy, so a client that shows the source must hear that the copy ended.
func (w *Worker) publishFinal(ctx context.Context, book books.Book) {
	w.publish(ctx, book)

	if source, ok := w.store.SourceOf(ctx, book); ok {
		w.publish(ctx, source)
	}
}

func charsOf(output Output) int {
	total := 0
	for _, page := range output.Pages {
		total += page.Chars
	}
	return total
}

// pageNumberOf turns an input object name back into the page number it carries.
func pageNumberOf(key string) string {
	base := path.Base(key)
	if index := strings.IndexByte(base, '.'); index > 0 {
		base = base[:index]
	}
	return strings.TrimLeft(base, "0")
}
