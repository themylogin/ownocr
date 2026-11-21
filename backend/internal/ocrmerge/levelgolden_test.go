package ocrmerge

import (
	"sort"
	"testing"
)

// What levelling answers for every fixture, page by page. The design of the
// align-broken-rows change holds the measurement each row was read off, and a
// change that moves any of these has to be argued for there first.
//
// A page levels a row for one of two reasons: the row stands taller than the
// distance to the next row, or the box engine read it in pieces that do not line
// up. A page with neither levels nothing.
var levelledPages = map[string]struct {
	pitch    int
	levelled int
	why      string
}{
	// Pages that do not slope and hold no broken row.
	"page0010": {53, 0, "level, every row under the row spacing"},
	"page0014": {53, 0, "level"},
	"page0116": {53, 0, "level"},
	"page0119": {53, 0, "level"},

	// Level, but the box engine drew two boxes over one printed row. That row
	// levels and nothing else does.
	"page0011": {53, 1, "one printed row read as two boxes"},

	// Barely sloping, and its boxes stand a pixel or two over the row spacing.
	"book0018page0036": {51, 16, "boxes just over the row spacing"},

	// Sloping pages. Every row of them stands taller than the row spacing.
	"book0017page0085": {79, 33, "slopes 0.014, and holds broken rows"},
	"book0017page0087": {81, 26, "slopes 0.044, the steepest measured"},
	"book0017page0090": {75, 31, "slopes 0.027"},
	"book0017page0102": {75, 31, "slopes 0.010"},
	"book0017page0103": {76, 34, "slopes 0.019"},

	// Results stored before the merge read corners. A line with no corners is
	// level by definition, so such a page keeps every box however tall it stands.
	"page0052":         {74, 0, "no corners"},
	"page0432":         {46, 0, "no corners"},
	"book0017page0116": {80, 0, "no corners"},
}

func TestLevellingAnswersTheMeasuredPages(t *testing.T) {
	for _, f := range loadFixtures(t) {
		want, named := levelledPages[f.Name]
		if !named {
			t.Errorf("%s is not in the measured table", f.Name)
			continue
		}
		if got := Merge(f.input()).Counts.Levelled; got != want.levelled {
			t.Errorf("%s levelled %d rows, want %d (%s)",
				f.Name, got, want.levelled, want.why)
		}
	}
}

// The guarantee the whole change rests on: no line the merge answers stands
// taller than the distance to the next printed row. Two printed rows do not
// overlap, so a box that tall cannot be one row.
//
// A page whose lines report no corners is exempt. The merge cannot measure a
// slope there, so it changes nothing and the page reads as it always did.
//
// A line the merge placed itself is exempt too. The interpolation tiles a block
// by the rows of that block, not by the median of the page, and the two differ
// where a block stands at its own scale: the top block of page 87 tiles at 95
// where the rest of that page stands 81 apart.
func TestNoAnsweredLineStandsTallerThanTheRowSpacing(t *testing.T) {
	for _, f := range loadFixtures(t) {
		if cornerlessFixtures[f.Name] {
			continue
		}

		in := f.input()
		result := Merge(in)
		pitch := levelledPages[f.Name].pitch

		for i, line := range result.Lines {
			if line.Estimated {
				continue
			}
			if got := line.Box[3] - line.Box[1]; got > pitch {
				t.Errorf("%s line %d stands %d tall on a page whose rows are %d apart: %q",
					f.Name, i, got, pitch, line.Text)
			}
		}
	}
}

// A page the merge levels nothing on answers exactly what it answered before
// levelling existed. This is the guard on the cost of the change: a book it has
// nothing to fix must not be rewritten.
func TestAPageThatLevelsNothingKeepsEveryBox(t *testing.T) {
	for _, f := range loadFixtures(t) {
		in := f.input()
		result := Merge(in)
		if result.Counts.Levelled != 0 {
			continue
		}

		given := map[Box]bool{}
		for _, line := range in.Lines {
			given[line.Box] = true
		}
		for i, line := range result.Lines {
			if line.Estimated {
				continue
			}
			if !given[line.Box] {
				t.Errorf("%s line %d carries %v, which the box engine never drew",
					f.Name, i, line.Box)
			}
		}
	}
}

// Page 85 holds the row this change was reported for. Its two pieces stand on one
// baseline at one height, and each keeps the column it covered.
func TestPageEightyFiveLevelsItsBrokenRow(t *testing.T) {
	result := Merge(fixtureOf(t, "book0017page0085").input())

	var pieces []MergedLine
	for _, line := range result.Lines {
		switch line.Text {
		case "viaje;", "etcétera. Incluso su figura individual está hecha a la":
			pieces = append(pieces, line)
		}
	}
	if len(pieces) != 2 {
		t.Fatalf("found %d pieces of the row, want 2", len(pieces))
	}
	sort.Slice(pieces, func(a, b int) bool { return pieces[a].Box[0] < pieces[b].Box[0] })

	first, second := pieces[0].Box, pieces[1].Box
	if first[1] != second[1] || first[3] != second[3] {
		t.Errorf("the pieces stand at %v and %v, want one top and one bottom",
			first, second)
	}
	if first[0] != 112 || first[2] != 261 {
		t.Errorf("`viaje;` covers %d to %d, want the column 112 to 261",
			first[0], first[2])
	}
	if second[0] != 231 || second[2] != 1735 {
		t.Errorf("the rest of the row covers %d to %d, want the column 231 to 1735",
			second[0], second[2])
	}
	if got := first[3] - first[1]; got != 79 {
		t.Errorf("the row stands %d tall, want the row spacing 79", got)
	}
}
