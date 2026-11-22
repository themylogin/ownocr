package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/ocr"
)

// glmMergedData is a merged Ultra reading: the boxes of the line reading
// carrying the text GLM-5.3 Flash answered.
const glmMergedData = `{"lines": [
	{"bbox": [100, 120, 300, 160], "text": "τὴν φιλοσοφίαν", "source": "glm", "score": 0.99},
	{"bbox": [100, 180, 320, 220], "text": "the second line read again", "source": "glm", "score": 0.98}
], "dropped": [], "counts": {"lines": 2, "fromText": 2, "fromBoxes": 0, "droppedChars": 0}}`

func readingsByProfile(readings []readingView) map[string]readingView {
	byProfile := make(map[string]readingView, len(readings))
	for _, reading := range readings {
		byProfile[reading.Profile] = reading
	}
	return byProfile
}

func TestThePageAnswerReportsOneReading(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	view := h.pageOCRView(t, book.ID, 1)

	if len(view.Readings) != 1 {
		t.Fatalf("the page reports %+v, want one reading", view.Readings)
	}
	reading := view.Readings[0]
	if reading.Profile != "Normal" || reading.Engine != "PP-OCRv6" {
		t.Errorf("the reading is named %q (%q)", reading.Profile, reading.Engine)
	}
	if !reading.Answers {
		t.Error("the one reading of the page does not answer")
	}
}

func TestThePageAnswerReportsEveryDrawableReading(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	h.addResult(t, page.ID, ocr.ModeVL, "reflowed text", `{"parsing_res_list": []}`)
	h.addResult(t, page.ID, ocr.ModelMerged, "τὴν φιλοσοφίαν", mergedData)
	h.addResult(t, page.ID, ocr.ModeGLM, "plain text", `{"text": "plain text"}`)
	h.addResult(t, page.ID, ocr.ModelGLMMerged, "τὴν φιλοσοφίαν", glmMergedData)

	view := h.pageOCRView(t, book.ID, 1)

	if len(view.Readings) != 3 {
		t.Fatalf("the page reports %d readings, want the 3 a renderer can draw: %+v",
			len(view.Readings), view.Readings)
	}

	byProfile := readingsByProfile(view.Readings)
	for profile, engine := range map[string]string{
		"Normal": "PP-OCRv6",
		"Extra":  "PaddleOCR-VL",
		"Ultra":  "GLM-5.3 Flash",
	} {
		reading, reported := byProfile[profile]
		if !reported {
			t.Errorf("the page reports no %q reading", profile)
			continue
		}
		if reading.Engine != engine {
			t.Errorf("the %q reading names the engine %q, want %q", profile,
				reading.Engine, engine)
		}
	}

	// The raw readings hold no lines, so neither is offered.
	for _, reading := range view.Readings {
		if reading.Model == ocr.ModeVL || reading.Model == ocr.ModeGLM {
			t.Errorf("a raw reading is offered: %+v", reading)
		}
	}

	// Exactly one answers, and it is the best ranked.
	answering := ""
	for _, reading := range view.Readings {
		if reading.Answers {
			if answering != "" {
				t.Errorf("two readings answer: %q and %q", answering, reading.Model)
			}
			answering = reading.Model
		}
	}
	if answering != ocr.ModelGLMMerged {
		t.Errorf("the reading that answers is %q, want the Ultra reading", answering)
	}
	if view.Model != ocr.ModelGLMMerged {
		t.Errorf("the page answers the model %q", view.Model)
	}
}

func TestAReaderChoosesTheLineReading(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	h.addResult(t, page.ID, ocr.ModelMerged, "τὴν φιλοσοφίαν", mergedData)

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/reading", book.ID), `{"model": "v6"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, body)
	}

	// The answer is the page it just changed, so the reader redraws from it.
	var view pageOCRView
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.Model != ocr.ModeV6 {
		t.Errorf("the answer names the model %q, want the line reading", view.Model)
	}
	if len(view.Lines) == 0 || view.Lines[0].Text != "the first line" {
		t.Errorf("the answer holds the lines %+v", view.Lines)
	}

	// And it survives the next request.
	again := h.pageOCRView(t, book.ID, 1)
	if again.Model != ocr.ModeV6 {
		t.Errorf("the page answers %q on the next request", again.Model)
	}
	if mark := readingsByProfile(again.Readings)["Normal"]; !mark.Answers {
		t.Error("the mark did not move to the reading the reader chose")
	}
}

func TestSettingAReadingThePageDoesNotHoldIsRefused(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/reading", book.ID),
		`{"model": "glm_merged"}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s", status, body)
	}
	if !strings.Contains(string(body), "glm_merged") {
		t.Errorf("the refusal %q does not name the reading", body)
	}

	// The page answers what it answered.
	if view := h.pageOCRView(t, book.ID, 1); view.Model != ocr.ModeV6 {
		t.Errorf("the page answers %q after the refusal", view.Model)
	}
}

func TestSettingAReadingWithNoLinesIsRefused(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	h.addResult(t, page.ID, ocr.ModeVL, "reflowed text", `{"parsing_res_list": []}`)

	status, body := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/reading", book.ID), `{"model": "vl"}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d: %s", status, body)
	}
	if !strings.Contains(string(body), "no lines") {
		t.Errorf("the refusal %q does not say the reading holds no lines", body)
	}
}

func TestSettingNoReadingIsRefused(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line", v6Data)

	status, _ := h.send(t, http.MethodPost,
		fmt.Sprintf("/api/books/%d/pages/1/reading", book.ID), `{}`)
	if status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want a refusal", status)
	}
}

