package ocrmerge

import (
	"testing"
)

// quad answers a line whose corners fall by `fall` pixels from left to right, and
// whose box is the rectangle round those corners.
func quad(left, right, bottom, height, fall int, text string) Line {
	lowLeft, lowRight := bottom, bottom-fall
	highLeft, highRight := lowLeft-height, lowRight-height

	top := min(highLeft, highRight)
	low := max(lowLeft, lowRight)
	return Line{
		Box:  Box{left, top, right, low},
		Text: text,
		Corners: []Point{
			{left, highLeft}, {right, highRight}, {right, lowRight}, {left, lowLeft},
		},
	}
}

// rowsFor puts every line of a slice in one row of one block, which is what the
// merge hands the levelling.
func rowsFor(count int) map[int][][]int {
	row := make([]int, count)
	for i := range row {
		row[i] = i
	}
	return map[int][][]int{0: {row}}
}

func TestLevellingReadsTheBaselineAtTheMiddleOfTheRow(t *testing.T) {
	// One printed row read as a short left piece and a long right piece, falling
	// 28 pixels over the 1623 it covers.
	lines := []Line{
		quad(112, 261, 462, 73, 3, "viaje;"),
		quad(231, 1735, 460, 98, 26, "etcétera. Incluso su figura"),
	}
	levelled, moved := levelRows(lines, rowsFor(2), nil, 80)

	if moved != 1 {
		t.Fatalf("levelled %d rows, want 1", moved)
	}
	first, second := levelled[0].Box, levelled[1].Box
	if first[1] != second[1] || first[3] != second[3] {
		t.Errorf("the pieces answer %v and %v, want one top and one bottom",
			first, second)
	}
	if first[0] != 112 || first[2] != 261 {
		t.Errorf("the first piece answers the column %d to %d, want 112 to 261",
			first[0], first[2])
	}
	if second[0] != 231 || second[2] != 1735 {
		t.Errorf("the second piece answers the column %d to %d, want 231 to 1735",
			second[0], second[2])
	}
}

// The slope and the bound are two gates, and this is the row where the slope
// alone answers: 124 tall stands over the 110 the page allows, and taking the
// slope off leaves 101, which stands under it.
func TestLevellingTakesTheSlopeOffTheHeight(t *testing.T) {
	lines := []Line{quad(100, 1741, 460, 101, 23, "a line of the page")}
	levelled, moved := levelRows(lines, rowsFor(1), nil, 110)

	if moved != 1 {
		t.Fatalf("levelled %d rows, want 1", moved)
	}
	if got := levelled[0].Box[3] - levelled[0].Box[1]; got != 101 {
		t.Errorf("the row answers the height %d, want 101 and not the bound 110", got)
	}
}

func TestLevellingNeverStandsTallerThanTheRowSpacing(t *testing.T) {
	// Page 85 of book 17: a row 124 tall over 1641, falling 0.014, on a page whose
	// printed rows stand 80 apart. The slope alone leaves 102, which is still
	// taller than the distance to the next row.
	lines := []Line{quad(100, 1741, 460, 102, 22, "a line of the page")}
	levelled, moved := levelRows(lines, rowsFor(1), nil, 80)

	if moved != 1 {
		t.Fatalf("levelled %d rows, want 1", moved)
	}
	if got := levelled[0].Box[3] - levelled[0].Box[1]; got != 80 {
		t.Errorf("the row answers the height %d, want the row spacing 80", got)
	}
}

func TestLevellingLeavesARowThatMeasuresUnderTheBound(t *testing.T) {
	// Page 10 of book 3292: 44 tall with no slope worth the name, on a page whose
	// rows stand 53 apart. Such a row already draws as one straight line.
	lines := []Line{quad(234, 1357, 1025, 44, 1, "a line of a level page")}
	before := lines[0].Box

	levelled, moved := levelRows(lines, rowsFor(1), nil, 53)

	if moved != 0 {
		t.Errorf("levelled %d rows, want none", moved)
	}
	if levelled[0].Box != before {
		t.Errorf("the row answers %v, want the box it came with %v",
			levelled[0].Box, before)
	}
}

func TestLevellingRefusesAPageWithNoRowSpacing(t *testing.T) {
	lines := []Line{quad(100, 1741, 460, 102, 22, "a line of the page")}
	before := lines[0].Box

	levelled, moved := levelRows(lines, rowsFor(1), nil, 0)

	if moved != 0 {
		t.Errorf("levelled %d rows, want none", moved)
	}
	if levelled[0].Box != before {
		t.Errorf("the row answers %v, want the box it came with %v",
			levelled[0].Box, before)
	}
}

func TestLevellingRefusesARowWithNoCorners(t *testing.T) {
	// A result stored before the merge read corners carries none.
	lines := []Line{
		{Box: Box{112, 375, 261, 461}, Text: "viaje;"},
		{Box: Box{231, 336, 1735, 459}, Text: "etcétera. Incluso su figura"},
	}
	before := []Box{lines[0].Box, lines[1].Box}

	levelled, moved := levelRows(lines, rowsFor(2), nil, 80)

	if moved != 0 {
		t.Errorf("levelled %d rows, want none", moved)
	}
	for i, want := range before {
		if levelled[i].Box != want {
			t.Errorf("line %d answers %v, want %v", i, levelled[i].Box, want)
		}
	}
}

func TestLevellingLeavesTheLinesOfAReplacedBlockAlone(t *testing.T) {
	lines := []Line{quad(100, 1741, 460, 102, 22, "a line of a disaster block")}
	before := lines[0].Box

	levelled, moved := levelRows(lines, rowsFor(1), map[int]bool{0: true}, 80)

	if moved != 0 {
		t.Errorf("levelled %d rows, want none", moved)
	}
	if levelled[0].Box != before {
		t.Errorf("the row answers %v, want the box it came with %v",
			levelled[0].Box, before)
	}
}

func TestLevellingDoesNotTouchTheCallersLines(t *testing.T) {
	lines := []Line{quad(100, 1741, 460, 102, 22, "a line of the page")}
	before := lines[0].Box

	if _, moved := levelRows(lines, rowsFor(1), nil, 80); moved != 1 {
		t.Fatalf("levelled %d rows, want 1", moved)
	}
	if lines[0].Box != before {
		t.Errorf("the caller's line now carries %v, want %v", lines[0].Box, before)
	}
}
