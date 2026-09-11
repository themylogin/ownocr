package ocrmerge

import (
	"math"
	"sort"
	"unicode/utf8"
)

// reference is what the merge measures on the blocks whose boxes it trusts: the
// distance from one printed row to the next, and the runes a printed row holds.
//
// Both leave the unusable blocks out. The rows of such a block are not the rows
// of the page, so its steps poison the pitch every other block rests on: on the
// measured page its own rows answer 68 pixels where the rest of the page answers
// 81.
//
// A figure it cannot measure is zero, and a zero refuses the interpolation. A
// page whose every block is unusable measures nothing, so it keeps every box it
// came with.
type reference struct {
	pitch int
	runes int
}

// measure reads the pitch from row to row, each row through its anchor, and the
// runes a row holds from the rows themselves. Two boxes of one row are no step
// and one row, so a page the detector broke in half measures the same as a page
// it read whole.
//
// The order the blocks are read in does not reach the answer, because both
// figures are medians over a sorted list.
func measure(lines []Line, rows map[int][][]int, skip map[int]bool) reference {
	var steps, held []int
	for owner, blockRows := range rows {
		if skip[owner] {
			continue
		}
		for at, row := range blockRows {
			if at > 0 {
				steps = append(steps, centreOf(anchorOf(lines, row))-
					centreOf(anchorOf(lines, blockRows[at-1])))
			}

			runes := 0
			for _, index := range row {
				runes += utf8.RuneCountInString(lines[index].Text)
			}
			// A row the box engine read badly holds a handful of runes and would
			// drag the median down, and a row that ends a paragraph is short
			// because the printer set it short. Neither says what a row holds.
			if runes > shortLine {
				held = append(held, runes)
			}
		}
	}

	var ref reference
	if len(steps) >= minPitchSamples {
		sort.Ints(steps)
		ref.pitch = steps[len(steps)/2]
	}
	if len(held) >= minPitchSamples {
		sort.Ints(held)
		ref.runes = held[len(held)/2]
	}
	return ref
}

// unusableBoxes reports whether the boxes of a block say nothing about where its
// printed rows are. Two tests have to hold, and they answer different questions.
//
// The first asks whether any reading order follows from the boxes, and boxLie
// measures it.
//
// The second asks which of the two readings is broken, because the first cannot.
// Two engines that read the same pixels and report the same amount of text
// disagree only about where that text goes, and the first test says the box
// engine is the side that cannot place it. A text engine that collapsed reports
// far too little text or repeats itself without end, and the merge leaves such a
// block to the gates that refuse a line the two engines did not read alike.
//
// The pitch comes from the page and not from the rows of this block. A block of
// two rows spans one step, and one step is no measurement: the footnote of the
// measured page answers 24 pixels where its print stands 60 apart, and a pitch
// that small calls any block unusable.
func unusableBoxes(lines []Line, members []int, block Block, pitch int) bool {
	if boxLie(lines, members, block.Box, pitch) < maxBoxLie {
		return false
	}
	fill := blockFill(lines, members, block.Text)
	return fill >= minBlockFill && fill <= maxBlockFill
}

// boxLie answers the largest vertical error, in row pitches, between the top
// edges of two boxes of a block that stand beside each other in the column.
//
// A rectangle drawn round a sloping line reaches the highest point of that line,
// so its top edge stands above the ink by however far the line rises from that
// point to its own end. Two boxes that cover different stretches of the column
// therefore carry different errors, and the difference is the slope times the
// distance between the points where each one reaches its highest.
//
// Rows are grouped by that top edge. Once the difference reaches the distance
// from one printed row to the next, the two boxes stand a whole row apart in a
// grouping that should have put them in one, and the reading order is wrong.
//
// Only boxes that stand beside each other are compared, because only they can be
// two halves of one printed row. That is what tells a broken row from a paragraph
// ending in a short line. Both are a short box in a block of long ones, and the
// short box of a broken row shares no column with its own row mate while the last
// line of a paragraph shares its whole column with the line above it. Measuring
// widths alone called three blocks of the measured books unusable that merge
// correctly, one of them a footnote whose last line reads `nos puntos.`
func boxLie(lines []Line, members []int, block Box, pitch int) float64 {
	slope := blockSlope(lines, members)
	if pitch <= 0 || slope == 0 {
		return 0
	}

	worst := 0
	for at, one := range members {
		for _, other := range members[at+1:] {
			if !besideInColumn(lines[one].Box, lines[other].Box) {
				continue
			}
			gap := highestAt(lines[one].Box, slope) - highestAt(lines[other].Box, slope)
			worst = max(worst, abs(gap))
		}
	}
	return math.Abs(slope) * float64(worst) / float64(pitch)
}

