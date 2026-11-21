package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/anthropic"
	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/runpod"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

// RescanWorker reads pages of a book that has already been read, with the one
// engine the profile of the job names. The extra profile runs the visual engine
// and merges with the PP-OCRv6 result the store holds, so the endpoint is never
// asked for that engine again. The normal profile runs PP-OCRv6 and replaces
// the reading of the page.
//
// It shares a queue and a bucket layout with the OCR worker, not a control
// flow. A rescan changes the results of pages that stay where they are, where
// the OCR worker deletes the pages of a copy and builds them again.
type RescanWorker struct {
	river.WorkerDefaults[RescanArgs]

	store    *books.Store
	files    *filestore.Store
	hub      *ws.Hub
	objects  Objects
	client   *runpod.Client
	claude   *anthropic.Client
	settings Settings
}

func NewRescanWorker(store *books.Store, files *filestore.Store, hub *ws.Hub,
	objects Objects, settings Settings) *RescanWorker {
	return &RescanWorker{
		store:    store,
		files:    files,
		hub:      hub,
		objects:  objects,
		client:   runpod.NewClient("OCR rescan", settings.APIURL, settings.APIToken),
		claude:   anthropic.NewClient("Claude", settings.ClaudeAPIURL, settings.ClaudeAPIToken),
		settings: settings,
	}
}

func (w *RescanWorker) Timeout(*river.Job[RescanArgs]) time.Duration {
	return workTimeout
}

// rescanMode answers the one engine a rescan of this profile runs. Every
// profile a rescan knows names exactly one.
func rescanMode(profile string) (string, bool) {
	modes, known := RescanModesOf(profile)
	if !known || len(modes) != 1 {
		return "", false
	}
	return modes[0], true
}

// Work never sets the book to `failed`. The book is whole before a rescan and
// whole after one that fails, and a failed book offers a retry that deletes
// every page of it, which would destroy the results of every earlier rescan.
func (w *RescanWorker) Work(ctx context.Context, job *river.Job[RescanArgs]) error {
	started := time.Now()

	book, pages, err := w.load(ctx, job.Args.BookID, job.Args.PageIndexes)
	if errors.Is(err, books.ErrNotFound) {
		// The book was rejected while the job waited. There is nothing to do.
		return nil
	}
	if err != nil {
		return w.abandon(ctx, job, err)
	}

	log := slog.With("book", book.ID, "key", book.StorageKey)

	mode, known := rescanMode(job.Args.Profile)
	if !known {
		return w.abandon(ctx, job,
			fmt.Errorf("the rescan names the unknown OCR profile %q", job.Args.Profile))
	}

	if len(pages) == 0 {
		log.Warn("rescan found none of its pages", "asked", job.Args.PageIndexes)
		return w.finish(ctx, log, book)
	}

	log.Info("rescan started", "pages", len(pages), "profile", job.Args.Profile,
		"engine", mode, "attempt", job.Attempt)

	read, err := w.rescan(ctx, log, book, pages, mode)
	if err != nil {
		log.Error("rescan failed", "attempt", job.Attempt, "error", err)
		return w.abandon(ctx, job, err)
	}

	if mode != ModeClaude {
		if err := w.objects.DeletePrefix(ctx, Prefix(book.StorageKey)); err != nil {
			log.Error("cleanup failed", "prefix", Prefix(book.StorageKey), "error", err)
			return w.abandon(ctx, job, err)
		}
	}

	if err := w.finish(ctx, log, book); err != nil {
		return err
	}

	log.Info("rescan finished", "read", read, "asked", len(pages),
		"took", time.Since(started).Round(time.Second))

	return nil
}

// load answers the book and the pages the job names, in page order. A page the
// job names that the book no longer holds is left out: a rescan stores what it
// can and abandons the rest.
func (w *RescanWorker) load(ctx context.Context, bookID int64,
	indexes []int) (books.Book, []books.Page, error) {
	book, err := w.store.Get(ctx, bookID)
	if err != nil {
		return books.Book{}, nil, err
	}

	all, err := w.store.Pages(ctx, bookID)
	if err != nil {
		return books.Book{}, nil, err
	}

	named := make(map[int]bool, len(indexes))
	for _, index := range indexes {
		named[index] = true
	}

	pages := make([]books.Page, 0, len(indexes))
	for _, page := range all {
		if named[page.PageIndex] {
			pages = append(pages, page)
		}
	}

	return book, pages, nil
}

