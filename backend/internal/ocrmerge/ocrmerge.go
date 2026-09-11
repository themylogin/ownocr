// Package ocrmerge joins the line boxes of one OCR engine with the text of
// another. One engine reports a box for every line it read, which a renderer
// needs. The other reads characters the first cannot, but reports blocks of
// reflowed text instead of lines.
//
// The merge holds no list of characters and knows no alphabet. It compares
// runes after Unicode folding and decides everything else from geometry and
// from the alignment of the two readings.
package ocrmerge

import "math"

// Box is a rectangle in the pixels of the page: left, top, right, bottom.
type Box [4]int

// Point is one corner in the pixels of the page: x, y.
type Point [2]int

// Line is one line of the box engine.
//
// Corners are the four corners of the line as that engine found them, clockwise
// from the top left. A box is a rectangle, so a line that slopes across the page
// cannot report its slope through its box alone, and the slope is what tells a
// box that names its printed row from a box that names another.
//
// Corners are empty for an engine that reports none, and for a result stored
// before the merge read them. A line with no corners counts as level, so such a
// page merges as it did before they existed.
type Line struct {
	Box     Box     `json:"bbox"`
	Text    string  `json:"text"`
	Score   float64 `json:"score"`
	Corners []Point `json:"-"`
}

// Block is one region of the text engine.
type Block struct {
	Box  Box    `json:"bbox"`
	Text string `json:"text"`
}

// WithCorners pairs each line with the quad the box engine found it in. That
// engine reports one quad per line, in the order of the lines, as four points of
// two coordinates.
//
// It attaches nothing when the two lists disagree in length, because then they
// cannot be paired by index at all, and nothing to a line whose quad is not four
// points. A result stored before the merge read quads carries none, so every one
// of its lines counts as level and the page merges as it did before.
func WithCorners(lines []Line, polys [][][]float64) []Line {
	if len(polys) != len(lines) {
		return lines
	}
	for i, poly := range polys {
		if len(poly) != 4 {
			continue
		}
		corners := make([]Point, 4)
		for k, point := range poly {
			if len(point) != 2 {
				corners = nil
				break
			}
			corners[k] = Point{int(math.Round(point[0])), int(math.Round(point[1]))}
		}
		lines[i].Corners = corners
	}
	return lines
}

// Input is two readings of one page. BoxEngine and TextEngine name the engines,
// so a merged line can say where its text came from without this package
// knowing either engine.
type Input struct {
	Lines      []Line
	Blocks     []Block
	BoxEngine  string
	TextEngine string
}

// MergedLine carries the box and the confidence of its box engine line, whatever
// engine its text came from. Neither is the merge's to change.
//
// Estimated marks the exception: a line no engine read, whose box the merge
// placed itself. Its score is zero, because no engine scored it. The merge
// places one for a printed row the box engine missed, and one for every row of a
// block whose boxes cannot say where its rows are.
type MergedLine struct {
	Box       Box     `json:"bbox"`
	Text      string  `json:"text"`
	Source    string  `json:"source"`
	Score     float64 `json:"score"`
	Estimated bool    `json:"estimated,omitempty"`
}

