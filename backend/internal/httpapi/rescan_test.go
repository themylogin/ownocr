package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/ocr"
)

// seedOCRBook builds a complete source book and a complete OCR copy of it that
// records the given profile. Every page of the copy holds a PP-OCRv6 result, and
// a page named in visual also holds the two results a reading by both engines
// leaves.
func (h *pageHarness) seedOCRBook(t *testing.T, profile string, pageCount int,
	visual ...int) books.Book {
	t.Helper()
	ctx := context.Background()

	source := h.createBook(t, "Read")
	sourcePages := make([]books.PageOfBook, 0, pageCount)
	for index := 1; index <= pageCount; index++ {
		sourcePages = append(sourcePages, h.addPage(t, source, index, nil, 400, 600, uint8(10*index)))
	}
	h.complete(t, source, pageCount)

	action, ok := books.ActionByID(books.ActionOCR)
	if !ok {
		t.Fatal("the ocr action is missing")
	}

	key, err := books.NewStorageKey()
	if err != nil {
		t.Fatalf("storage key: %v", err)
	}

	tx, err := h.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	read, err := h.store.CreateDerivedTx(ctx, tx, source.Name, key, source.ID, action, profile)
	if err != nil {
		t.Fatalf("CreateDerivedTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	readByVisual := make(map[int]bool, len(visual))
	for _, index := range visual {
		readByVisual[index] = true
	}

	for index := 1; index <= pageCount; index++ {
		page := h.addPage(t, read, index, &sourcePages[index-1].ID, 400, 600, uint8(10*index))
		h.addResult(t, page.ID, ocr.ModeV6, "v6 text", `{"lines": []}`)
		if readByVisual[index] {
			h.addResult(t, page.ID, ocr.ModeVL, "vl text", `{"parsing_res_list": []}`)
			h.addResult(t, page.ID, ocr.ModelMerged, "merged text", `{"lines": []}`)
		}
	}

	return h.complete(t, read, pageCount)
}

// The offer belongs to the book. Every page of a book that offers the rescan
// holds it, whatever results that page holds.
func TestBookDetailReportsTheRescanOffer(t *testing.T) {
	t.Run("a book read with the normal profile offers it", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		detail := h.detail(t, read.ID)
		if !detail.PageRescan {
			t.Error("the book does not offer the rescan, want it offered")
		}
		if len(detail.Pages) != 3 {
			t.Errorf("the book answers %d pages, want 3", len(detail.Pages))
		}
	})

	t.Run("a book read with the extra profile offers it", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileExtra, 3, 1, 2, 3)

		if detail := h.detail(t, read.ID); !detail.PageRescan {
			t.Error("the book does not offer the rescan, want it offered")
		}
	})

	t.Run("a part rescanned book offers it", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3, 2)

		if detail := h.detail(t, read.ID); !detail.PageRescan {
			t.Error("the book does not offer the rescan, want it offered")
		}
	})

	t.Run("a running book offers no rescan", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		if _, err := h.store.SetProcessing(context.Background(), read.ID,
			books.PhaseUploading); err != nil {
			t.Fatalf("SetProcessing: %v", err)
		}

		if detail := h.detail(t, read.ID); detail.PageRescan {
			t.Error("a running book offers the rescan, want it withheld")
		}
	})

	t.Run("a book of another stage offers no rescan", func(t *testing.T) {
		h := newPageHarness(t)
		_, dewarped := h.seedChain(t)

		if detail := h.detail(t, dewarped.ID); detail.PageRescan {
			t.Error("a dewarped book offers the rescan, want it withheld")
		}
	})
}

// postRescan sends a rescan request and answers the status and the body.
func (h *pageHarness) postRescan(t *testing.T, bookID int64, indexes []int,
	profile string) (int, []byte) {
	t.Helper()

	body, err := json.Marshal(rescanRequest{PageIndexes: indexes, Profile: profile})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/books/%d/rescan", bookID), bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, request)

	return recorder.Code, recorder.Body.Bytes()
}

