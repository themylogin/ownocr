package ocrmerge

import (
	"strings"
	"testing"
)

func TestLineSlopeReadsTheBottomEdge(t *testing.T) {
	cases := []struct {
		name    string
		line    Line
		slope   float64
		reports bool
	}{
		{
			name:    "a line with no corners reports nothing",
			line:    Line{Box: Box{0, 0, 100, 20}},
			reports: false,
		},
		{
			name: "a level line",
			line: Line{Corners: []Point{{0, 0}, {100, 0}, {100, 20}, {0, 20}}},
			// The top edge of this line rises as much as the bottom edge, and
			// only the bottom edge follows the baseline of the print.
			slope:   0,
			reports: true,
		},
		{
			name:    "a line that falls to the right",
			line:    Line{Corners: []Point{{0, 0}, {100, 10}, {100, 30}, {0, 20}}},
			slope:   0.1,
			reports: true,
		},
		{
			name:    "a line that rises to the right",
			line:    Line{Corners: []Point{{0, 10}, {100, 0}, {100, 20}, {0, 30}}},
			slope:   -0.1,
			reports: true,
		},
		{
			name: "a taller glyph on the right moves the top edge and not the slope",
			line: Line{Corners: []Point{{0, 10}, {100, 0}, {100, 20}, {0, 20}}},
			// The bottom edge is level, so the line is level whatever the top
			// edge does.
			slope:   0,
			reports: true,
		},
		{
			name:    "a line of no width reports nothing",
			line:    Line{Corners: []Point{{5, 0}, {5, 0}, {5, 20}, {5, 20}}},
			reports: false,
		},
	}

	for _, c := range cases {
		slope, reports := lineSlope(c.line)
		if reports != c.reports {
			t.Errorf("%s: reports=%v, want %v", c.name, reports, c.reports)
			continue
		}
		if reports && slope != c.slope {
			t.Errorf("%s: slope=%v, want %v", c.name, slope, c.slope)
		}
	}
}

// A narrow fragment reports a slope of its own that has nothing to do with the
// page. Reading it would answer three times the truth, and three times the truth
// calls a block unusable that the merge handles correctly.
func TestBlockSlopeIgnoresNarrowFragments(t *testing.T) {
	lines := []Line{
		slopingLine(0, 0, 1500, 60, 0.02),
		slopingLine(0, 80, 1500, 60, 0.02),
		slopingLine(0, 160, 1500, 60, 0.02),
		slopingLine(0, 60, 200, 60, 0.35),
		slopingLine(0, 140, 200, 60, 0.35),
	}

	if got := blockSlope(lines, []int{0, 1, 2, 3, 4}); got != 0.02 {
		t.Errorf("the block slopes %v, want the 0.02 of its full-width lines", got)
	}
	if got := blockSlope(lines, []int{3, 4}); got != 0.35 {
		t.Errorf("a block of fragments alone slopes %v, want 0.35", got)
	}
	if got := blockSlope([]Line{{Box: Box{0, 0, 100, 20}}}, []int{0}); got != 0 {
		t.Errorf("a block whose lines report no corners slopes %v, want 0", got)
	}
}

// The one block of the two measured books whose boxes say nothing about where its
// printed rows are. Nothing else may join it: every other block of every fixture
// merges, and replacing one would throw away boxes that are right.
func TestOnlyTheMeasuredBlockIsUnusable(t *testing.T) {
	const want = "book0017page0087 block 1"

	found := []string{}
	for _, f := range loadFixtures(t) {
		in := f.input()
		groups := groupLines(in.Lines, in.Blocks)
		for owner, members := range groups {
			if !hasText(in.Blocks[owner].Text) {
				continue
			}
			if unusableBoxes(in.Lines, members, in.Blocks[owner], pagePitch(in)) {
				found = append(found, f.Name+" block "+itoa(owner))
			}
		}
	}

	if len(found) != 1 || found[0] != want {
		t.Errorf("found %v unusable, want only %q", found, want)
	}
}

