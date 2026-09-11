package ocrmerge

import (
	"testing"
)

func TestGroupLinesOnASpreadEdge(t *testing.T) {
	f := fixtureOf(t, "page0011")
	in := f.input()

	groups := groupLines(in.Lines, in.Blocks)

	if len(groups) != len(in.Blocks) {
		t.Fatalf("%d of %d blocks own a line, want all", len(groups), len(in.Blocks))
	}

	owned := map[int]bool{}
	for _, members := range groups {
		for _, index := range members {
			if owned[index] {
				t.Fatalf("line %d belongs to two blocks", index)
			}
			owned[index] = true
		}
	}
	if len(owned) != 35 {
		t.Errorf("%d lines were grouped, want 35", len(owned))
	}

	// Page 11 is a split page carrying a sliver of the facing page down its right
	// edge. The box engine read that sliver as 36 two letter lines and the text
	// engine ignored it. Every one of them must stay outside every group.
	for i, line := range in.Lines {
		if owned[i] {
			continue
		}
		for k, block := range in.Blocks {
			if share := overlapShare(line.Box, block.Box); share >= minOverlap {
				t.Errorf("ungrouped line %d covers %.2f of block %d", i, share, k)
			}
		}
	}
	if len(in.Lines)-len(owned) != 36 {
		t.Errorf("%d lines were left out, want 36", len(in.Lines)-len(owned))
	}
}

func TestReadingOrderSortsTopToBottomThenLeft(t *testing.T) {
	f := fixtureOf(t, "page0011")
	in := f.input()
	groups := groupLines(in.Lines, in.Blocks)

	for owner, members := range groups {
		ordered := readingOrder(in.Lines, members)
		if len(ordered) != len(members) {
			t.Fatalf("block %d: reading order has %d of %d members",
				owner, len(ordered), len(members))
		}

		for at := 1; at < len(ordered); at++ {
			before, after := in.Lines[ordered[at-1]], in.Lines[ordered[at]]
			height := max(1, before.Box[3]-before.Box[1])
			if after.Box[1] < before.Box[1]-height/2 {
				t.Errorf("block %d: line %d sits above line %d in reading order",
					owner, ordered[at], ordered[at-1])
			}
		}
	}
}

func TestReadingOrderPutsTwoLinesOfOneRowLeftToRight(t *testing.T) {
	// The box engine reported these two boxes of page 11 on one text row, and
	// reported the right hand one first.
	lines := []Line{
		{Box: Box{936, 462, 1372, 506}, Text: "e «no» se tiene, pero"},
		{Box: Box{179, 461, 958, 506}, Text: "abstracto φiλiα; se «ama» aquello que"},
	}

	ordered := readingOrder(lines, []int{0, 1})

	if ordered[0] != 1 || ordered[1] != 0 {
		t.Errorf("reading order is %v, want [1 0]", ordered)
	}
}

func TestReadingOrderIgnoresTheOrderItWasGiven(t *testing.T) {
	lines := []Line{
		{Box: Box{100, 300, 900, 340}, Text: "third"},
		{Box: Box{100, 100, 900, 140}, Text: "first"},
		{Box: Box{100, 200, 900, 240}, Text: "second"},
	}

	ordered := readingOrder(lines, []int{0, 1, 2})

	want := []string{"first", "second", "third"}
	for at, index := range ordered {
		if lines[index].Text != want[at] {
			t.Fatalf("reading order gives %q at %d, want %q", lines[index].Text, at, want[at])
		}
	}
}

// The three lines of the last text block of page 432 stand 45 and 44 pixels
// apart, measured top to top, and their boxes are 88, 90 and 97 pixels tall. Half
// a box height is therefore as much as a whole row, which is what used to read all
// three as one row and hand them back in the order they sit left to right.
func TestRowsOfKeepsRowsApartWhenTheBoxesAreTallerThanThePitch(t *testing.T) {
	lines := []Line{
		{Box: Box{222, 2288, 1622, 2376}, Text: "2. Dilthey rechaza la aplicación a la historia de nociones suprahis-"},
		{Box: Box{182, 2333, 1624, 2423}, Text: "tóricas. Si se admite, por ejemplo, «la naturaleza humana» como algo"},
		{Box: Box{182, 2377, 1628, 2474}, Text: "que tiene que ser común al hombre del siglo xix, al griego arcaico y al"},
	}

	rows := rowsOf(lines, []int{0, 1, 2})

	if len(rows) != 3 {
		t.Fatalf("three stacked lines made %d rows, want 3: %v", len(rows), rows)
	}
	for at, row := range rows {
		if len(row) != 1 || row[0] != at {
			t.Errorf("row %d holds %v, want [%d]", at, row, at)
		}
	}
}