// queuedRescans answers the arguments of every rescan job queued for a book.
func (h *pageHarness) queuedRescans(t *testing.T, bookID int64) []ocr.RescanArgs {
	t.Helper()

	rows, err := h.store.Pool().Query(context.Background(),
		`SELECT args FROM river_job WHERE kind = 'ocr_rescan'
		AND (args->>'bookId')::bigint = $1 ORDER BY id`, bookID)
	if err != nil {
		t.Fatalf("query jobs: %v", err)
	}
	defer rows.Close()

	var queued []ocr.RescanArgs
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scan: %v", err)
		}
		var args ocr.RescanArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			t.Fatalf("decode args: %v", err)
		}
		queued = append(queued, args)
	}
	return queued
}

func TestRescanQueuesOneJobAndSetsTheBookRunning(t *testing.T) {
	h := newPageHarness(t)
	read := h.seedOCRBook(t, ocr.ProfileNormal, 4)

	status, body := h.postRescan(t, read.ID, []int{2, 3}, ocr.ProfileExtra)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	queued := h.queuedRescans(t, read.ID)
	if len(queued) != 1 {
		t.Fatalf("the request queued %d jobs, want 1", len(queued))
	}
	if pages := queued[0].PageIndexes; len(pages) != 2 || pages[0] != 2 || pages[1] != 3 {
		t.Errorf("the job names %v, want [2 3]", pages)
	}

	after, err := h.store.Get(context.Background(), read.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.State != books.StateProcessing {
		t.Errorf("the book is %q, want %q", after.State, books.StateProcessing)
	}
	if after.PageCount != 4 {
		t.Errorf("the book reports %d pages, want 4", after.PageCount)
	}
}

// The profile of the request describes this rescan alone. The book keeps the
// profile it records, which describes the reading that built it.
func TestRescanCarriesTheProfileOfTheRequest(t *testing.T) {
	for _, profile := range []string{ocr.ProfileNormal, ocr.ProfileExtra} {
		t.Run(profile, func(t *testing.T) {
			h := newPageHarness(t)
			read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

			if status, body := h.postRescan(t, read.ID, []int{1}, profile); status != http.StatusOK {
				t.Fatalf("status = %d: %s", status, body)
			}

			queued := h.queuedRescans(t, read.ID)
			if len(queued) != 1 {
				t.Fatalf("the request queued %d jobs, want 1", len(queued))
			}
			if queued[0].Profile != profile {
				t.Errorf("the job carries the profile %q, want %q", queued[0].Profile, profile)
			}

			after, err := h.store.Get(context.Background(), read.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if after.OCRProfile == nil || *after.OCRProfile != ocr.ProfileNormal {
				t.Errorf("the book records %v, want %q", after.OCRProfile, ocr.ProfileNormal)
			}
		})
	}
}

func TestRescanCollapsesARepeatedPage(t *testing.T) {
	h := newPageHarness(t)
	read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

	status, body := h.postRescan(t, read.ID, []int{2, 2, 1}, ocr.ProfileExtra)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	queued := h.queuedRescans(t, read.ID)
	if len(queued) != 1 {
		t.Fatalf("the request queued %d jobs, want 1", len(queued))
	}
	if pages := queued[0].PageIndexes; len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
		t.Errorf("the job names %v, want [1 2]", pages)
	}
}

// A page the visual engine has read is read again like any other. A reading is
// a choice, not a rank.
func TestRescanAcceptsAPageTheVisualEngineHasRead(t *testing.T) {
	h := newPageHarness(t)
	read := h.seedOCRBook(t, ocr.ProfileNormal, 3, 2)

	status, body := h.postRescan(t, read.ID, []int{1, 2}, ocr.ProfileExtra)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	queued := h.queuedRescans(t, read.ID)
	if len(queued) != 1 {
		t.Fatalf("the request queued %d jobs, want 1", len(queued))
	}
	if pages := queued[0].PageIndexes; len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
		t.Errorf("the job names %v, want [1 2]", pages)
	}
}