// The figures the criterion rests on, on the page it was measured from.
func TestPageEightySevenReportsTheMeasuredLieAndFill(t *testing.T) {
	in := fixtureOf(t, "book0017page0087").input()
	groups := groupLines(in.Lines, in.Blocks)
	members := groups[1]
	block := in.Blocks[1]

	if got := len(members); got != 14 {
		t.Errorf("the block holds %d boxes, want 14 over its 8 printed rows", got)
	}
	if got := len(rowsOf(in.Lines, members)); got != 10 {
		t.Errorf("the block reads as %d rows, want the 10 the wrong order gives", got)
	}
	if got := blockSlope(in.Lines, members); !near(got, -0.086, 0.001) {
		t.Errorf("the block slopes %.4f, want -0.086", got)
	}
	if got := blockFill(in.Lines, members, block.Text); !near(got, 1.02, 0.01) {
		t.Errorf("the block holds %.2f of what its lines hold, want 1.02", got)
	}
	if got := boxLie(in.Lines, members, block.Box, pagePitch(in)); !near(got, 1.66, 0.02) {
		t.Errorf("its widest and narrowest boxes differ by %.2f pitches, want 1.66", got)
	}
}

// A block whose every printed row is one box names its rows however far it
// slopes, because every one of its boxes is displaced by the same amount.
//
// Both blocks here slope past a whole pitch of the width of their block, and both
// merge correctly. Measuring the width of the block rather than the spread of the
// boxes called them unusable and threw away boxes that were right.
func TestABlockOfWholeRowsIsNeverUnusableHoweverFarItSlopes(t *testing.T) {
	const width, pitch, height, rows = 1540, 76, 200, 3

	lines := make([]Line, rows)
	held := make([]string, rows)
	for i := range lines {
		lines[i] = slopingLine(0, i*pitch, width, height, 0.066)
		lines[i].Text = words(9)
		held[i] = words(9)
	}
	block := Block{
		Box:  Box{0, 0, width, rows*pitch + 100},
		Text: strings.Join(held, " "),
	}

	if got := blockSlope(lines, indexes(rows)); !near(got, 0.066, 0.001) {
		t.Fatalf("the block slopes %.4f, want 0.066", got)
	}
	if got := 0.066 * float64(width) / float64(pitch); got < maxBoxLie {
		t.Fatalf("the width of the block lies by %.2f pitches, so this guards nothing", got)
	}
	if got := boxLie(lines, indexes(rows), block.Box, pitch); got != 0 {
		t.Errorf("its boxes differ by %.2f pitches, want 0: every row is one box", got)
	}
	if unusableBoxes(lines, indexes(rows), block, pitch) {
		t.Error("a block of whole rows was called unusable")
	}
}

// The reference has to come from the blocks the merge trusts. The rows of the
// unusable block answer a pitch of 68, and the page is set on 81.
func TestTheReferenceLeavesTheUnusableBlockOut(t *testing.T) {
	in := fixtureOf(t, "book0017page0087").input()
	groups := groupLines(in.Lines, in.Blocks)

	rows := make(map[int][][]int, len(groups))
	for owner, members := range groups {
		rows[owner] = rowsOf(in.Lines, members)
	}

	ref := measure(in.Lines, rows, map[int]bool{1: true})
	if ref.pitch != 81 {
		t.Errorf("the page measures a pitch of %d, want 81", ref.pitch)
	}
	if ref.runes != 56 {
		t.Errorf("a printed row holds %d runes, want 56", ref.runes)
	}

	if polluted := measure(in.Lines, rows, nil); polluted.pitch == ref.pitch {
		t.Errorf("the unusable block changed nothing, so this test guards nothing")
	}
}

func TestRowCountNeedsBothEstimatesToAgree(t *testing.T) {
	ref := reference{pitch: 81, runes: 56}
	block := Box{83, 79, 1761, 844}

	cases := []struct {
		name  string
		ref   reference
		slope float64
		text  string
		rows  int
		says  bool
	}{
		{
			name: "the measured block, where the runes say 8.02 and the height 7.66",
			// 765 pixels tall over a pitch of 81 is 9.4 without the slope, and
			// 7.66 with it. The runes answer 8.02.
			ref: ref, slope: 0.086, text: runs(453), rows: 8, says: true,
		},
		{
			name: "the same block read level, where the height says 9.4",
			ref:  ref, slope: 0, text: runs(453), says: false,
		},
		{
			name: "text for 8 rows in a block 4 rows tall",
			ref:  ref, slope: 0.086, text: runs(480), says: false,
		},
		{
			name: "a page that measures no pitch",
			ref:  reference{runes: 56}, slope: 0.086, text: runs(453), says: false,
		},
		{
			name: "a page that measures no row",
			ref:  reference{pitch: 81}, slope: 0.086, text: runs(453), says: false,
		},
		{
			name: "a block the slope leaves no height at all",
			ref:  ref, slope: 1.0, text: runs(453), says: false,
		},
	}

	for _, c := range cases {
		rows, says := rowCount(c.ref, block, c.slope, c.text)
		if says != c.says {
			t.Errorf("%s: says=%v, want %v (rows=%d)", c.name, says, c.says, rows)
			continue
		}
		if says && rows != c.rows {
			t.Errorf("%s: %d rows, want %d", c.name, rows, c.rows)
		}
	}
}

