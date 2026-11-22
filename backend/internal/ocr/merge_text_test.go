package ocr

import (
	"fmt"
	"strings"
	"testing"
)

// pageOfLines is a v6 result of a page of running prose: fourteen full lines,
// each long enough that the merge measures how well the two readings agree
// rather than accepting it unread.
func pageOfLines() []byte {
	lines := []string{
		"El sentido de la palabra no reside en la cosa",
		"nombrada sino en el uso que de ella hacemos.",
		"Aristoteles llama a esto la significacion",
		"primera, y distingue entre aquello que se dice",
		"de suyo y aquello que se dice por accidente.",
		"La cuestion aparece tratada en el libro cuarto",
		"de la Metafisica, donde el filosofo sostiene",
		"que ninguna demostracion alcanza a los",
		"principios. Aquel que niega el principio de",
		"contradiccion no puede argumentar, porque todo",
		"argumento lo supone. Por eso la refutacion no",
		"procede contra la tesis sino contra quien la",
		"sostiene. El adversario habla y al hablar",
		"concede lo que pretende negar.",
	}

	var rows []string
	for i, text := range lines {
		top := 200 + i*60
		rows = append(rows, fmt.Sprintf(
			`{"bbox": [150, %d, 1850, %d], "text": %q, "score": 0.98}`,
			top, top+45, text))
	}
	return []byte(`{"raw": {}, "lines": [` + strings.Join(rows, ",") + `]}`)
}

func textOfThePage() string {
	lines := []string{
		"El sentido de la palabra no reside en la cosa",
		"nombrada sino en el uso que de ella hacemos.",
		"Aristóteles llama a esto la significación",
		"primera, y distingue entre aquello que se dice",
		"de suyo y aquello que se dice por accidente.",
		"La cuestión aparece tratada en el libro cuarto",
		"de la Metafísica, donde el filósofo sostiene",
		"que ninguna demostración alcanza a los",
		"principios. Aquel que niega el principio de",
		"contradicción no puede argumentar, porque todo",
		"argumento lo supone. Por eso la refutación no",
		"procede contra la tesis sino contra quien la",
		"sostiene. El adversario habla y al hablar",
		"concede lo que pretende negar.",
	}
	return strings.Join(lines, "\n")
}

// A transcription of the page merges onto the boxes of that page, and every line
// of the answer carries a box the box engine read.
func TestMergeOfTextPlacesATranscriptionOnTheLineBoxes(t *testing.T) {
	merged, read, err := MergeOfText(pageOfLines(), textOfThePage(), 2000, 3000)
	if err != nil {
		t.Fatalf("MergeOfText: %v", err)
	}
	if !read {
		t.Fatalf("a transcription of the page reads as no transcription: %+v", merged.Counts)
	}

	if merged.Counts.Discarded != 0 {
		t.Errorf("the merge discarded %d boxes of the page", merged.Counts.Discarded)
	}
	if merged.Counts.Estimated != 0 {
		t.Errorf("the merge placed %d boxes of its own", merged.Counts.Estimated)
	}
	if merged.Counts.Lines != 14 {
		t.Errorf("the merge answers %d lines, want the 14 the box engine read",
			merged.Counts.Lines)
	}
	if merged.Counts.FromText != 14 {
		t.Errorf("%d lines carry the text of the transcription, want 14",
			merged.Counts.FromText)
	}

	// The accents of the transcription are what the Ultra rescan exists for, so
	// they have to reach the merged lines.
	if !strings.Contains(MergedText(merged), "Aristóteles") {
		t.Errorf("the merged text lost the accents: %q", MergedText(merged))
	}
}

