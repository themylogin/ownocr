package ocrmerge

import "math"

// levelRows moves the top and the bottom edge of every line of a printed row onto
// one baseline and one height. It answers the lines with those boxes, and how many
// rows it moved.
//
// A box is a rectangle and a printed row on a photographed page is not. The
// rectangle reaches the highest point of the ink and the lowest, so it stands
// taller than the row by the slope times its own width, and two boxes covering
// different stretches of one row stand at different heights. A renderer reads both
// the glyph size and the baseline off that rectangle, so it draws such a row too
// large and in pieces that do not line up.
//
// Only the top and the bottom move. Every line keeps the column it covered, so no
// line reaches ink that is not its own.
//
// The lines of a block the interpolation replaces are left alone: they leave the
// answer, and the boxes that stand in their place are already level.
func levelRows(lines []Line, rows map[int][][]int, unusable map[int]bool, pitch int) ([]Line, int) {
	if pitch <= 0 {
		return lines, 0
	}

	levelled := make([]Line, len(lines))
	copy(levelled, lines)

	moved := 0
	for owner, blockRows := range rows {
		if unusable[owner] {
			continue
		}
		for _, row := range blockRows {
			if levelRow(levelled, row, pitch) {
				moved++
			}
		}
	}
	return levelled, moved
}

// levelRow answers whether it moved the row. A row it cannot measure keeps the
// boxes the box engine gave.
func levelRow(lines []Line, row []int, pitch int) bool {
	left, top, right, bottom := rowBox(lines, row)

	// A row of one line that stands no taller than the distance to the next row
	// already draws as one straight line of a size the page allows. Levelling it
	// would move it by the pixel or two its own slight slope accounts for, which
	// no reader can see and every stored result would have to be rewritten for.
	if len(row) == 1 && bottom-top <= pitch {
		return false
	}

	slope, ok := slopeOf(lines, row)
	if !ok {
		return false
	}

	width := right - left
	drop := math.Abs(slope) * float64(width)

	height := float64(bottom-top) - drop
	if height > float64(pitch) {
		// Two printed rows do not overlap, so no row stands taller than the
		// distance to the next. This bounds what the slope does not account for:
		// the box engine pads a quad, and on the measured pages it pads one by
		// more than a row.
		height = float64(pitch)
	}
	if height < 1 {
		return false
	}

	// The bottom of the row box is the lowest ink of the row, which stands at
	// whichever end the row falls to. The middle of the row stands half the fall
	// above it, and the middle is where a level box belongs.
	//
	// Anchoring on the box and not on the corners is what leaves a page that does
	// not slope exactly as it was: the fall is zero, so the row answers the box it
	// came with, to the pixel.
	low := bottom - int(math.Round(drop/2))
	high := low - int(math.Round(height))

	changed := false
	for _, index := range row {
		box := lines[index].Box
		if box[1] == high && box[3] == low {
			continue
		}
		lines[index].Box = Box{box[0], high, box[2], low}
		changed = true
	}
	return changed
}

// slopeOf fits the bottom edges of every line of a row and answers how far that
// fit falls per pixel of width. The bottom edge of a quad follows the baseline of
// its line, so a row read in pieces is fitted over every piece and the widest one
// carries the fit.
//
// A row holding a line that reports no corners is not measured. A result stored
// before the merge read corners carries none, so such a page keeps every box.
func slopeOf(lines []Line, row []int) (float64, bool) {
	xs := make([]float64, 0, 2*len(row))
	ys := make([]float64, 0, 2*len(row))
	for _, index := range row {
		corners := lines[index].Corners
		if len(corners) != 4 {
			return 0, false
		}
		for _, at := range [2]Point{corners[3], corners[2]} {
			xs = append(xs, float64(at[0]))
			ys = append(ys, float64(at[1]))
		}
	}
	if len(xs) < 2 {
		return 0, false
	}

	meanX, meanY := mean(xs), mean(ys)
	spread, covariance := 0.0, 0.0
	for i := range xs {
		spread += (xs[i] - meanX) * (xs[i] - meanX)
		covariance += (xs[i] - meanX) * (ys[i] - meanY)
	}
	if spread <= 0 {
		return 0, true
	}
	return covariance / spread, true
}

// rowBox answers the box that covers every line of a row.
func rowBox(lines []Line, row []int) (int, int, int, int) {
	box := lines[row[0]].Box
	left, top, right, bottom := box[0], box[1], box[2], box[3]
	for _, index := range row[1:] {
		box = lines[index].Box
		left = min(left, box[0])
		top = min(top, box[1])
		right = max(right, box[2])
		bottom = max(bottom, box[3])
	}
	return left, top, right, bottom
}

func mean(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
