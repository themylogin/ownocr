package ocr

import (
	"encoding/json"
	"fmt"

	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

// RenderModels names the results a renderer can draw, best first. A merged
// result holds the boxes of PP-OCRv6 with the text of a reading engine, so it
// wins wherever a page holds one. The raw PaddleOCR-VL and Claude results hold
// blocks of reflowed text and no lines, so neither is on the list.
//
// It doubles as the rank a better reading takes over by, so one list is the
// single source of "better".
var RenderModels = []string{ModelClaudeMerged, ModelMerged, ModeV6}

// textEngines names, for each merged reading, the engine whose text it carries.
var textEngines = map[string]string{
	ModelMerged:       ModeVL,
	ModelClaudeMerged: ModeClaude,
}

// TextEngineOf answers the engine whose raw text a merged reading carries. The
// second answer is false for a reading that is no merge of two engines.
//
// A merged reading holds that engine's text only where it aligned with a line
// box, and PP-OCRv6 text everywhere else. Anything judging the reading engine
// therefore has to read what that engine wrote, not the merge built from it.
func TextEngineOf(model string) (string, bool) {
	engine, merged := textEngines[model]
	return engine, merged
}

// ReadingOrder lists the drawable readings as a reader is offered them: Normal,
// then Extra, then Ultra. It is RenderModels read backwards, because that list is
// the order of preference and this is the order of the menu — the same order the
// dialog that starts a rescan offers its models in.
func ReadingOrder() []string {
	order := make([]string, 0, len(RenderModels))
	for i := len(RenderModels) - 1; i >= 0; i-- {
		order = append(order, RenderModels[i])
	}
	return order
}

// RankOf answers how good a reading is, higher being better. The second answer
// is false for a model a renderer cannot draw, which therefore never answers for
// a page and never takes over from anything.
func RankOf(model string) (int, bool) {
	for i, name := range RenderModels {
		if name == model {
			return len(RenderModels) - i, true
		}
	}
	return 0, false
}

// StoredLine is one line of a stored result as a renderer reads it.
//
// Source names the engine its text came from. A merged result records one per
// line, because the merge takes the text of a line from the reading engine where
// the two engines read it alike and from the box engine where they did not. An
// engine result records none: every line of it is that engine's own.
type StoredLine struct {
	ocrmerge.Line
	Source string `json:"source"`
}

type storedResult struct {
	Lines []StoredLine `json:"lines"`
}

// LinesOf reads the lines of a stored result. The PP-OCRv6 result and the
// merged result both hold them under `lines`, with the same box and text, so
// one reader serves both.
func LinesOf(data []byte) ([]StoredLine, error) {
	var result storedResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("the stored result: %w", err)
	}
	return result.Lines, nil
}

// SubstitutedLine answers whether one line of a reading carries the text of the
// line engine rather than of the engine that reading is named for.
//
// Only a merged reading can hold such a line. An engine result holds its own text
// throughout, so nothing in it is substituted and a reader is told nothing.
func SubstitutedLine(model, source string) bool {
	if _, merged := TextEngineOf(model); !merged {
		return false
	}
	return source == ModeV6
}

// readingNames is what a reader sees for a drawable reading: the profile that
// made it and the engine behind it. A reader chooses between readings by these
// names and never by the name of a stored model.
var readingNames = map[string]struct{ Profile, Engine string }{
	ModeV6:            {"Normal", "PP-OCRv6"},
	ModelMerged:       {"Extra", "PaddleOCR-VL"},
	ModelClaudeMerged: {"Ultra", "Claude Opus 5"},
}

// ReadingNamesOf answers the profile and the engine of a drawable reading. The
// third answer is false for a reading a renderer cannot draw, which a reader is
// never offered.
func ReadingNamesOf(model string) (string, string, bool) {
	names, drawable := readingNames[model]
	return names.Profile, names.Engine, drawable
}