// A result stored before the merge read corners carries none, so every one of its
// lines counts as level and no block of it is ever unusable. That is what lets
// the merge run over a book read before this change without replacing anything.
func TestAPageWithoutCornersIsNeverUnusable(t *testing.T) {
	for _, f := range loadFixtures(t) {
		in := f.cornerless()
		for owner, members := range groupLines(in.Lines, in.Blocks) {
			if unusableBoxes(in.Lines, members, in.Blocks[owner], pagePitch(in)) {
				t.Errorf("%s block %d is unusable without corners", f.Name, owner)
			}
		}
		if got := Merge(in).Counts.Discarded; got != 0 {
			t.Errorf("%s discarded %d boxes without corners, want none", f.Name, got)
		}
	}

	for name := range cornerlessFixtures {
		f := fixtureOf(t, name)
		if len(f.V6.Raw.Polys) != 0 {
			t.Errorf("%s carries corners now, so it guards the old path no longer", name)
		}
	}
}

// pageEightySevenTopBlock is what the merge answers for the top block of page 87
// of book 17, line for line.
//
// The block slopes 0.086 across 1678 pixels on a page set on a pitch of 81, so no
// box top there names its own printed row. PP-OCRv6 reported 14 boxes over the 8
// printed rows and never read one of the rows at all.
//
// The first and the last line are the printed lines exactly. The six between them
// hold the right words in the right order and break in the wrong places, because
// the printer broke `Ate-` and `nas` with a hyphen, PaddleOCR-VL wrote `Atenas`,
// and the merge never breaks a word.
var pageEightySevenTopBlock = []struct {
	box  Box
	text string
}{
	{Box{83, 79, 1761, 174}, "exponer la extrema suspicacia de los atenienses en ciertas"},
	{Box{83, 174, 1761, 270}, "cuestiones y el ropaje tradicional con el que esa"},
	{Box{83, 270, 1761, 365}, "suspicacia se expresa. En cambio, con la «tiranía» en el"},
	{Box{83, 365, 1761, 461}, "sentido en que hemos hablado de ella, y en el sentido en"},
	{Box{83, 461, 1761, 557}, "que lo fue en Atenas de Pisístrato y sus hijos, con el"},
	{Box{83, 557, 1761, 652}, "significado que aquello tuvo y el papel que desempeñó, nada"},
	{Box{83, 652, 1761, 748}, "de lo que pueda ocurrir en la Atenas de Tucídides tiene"},
	{Box{83, 748, 1761, 844}, "nada que ver y, desde luego, nada tiene que ver Alcibíades."},
}

func TestPageEightySevenReadsItsTopBlockInOrder(t *testing.T) {
	in := fixtureOf(t, "book0017page0087").input()
	result := Merge(in)

	if got := result.Counts.Discarded; got != 14 {
		t.Errorf("discarded %d boxes, want the 14 of the top block", got)
	}

	for i, want := range pageEightySevenTopBlock {
		line := result.Lines[i+1]
		if line.Text != want.text {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, line.Text, want.text)
		}
		if line.Box != want.box {
			t.Errorf("line %d carries %v, want %v", i+1, line.Box, want.box)
		}
		if line.Source != in.TextEngine {
			t.Errorf("line %d names %q, want the text engine", i+1, line.Source)
		}
		if !line.Estimated {
			t.Errorf("line %d is not marked as placed by the merge", i+1)
		}
		if line.Score != 0 {
			t.Errorf("line %d carries a score of %v, want none", i+1, line.Score)
		}
	}

	// Not one box the box engine reported for that block survives, and the text
	// of the block reaches a line whole.
	for _, index := range groupLines(in.Lines, in.Blocks)[1] {
		for _, line := range result.Lines {
			if line.Box == in.Lines[index].Box {
				t.Errorf("the box %v of the replaced block is still in the answer",
					line.Box)
			}
		}
	}
	if got := result.Counts.DroppedChars; got != 0 {
		t.Errorf("dropped %d runes, want none: %v", got, result.Dropped)
	}
	if got := nonSpaceCount(in.Blocks[1].Text); got != joinedCount(result.Lines[1:9]) {
		t.Errorf("the answered rows hold %d runes of the block's %d",
			joinedCount(result.Lines[1:9]), got)
	}
}

