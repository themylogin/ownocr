// Package ocrmerge joins the line boxes of one OCR engine with the text of
// another. One engine reports a box for every line it read, which a renderer
// needs. The other reads characters the first cannot, but reports blocks of
// reflowed text instead of lines.
//
// The merge holds no list of characters and knows no alphabet. It compares
// runes after Unicode folding and decides everything else from geometry and
// from the alignment of the two readings.
package ocrmerge

// Box is a rectangle in the pixels of the page: left, top, right, bottom.
type Box [4]int

// Line is one line of the box engine.
type Line struct {
	Box   Box     `json:"bbox"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// Block is one region of the text engine.
type Block struct {
	Box  Box    `json:"bbox"`
	Text string `json:"text"`
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
// Estimated marks the exception: a line the box engine never read, whose box the
// merge placed from the spacing of the lines around it. Its score is zero,
// because no engine scored it.
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
)

// Counts reports one page. FromText and FromBoxes together count the lines the
// box engine read, so they add up to the input. Estimated counts the lines added
// on top of it.
type Counts struct {
	Lines        int `json:"lines"`
	FromText     int `json:"fromText"`
	FromBoxes    int `json:"fromBoxes"`
	Estimated    int `json:"estimated"`
	DroppedChars int `json:"droppedChars"`
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
	// minRowOverlap is the share of the shorter box that two row mates must share
	// of each other's height. The halves of one row there share 0.59 to 1.00 of
	// it, and boxes of two rows share at most 0.32.
	maxRowShare   = 0.25
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