// rescan reads the named pages and answers how many of them gained results.
func (w *RescanWorker) rescan(ctx context.Context, log *slog.Logger, book books.Book,
	pages []books.Page, mode string) (int, error) {
	if mode == ModeClaude {
		return w.readWithClaude(ctx, log, book, pages)
	}

	if err := w.objects.DeletePrefix(ctx, Prefix(book.StorageKey)); err != nil {
		return 0, err
	}

	if err := w.upload(ctx, log, book, pages); err != nil {
		return 0, err
	}

	output, err := w.run(ctx, log, book, mode)
	if err != nil {
		return 0, err
	}

	return w.storeResults(ctx, log, book, pages, output, mode)
}

func (w *RescanWorker) upload(ctx context.Context, log *slog.Logger, book books.Book,
	pages []books.Page) error {
	started := time.Now()
	log.Info("uploading pages", "pages", len(pages), "prefix", InputPrefix(book.StorageKey))

	w.progress(ctx, book, books.PhaseUploading)

	for _, page := range pages {
		local := w.files.PagePath(page.ImageKey(book.StorageKey), page.StorageKey,
			filestore.RenditionOriginal, page.OriginalExt)
		key := InputKey(book.StorageKey, page.PageIndex, page.OriginalExt)

		if err := w.objects.PutFile(ctx, key, local); err != nil {
			log.Error("upload failed", "page", page.PageIndex, "key", key, "error", err)
			return fmt.Errorf("page %d: %w", page.PageIndex, err)
		}
	}

	log.Info("pages uploaded", "pages", len(pages),
		"took", time.Since(started).Round(time.Millisecond))
	return nil
}