// The detector broke seven rows of the first block of page 432 into a left box
// and a right box. The right box of a row stands lower than its left box, and
// lower than the left box of the row under it, so only the columns tell them
// apart.
func TestRowsOfPairsTheHalvesOfABrokenRow(t *testing.T) {
	in := fixtureOf(t, "page0432").input()
	groups := groupLines(in.Lines, in.Blocks)

	rows := rowsOf(in.Lines, groups[0])

	want := [][]int{{0}, {1, 2}, {3, 4}, {5, 6}, {7, 8}, {9}, {10}}
	if len(rows) != len(want) {
		t.Fatalf("the block made %d rows, want %d: %v", len(rows), len(want), rows)
	}
	for at := range want {
		if len(rows[at]) != len(want[at]) {
			t.Fatalf("row %d holds %v, want %v", at, rows[at], want[at])
		}
		for k := range want[at] {
			if rows[at][k] != want[at][k] {
				t.Errorf("row %d holds %v, want %v", at, rows[at], want[at])
			}
		}
	}
}

// Every row of every block of page 432 stands under the row before it, measured
// through the anchors. A row out of order means the cut aligns a stream that is
// not the page.
func TestEveryRowOfPageFourThirtyTwoStandsUnderTheOneBeforeIt(t *testing.T) {
	in := fixtureOf(t, "page0432").input()
	groups := groupLines(in.Lines, in.Blocks)

	for _, owner := range sortedKeys(groups) {
		rows := rowsOf(in.Lines, groups[owner])
		for at := 1; at < len(rows); at++ {
			low := centreOf(anchorOf(in.Lines, rows[at-1]))
			high := centreOf(anchorOf(in.Lines, rows[at]))
			if high <= low {
				t.Errorf("block %d: row %d sits at %d, not under row %d at %d",
					owner, at, high, at-1, low)
			}
		}
	}
}

// A row is read left to right, so its anchor is its leftmost box. Measuring a row
// through any other box measures the fall of the baseline as well as the pitch.
func TestAnchorOfIsTheLeftmostBoxOfTheRow(t *testing.T) {
	lines := []Line{
		{Box: Box{1411, 298, 1608, 381}, Text: "tenga una"},
		{Box: Box{183, 269, 1129, 340}, Text: "«propiedades químicas», la taxonomía vegetal"},
	}

	rows := rowsOf(lines, []int{0, 1})

	if len(rows) != 1 {
		t.Fatalf("two halves of one row made %d rows, want 1", len(rows))
	}
	if got := anchorOf(lines, rows[0]); got != (Box{183, 269, 1129, 340}) {
		t.Errorf("the anchor is %v, want the left box", got)
	}
}

// Two boxes standing in one column are two rows however close their tops are, and
// two boxes side by side are one row however far apart their tops are.
func TestBesideReadsColumnsAndNotTops(t *testing.T) {
	for _, c := range []struct {
		name string
		row  Box
		next Box
		want bool
	}{
		{"side by side at one height", Box{100, 100, 700, 180}, Box{710, 100, 900, 180}, true},
		{"side by side, the tail lower", Box{100, 100, 700, 180}, Box{710, 130, 900, 210}, true},
		{"one under the other", Box{100, 100, 900, 180}, Box{100, 145, 900, 225}, false},
		{"side by side, a row apart", Box{100, 100, 700, 180}, Box{710, 200, 900, 280}, false},
		{"sharing most of the column", Box{100, 100, 700, 180}, Box{300, 110, 900, 190}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			lines := []Line{{Box: c.row}, {Box: c.next}}
			if got := beside(lines, []int{0}, 1); got != c.want {
				t.Errorf("beside answered %v, want %v", got, c.want)
			}
		})
	}
}

func TestOverlapShareIgnoresADistantBlock(t *testing.T) {
	line := Box{100, 100, 200, 140}
	if share := overlapShare(line, Box{500, 500, 900, 600}); share != 0 {
		t.Errorf("a block elsewhere on the page covers %.2f, want 0", share)
	}
	if share := overlapShare(line, Box{90, 90, 210, 150}); share != 1 {
		t.Errorf("a block holding the whole line covers %.2f, want 1", share)
	}
}

// A footnote of one row stands as far inside the foot of the block above it as
// inside its own, and the block above covers more of it because it is taller.
// Book 17 page 116 ends with exactly that pair of boxes.
func TestGroupLinesGivesALineToTheBlockItIsALineOf(t *testing.T) {
	line := Box{289, 2936, 560, 3013}
	above := Box{297, 2832, 1804, 2999}
	own := Box{303, 2948, 558, 3012}

	if overlapShare(line, above) <= overlapShare(line, own) {
		t.Fatalf("the block above covers %.3f of the line against %.3f, so this "+
			"no longer tests the case it was written for",
			overlapShare(line, above), overlapShare(line, own))
	}

	groups := groupLines(
		[]Line{{Box: line, Text: "51 Cfr. ibid."}},
		[]Block{{Box: above, Text: "a footnote of two rows"}, {Box: own, Text: "51 Cfr. ibid."}})

	if _, took := groups[0]; took {
		t.Error("the block above took the line")
	}
	if members := groups[1]; len(members) != 1 || members[0] != 0 {
		t.Errorf("the footnote block owns %v, want the one line", members)
	}
}