// An answer that is no transcription of this page aligns with no line. The merge
// then falls back to the box engine for every line, and the result would be
// stored as a better reading holding none of its own text.
func TestMergeOfTextRefusesAnAnswerThatIsNoTranscription(t *testing.T) {
	narration := "I'm looking at this image, and it appears to be a blank page — " +
		"essentially an empty scanned leaf with no legible text on it, so there is " +
		"nothing here for me to transcribe for you."

	merged, read, err := MergeOfText(pageOfLines(), narration, 2000, 3000)
	if err != nil {
		t.Fatalf("MergeOfText: %v", err)
	}
	if read {
		t.Errorf("a description of the page reads as a transcription: %+v", merged.Counts)
	}
}

func TestMergeOfTextRefusesAPageWithNoLines(t *testing.T) {
	_, read, err := MergeOfText([]byte(`{"raw": {}, "lines": []}`), textOfThePage(), 2000, 3000)
	if err != nil {
		t.Fatalf("MergeOfText: %v", err)
	}
	if read {
		t.Error("a page the box engine read nothing on reads as a transcription")
	}
}

func TestMergeOfTextNamesAnUnreadableBoxResult(t *testing.T) {
	_, _, err := MergeOfText([]byte(`not json`), textOfThePage(), 2000, 3000)
	if err == nil {
		t.Fatal("an unreadable box result was accepted")
	}
	if !strings.Contains(err.Error(), ModeV6) {
		t.Errorf("error %q does not name the result it could not read", err)
	}
}

// The block has to hold the lines it merges. A page carrying a stored size that
// does not describe its image would otherwise own none of its own lines, and the
// whole page would read as no transcription of itself.
func TestMergeOfTextCoversTheLinesOfAPageOfTheWrongStoredSize(t *testing.T) {
	merged, read, err := MergeOfText(pageOfLines(), textOfThePage(), 100, 200)
	if err != nil {
		t.Fatalf("MergeOfText: %v", err)
	}
	if !read {
		t.Fatalf("a page of the wrong stored size reads as no transcription: %+v",
			merged.Counts)
	}
	if merged.Counts.FromText != 14 {
		t.Errorf("%d lines carry the transcription, want 14", merged.Counts.FromText)
	}
}

// The markdown of PaddleOCR-VL leaves out the footnote and the page number
// blocks, so it is not the text that engine read. A judge that took the markdown
// for the reading would call every page carrying a footnote truncated.
func TestReadingTextOfTheVisualEngineHoldsEveryBlock(t *testing.T) {
	const markdown = "the body of the page"
	const data = `{"parsing_res_list": [
		{"block_label": "text", "block_bbox": [1, 1, 2, 2], "block_content": "the body of the page"},
		{"block_label": "footnote", "block_bbox": [1, 3, 2, 4], "block_content": "4 the note the markdown drops"},
		{"block_label": "number", "block_bbox": [1, 5, 2, 6], "block_content": "15"}
	]}`

	text := ReadingText(ModeVL, markdown, []byte(data))

	for _, want := range []string{"the body of the page", "the note the markdown drops", "15"} {
		if !strings.Contains(text, want) {
			t.Errorf("the reading text %q does not hold %q", text, want)
		}
	}
	if len(text) <= len(markdown) {
		t.Errorf("the reading text is no longer than the markdown: %q", text)
	}
}

// An engine that answers text and no blocks wrote what it wrote.
func TestReadingTextOfATextEngineIsItsText(t *testing.T) {
	if got := ReadingText(ModeGLM, "the page", []byte(`{}`)); got != "the page" {
		t.Errorf("ReadingText = %q", got)
	}
	if got := ReadingText(ModeV6, "the page", nil); got != "the page" {
		t.Errorf("ReadingText = %q", got)
	}
}

// A result this cannot read, or one holding no block, falls back to the text the
// engine wrote rather than answering nothing.
func TestReadingTextFallsBackToTheMarkdown(t *testing.T) {
	for _, data := range []string{`not json`, `{"parsing_res_list": []}`} {
		if got := ReadingText(ModeVL, "the page", []byte(data)); got != "the page" {
			t.Errorf("ReadingText(%s) = %q", data, got)
		}
	}
}