// The boxes of the rest of the page are not the merge's to touch. Block 2 of the
// same page slopes 0.63 of a pitch, which is a reading order the merge derives
// correctly.
func TestPageEightySevenKeepsTheBoxesOfItsOtherBlocks(t *testing.T) {
	in := fixtureOf(t, "book0017page0087").input()
	result := Merge(in)

	answered := map[[2]int]bool{}
	for _, line := range result.Lines {
		answered[column(line.Box)] = true
	}
	for owner, members := range groupLines(in.Lines, in.Blocks) {
		if owner == 1 {
			continue
		}
		for _, index := range members {
			if !answered[column(in.Lines[index].Box)] {
				t.Errorf("block %d lost the box %v", owner, in.Lines[index].Box)
			}
		}
	}
}

// The three shapes that a looser criterion mistook for a broken row. Each one
// fails the criterion for its own reason, and every one of them has to merge with
// every box it came with and with nothing reported unordered.
//
//   - page 90 ends a sloping footnote with `nos puntos.`, a short box that shares
//     its whole column with the line above it.
//   - page 102 holds a footnote of two rows, whose own two anchors stand 24 pixels
//     apart where its print stands 60 apart.
//   - page 103 holds three whole rows sloping 0.066 across 1540 pixels, which is
//     1.34 pitches of the width of the block.
func TestTheFalselySuspectBlocksKeepEveryBox(t *testing.T) {
	for _, name := range falselySuspectFixtures {
		in := fixtureOf(t, name).input()
		result := Merge(in)

		if got := result.Counts.Discarded; got != 0 {
			t.Errorf("%s discarded %d boxes, want none", name, got)
		}

		// Not a line for a line: page 90 holds a printed row the box engine
		// missed, and the filler places one. Every box it did read survives.
		answered := map[[2]int]bool{}
		for _, line := range result.Lines {
			answered[column(line.Box)] = true
		}
		for i, given := range in.Lines {
			if !answered[column(given.Box)] {
				t.Errorf("%s lost the box %v of line %d", name, given.Box, i)
			}
		}
		for _, dropped := range result.Dropped {
			if dropped.Reason == ReasonUnordered {
				t.Errorf("%s reported %q as unordered", name, dropped.Text)
			}
		}
	}
}

// Page 85 breaks one printed row of three boxes, and the collapsed block of book
// 18 is the text engine failing and not the box engine. Neither is this change's
// to repair, and both must come out as they always did.
func TestTheNearMissAndTheCollapseKeepEveryBox(t *testing.T) {
	for _, name := range []string{"book0017page0085", "book0018page0036"} {
		in := fixtureOf(t, name).input()
		result := Merge(in)

		if got := result.Counts.Discarded; got != 0 {
			t.Errorf("%s discarded %d boxes, want none", name, got)
		}

		// Not a line for a line: page 85 gains a box over the ink its own boxes
		// left uncovered. Every box the engine read survives, in order.
		read := readLines(result)
		if len(read) != len(in.Lines) {
			t.Errorf("%s answered %d read lines for %d boxes",
				name, len(read), len(in.Lines))
			continue
		}
		for i, line := range read {
			if column(line.Box) != column(in.Lines[i].Box) {
				t.Errorf("%s read line %d carries %v, want %v",
					name, i, line.Box, in.Lines[i].Box)
			}
		}
	}
}