func TestRescanRefusesTheWholeRequest(t *testing.T) {
	t.Run("an unknown book", func(t *testing.T) {
		h := newPageHarness(t)
		status, _ := h.postRescan(t, 999999, []int{1}, ocr.ProfileExtra)
		if status != http.StatusNotFound {
			t.Errorf("status = %d, want %d", status, http.StatusNotFound)
		}
	})

	t.Run("no profile", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		status, body := h.postRescan(t, read.ID, []int{1}, "")
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d: %s", status, body)
		}
		if errorOf(t, body) == "" {
			t.Error("the refusal names no reason")
		}
		if queued := h.queuedRescans(t, read.ID); len(queued) != 0 {
			t.Errorf("the refused request queued %v", queued)
		}
		after, _ := h.store.Get(context.Background(), read.ID)
		if after.State != books.StateComplete {
			t.Errorf("the book is %q after a refusal, want %q", after.State, books.StateComplete)
		}
	})

	t.Run("an unknown profile", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		status, body := h.postRescan(t, read.ID, []int{1}, "nonsense")
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d: %s", status, body)
		}
		if errorOf(t, body) == "" {
			t.Error("the refusal names no reason")
		}
		if queued := h.queuedRescans(t, read.ID); len(queued) != 0 {
			t.Errorf("the refused request queued %v", queued)
		}
	})

	t.Run("no page", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		status, body := h.postRescan(t, read.ID, nil, ocr.ProfileExtra)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d: %s", status, body)
		}
		if errorOf(t, body) == "" {
			t.Error("the refusal names no reason")
		}
		if queued := h.queuedRescans(t, read.ID); len(queued) != 0 {
			t.Errorf("the refused request queued %v", queued)
		}
	})

	t.Run("a page the book does not hold", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		status, body := h.postRescan(t, read.ID, []int{1, 900}, ocr.ProfileExtra)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d: %s", status, body)
		}
		if queued := h.queuedRescans(t, read.ID); len(queued) != 0 {
			t.Errorf("the refused request queued %v, want nothing for page 1 either", queued)
		}
		after, _ := h.store.Get(context.Background(), read.ID)
		if after.State != books.StateComplete {
			t.Errorf("the book is %q after a refusal, want %q", after.State, books.StateComplete)
		}
	})

	t.Run("a book of another stage", func(t *testing.T) {
		h := newPageHarness(t)
		_, dewarped := h.seedChain(t)

		status, body := h.postRescan(t, dewarped.ID, []int{1}, ocr.ProfileExtra)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d: %s", status, body)
		}
	})

	t.Run("a book that is not complete", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		if _, err := h.store.SetProcessing(context.Background(), read.ID,
			books.PhaseUploading); err != nil {
			t.Fatalf("SetProcessing: %v", err)
		}

		status, body := h.postRescan(t, read.ID, []int{1}, ocr.ProfileExtra)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d: %s", status, body)
		}
		if queued := h.queuedRescans(t, read.ID); len(queued) != 0 {
			t.Errorf("the refused request queued %v", queued)
		}
	})

	// The in-flight guard walks the chain downward, from the book to the copies
	// made from it. An OCR book is the end of the pipeline and has no copy, so
	// a second rescan is the only work the guard can find. It finds it through
	// the state of the book itself, which the case above covers.
	t.Run("a second rescan while the first runs", func(t *testing.T) {
		h := newPageHarness(t)
		read := h.seedOCRBook(t, ocr.ProfileNormal, 3)

		status, body := h.postRescan(t, read.ID, []int{1}, ocr.ProfileExtra)
		if status != http.StatusOK {
			t.Fatalf("the first rescan = %d: %s", status, body)
		}

		status, body = h.postRescan(t, read.ID, []int{2}, ocr.ProfileExtra)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("the second rescan = %d: %s", status, body)
		}

		if queued := h.queuedRescans(t, read.ID); len(queued) != 1 {
			t.Errorf("the book holds %d rescan jobs, want the 1 that started", len(queued))
		}
	})
}