// highestAt answers the x at which a box reaches its highest point: the right
// edge of a line that rises to the right, and the left edge of one that falls.
func highestAt(box Box, slope float64) int {
	if slope < 0 {
		return box[2]
	}
	return box[0]
}

// besideInColumn reports whether two boxes stand beside each other rather than
// one over the other. It is the column half of what tells two boxes of one
// printed row from two rows, and it holds whatever the two boxes do vertically.
func besideInColumn(one, other Box) bool {
	shared := spanOverlap(one[0], one[2], other[0], other[2])
	narrower := min(one[2]-one[0], other[2]-other[0])
	return float64(shared) <= maxRowShare*float64(narrower)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// blockFill answers the runes of a block against the runes of the lines assigned
// to it, whitespace left out of both.
//
// A block whose lines hold nothing answers zero, which no bound accepts. The box
// engine read nothing there, so it has no boxes worth taking away.
func blockFill(lines []Line, members []int, text string) float64 {
	read := 0
	for _, index := range members {
		read += nonSpaceCount(lines[index].Text)
	}
	if read == 0 {
		return 0
	}
	return float64(nonSpaceCount(text)) / float64(read)
}

// rowCount answers how many printed rows a block holds, and whether the page says
// so at all.
//
// It derives the count two ways and answers nothing unless both round to the same
// number. Neither way is safe alone. The runes overstate a block that ends a
// paragraph, because the printer set its last row short. The height is worthless
// without the slope: the measured block of eight rows stands 765 pixels tall where
// its rows are 81 apart, and 765 over 81 is 9.4. Taking the slope off the height
// answers 7.7, and the runes answer 8.0.
func rowCount(ref reference, block Box, slope float64, text string) (int, bool) {
	if ref.pitch <= 0 || ref.runes <= 0 {
		return 0, false
	}

	// The magnitude of the slope, because a block of sloping rows stands taller
	// than its rows whichever way they slope.
	width, height := block[2]-block[0], block[3]-block[1]
	span := float64(height) - math.Abs(slope)*float64(width)
	if span <= 0 {
		return 0, false
	}

	fromText := float64(utf8.RuneCountInString(text)) / float64(ref.runes)
	fromSpan := span / float64(ref.pitch)

	rows := int(fromText + 0.5)
	if rows < 1 || int(fromSpan+0.5) != rows {
		return 0, false
	}
	return rows, true
}

// interpolate answers the text of a block as the rows the page says it holds, in
// boxes that tile the block from its top to its bottom.
//
// The boxes tile it because the region and the number of rows in it are the only
// two things the merge knows here. It does not know where the ink of a row starts
// or stops, so every line spans the block. It does not know how tall a row is, so
// the rows divide the block evenly.
//
// The lines are level, and on a block this exists to repair the ink is not. The
// text is right and in reading order, which is the fault being repaired; a line
// that followed the ink would need a box that is not a rectangle.
func interpolate(block Block, rows int, engine string) []MergedLine {
	texts := wrapInto(block.Text, rows)
	if texts == nil {
		return nil
	}

	top, span := block.Box[1], block.Box[3]-block.Box[1]
	placed := make([]MergedLine, rows)
	for i, text := range texts {
		placed[i] = MergedLine{
			Box: Box{
				block.Box[0], top + span*i/rows,
				block.Box[2], top + span*(i+1)/rows,
			},
			Text:      text,
			Source:    engine,
			Estimated: true,
		}
	}
	return placed
}
