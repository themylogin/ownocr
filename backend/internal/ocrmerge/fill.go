package ocrmerge

import (
	"sort"
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

// newFiller measures the pitch from row to row, each row through its anchor. Two
// boxes of one row are no step and never reach the sample, so a page the detector
// broke in half measures the same pitch as a page it read whole.
func newFiller(lines []Line, groups map[int][]int) filler {
	var steps []int
	for _, owner := range sortedKeys(groups) {
		rows := rowsOf(lines, groups[owner])
		for at := 1; at < len(rows); at++ {
			steps = append(steps,
				centreOf(anchorOf(lines, rows[at]))-centreOf(anchorOf(lines, rows[at-1])))
		}
	}
	if len(steps) < minPitchSamples {
		return filler{}
	}
	sort.Ints(steps)
	return filler{pitch: steps[len(steps)/2]}
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
