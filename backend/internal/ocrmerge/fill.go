package ocrmerge

import (
	"strings"
	"unicode/utf8"
)

// filler places boxes for lines the box engine never read.
//
// It rests on one fact about a printed page: the lines of a block stand a fixed
// distance apart. Measure that distance on the lines the box engine did read,
// and a gap wide enough for whole lines says how many are missing and where each
// one sits. The filler answers centres; the caller turns them into boxes.
//
// A pitch it cannot measure is zero, and a zero pitch fills nothing.
type filler struct{ pitch int }

// newFiller measures the pitch over every block of a page. Merge builds its own
// filler from a reference that leaves the unusable blocks out.
func newFiller(lines []Line, groups map[int][]int) filler {
	rows := make(map[int][][]int, len(groups))
	for owner, members := range groups {
		rows[owner] = rowsOf(lines, members)
	}
	return filler{pitch: measure(lines, rows, nil).pitch}
}

// between answers the centre of every line that fits in the gap between two lines
// that the box engine read.
//
// A gap has to divide into a whole number of pitches to be read as missing
// lines. That test is what tells a paragraph break, which is wider than a line
// and no line of text, from lines the detector missed.
func (f filler) between(above, below Box) []int {
	if f.pitch <= 0 {
		return nil
	}
	low, high := centreOf(above), centreOf(below)
	gap := float64(high - low)
	steps := int((gap + float64(f.pitch)/2) / float64(f.pitch))
	if steps < 2 {
		return nil
	}
	if diff := gap/float64(f.pitch) - float64(steps); diff > maxPitchError || diff < -maxPitchError {
		return nil
	}

	centres := make([]int, 0, steps-1)
	for i := 1; i < steps; i++ {
		centres = append(centres, low+(high-low)*i/steps)
	}
	return centres
}

// before answers the centre of every line that fits between the top of a block
// and the first line the box engine read in it.
func (f filler) before(block, first Box, height int) []int {
	if f.pitch <= 0 {
		return nil
	}
	var centres []int
	for c := centreOf(first) - f.pitch; f.inside(block, c, height); c -= f.pitch {
		centres = append(centres, c)
	}
	for i, j := 0, len(centres)-1; i < j; i, j = i+1, j-1 {
		centres[i], centres[j] = centres[j], centres[i]
	}
	return centres
}

// after answers the centre of every line that fits between the last line the box
// engine read in a block and the bottom of that block.
func (f filler) after(block, last Box, height int) []int {
	if f.pitch <= 0 {
		return nil
	}
	var centres []int
	for c := centreOf(last) + f.pitch; f.inside(block, c, height); c += f.pitch {
		centres = append(centres, c)
	}
	return centres
}

// below answers the centre of every line that fits under one row: between it and
// the row that follows, or between it and the foot of the block when none does.
// Both measure from anchor to anchor, because no other box of a row can be
// compared with another row.
func (f filler) below(block Box, lines []Line, rows [][]int, at, height int) []int {
	if at+1 < len(rows) {
		return f.between(anchorOf(lines, rows[at]), anchorOf(lines, rows[at+1]))
	}
	return f.after(block, anchorOf(lines, rows[at]), height)
}

func (f filler) inside(block Box, centre, height int) bool {
	slack := int(float64(height) * edgeSlack)
	return centre-height/2 >= block[1]-slack && centre+height/2 <= block[3]+slack
}

// box answers a line box for a filled centre. It spans the block, because the
// merge knows the column the line stands in and not where its text stops.
func box(block Box, centre, height int) Box {
	return Box{block[0], centre - height/2, block[2], centre + height/2}
}

// gapBox answers a box for the ink standing between two boxes of one printed row.
//
// The row stands where its two known boxes stand, so the centre of the gap is
// read off theirs, weighted by how near each one lies. The height is the shorter
// of the two, because a box stands taller than its ink by the rise of its line
// over its own width and the gap is narrower than either neighbour.
func gapBox(left, right Box) Box {
	height := min(left[3]-left[1], right[3]-right[1])
	centre := centreOf(left)

	from, to := (left[0]+left[2])/2, (right[0]+right[2])/2
	if to > from {
		at := (left[2] + right[0]) / 2
		centre += (centreOf(right) - centreOf(left)) * (at - from) / (to - from)
	}
	return Box{left[2], centre - height/2, right[0], centre + height/2}
}