// run starts one endpoint job naming the one engine of the profile. The extra
// profile leaves out the box engine: it has already read these pages, and its
// result is in the store.
func (w *RescanWorker) run(ctx context.Context, log *slog.Logger,
	book books.Book, mode string) (Output, error) {
	input := JobInput{
		Bucket:          w.settings.Bucket,
		EndpointURL:     w.settings.Endpoint,
		Region:          w.settings.Region,
		AccessKeyID:     w.settings.AccessKeyID,
		SecretAccessKey: w.settings.SecretAccessKey,
		InputPrefix:     InputPrefix(book.StorageKey),
		OutputPrefix:    OutputPrefix(book.StorageKey),
		Modes:           []string{mode},
	}

	started := time.Now()
	log.Info("starting endpoint job", "engines", input.Modes)

	w.progress(ctx, book, books.PhaseWaitingAI)

	jobID, err := w.client.Run(ctx, input)
	if err != nil {
		log.Error("endpoint job did not start", "error", err)
		return Output{}, err
	}

	raw, err := w.client.Wait(ctx, jobID, log, func(status string) {
		if phase, ok := runpod.PhaseOfStatus(status); ok {
			w.progress(ctx, book, phase)
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

// storeResults writes the results of every page the endpoint read. A page it could
// not read, or one this system cannot merge, is logged and left as it was. A
// whole book reading refuses a partial result, because a copy holding fewer
// pages than its source is worse than no copy. A rescan only adds results, so a
// page stored is a page improved.
func (w *RescanWorker) storeResults(ctx context.Context, log *slog.Logger, book books.Book,
	pages []books.Page, output Output, mode string) (int, error) {
	workDir, err := os.MkdirTemp(w.files.TempRoot(), "rescan-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(workDir)

	for _, failure := range output.Failed {
		log.Warn("the endpoint could not read a page", "key", failure.InputKey,
			"error", failure.Error)
	}

	byInput := make(map[string]OutputPage, len(output.Pages))
	for _, page := range output.Pages {
		if page.Mode() == mode {
			byInput[page.InputKey] = page
		}
	}

	w.progress(ctx, book, books.PhaseDownloading)

	read := 0
	for _, page := range pages {
		stored, err := w.storePage(ctx, log, book, page, byInput, workDir, mode)
		if err != nil {
			return read, err
		}
		if stored {
			read++
		}
	}

	w.progress(ctx, book, books.PhaseConverting)

	log.Info("pages stored", "read", read, "asked", len(pages))
	return read, nil
}

// storePage answers whether the page gained its results. It answers an error
// only for a failure that the whole job shares, never for one page the endpoint
// could not read.
func (w *RescanWorker) storePage(ctx context.Context, log *slog.Logger, book books.Book,
	page books.Page, byInput map[string]OutputPage, workDir, mode string) (bool, error) {
	inputKey := InputKey(book.StorageKey, page.PageIndex, page.OriginalExt)

	result, ok := byInput[inputKey]
	if !ok {
		log.Warn("the endpoint returned no result for a page", "page", page.PageIndex)
		return false, nil
	}

	textBody, dataBody, ok, err := w.download(ctx, log, page, result, workDir)
	if err != nil || !ok {
		return false, err
	}

	if mode == ModeV6 {
		return w.storeLineReading(ctx, log, page, textBody, dataBody)
	}
	return w.storeVisualReading(ctx, log, page, textBody, dataBody)
}

// download answers the text and the full result of one page. The second answer
// is false for a page this job cannot read further, which costs no other page.
func (w *RescanWorker) download(ctx context.Context, log *slog.Logger, page books.Page,
	result OutputPage, workDir string) ([]byte, []byte, bool, error) {
	textKey, hasText := result.TextKey()
	dataKey, hasData := result.JSONKey()
	if !hasText || !hasData {
		log.Warn("the result of a page names no text and result", "page", page.PageIndex,
			"keys", result.OutputKeys)
		return nil, nil, false, nil
	}

	text := filepath.Join(workDir, fmt.Sprintf("%04d.txt", page.PageIndex))
	data := filepath.Join(workDir, fmt.Sprintf("%04d.json", page.PageIndex))
	for key, local := range map[string]string{textKey: text, dataKey: data} {
		if err := w.objects.GetFile(ctx, key, local); err != nil {
			log.Warn("a result of a page did not download", "page", page.PageIndex,
				"key", key, "error", err)
			return nil, nil, false, nil
		}
	}

	textBody, err := os.ReadFile(text)
	if err != nil {
		return nil, nil, false, err
	}
	dataBody, err := os.ReadFile(data)
	if err != nil {
		return nil, nil, false, err
	}

	return textBody, dataBody, true, nil
}

// storeLineReading replaces the PP-OCRv6 result of the page and takes the
// visual result and both merged results with it. A merged result holds the boxes
// of the reading this one replaced, so it no longer describes the page.
//
// The transcription of the metered API survives, and its merged result is written
// again from the new boxes. That text is a reading of the page image and derives
// from no other result, so new line boxes do not make it wrong — and reading it
// again costs money where merging it again costs nothing.
func (w *RescanWorker) storeLineReading(ctx context.Context, log *slog.Logger,
	page books.Page, textBody, dataBody []byte) (bool, error) {
	err := w.store.ReplacePageOCR(ctx, books.PageOCR{
		PageID: page.ID, Model: ModeV6, Text: string(textBody), Data: dataBody,
	}, []string{ModeVL, ModelMerged, ModelClaudeMerged}, RenderModels)
	if err != nil {
		return false, fmt.Errorf("page %d, %s: %w", page.PageIndex, ModeV6, err)
	}

	kept, held, err := w.store.PageOCRIn(ctx, page.ID, []string{ModeClaude})
	if err != nil {
		return false, err
	}
	if !held {
		return true, nil
	}

	if _, err := w.storeClaudeReading(ctx, log, page, dataBody, kept.Text); err != nil {
		return false, err
	}
	return true, nil
}

// storeVisualReading stores the visual result of the page beside the merged
// result it makes with the stored line result. A page the store holds no line
// result for is left as it was: there is nothing to merge with.
func (w *RescanWorker) storeVisualReading(ctx context.Context, log *slog.Logger,
	page books.Page, textBody, dataBody []byte) (bool, error) {
	boxes, held, err := w.store.PageOCRIn(ctx, page.ID, []string{ModeV6})
	if err != nil {
		return false, err
	}
	if !held {
		log.Warn("a page holds no box result to merge with", "page", page.PageIndex)
		return false, nil
	}

	merged, err := MergeOf(boxes.Data, dataBody)
	if err != nil {
		log.Warn("a page did not merge", "page", page.PageIndex, "error", err)
		return false, nil
	}

	mergedData, err := json.Marshal(merged)
	if err != nil {
		return false, err
	}

	err = w.store.InsertPageOCR(ctx, books.PageOCR{
		PageID: page.ID, Model: ModeVL, Text: string(textBody), Data: dataBody,
	}, RenderModels)
	if err != nil {
		return false, fmt.Errorf("page %d, %s: %w", page.PageIndex, ModeVL, err)
	}

	err = w.store.InsertPageOCR(ctx, books.PageOCR{
		PageID: page.ID, Model: ModelMerged, Text: MergedText(merged), Data: mergedData,
	}, RenderModels)
	if err != nil {
		return false, fmt.Errorf("page %d, %s: %w", page.PageIndex, ModelMerged, err)
	}

	return true, nil
}

// finish returns the book to `complete`. Every path through the worker ends
// here, so the book never keeps a phase and never carries a failure.
func (w *RescanWorker) finish(ctx context.Context, log *slog.Logger, book books.Book) error {
	done, err := w.store.SetCompleteKeepingCount(ctx, book.ID)
	if err != nil {
		log.Error("cannot complete the book after a rescan", "error", err)
		return err
	}
	w.publishFinal(ctx, done)
	return nil
}

// abandon leaves the book as it was. While attempts remain it returns the cause
// alone, so River runs the job again and the book stays `processing`. On the
// last attempt it returns the book to `complete` and the rescan is forgotten.
func (w *RescanWorker) abandon(ctx context.Context, job *river.Job[RescanArgs],
	cause error) error {
	if job.Attempt < job.MaxAttempts {
		return cause
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	slog.Error("rescan abandoned", "book", job.Args.BookID, "pages", job.Args.PageIndexes,
		"error", cause)

	done, err := w.store.SetCompleteKeepingCount(writeCtx, job.Args.BookID)
	if err != nil {
		return errors.Join(cause, err)
	}
	w.publishFinal(writeCtx, done)

	return cause
}

func (w *RescanWorker) progress(ctx context.Context, book books.Book, phase string) {
	updated, err := w.store.SetProgress(ctx, book.ID, phase)
	if err != nil {
		return
	}
	w.publish(ctx, updated)
}

func (w *RescanWorker) publish(ctx context.Context, book books.Book) {
	w.hub.Publish("book.updated", books.Event(book, w.store.Cover(ctx, book.ID)))
}

func (w *RescanWorker) publishFinal(ctx context.Context, book books.Book) {
	w.publish(ctx, book)

	if source, ok := w.store.SourceOf(ctx, book); ok {
		w.publish(ctx, source)
	}
}

// readWithClaude reads the named pages against the metered API, in the server
// process alone. Nothing reaches the bucket and nothing reaches the endpoint: the
// page image is already on this disk, and the answer is text.
func (w *RescanWorker) readWithClaude(ctx context.Context, log *slog.Logger,
	book books.Book, pages []books.Page) (int, error) {
	w.progress(ctx, book, books.PhaseWaitingAI)

	read, spent := 0, anthropic.Usage{}
	for _, page := range pages {
		stored, usage, err := w.readPageWithClaude(ctx, log, book, page)
		spent.InputTokens += usage.InputTokens
		spent.OutputTokens += usage.OutputTokens
		if err != nil {
			return read, err
		}
		if stored {
			read++
		}
	}

	log.Info("the metered API read the pages", "read", read, "asked", len(pages),
		"inputTokens", spent.InputTokens, "outputTokens", spent.OutputTokens)

	w.progress(ctx, book, books.PhaseConverting)
	return read, nil
}

// readPageWithClaude answers whether the page gained its results. A page the
// model would not read, or one whose answer is no transcription of it, keeps
// every result it held: the rescan stores what it can and abandons the rest.
//
// It answers an error only for a failure the whole job shares, such as the API
// refusing every request. That costs a retry of the pages already read, which is
// the price of not reporting a rescan that read nothing as a rescan that worked.
func (w *RescanWorker) readPageWithClaude(ctx context.Context, log *slog.Logger,
	book books.Book, page books.Page) (bool, anthropic.Usage, error) {
	mediaType, ok := mediaTypeOf(page.OriginalExt)
	if !ok {
		log.Warn("a page holds an image the metered API does not read",
			"page", page.PageIndex, "ext", page.OriginalExt)
		return false, anthropic.Usage{}, nil
	}

	boxes, held, err := w.store.PageOCRIn(ctx, page.ID, []string{ModeV6})
	if err != nil {
		return false, anthropic.Usage{}, err
	}
	if !held {
		log.Warn("a page holds no box result to merge with", "page", page.PageIndex)
		return false, anthropic.Usage{}, nil
	}

	local := w.files.PagePath(page.ImageKey(book.StorageKey), page.StorageKey,
		filestore.RenditionOriginal, page.OriginalExt)
	image, err := os.ReadFile(local)
	if err != nil {
		return false, anthropic.Usage{}, fmt.Errorf("page %d: %w", page.PageIndex, err)
	}

	text, usage, err := w.claude.ReadPage(ctx, image, mediaType)
	if errors.Is(err, anthropic.ErrNotRead) {
		log.Warn("the metered API did not read a page", "page", page.PageIndex, "error", err)
		return false, usage, nil
	}
	if err != nil {
		log.Error("the metered API failed", "page", page.PageIndex, "error", err)
		return false, usage, err
	}

	stored, err := w.storeClaudeReading(ctx, log, page, boxes.Data, text)
	return stored, usage, err
}

// storeClaudeReading stores the transcription and the merged result it makes with
// the stored line result of the page.
func (w *RescanWorker) storeClaudeReading(ctx context.Context, log *slog.Logger,
	page books.Page, boxData []byte, text string) (bool, error) {
	merged, read, err := MergeOfText(boxData, text, page.Width, page.Height)
	if err != nil {
		log.Warn("a page did not merge", "page", page.PageIndex, "error", err)
		return false, nil
	}
	if !read {
		// The answer aligned with too few lines to be a reading of this page,
		// which is what a model describing the page instead of reading it
		// produces. Storing it would name a reading holding none of its own text.
		log.Warn("the answer of the metered API is no transcription of the page",
			"page", page.PageIndex, "lines", merged.Counts.Lines,
			"fromText", merged.Counts.FromText)
		return false, nil
	}

	mergedData, err := json.Marshal(merged)
	if err != nil {
		return false, err
	}

	err = w.store.InsertPageOCR(ctx, books.PageOCR{
		PageID: page.ID, Model: ModeClaude, Text: text, Data: claudeData(text),
	}, RenderModels)
	if err != nil {
		return false, fmt.Errorf("page %d, %s: %w", page.PageIndex, ModeClaude, err)
	}

	err = w.store.InsertPageOCR(ctx, books.PageOCR{
		PageID: page.ID, Model: ModelClaudeMerged, Text: MergedText(merged), Data: mergedData,
	}, RenderModels)
	if err != nil {
		return false, fmt.Errorf("page %d, %s: %w", page.PageIndex, ModelClaudeMerged, err)
	}

	return true, nil
}

// claudeData is the full result of a reading that answers text and nothing else.
// Every stored result carries one, so the column holds the same shape for every
// engine.
func claudeData(text string) []byte {
	data, err := json.Marshal(map[string]any{"model": anthropic.Model, "text": text})
	if err != nil {
		// A string always marshals.
		return []byte(`{}`)
	}
	return data
}

// mediaTypeOf answers the type of a stored page image. The second answer is false
// for an image the API does not read, which costs no other page.
func mediaTypeOf(ext string) (string, bool) {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg":
		return "image/jpeg", true
	case "png":
		return "image/png", true
	case "webp":
		return "image/webp", true
	case "gif":
		return "image/gif", true
	default:
		return "", false
	}
}