// Page 85 reads `pero no en el sentido` over three boxes the detector drew
// overlapping. Their columns share 0.37 of the narrower, which the row grouping
// once refused, so the row split: the box above took the head of it on top of its
// own text, the two boxes holding the head kept what the box engine read, and the
// page carried `pero no` twice.
//
// Every one of the three now holds its own ink and nothing else.
func TestPageEightyFiveReadsItsBrokenRowOnce(t *testing.T) {
	in := fixtureOf(t, "book0017page0085").input()
	result := Merge(in)
	text := textByBox(result)

	row := []struct {
		box  Box
		text string
	}{
		{Box{123, 1108, 1763, 1264}, "a directamente el caso Alcibíades con la ruina de Atenas,"},
		{Box{137, 1232, 353, 1324}, "pero no"},
		{Box{326, 1230, 456, 1305}, "en"},
		{Box{408, 1189, 1771, 1322}, "el sentido de que la actuación de Alcibíades"},
	}
	for _, want := range row {
		if got := text.at(want.box); got != want.text {
			t.Errorf("the box %v:\n got %q\nwant %q", want.box, got, want.text)
		}
	}

	twice := 0
	for _, line := range result.Lines {
		if strings.HasSuffix(line.Text, "pero no en") {
			twice++
		}
	}
	if twice != 0 {
		t.Errorf("%d lines still swallow the head of the row below them", twice)
	}

	// Every box of the page takes text engine text now, where two kept their own.
	if result.Counts.FromBoxes != 0 {
		t.Errorf("%d lines fell back to the box engine, want none",
			result.Counts.FromBoxes)
	}
}

// Page 85 holds two printed rows the box engine drew as two boxes with ink left
// uncovered between them. The cut handed that ink to the box before the gap,
// which then held `otro—, tien` in a box drawn around `otro—,` and `cierto es qu`
// in one drawn around `cierto`.
//
// A box now stands over each gap, and every box holds the ink it was drawn
// around. Read in order the row is whole: `otro—,` `tien` `e deseos costosos`.
func TestPageEightyFivePlacesABoxOverTheInkItsBoxesMissed(t *testing.T) {
	in := fixtureOf(t, "book0017page0085").input()
	result := Merge(in)
	text := textByBox(result)

	rows := []struct {
		head, gap, tail Box
		want            [3]string
	}{
		{
			head: Box{112, 729, 275, 817},
			gap:  Box{275, 726, 393, 814},
			tail: Box{393, 701, 1737, 805},
			want: [3]string{"otro—,", "tien",
				"e deseos costosos, etcétera). No vale simplificar la"},
		},
		{
			head: Box{119, 876, 273, 959},
			gap:  Box{273, 873, 418, 955},
			tail: Box{418, 843, 1741, 947},
			want: [3]string{"cierto", "es qu",
				"e también es leal, pues sólo cambia de bando"},
		},
	}

	for _, row := range rows {
		for at, box := range []Box{row.head, row.gap, row.tail} {
			if got := text.at(box); got != row.want[at] {
				t.Errorf("the box %v:\n got %q\nwant %q", box, got, row.want[at])
			}
		}

		// The gap box fills the gap exactly and stands between its neighbours.
		if row.gap[0] != row.head[2] || row.gap[2] != row.tail[0] {
			t.Errorf("the gap box %v does not meet %v and %v",
				row.gap, row.head, row.tail)
		}
		placed := lineOf(result, row.gap)
		if placed == nil {
			t.Fatalf("no line carries the gap box %v", row.gap)
		}
		if !placed.Estimated {
			t.Errorf("the gap box %v is not marked as placed by the merge", row.gap)
		}
		if placed.Score != 0 {
			t.Errorf("the gap box %v carries a score of %v", row.gap, placed.Score)
		}
		if placed.Source != in.TextEngine {
			t.Errorf("the gap box %v names %q", row.gap, placed.Source)
		}
	}

	if got := result.Counts.Estimated; got != 2 {
		t.Errorf("the merge placed %d lines, want the 2 gap boxes", got)
	}
	if got := result.Counts.DroppedChars; got != 0 {
		t.Errorf("dropped %d runes, want none: %v", got, result.Dropped)
	}
}

