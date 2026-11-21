package ocr

import (
	"encoding/json"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

// The stored v6 result and the stored merged result are two shapes, and a
// renderer reads both with one call.
func TestLinesOfReadsBothStoredShapes(t *testing.T) {
	boxData, textData := fixturePageTen(t)

	v6Lines, err := LinesOf(boxData)
	if err != nil {
		t.Fatalf("LinesOf the v6 result: %v", err)
	}
	if len(v6Lines) != 22 {
		t.Fatalf("the v6 result gave %d lines, want 22", len(v6Lines))
	}
	if v6Lines[0].Box != (ocrmerge.Box{164, 926, 1359, 972}) {
		t.Errorf("v6 line 0 carries the box %v, want the box the engine wrote", v6Lines[0].Box)
	}
	if v6Lines[0].Text == "" {
		t.Error("v6 line 0 carries no text")
	}

	merged, err := MergeOf(boxData, textData)
	if err != nil {
		t.Fatalf("MergeOf: %v", err)
	}
	mergedData, err := json.Marshal(merged)
	if err != nil {
		t.Fatalf("marshal the merged result: %v", err)
	}

	mergedLines, err := LinesOf(mergedData)
	if err != nil {
		t.Fatalf("LinesOf the merged result: %v", err)
	}
	if len(mergedLines) != len(merged.Lines) {
		t.Fatalf("the merged result gave %d lines, want %d", len(mergedLines), len(merged.Lines))
	}
	for i, line := range mergedLines {
		if line.Box != merged.Lines[i].Box || line.Text != merged.Lines[i].Text {
			t.Fatalf("merged line %d reads %v %q, want %v %q",
				i, line.Box, line.Text, merged.Lines[i].Box, merged.Lines[i].Text)
		}
	}
}

func TestLinesOfRefusesAnUnreadableResult(t *testing.T) {
	if _, err := LinesOf([]byte("not json")); err == nil {
		t.Error("an unreadable result was accepted")
	}
}

// A page a renderer can draw holds the merged result or the v6 result, and the
// merged result comes first.
func TestRenderModelsPreferTheMergedResult(t *testing.T) {
	want := []string{ModelClaudeMerged, ModelMerged, ModeV6}
	if len(RenderModels) != len(want) {
		t.Fatalf("RenderModels holds %v, want %v", RenderModels, want)
	}
	for i, model := range want {
		if RenderModels[i] != model {
			t.Errorf("RenderModels is %v, want %v", RenderModels, want)
			break
		}
	}
}

func TestRankOfOrdersTheReadingsARendererCanDraw(t *testing.T) {
	ultra, ok := RankOf(ModelClaudeMerged)
	if !ok {
		t.Fatal("the Ultra reading has no rank")
	}
	merged, ok := RankOf(ModelMerged)
	if !ok {
		t.Fatal("the merged reading has no rank")
	}
	line, ok := RankOf(ModeV6)
	if !ok {
		t.Fatal("the line reading has no rank")
	}

	if !(ultra > merged && merged > line) {
		t.Errorf("the ranks are %d, %d and %d, want Ultra over merged over the line reading",
			ultra, merged, line)
	}
}

// A reading a renderer cannot draw never answers for a page, so it has no rank
// and takes over from nothing.
func TestRankOfRefusesAReadingWithNoLines(t *testing.T) {
	for _, model := range []string{ModeVL, ModeClaude, "nonsense", ""} {
		if _, ok := RankOf(model); ok {
			t.Errorf("%q carries a rank", model)
		}
	}
}
