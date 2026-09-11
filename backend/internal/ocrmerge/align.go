package ocrmerge

import "unicode"

// The cost of one step of the alignment, in tenths, so the table holds no float.
//
// The relation between costSub and costGap is the whole design. A substitution
// costs 10 and a pair of gaps costs 16, so the alignment prefers to pair two
// unlike runes over dropping both. A substitution says both engines saw a glyph
// here and read it differently, which is the case the merge exists to resolve. A
// gap says one engine saw text the other never saw, which is the case the merge
// has to be careful about. Reverse the relation and a line of Greek the box
// engine mangled tears into gaps instead of aligning.
const (
	costEqual = 0
	costFold  = 1
	costSub   = 10
	costGap   = 8
)

const (
	opMatch byte = iota
	opDropLine
	opDropBlock
)

// alignment answers where each rune of the line stream lands in the block stream.
//
// cut[i] is the block position reached just before the line stream consumes rune
// i, and cut[len(line)] is the end of the block. agree[i] reports whether rune i
// paired with a block rune equal after folding; a rune that paired with nothing,
// or with something else, does not agree.
//
// A pair of unlike whitespace costs nothing to align and still does not agree. A
// space is evidence of nothing, and the thresholds in ocrmerge.go were measured
// against a share that leaves whitespace out.
type alignment struct {
	cut   []int
	agree []bool
}

// align runs Needleman-Wunsch over the two rune streams. It answers nil when the
// streams are too large to align, which the caller reports rather than hides.
func align(line, block []rune) *alignment {
	n, m := len(line), len(block)
	if (n+1)*(m+1) > maxAlignCells {
		return nil
	}

	table := make(map[string]int, n+m)
	lineIDs := foldIDs(line, table)
	blockIDs := foldIDs(block, table)

	lineSpace := spaceFlags(line)
	blockSpace := spaceFlags(block)

	back := make([]byte, (n+1)*(m+1))
	for j := 1; j <= m; j++ {
		back[j] = opDropBlock
	}

	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for j := 1; j <= m; j++ {
		prev[j] = j * costGap
	}

	for i := 1; i <= n; i++ {
		row := back[i*(m+1):]
		row[0] = opDropLine
		cur[0] = prev[0] + costGap

		for j := 1; j <= m; j++ {
			var pair int
			switch {
			case line[i-1] == block[j-1] || (lineSpace[i-1] && blockSpace[j-1]):
				pair = costEqual
			case lineIDs[i-1] == blockIDs[j-1]:
				pair = costFold
			default:
				pair = costSub
			}

			best, op := prev[j-1]+pair, opMatch
			if v := prev[j] + costGap; v < best {
				best, op = v, opDropLine
			}
			if v := cur[j-1] + costGap; v < best {
				best, op = v, opDropBlock
			}
			cur[j], row[j] = best, op
		}
		prev, cur = cur, prev
	}

	// Walking the path backwards and writing cut after each step gives the block
	// position the forward walk would have held when it consumed that rune.
	result := &alignment{cut: make([]int, n+1), agree: make([]bool, n)}
	result.cut[n] = m
	for i, j := n, m; i > 0 || j > 0; {
		switch back[i*(m+1)+j] {
		case opMatch:
			i, j = i-1, j-1
			result.cut[i] = j
			result.agree[i] = lineIDs[i] == blockIDs[j]
		case opDropLine:
			i--
			result.cut[i] = j
		default:
			j--
		}
	}
	return result
}

func spaceFlags(runes []rune) []bool {
	flags := make([]bool, len(runes))
	for i, r := range runes {
		flags[i] = unicode.IsSpace(r)
	}
	return flags
}