// A gap of a character or two is the space between two words, which every printed
// row has. A piece that fits the box it was cut for says the gap held nothing.
func TestShareAcrossGapNeedsBothAGapAndAnOverflow(t *testing.T) {
	const density = 0.0358

	cases := []struct {
		name       string
		text       string
		box, next  Box
		room       int
		head, tail string
		divides    bool
	}{
		{
			name: "the measured row, 154 pixels of box against 145 of gap",
			text: "cierto es qu",
			box:  Box{119, 876, 273, 959}, next: Box{418, 843, 1741, 947},
			room: 6, head: "cierto", tail: "es qu", divides: true,
		},
		{
			name: "the same row with no gap to speak of",
			text: "cierto es qu",
			box:  Box{119, 876, 273, 959}, next: Box{290, 843, 1741, 947},
			room: 6, divides: false,
		},
		{
			name: "a gap beside a piece that fits its box",
			text: "cierto",
			box:  Box{119, 876, 273, 959}, next: Box{418, 843, 1741, 947},
			room: 6, divides: false,
		},
		{
			// The gaps of a contents page are rows of leader dots: sparse ink
			// over 1011 pixels beside a box of 189. Dividing by width there puts
			// three letters of the title into the dots.
			name: "a gap wider than the box it is measured from",
			text: "Notas .....",
			box:  Box{100, 0, 289, 60}, next: Box{1300, 0, 1400, 60},
			room: 7, divides: false,
		},
		{
			name: "a box the next one overlaps",
			text: "cierto es qu",
			box:  Box{119, 876, 273, 959}, next: Box{200, 843, 1741, 947},
			room: 6, divides: false,
		},
	}

	for _, c := range cases {
		head, tail, divides := shareAcrossGap(c.text, c.box, c.next, c.room, density)
		if divides != c.divides {
			t.Errorf("%s: divides=%v, want %v (%q | %q)",
				c.name, divides, c.divides, head, tail)
			continue
		}
		if divides && (head != c.head || tail != c.tail) {
			t.Errorf("%s:\n got %q | %q\nwant %q | %q",
				c.name, head, tail, c.head, c.tail)
		}
	}
}

// A page whose only block is unusable can measure no pitch and no row, so it
// cannot say how many rows to draw. It keeps every box it came with, and the text
// of the block reaches no line and is reported.
func TestAPageOfOneUnusableBlockKeepsItsBoxes(t *testing.T) {
	in := unusablePage(8, 8, false)
	result := Merge(in)

	if got := result.Counts.Discarded; got != 0 {
		t.Fatalf("discarded %d boxes with no reference to place lines by", got)
	}
	if len(result.Lines) != len(in.Lines) {
		t.Errorf("answered %d lines for %d boxes", len(result.Lines), len(in.Lines))
	}
	for i, line := range result.Lines {
		if line.Source != in.BoxEngine {
			t.Errorf("line %d names %q, want the box engine", i, line.Source)
		}
	}
	if len(result.Dropped) != 1 || result.Dropped[0].Reason != ReasonUnordered {
		t.Errorf("dropped %v, want the block reported as %q",
			result.Dropped, ReasonUnordered)
	}
}

// A block the page can count the rows of is replaced, and the two counts have to
// agree before it is. The second page here holds the same eight rows of text in a
// block box tall enough for eleven.
func TestAnUnusableBlockIsReplacedOnlyWhenTheCountsAgree(t *testing.T) {
	agreeing := Merge(unusablePage(8, 8, true))
	if got := agreeing.Counts.Discarded; got != 16 {
		t.Errorf("discarded %d boxes, want the 16 that the 8 broken rows hold", got)
	}
	if got := agreeing.Counts.Estimated; got != 8 {
		t.Errorf("placed %d lines for the replaced block, want 8", got)
	}
	if len(agreeing.Dropped) != 0 {
		t.Errorf("dropped %v, want nothing", agreeing.Dropped)
	}

	disagreeing := Merge(unusablePage(11, 8, true))
	if got := disagreeing.Counts.Discarded; got != 0 {
		t.Errorf("discarded %d boxes though the two row counts disagree", got)
	}
	if len(disagreeing.Dropped) != 1 ||
		disagreeing.Dropped[0].Reason != ReasonUnordered {
		t.Errorf("dropped %v, want the block reported as %q",
			disagreeing.Dropped, ReasonUnordered)
	}
}