// Dropped is text of the text engine that reached no line. Every one of them is
// reported, so no character is lost in silence.
type Dropped struct {
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// The reasons a piece of text engine text reaches no line.
const (
	// ReasonLine: the two engines did not read that line alike.
	ReasonLine = "line"
	// ReasonBlock: no line of the box engine lies inside that block.
	ReasonBlock = "block"
	// ReasonUnaligned: the block and its lines are too far apart to align.
	ReasonUnaligned = "unaligned"
	// ReasonUnordered: the boxes of the block cannot say where its printed rows
	// are, and the merge could not work out how many rows to draw instead.
	ReasonUnordered = "unordered"
)

// Counts reports one page. FromText and FromBoxes together count the lines the
// box engine read that the merge kept, so with Discarded they add up to the
// input. Estimated counts the lines the merge placed itself.
//
// Discarded counts the boxes of a block whose boxes said nothing about where its
// printed rows are. It is what keeps that loss visible: such a block throws away
// everything the box engine read of a region, and no page may lose a box in
// silence.
type Counts struct {
	Lines        int `json:"lines"`
	FromText     int `json:"fromText"`
	FromBoxes    int `json:"fromBoxes"`
	Estimated    int `json:"estimated"`
	Discarded    int `json:"discarded"`
	DroppedChars int `json:"droppedChars"`
	// Levelled counts the printed rows whose boxes the merge moved onto one
	// baseline and one height. A page that does not slope answers zero.
	Levelled int `json:"levelled"`
}

type Result struct {
	Lines   []MergedLine `json:"lines"`
	Dropped []Dropped    `json:"dropped"`
	Counts  Counts       `json:"counts"`
}

// The measured constants. Every one comes from book 3292, and design.md of the
// add-ocr-line-merge change holds the distribution each was read off.
const (
	// minOverlap is the share of a line box a block must cover to own that line.
	// Every real overlap in the measured book was above 0.9 or zero, so this
	// only guards a line clipping the corner of a neighbouring block.
	minOverlap = 0.5

	// minAgreement is the share of the characters of a line that must survive
	// folding and pair with an equal character of the block. Refusals in the
	// measured book cluster below 0.27 and acceptances above 0.42.
	minAgreement = 0.30

	// shortLine is the length below which a line is accepted unmeasured. Every
	// correct line that scored under 0.55 was shorter than this, and every
	// failure longer than this scored under 0.30.
	shortLine = 20

	// maxPieceRatio and pieceSlack bound how much longer than its line a piece
	// may be. A share cannot catch a text engine that began a line correctly and
	// then ran on, because the correct opening is what it measures.
	//
	// Over the 3840 accepted pieces of the measured book this trips exactly once,
	// on a piece of 699 characters against a line of 64. The widest correct piece
	// is 2.4 times its line, which is the text engine writing Greek as LaTeX.
	//
	// pieceSlack is also what lets one rule cover a short line. `SR` against
	// `2(30)(24)MAK` is a ratio of 6 and still passes, so this gate needs no
	// exemption of its own.
	maxPieceRatio = 4
	pieceSlack    = 20

	// maxLengthRatio refuses a block that cannot be one reading of the region
	// its lines cover. It also bounds the work below, because a text engine that
	// repeats itself without end has no length at all.
	maxLengthRatio = 8

	// maxAlignCells bounds one alignment. It costs the product of the two
	// lengths in time and in bytes of back pointers.
	maxAlignCells = 16 << 20

	// maxRowShare and minRowOverlap tell two boxes of one printed row from two
	// rows. Both come from page 1 of book 4015, where the lines fall by most of a
	// row across the column and the detector broke seven rows in half.
	//
	// maxRowShare is the share of the narrower box that two row mates may share of
	// each other's column. The halves of one row there share at most 0.04 of it,
	// and two rows share all of the narrower.
	//
	// It stood at 0.25 and let two halves of one row through that share 0.37,
	// which is the row of page 85 of book 17 reading `pero no en el sentido`. The
	// detector drew its three boxes overlapping, the row split, and the page read
	// `pero no` twice.
	//
	// Over the 183 pages of books 17 and 18 the share is bimodal: halves of one
	// row reach 0.37 and two rows reach 1.00, with nothing between. Every value
	// from 0.40 to 0.90 answers those pages alike, so this sits at the near end of
	// an empty gap.
	//
	// minRowOverlap is the share of the shorter box that two row mates must share
	// of each other's height. The halves of one row there share 0.59 to 1.00 of
	// it, and boxes of two rows share at most 0.32. It is what keeps two rows
	// apart when they do share a column, and it is why maxRowShare has that much
	// room to give.
	maxRowShare   = 0.40
	minRowOverlap = 0.5
)

// The constants of the line filler, which places a box for a line the box engine
// never read. Every one comes from page 52 of book 3313, which is one page: treat
// them as a starting point and not as a measured distribution.
const (
	// maxPitchError is how far the spacing of a gap may sit from a whole number
	// of line pitches and still be read as that many lines. The gap of the
	// measured page divides into 3.92 pitches, and the pitch itself varies by
	// 3% over the 24 lines it was measured on.
	maxPitchError = 0.25

	// minPitchSamples is the number of gaps between lines that follow each other
	// a pitch has to rest on. Below it the merge fills nothing, because a wrong
	// pitch invents lines that are not on the page.
	minPitchSamples = 4

	// edgeSlack is the share of a line height by which a filled line may stand
	// out of the block that holds it. A block box is drawn around the ink, and a
	// line box around the ink plus what the detector unclips, so the two do not
	// end at the same pixel.
	edgeSlack = 0.25
)

// The constants that tell a block whose boxes say where its printed rows are
// from a block whose boxes do not. Every one comes from the 249 text blocks of
// books 17 and 18, and design.md of the add-ocr-disaster-interpolation change
// holds the distribution each was read off.
const (
	// maxBoxLie is the vertical error, in row pitches, that the rectangle of a
	// full-width sloping line may carry. The error is the slope times the width
	// of the block, because a rectangle drawn round a sloping line is as much
	// taller than that line as the line rises over its own length.
	//
	// At one pitch the top edge of a rectangle stands a whole row from the row
	// its line belongs to. Rows are grouped by that top edge, so above it no
	// order derived from the rectangles can be right. The number follows from
	// what it measures and is not read off a distribution.
	//
	// One block of the 249 measured lies above it, at 2.12. The next stands at
	// 0.85 and merges with no weak piece at all.
	maxBoxLie = 1.0

	// minBlockFill and maxBlockFill bound the runes of a block against the runes
	// of the lines assigned to it. Inside them the two engines read the same
	// amount of text over the same pixels, so they disagree only about where that
	// text goes, and maxBoxLie says the box engine is the side that cannot place
	// it.
	//
	// Outside them the text engine is the broken reading, and the merge leaves the
	// block to the gates that refuse a line the two engines did not read alike.
	// Of the 283 measured blocks that merge with no weak piece, every one falls
	// between 0.97 and 1.32. The four blocks where the text engine collapsed into
	// repetition or into nothing score 0.08, 0.33, 1.75 and 44.86.
	minBlockFill = 0.7
	maxBlockFill = 1.4

	// minGapChars is the characters a gap between two boxes of one printed row
	// must have room for before the merge draws a box over it.
	//
	// Below it the gap is the space between two words, which every row has. Over
	// the 183 pages of books 17 and 18, 31 pairs of boxes of one row leave a gap
	// at all, and the median holds 2.8 characters.
	minGapChars = 2.0

	// minGapOverflow is the share of what a box was drawn around that its piece
	// must hold before the merge reads the excess as ink standing in the gap
	// beside it. A piece that fits its box says the gap held nothing worth a box.
	//
	// Over the gaps no wider than the box beside them, the two boxes this exists
	// for hold 1.83 and 2.00 times their room, and nothing else holds more than
	// 1.23. Measured against every gap the share runs to 1.71, and the width test
	// in shareAcrossGap is what opens that distance up.
	minGapOverflow = 1.5

	// fullWidthShare is the share of the widest line of a block that a line must
	// span before its slope is measured.
	//
	// A detector fits a steep quad to a narrow fragment. On the measured page the
	// fragments report slopes of 0.27 to 0.37 against a page that slopes 0.086, so
	// a median over every line of the block would read the fragments and not the
	// page.
	fullWidthShare = 0.6
)