// A book whose pages hold one reading each reports no disagreement: one engine
// cannot disagree with itself.
func TestABookOfOneReadingPerPageReportsNoDisagreement(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	view := h.bookOCRView(t, book.ID)

	if view.Disagreeing != 0 {
		t.Errorf("the book reports %d disagreeing pages", view.Disagreeing)
	}
	for _, page := range view.Pages {
		if page.Disagrees {
			t.Errorf("page %d disagrees", page.Index)
		}
	}
}

// The reading engine abandoned this page: it wrote 14 characters where the line
// engine read a page of prose. The merged reading is as long as the line reading,
// because the merge filled it out with line text, so only the raw reading shows it.
func TestABookReportsThePageWhoseReadingEngineAbandonedIt(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, longLineReading, v6Data)
	h.addResult(t, page.ID, ocr.ModeVL, "una palabra", `{"parsing_res_list": []}`)
	h.addResult(t, page.ID, ocr.ModelMerged, longLineReading, mergedData)

	view := h.bookOCRView(t, book.ID)

	if view.Disagreeing != 1 {
		t.Fatalf("the book reports %d disagreeing pages, want 1", view.Disagreeing)
	}
	if !view.Pages[0].Disagrees {
		t.Error("the page does not say it disagrees")
	}
}

// The one page answer and the whole book answer may never name two models for one
// page, including a page that names a reading.
func TestThePageAnswerAndTheBookAnswerNameOneModel(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	h.addResult(t, page.ID, ocr.ModelMerged, "τὴν φιλοσοφίαν", mergedData)
	h.addResult(t, page.ID, ocr.ModelGLMMerged, "τὴν φιλοσοφίαν", glmMergedData)

	for _, model := range []string{"", ocr.ModeV6, ocr.ModelMerged, ocr.ModelGLMMerged} {
		if model != "" {
			status, body := h.send(t, http.MethodPost,
				fmt.Sprintf("/api/books/%d/pages/1/reading", book.ID),
				fmt.Sprintf(`{"model": %q}`, model))
			if status != http.StatusOK {
				t.Fatalf("choosing %s: status = %d: %s", model, status, body)
			}
		}

		alone := h.pageOCRView(t, book.ID, 1)
		whole := h.bookOCRView(t, book.ID)
		if len(whole.Pages) == 0 {
			t.Fatal("the book answer holds no page")
		}
		if alone.Model != whole.Pages[0].Model {
			t.Errorf("after choosing %q the page answers %q alone and %q in the book",
				model, alone.Model, whole.Pages[0].Model)
		}
	}
}

// A reader asks the page what is wrong with it, so the book answer carries the
// words beside the flag.
func TestTheBookAnswerSaysWhyAPageDisagrees(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)

	h.addResult(t, page.ID, ocr.ModeV6, longLineReading, v6Data)
	h.addResult(t, page.ID, ocr.ModeVL, "una palabra", `{"parsing_res_list": []}`)
	h.addResult(t, page.ID, ocr.ModelMerged, longLineReading, mergedData)

	view := h.bookOCRView(t, book.ID)
	reasons := strings.Join(view.Pages[0].Disagreement, " | ")

	if reasons == "" {
		t.Fatal("a disagreeing page explains nothing")
	}
	for _, want := range []string{"% of the text", "% of words"} {
		if !strings.Contains(reasons, want) {
			t.Errorf("the reasons %q do not hold %q", reasons, want)
		}
	}
}

func TestAPageThatAgreesExplainsNothing(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	view := h.bookOCRView(t, book.ID)
	if len(view.Pages[0].Disagreement) != 0 {
		t.Errorf("a page that agrees explains %v", view.Pages[0].Disagreement)
	}
}

// A merged reading holds the text of its reading engine where the two engines read
// a line alike and the text of the line engine where they did not, so every line
// says which of the two is on the page.
func TestTheAnswerSaysWhichLinesCameFromTheLineEngine(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)
	h.addResult(t, page.ID, ocr.ModelMerged, "τὴν φιλοσοφίαν", mergedData)

	view := h.pageOCRView(t, book.ID, 1)

	if len(view.Lines) != 2 {
		t.Fatalf("the page answers %d lines", len(view.Lines))
	}
	// mergedData holds one line from the reading engine and one from the line
	// engine.
	if view.Lines[0].FromLineReading {
		t.Error("a line of the reading engine says it came from the line engine")
	}
	if !view.Lines[1].FromLineReading {
		t.Error("a line the merge filled from the line engine does not say so")
	}
}

// An engine result holds its own text throughout, so nothing in it is substituted
// and a reader is told nothing.
func TestALineReadingNamesNoSubstitutedLine(t *testing.T) {
	h := newPageHarness(t)
	book, page := h.seedRead(t)
	h.addResult(t, page.ID, ocr.ModeV6, "the first line\nthe second line", v6Data)

	for _, line := range h.pageOCRView(t, book.ID, 1).Lines {
		if line.FromLineReading {
			t.Errorf("a line of the line reading says it came from the line engine: %q",
				line.Text)
		}
	}
}

// longLineReading is a page of prose, long enough that the share of its wording
// and the share of its length mean something.
const longLineReading = `El sentido de la palabra no reside en la cosa nombrada
sino en el uso que de ella hacemos. Aristoteles llama a esto la significacion
primera, y distingue entre aquello que se dice de suyo y aquello que se dice por
accidente. La cuestion aparece tratada en el libro cuarto de la Metafisica,
donde el filosofo sostiene que ninguna demostracion alcanza a los principios.
Aquel que niega el principio de contradiccion no puede argumentar, porque todo
argumento lo supone. Por eso la refutacion no procede contra la tesis sino
contra quien la sostiene.`
