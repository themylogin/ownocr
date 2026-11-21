package dewarp

import (
	"context"
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
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/runpod"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

const (
	workTimeout      = 6 * time.Hour
	progressInterval = 10
	jpegQuality      = 95
)

// Settings carries the bucket the worker and the endpoint share, plus the
// endpoint itself.
type Settings struct {
	objectstore.Settings

	APIURL   string
	APIToken string
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
		client:   runpod.NewClient("dewarp", settings.APIURL, settings.APIToken),
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

	log := slog.With("book", copyBook.ID, "source", source.ID, "key", copyBook.StorageKey)
	log.Info("dewarp started", "pages", len(sourcePages), "attempt", job.Attempt)

	processing, err := w.store.SetProcessing(ctx, copyBook.ID, books.PhaseUploading)
	if err != nil {
		return err
	}
	w.publish(ctx, processing)

	pageCount, err := w.rebuild(ctx, log, copyBook, source, sourcePages)
	if err != nil {
		log.Error("dewarp failed", "attempt", job.Attempt, "error", err)
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

	log.Info("dewarp finished", "pages", pageCount, "took", time.Since(started).Round(time.Second))

	return nil
}

func (w *Worker) load(ctx context.Context, bookID int64) (books.Book, books.Book, []books.Page, error) {
	copyBook, err := w.store.Get(ctx, bookID)
	if err != nil {
		return books.Book{}, books.Book{}, nil, err
	}
	if copyBook.SourceBookID == nil {
		return books.Book{}, books.Book{}, nil, errors.New("the book has no source book to dewarp")
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

// rebuild discards whatever an interrupted attempt left behind, locally and in
// the bucket, then dewarps the whole book again.
func (w *Worker) rebuild(ctx context.Context, log *slog.Logger, copyBook, source books.Book,
	sourcePages []books.Page) (int, error) {
	if err := w.store.DeletePages(ctx, copyBook.ID); err != nil {
		return 0, err
	}

	pageDir := w.files.PageDir(copyBook.StorageKey)
	if err := os.RemoveAll(pageDir); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return 0, err
	}

	if err := w.objects.DeletePrefix(ctx, Prefix(copyBook.StorageKey)); err != nil {
		return 0, err
	}

	if err := w.upload(ctx, log, copyBook, source, sourcePages); err != nil {
		return 0, err
	}

	output, err := w.run(ctx, log, copyBook)
	if err != nil {
		return 0, err
	}
	if err := checkOutput(output, sourcePages); err != nil {
		return 0, err
	}

	return len(sourcePages), w.download(ctx, log, copyBook, sourcePages, output)
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

func (w *Worker) run(ctx context.Context, log *slog.Logger, copyBook books.Book) (Output, error) {
	input := JobInput{
		Bucket:          w.settings.Bucket,
		EndpointURL:     w.settings.Endpoint,
		Region:          w.settings.Region,
		AccessKeyID:     w.settings.AccessKeyID,
		SecretAccessKey: w.settings.SecretAccessKey,
		InputPrefix:     InputPrefix(copyBook.StorageKey),
		OutputPrefix:    OutputPrefix(copyBook.StorageKey),
		JPEGQuality:     jpegQuality,
	}

	started := time.Now()
	log.Info("starting endpoint job", "url", w.settings.APIURL,
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
		"failed", len(output.Failed), "took", time.Since(started).Round(time.Second))
	return output, nil
}

// checkOutput refuses a partial result. A copy with fewer pages than its source
// is worse than no copy at all.
func checkOutput(output Output, sourcePages []books.Page) error {
	if len(output.Failed) > 0 {
		pages := make([]string, 0, len(output.Failed))
		for _, failure := range output.Failed {
			pages = append(pages, fmt.Sprintf("page %s (%s)",
				pageNumberOf(failure.InputKey), failure.Error))
		}
		sort.Strings(pages)
		return fmt.Errorf("the endpoint could not dewarp %s", strings.Join(pages, ", "))
	}

	if len(output.Pages) != len(sourcePages) {
		return fmt.Errorf("the endpoint returned %d pages, the book has %d",
			len(output.Pages), len(sourcePages))
	}

	return nil
}

// download fetches every page first, then converts them. The two passes keep
// the reported phase honest: a page by page loop would flip between
// "Downloading" and "Converting" once per page.
func (w *Worker) download(ctx context.Context, log *slog.Logger, copyBook books.Book,
	sourcePages []books.Page, output Output) error {
	workDir, err := os.MkdirTemp(w.files.TempRoot(), "dewarp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	fetched, err := w.fetchPages(ctx, log, copyBook, sourcePages, output, workDir)
	if err != nil {
		return err
	}

	return w.convertPages(ctx, log, copyBook, sourcePages, fetched)
}

func (w *Worker) fetchPages(ctx context.Context, log *slog.Logger, copyBook books.Book,
	sourcePages []books.Page, output Output, workDir string) ([]string, error) {
	started := time.Now()
	log.Info("downloading pages", "pages", len(output.Pages),
		"prefix", OutputPrefix(copyBook.StorageKey))

	w.progress(ctx, copyBook, books.PhaseDownloading)

	byInput := make(map[string]OutputPage, len(output.Pages))
	for _, page := range output.Pages {
		byInput[page.InputKey] = page
	}

	fetched := make([]string, len(sourcePages))

	for index, source := range sourcePages {
		inputKey := InputKey(copyBook.StorageKey, source.PageIndex, source.OriginalExt)
		result, ok := byInput[inputKey]
		if !ok || result.OutputKey == "" {
			return nil, fmt.Errorf("the endpoint returned no page %d", source.PageIndex)
		}

		local := filepath.Join(workDir, fmt.Sprintf("%04d%s", source.PageIndex,
			outputExt(result.OutputKey)))
		if err := w.objects.GetFile(ctx, result.OutputKey, local); err != nil {
			log.Error("download failed", "page", source.PageIndex,
				"key", result.OutputKey, "error", err)
			return nil, fmt.Errorf("page %d: %w", source.PageIndex, err)
		}
		fetched[index] = local

		log.Debug("page downloaded", "page", source.PageIndex, "key", result.OutputKey,
			"size", fmt.Sprintf("%dx%d", result.Width, result.Height))
		if source.PageIndex%progressInterval == 0 {
			log.Info("downloading", "done", source.PageIndex, "of", len(sourcePages))
		}
	}

	log.Info("pages downloaded", "pages", len(sourcePages),
		"took", time.Since(started).Round(time.Millisecond))
	return fetched, nil
}

func (w *Worker) convertPages(ctx context.Context, log *slog.Logger, copyBook books.Book,
	sourcePages []books.Page, fetched []string) error {
	started := time.Now()
	log.Info("converting pages", "pages", len(sourcePages))

	w.progress(ctx, copyBook, books.PhaseConverting)

	pageDir := w.files.PageDir(copyBook.StorageKey)

	for index, source := range sourcePages {
		if err := w.storePage(ctx, copyBook, fetched[index], pageDir, source); err != nil {
			return err
		}
		if source.PageIndex%progressInterval == 0 {
			log.Info("converting", "done", source.PageIndex, "of", len(sourcePages))
		}

		w.reportProgress(ctx, copyBook, source.PageIndex, len(sourcePages))
	}

	log.Info("pages converted", "pages", len(sourcePages),
		"took", time.Since(started).Round(time.Millisecond))
	return nil
}

func (w *Worker) progress(ctx context.Context, book books.Book, phase string) {
	updated, err := w.store.SetProgress(ctx, book.ID, phase)
	if err != nil {
		return
	}
	w.publish(ctx, updated)
}

func (w *Worker) storePage(ctx context.Context, copyBook books.Book, dewarped, pageDir string,
	source books.Page) error {
	pageKey, err := books.NewStorageKey()
	if err != nil {
		return err
	}

	ext, width, height, err := ingest.WriteRenditions(dewarped, pageDir, pageKey)
	if err != nil {
		return fmt.Errorf("page %d: %w", source.PageIndex, err)
	}

	return w.store.InsertPage(ctx, books.Page{
		BookID:       copyBook.ID,
		PageIndex:    source.PageIndex,
		StorageKey:   pageKey,
		OriginalExt:  ext,
		Width:        width,
		Height:       height,
		SourcePageID: &source.ID,
	})
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

// pageNumberOf turns an input object name back into the page number it carries.
func pageNumberOf(key string) string {
	base := path.Base(key)
	if index := strings.IndexByte(base, '.'); index > 0 {
		base = base[:index]
	}
	return strings.TrimLeft(base, "0")
}

func outputExt(key string) string {
	ext := path.Ext(key)
	if ext == "" {
		return ".jpg"
	}
	return ext
}