// unusablePage builds a page holding one block whose boxes cannot say where its
// rows are, in the shape the measured page has: eight printed rows on a pitch of
// 80, each broken into a narrow head and a wide tail, rising 0.12 to the right
// across 1000 pixels.
//
// The head of a row reaches 200 pixels and the tail reaches 1000, so their top
// edges stand 96 pixels apart. That is 1.2 pitches, and a head is grouped with
// the row above or below it rather than with its own tail.
//
// spanRows is how many rows the block box is drawn tall enough for, and textRows
// how many rows of text it holds, so a caller can drive the two row counts apart.
// A second, level block gives the page a reference to measure; without it the page
// can measure none.
func unusablePage(spanRows, textRows int, reference bool) Input {
	const width, head, pitch, height, rows = 1000, 200, 80, 60, 8
	const slope = -0.12
	drop := int(-slope * width)

	lines := make([]Line, 0, 2*rows+rows)
	for i := 0; i < rows; i++ {
		base := drop + i*pitch
		left := slopingLine(0, base, head, height, slope)
		left.Text = words(1)
		tail := slopingLine(head, base+int(slope*head), width-head, height, slope)
		tail.Text = words(5)
		lines = append(lines, left, tail)
	}
	blocks := []Block{{
		Box:  Box{0, 0, width, spanRows*pitch + drop},
		Text: strings.Repeat(words(6)+" ", textRows),
	}}

	if reference {
		for i := 0; i < rows; i++ {
			line := slopingLine(2000, i*pitch, width, height, 0)
			line.Text = words(6)
			lines = append(lines, line)
		}
		blocks = append(blocks, Block{
			Box:  Box{2000, 0, 2000 + width, rows * pitch},
			Text: strings.Repeat(words(6)+" ", rows),
		})
	}

	return Input{Lines: lines, Blocks: blocks, BoxEngine: "v6", TextEngine: "vl"}
}

// slopingLine answers a line whose bottom edge rises by slope over its width. top
// is the y of its top left corner, and the box is drawn round every corner, so a
// line that rises to the right has a box top above that corner.
func slopingLine(left, top, width, height int, slope float64) Line {
	rise := int(slope * float64(width))
	corners := []Point{
		{left, top}, {left + width, top + rise},
		{left + width, top + rise + height}, {left, top + height},
	}

	box := Box{corners[0][0], corners[0][1], corners[0][0], corners[0][1]}
	for _, corner := range corners {
		box[0] = min(box[0], corner[0])
		box[1] = min(box[1], corner[1])
		box[2] = max(box[2], corner[0])
		box[3] = max(box[3], corner[1])
	}
	return Line{Box: box, Corners: corners, Score: 1}
}

// words answers n words of six letters, which is what a printed row of the
// synthetic pages holds.
func words(n int) string {
	return strings.TrimSpace(strings.Repeat("abcdef ", n))
}

// runs answers n runes as words, so a caller can give rowCount a length without
// caring what the text says.
func runs(n int) string {
	text := strings.Repeat("abcde ", n/6+2)
	return text[:n]
}

// pagePitch is the pitch Merge judges a block against: the one measured over
// every block of the page.
func pagePitch(in Input) int {
	groups := groupLines(in.Lines, in.Blocks)
	rows := make(map[int][][]int, len(groups))
	for owner, members := range groups {
		rows[owner] = rowsOf(in.Lines, members)
	}
	return measure(in.Lines, rows, nil).pitch
}

func indexes(n int) []int {
	all := make([]int, n)
	for i := range all {
		all[i] = i
	}
	return all
}

// textByBox answers the text of every answered line, keyed by its box, so a test
// names a box rather than a position that a placed line can shift.
// column answers the horizontal extent of a box. Levelling moves the top and the
// bottom edge of a printed row and leaves its column alone, so a test that names a
// line by its box names it by its column.
func column(box Box) [2]int { return [2]int{box[0], box[2]} }

// byColumn holds the text of every answered line under the column it covers.
type byColumn map[[2]int]string

func (b byColumn) at(box Box) string { return b[column(box)] }

func textByBox(result Result) byColumn {
	text := make(byColumn, len(result.Lines))
	for _, line := range result.Lines {
		text[column(line.Box)] = line.Text
	}
	return text
}

func lineOf(result Result, box Box) *MergedLine {
	for i, line := range result.Lines {
		if column(line.Box) == column(box) {
			return &result.Lines[i]
		}
	}
	return nil
}

func joinedCount(lines []MergedLine) int {
	count := 0
	for _, line := range lines {
		count += nonSpaceCount(line.Text)
	}
	return count
}

func near(got, want, slack float64) bool {
	return got-want < slack && want-got < slack
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
