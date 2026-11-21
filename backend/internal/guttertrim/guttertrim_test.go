package guttertrim

import (
	"testing"

	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

// bodyLines writes count lines of a page `width` wide, each beginning at `start`
// and running to the far margin. They are what names the body edge.
func bodyLines(width, start, count int) []ocrmerge.Box {
	boxes := make([]ocrmerge.Box, count)
	for i := range boxes {
		top := 200 + i*80
		boxes[i] = ocrmerge.Box{start, top, width - start, top + 60}
	}
	return boxes
}

// mirror turns the boxes of a right half into the boxes of the left half that
// would measure the same. It is what lets one case prove both sides.
func mirror(boxes []ocrmerge.Box, width int) []ocrmerge.Box {
	flipped := make([]ocrmerge.Box, len(boxes))
	for i, box := range boxes {
		flipped[i] = ocrmerge.Box{width - box[2], box[1], width - box[0], box[3]}
	}
	return flipped
}

func TestInsetsMeasureFromTheGutterEdge(t *testing.T) {
	near, far := insets(ocrmerge.Box{0, 273, 55, 331}, 2083, "left")
	if near != 0 || far != 55 {
		t.Errorf("a right half gives the insets %d and %d, want 0 and 55", near, far)
	}

	near, far = insets(ocrmerge.Box{2003, 2667, 2064, 2728}, 2067, "right")
	if near != 3 || far != 64 {
		t.Errorf("a left half gives the insets %d and %d, want 3 and 64", near, far)
	}
}

func TestTheCutSideNamesTheOppositeEdge(t *testing.T) {
	for _, c := range []struct{ cutSide, edge string }{
		{"right", "left"},
		{"left", "right"},
	} {
		edge, ok := gutterEdge(c.cutSide)
		if !ok || edge != c.edge {
			t.Errorf("the %s half has the gutter edge %q, want %q", c.cutSide, edge, c.edge)
		}
	}

	if _, ok := gutterEdge(""); ok {
		t.Error("a page with no cut side named a gutter edge")
	}
}

func TestAPageThatIsNotAHalfOfASpreadGetsNoTrim(t *testing.T) {
	boxes := append(bodyLines(2083, 182, 30),
		ocrmerge.Box{0, 273, 55, 331}, ocrmerge.Box{0, 343, 58, 406})

	if _, ok := Of(boxes, 2083, ""); ok {
		t.Error("a page with no cut side was trimmed")
	}
}

func TestAPageWithNoLinesGetsNoTrim(t *testing.T) {
	if _, ok := Of(nil, 2083, "right"); ok {
		t.Error("a page with no lines was trimmed")
	}
}

func TestATruncatedWordOfTheNeighbouringPageIsAFragment(t *testing.T) {
	// Page 45 of book 17: 18 fragments against the edge, a body beginning at 182.
	boxes := append(bodyLines(2083, 182, 33),
		ocrmerge.Box{0, 273, 55, 331},
		ocrmerge.Box{0, 409, 61, 485},
		ocrmerge.Box{0, 639, 52, 703})

	trim, ok := Of(boxes, 2083, "right")
	if !ok {
		t.Fatal("a page carrying three fragments was not trimmed")
	}
	if trim.Side != "left" {
		t.Errorf("the trim lies against %q, want the left edge", trim.Side)
	}
	if trim.Column != 122 {
		t.Errorf("the trim column is %d, want 122", trim.Column)
	}
}

// A page photographed at an angle makes the detector break its printed rows in
// half. The left part of each broken row is short and stands in a column against
// the inner edge, and none of them reaches it. This is page 87 of book 17.
func TestTheLeftPartOfABrokenRowIsNoFragment(t *testing.T) {
	boxes := bodyLines(2107, 98, 30)
	for i, near := range []int{65, 71, 77, 75, 81, 86, 102} {
		top := 232 + i*74
		boxes = append(boxes, ocrmerge.Box{near, top, near + 200, top + 60})
	}

	if _, ok := Of(boxes, 2107, "right"); ok {
		t.Error("the left parts of broken rows were read as a strip")
	}
}

func TestALineThatReachesTheEdgeButIsNotShortIsNoFragment(t *testing.T) {
	// Page 132 of book 17 holds a footnote line that runs to the edge and takes
	// 0.21 of the width. One more like it would still not make a strip.
	boxes := append(bodyLines(1988, 200, 30),
		ocrmerge.Box{1578, 2638, 1984, 2700},
		ocrmerge.Box{1578, 2710, 1984, 2772})

	if _, ok := Of(boxes, 1988, "left"); ok {
		t.Error("two footnote lines reaching the edge were read as a strip")
	}
}

func TestAFullLineThatReachesTheEdgeIsNoFragment(t *testing.T) {
	// Page 48 of book 18 holds two lines that reach the edge and take 0.94 of the
	// width. They are body lines, so they name the margin instead.
	boxes := append(bodyLines(1425, 60, 20),
		ocrmerge.Box{0, 300, 1342, 360},
		ocrmerge.Box{0, 380, 1342, 440})

	if _, ok := Of(boxes, 1425, "right"); ok {
		t.Error("two full lines reaching the edge were read as a strip")
	}
}

func TestTheBodyEdgeComesFromTheFullLines(t *testing.T) {
	// The near insets of the body run 182 to 257. A running head at 784 and a
	// folio at 1738 must not move the edge, and neither may set it.
	boxes := bodyLines(2083, 182, 32)
	boxes = append(boxes,
		ocrmerge.Box{254, 285, 1820, 368},
		ocrmerge.Box{784, 154, 1309, 214},
		ocrmerge.Box{1738, 142, 1829, 214},
		ocrmerge.Box{0, 273, 55, 331},
		ocrmerge.Box{0, 343, 58, 406})

	trim, ok := Of(boxes, 2083, "right")
	if !ok {
		t.Fatal("the page was not trimmed")
	}
	// The middle of 58 and the body edge 182.
	if trim.Column != 120 {
		t.Errorf("the trim column is %d, want 120, so the body edge is 182", trim.Column)
	}
}

func TestAPageThatHoldsNoBodyLineGetsNoTrim(t *testing.T) {
	// Page 1 of book 18: three fragments and nothing wide enough to name a margin.
	boxes := []ocrmerge.Box{
		{1519, 2169, 1594, 2230},
		{1534, 691, 1594, 752},
		{1555, 575, 1594, 636},
	}

	if _, ok := Of(boxes, 1595, "left"); ok {
		t.Error("a page with no body line was trimmed")
	}
}

func TestTheFragmentsMustNotReachTheBody(t *testing.T) {
	// The body begins at 90 and a fragment ends at 61, so the corridor is 29 of a
	// width of 2083. That is below 0.03 of it.
	boxes := append(bodyLines(2083, 90, 30),
		ocrmerge.Box{0, 273, 55, 331},
		ocrmerge.Box{0, 343, 61, 406})

	if _, ok := Of(boxes, 2083, "right"); ok {
		t.Error("a page whose corridor is too narrow was trimmed")
	}
}

func TestASingleFragmentIsNoStrip(t *testing.T) {
	// Page 50 of book 17 holds one mark against the edge and nothing else.
	boxes := append(bodyLines(2067, 250, 30), ocrmerge.Box{2003, 2667, 2064, 2728})

	if _, ok := Of(boxes, 2067, "left"); ok {
		t.Error("a single mark was read as a strip")
	}
}

func TestTwoFragmentsAreAColumn(t *testing.T) {
	// Page 29 of book 17: two fragments ending at 28, a body edge of 152.
	boxes := append(bodyLines(2033, 152, 30),
		ocrmerge.Box{0, 290, 28, 325},
		ocrmerge.Box{0, 435, 25, 476})

	trim, ok := Of(boxes, 2033, "right")
	if !ok {
		t.Fatal("two fragments were not read as a strip")
	}
	if trim.Column != 90 {
		t.Errorf("the trim column is %d, want 90", trim.Column)
	}
}

// The column is the middle of the corridor and not the last fragment found, so a
// strip the detector barely read is covered as far as one it read well.
func TestTheColumnIsTheMiddleOfTheCorridor(t *testing.T) {
	for _, c := range []struct {
		name       string
		width      int
		bodyEdge   int
		fragmentTo int
		want       int
	}{
		{"book 17 page 29", 2033, 152, 28, 90},
		{"book 17 page 31", 2090, 145, 43, 94},
		{"book 17 page 33", 2038, 193, 38, 116},
		{"book 17 page 45", 2083, 182, 61, 122},
		{"book 17 page 53", 2061, 160, 57, 109},
		{"book 17 page 55", 2026, 190, 28, 109},
		{"book 19 page 51", 1545, 212, 42, 127},
	} {
		boxes := append(bodyLines(c.width, c.bodyEdge, 30),
			ocrmerge.Box{0, 273, c.fragmentTo, 331},
			ocrmerge.Box{0, 343, c.fragmentTo - 3, 406})

		trim, ok := Of(boxes, c.width, "right")
		if !ok {
			t.Errorf("%s: the page was not trimmed", c.name)
			continue
		}
		if trim.Column != c.want {
			t.Errorf("%s: the trim column is %d, want %d", c.name, trim.Column, c.want)
		}
	}
}

// The rule is written in insets, so a left half answers what the right half it
// mirrors answers, against the other edge.
func TestTheRuleAnswersBothSidesAlike(t *testing.T) {
	const width = 2083
	boxes := append(bodyLines(width, 182, 33),
		ocrmerge.Box{0, 273, 55, 331},
		ocrmerge.Box{0, 343, 58, 406},
		ocrmerge.Box{0, 639, 52, 703})

	right, ok := Of(boxes, width, "right")
	if !ok {
		t.Fatal("the right half was not trimmed")
	}

	left, ok := Of(mirror(boxes, width), width, "left")
	if !ok {
		t.Fatal("the mirrored left half was not trimmed")
	}

	if left.Column != right.Column {
		t.Errorf("the left half trims at %d and the right half at %d",
			left.Column, right.Column)
	}
	if left.Side != "right" || right.Side != "left" {
		t.Errorf("the trims lie against %q and %q, want the right and the left edge",
			left.Side, right.Side)
	}
}

func TestCoversHoldsOnlyWhatLiesWhollyInside(t *testing.T) {
	trim := Trim{Side: "left", Column: 122}

	if !trim.Covers(ocrmerge.Box{0, 273, 55, 331}, 2083) {
		t.Error("a fragment ending at 55 is not covered by a trim of 122")
	}
	if !trim.Covers(ocrmerge.Box{0, 273, 122, 331}, 2083) {
		t.Error("a line ending exactly at the trim column is not covered")
	}
	if trim.Covers(ocrmerge.Box{0, 273, 123, 331}, 2083) {
		t.Error("a line reaching past the trim column is covered")
	}
	if trim.Covers(ocrmerge.Box{182, 285, 1805, 368}, 2083) {
		t.Error("a body line is covered")
	}

	against := Trim{Side: "right", Column: 127}
	if !against.Covers(ocrmerge.Box{1503, 2667, 1545, 2728}, 1545) {
		t.Error("a fragment against the right edge is not covered")
	}
	if against.Covers(ocrmerge.Box{212, 285, 1333, 368}, 1545) {
		t.Error("a body line is covered against the right edge")
	}
}
