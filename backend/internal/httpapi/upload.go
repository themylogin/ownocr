package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/ingest"
)

const (
	sniffSize      = 512
	maxIngestTries = 3
)

type uploadedFile struct {
	name        string
	contentType string
}

type invalidUpload struct {
	status  int
	message string
}

func (e invalidUpload) Error() string { return e.message }

func reject(status int, format string, args ...any) error {
	return invalidUpload{status: status, message: fmt.Sprintf(format, args...)}
}

func (a *api) createBook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.deps.MaxUploadBytes)

	uploadKey, err := books.NewStorageKey()
	if err != nil {
		slog.Error("upload key", "error", err)
		writeError(w, http.StatusInternalServerError, "cannot accept the upload")
		return
	}

	tempDir, err := a.deps.Files.NewTempDir(uploadKey)
	if err != nil {
		slog.Error("upload temp dir", "error", err)
		writeError(w, http.StatusInternalServerError, "cannot accept the upload")
		return
	}

	book, err := a.receiveBook(r, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)

		var invalid invalidUpload
		switch {
		case errors.As(err, &invalid):
			writeError(w, invalid.status, invalid.message)
		case isTooLarge(err):
			writeError(w, http.StatusRequestEntityTooLarge, "the upload exceeds the size limit")
		default:
			slog.Error("upload", "error", err)
			writeError(w, http.StatusInternalServerError, "cannot store the upload")
		}
		return
	}

	summary := a.summary(r.Context(), book)
	a.deps.Hub.Publish("book.created", summary)
	writeJSON(w, http.StatusCreated, summary)
}

// receiveBook streams the parts to disk, validates them, then commits the row
// and the job. Bytes land durably before the row exists, so no book row can
// point at files that are not there.
func (a *api) receiveBook(r *http.Request, tempDir string) (books.Book, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return books.Book{}, reject(http.StatusBadRequest, "the request must be multipart/form-data")
	}

	name := ""
	var files []uploadedFile

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return books.Book{}, err
		}

		if part.FileName() == "" {
			if part.FormName() == "name" {
				value, err := io.ReadAll(io.LimitReader(part, 1<<16))
				if err != nil {
					part.Close()
					return books.Book{}, err
				}
				name = string(value)
			}
			part.Close()
			continue
		}

		file, err := storePart(part, tempDir, len(files)+1)
		part.Close()
		if err != nil {
			return books.Book{}, err
		}
		files = append(files, file)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return books.Book{}, reject(http.StatusUnprocessableEntity, "name is required")
	}
	if err := validateFiles(files); err != nil {
		return books.Book{}, err
	}

	return a.commitBook(r.Context(), name, tempDir)
}

// storePart writes one part to a name prefixed with its position. The prefix
// carries the upload order through to the worker and keeps two files with the
// same uploaded name apart.
func storePart(part *multipart.Part, tempDir string, position int) (uploadedFile, error) {
	safeName := filestore.SanitizeName(part.FileName())
	target := filepath.Join(tempDir, fmt.Sprintf("%04d_%s", position, safeName))

	out, err := os.Create(target)
	if err != nil {
		return uploadedFile{}, err
	}
	defer out.Close()

	header := make([]byte, sniffSize)
	read, err := io.ReadFull(part, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return uploadedFile{}, err
	}
	header = header[:read]

	if _, err := out.Write(header); err != nil {
		return uploadedFile{}, err
	}
	if _, err := io.Copy(out, part); err != nil {
		return uploadedFile{}, err
	}
	if err := out.Close(); err != nil {
		return uploadedFile{}, err
	}

	return uploadedFile{name: safeName, contentType: sniff(header)}, nil
}

// The declared part header is unreliable across browsers, so the bytes decide.
func sniff(header []byte) string {
	detected := http.DetectContentType(header)
	if index := strings.IndexByte(detected, ';'); index >= 0 {
		detected = detected[:index]
	}
	return strings.TrimSpace(detected)
}

func validateFiles(files []uploadedFile) error {
	if len(files) == 0 {
		return reject(http.StatusUnprocessableEntity, "a book needs at least one file")
	}

	pdfs := 0
	images := 0

	for _, file := range files {
		switch file.contentType {
		case "application/pdf":
			pdfs++
		case "image/jpeg", "image/png":
			images++
		default:
			return reject(http.StatusUnprocessableEntity,
				"%s is not a PDF, a JPEG, or a PNG", file.name)
		}
	}

	if pdfs > 1 {
		return reject(http.StatusUnprocessableEntity, "a book is one PDF, not %d", pdfs)
	}
	if pdfs == 1 && images > 0 {
		return reject(http.StatusUnprocessableEntity,
			"a book is either one PDF or a set of images, not both")
	}

	return nil
}

func (a *api) commitBook(ctx context.Context, name, tempDir string) (books.Book, error) {
	bookKey, err := books.NewStorageKey()
	if err != nil {
		return books.Book{}, err
	}

	sourceDir := a.deps.Files.SourceDir(bookKey)
	if err := os.MkdirAll(filepath.Dir(sourceDir), 0o755); err != nil {
		return books.Book{}, err
	}
	if err := os.Rename(tempDir, sourceDir); err != nil {
		return books.Book{}, err
	}

	book, err := a.insertBook(ctx, name, bookKey)
	if err != nil {
		os.RemoveAll(sourceDir)
		return books.Book{}, err
	}

	return book, nil
}

// The row and the queue entry share one transaction. A rollback leaves neither.
func (a *api) insertBook(ctx context.Context, name, bookKey string) (books.Book, error) {
	tx, err := a.deps.Pool.Begin(ctx)
	if err != nil {
		return books.Book{}, err
	}
	defer tx.Rollback(ctx)

	book, err := a.deps.Books.CreateTx(ctx, tx, name, bookKey)
	if err != nil {
		return books.Book{}, err
	}

	opts := &river.InsertOpts{Queue: ingest.QueueName, MaxAttempts: maxIngestTries}
	if _, err := a.deps.Queue.InsertTx(ctx, tx, ingest.Args{BookID: book.ID}, opts); err != nil {
		return books.Book{}, err
	}

	return book, tx.Commit(ctx)
}

func isTooLarge(err error) bool {
	var maxBytes *http.MaxBytesError
	return errors.As(err, &maxBytes)
}