// shareAcrossGap divides the piece of a box between that box and the ink the box
// engine left uncovered between it and the next box of its row. It answers the
// text of each, and whether there is a gap to divide across at all.
//
// Three things have to hold. The gap has room for characters, which the space
// between two words does not. The gap is no wider than the box beside it. And the
// piece holds more than that box was drawn around, which a piece that fits its
// box does not.
//
// Each side takes the share of the text that its width is of the two together,
// which holds only where the ink runs at one density across both. A gap wider
// than the box it is measured from carries no such measurement, and the contents
// page of book 18 is what says so: its gaps are rows of leader dots, sparse ink
// over 1011 pixels beside a box of 189, and dividing by width there moves three
// letters of a title into the dots.
//
// Under those three the split falls where it falls, mid-word included, because
// the ink in the gap is mid-word: the row this exists for reads `otro—, tiene`
// over a box holding `otro—,`, a gap holding `tien`, and a box holding
// `e deseos costosos`.
func shareAcrossGap(text string, box, next Box, room int, density float64) (string, string, bool) {
	gap := next[0] - box[2]
	width := box[2] - box[0]
	if density <= 0 || room <= 0 || width <= 0 {
		return "", "", false
	}
	if float64(gap)*density < minGapChars || gap > width {
		return "", "", false
	}

	held := utf8.RuneCountInString(text)
	if float64(held) < minGapOverflow*float64(room) {
		return "", "", false
	}

	total := width + gap
	mine := (held*width + total - 1) / total
	if mine < 1 || mine >= held {
		return "", "", false
	}

	runes := []rune(text)
	head, tail := collapseSpace(string(runes[:mine])), collapseSpace(string(runes[mine:]))
	if !hasText(head) || !hasText(tail) {
		return "", "", false
	}
	return head, tail, true
}

func centreOf(b Box) int { return (b[1] + b[3]) / 2 }

func medianHeightOf(lines []Line, members []int) int {
	if len(members) == 0 {
		return 0
	}
	return medianHeight(lines, members)
}

// wrapInto answers text as exactly n lines, and nil when it cannot.
//
// The width of the column is not what decides where a line breaks here, because
// the text engine reflowed the block and kept no width. The line count does: it
// comes from the geometry, so the narrowest column that still holds the text in
// n lines is the one the page was set in. Reading it back greedily is how a
// justified page breaks.
//
// It never breaks a word, so a line the printer broke with a hyphen comes back
// whole on one of the two lines. The text is all there and one word stands in the
// wrong box.
func wrapInto(text string, n int) []string {
	words := strings.Fields(text)
	if n <= 0 || len(words) < n {
		return nil
	}

	low, high := 0, 0
	for _, word := range words {
		length := utf8.RuneCountInString(word)
		if length > low {
			low = length
		}
		high += length + 1
	}

	// The line count falls as the width grows, so the narrowest width that holds
	// the text in n lines or fewer is the one to read back.
	for low < high {
		mid := (low + high) / 2
		if len(wrapAt(words, mid)) <= n {
			high = mid
			continue
		}
		low = mid + 1
	}

	wrapped := wrapAt(words, low)
	if len(wrapped) != n {
		return nil
	}
	return wrapped
}

func wrapAt(words []string, width int) []string {
	var wrapped []string
	line, length := strings.Builder{}, 0
	for _, word := range words {
		size := utf8.RuneCountInString(word)
		if length > 0 && length+1+size > width {
			wrapped = append(wrapped, line.String())
			line.Reset()
			length = 0
		}
		if length > 0 {
			line.WriteByte(' ')
			length++
		}
		line.WriteString(word)
		length += size
	}
	if length > 0 {
		wrapped = append(wrapped, line.String())
	}
	return wrapped
}

// agreementWith answers the share of the runes of a line that pair with an equal
// rune of the text after folding. It is what agreementOf answers for a piece the
// block cut, measured for a piece the filler wrapped instead.
func agreementWith(lineText, text string) float64 {
	line := []rune(lineText)
	if len(line) == 0 {
		return 0
	}
	aligned := align(line, []rune(text))
	if aligned == nil {
		return 0
	}
	agreed := 0
	for _, ok := range aligned.agree {
		if ok {
			agreed++
		}
	}
	return float64(agreed) / float64(len(line))
}
