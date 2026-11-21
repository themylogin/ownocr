// Package guttertrim finds the strip of a split page that holds the ink of the
// page beside it.
//
// A spread is cut at a column and its gutter is a curve, so each half runs to
// the far extreme of the bow and carries a wedge of its neighbour. The wedge
// shows as a column of truncated words against the inner edge.
//
// The package reads boxes and nothing else. It opens no file, calls no engine,
// and knows no alphabet.
package guttertrim

import "github.com/themylogin/ownocr/backend/internal/ocrmerge"

// Trim is the strip a renderer covers: the edge it lies against, and how far
// from that edge it reaches.
type Trim struct {
	Side   string `json:"side"`
	Column int    `json:"column"`
}

// The measured constants. Every one comes from books 17 and 18 of the
// development database, and design.md of the add-gutter-trim change holds the
// distribution each was read off.
const (
	// edgeShare is how far from the gutter edge a fragment may begin. The cut
	// truncates a word of the neighbouring page in the middle of its strokes,
	// so the box drawn around what is left reaches the edge. Every measured
	// fragment begins 0 to 4 pixels from it.
	//
	// This is the test that carries the rule. Shortness alone does not: a page
	// photographed at an angle makes the detector break its own printed rows in
	// half, and the left part of a broken row is short and stands in a column
	// against the inner edge. The nearest measured one begins 65 pixels from the
	// edge, and it is text of the page itself.
	edgeShare = 0.005

	// fragmentShare is how far a fragment may reach. Measured fragments reach
	// 0.029 of the width. The nearest line that touches the edge and is not a
	// fragment reaches 0.21, and the next 0.28.
	fragmentShare = 0.10

	// bodyShare is what a line must take of the width to name the margin of the
	// page. It stands far above fragmentShare so that no fragment can ever set
	// the body edge, which is the whole of its job. Line widths run continuously
	// through 0.50, so the value separates no two populations and does not need
	// to: a short last line of a paragraph begins at the same margin as the line
	// above it, and losing it costs nothing.
	//
	// It also keeps out the left part of a printed row the detector broke in
	// half, which takes 0.07 to 0.15 of the width. Such a part sits inside the
	// body, so admitting it would name a margin that is not there.
	bodyShare = 0.50

	// corridorShare is the empty space a trim needs between the fragments and
	// the body. The six measured corridors run 0.050 to 0.080 of the width. Less
	// than this means the two are not separated, and a trim placed there could
	// cut the page it was meant to clean.
	corridorShare = 0.03

	// minFragments refuses a lone mark. A strip carries the ends of the lines of
	// the page beside it, so it stands in a column. The six measured strips hold
	// 2, 4, 7, 8, 18 and 23 fragments; a single mark is a speck, a fold or a
	// shadow.
	minFragments = 2
)

// Of answers the trim of a page, or false when the page holds no strip.
//
// cutSide names which half of a spread the page is, as books.CutLeft and
// books.CutRight do. The gutter edge is the opposite edge, because the fold
// stands between the two halves.
func Of(boxes []ocrmerge.Box, width int, cutSide string) (Trim, bool) {
	edge, ok := gutterEdge(cutSide)
	if !ok || width <= 0 {
		return Trim{}, false
	}

	bodyEdge, fragmentEnd, fragments := -1, -1, 0
	for _, box := range boxes {
		near, far := insets(box, width, edge)
		if far <= near {
			continue
		}

		if float64(far-near) > bodyShare*float64(width) {
			if bodyEdge < 0 || near < bodyEdge {
				bodyEdge = near
			}
			continue
		}

		if float64(near) <= edgeShare*float64(width) &&
			float64(far) <= fragmentShare*float64(width) {
			fragments++
			if far > fragmentEnd {
				fragmentEnd = far
			}
		}
	}

	if fragments < minFragments || bodyEdge < 0 {
		return Trim{}, false
	}
	if float64(bodyEdge-fragmentEnd) < corridorShare*float64(width) {
		return Trim{}, false
	}

	// The middle of the corridor, not the last fragment found. The detector read
	// 2 fragments of one measured strip and 23 of another, while the ink of both
	// runs the height of the page. The corridor is empty by definition, so its
	// middle covers the fragments the detector missed and still cannot reach the
	// body.
	return Trim{Side: edge, Column: (fragmentEnd + bodyEdge + 1) / 2}, true
}

// Covers answers whether a box lies wholly inside the trim.
func (t Trim) Covers(box ocrmerge.Box, width int) bool {
	near, far := insets(box, width, t.Side)
	return far > near && far <= t.Column
}

// gutterEdge answers the edge the fold stands against, which is the edge
// opposite the half the page was cut as.
func gutterEdge(cutSide string) (string, bool) {
	switch cutSide {
	case "right":
		return "left", true
	case "left":
		return "right", true
	default:
		return "", false
	}
}

// insets measure a box from the gutter edge instead of from the left of the
// page, so one rule serves both sides.
func insets(box ocrmerge.Box, width int, edge string) (int, int) {
	if edge == "left" {
		return box[0], box[2]
	}
	return width - box[2], width - box[0]
}
