package ocrmerge

import "sort"

// groupLines answers, for every block, the lines that block owns, and leaves a
// line no block covers out of every group.
//
// Geometry decides the pairing, never order. Both engines read the same pixels,
// so a box is the one thing they cannot disagree about. Their reading orders can
// and do differ, and grouping first makes that difference cost nothing.
func groupLines(lines []Line, blocks []Block) map[int][]int {
	groups := make(map[int][]int, len(blocks))

	for i, line := range lines {
		best, owner := 0.0, -1
		for k, block := range blocks {
			if overlapShare(line.Box, block.Box) < minOverlap {
				continue
			}

			// Covering a line is what qualifies a block to own it, and it does not
			// decide between two that both do. A footnote of one row stands as far
			// inside the foot of the block above it as inside its own, and the
			// block above covers more of it because it is taller. Which block the
			// line is a line of is the question, so the two are ranked by how much
			// of each other the boxes are.
			if fit := overlapFit(line.Box, block.Box); fit > best {
				best, owner = fit, k
			}
		}
		if owner >= 0 {
			groups[owner] = append(groups[owner], i)
		}
	}

	return groups
}

// overlapFit answers the share of the two boxes together that both of them cover.
func overlapFit(line, block Box) float64 {
	width := min(line[2], block[2]) - max(line[0], block[0])
	height := min(line[3], block[3]) - max(line[1], block[1])
	if width <= 0 || height <= 0 {
		return 0
	}
	shared := width * height
	union := boxArea(line) + boxArea(block) - shared
	if union <= 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

// overlapShare answers the share of the line box that the block box covers.
func overlapShare(line, block Box) float64 {
	width := min(line[2], block[2]) - max(line[0], block[0])
	height := min(line[3], block[3]) - max(line[1], block[1])
	if width <= 0 || height <= 0 {
		return 0
	}
	return float64(width*height) / float64(max(1, boxArea(line)))
}

func boxArea(b Box) int {
	width, height := b[2]-b[0], b[3]-b[1]
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height
}

// rowsOf groups the members of one block into the printed rows of the page, top
// to bottom, each row left to right. The order the box engine reported is not
// reading order: it can be detection order, which interleaves lines from opposite
// sides of the page.
//
// Two boxes of one row stand beside each other, and two rows share the column.
// That is what decides a row here, because on a page whose lines are not level a
// box sits nearer the top of the row below it than to its own row mate: the
// detector breaks such a row into a left box and a right box, and the right one
// stands most of a row lower. Tops alone cannot tell the two cases apart.
//
// A box has to clear the column of every box already in the row, and to stand at
// the height of the last of them. Comparing the height against the last rather
// than the first lets a row follow a baseline that drifts across the page.
func rowsOf(lines []Line, members []int) [][]int {
	if len(members) == 0 {
		return nil
	}

	ordered := make([]int, len(members))
	copy(ordered, members)
	sort.SliceStable(ordered, func(a, b int) bool {
		return lines[ordered[a]].Box[1] < lines[ordered[b]].Box[1]
	})

	rows := make([][]int, 0, len(ordered))
	row := []int{ordered[0]}
	for _, index := range ordered[1:] {
		if beside(lines, row, index) {
			row = append(row, index)
			continue
		}
		rows = append(rows, sortByLeft(lines, row))
		row = []int{index}
	}
	return append(rows, sortByLeft(lines, row))
}

func beside(lines []Line, row []int, index int) bool {
	box := lines[index].Box
	for _, member := range row {
		other := lines[member].Box
		shared := spanOverlap(other[0], other[2], box[0], box[2])
		narrower := min(other[2]-other[0], box[2]-box[0])
		if float64(shared) > maxRowShare*float64(narrower) {
			return false
		}
	}

	last := lines[row[len(row)-1]].Box
	shared := spanOverlap(last[1], last[3], box[1], box[3])
	shorter := min(last[3]-last[1], box[3]-box[1])
	return float64(shared) > minRowOverlap*float64(shorter)
}

func spanOverlap(lowA, highA, lowB, highB int) int {
	return min(highA, highB) - max(lowA, lowB)
}

// readingOrder reads the rows of one block out in order.
func readingOrder(lines []Line, members []int) []int {
	return flatten(rowsOf(lines, members), len(members))
}

func flatten(rows [][]int, size int) []int {
	ordered := make([]int, 0, size)
	for _, row := range rows {
		ordered = append(ordered, row...)
	}
	return ordered
}

// anchorOf answers the box a row is measured from: its leftmost, which is where
// the printed row starts. Every other box of the row stands further along a
// baseline that may fall, so no two rows can be compared through them.
func anchorOf(lines []Line, row []int) Box { return lines[row[0]].Box }

func sortByLeft(lines []Line, row []int) []int {
	sort.SliceStable(row, func(a, b int) bool {
		return lines[row[a]].Box[0] < lines[row[b]].Box[0]
	})
	return row
}

func medianHeight(lines []Line, members []int) int {
	heights := make([]int, len(members))
	for i, index := range members {
		heights[i] = lines[index].Box[3] - lines[index].Box[1]
	}
	sort.Ints(heights)
	return heights[len(heights)/2]
}
