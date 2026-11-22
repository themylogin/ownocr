package ocr

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/themylogin/ownocr/backend/internal/latex"
	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

// boxResult is the part of a PP-OCRv6 result the merge reads. The tags of
// ocrmerge.Line already match what the engine writes under `lines`.
//
// The quads live beside the lines and not in them. The engine writes one box per
// line under `lines`, and the four corners it found that line in under
// `raw.dt_polys`, so the merge reads both and pairs them by index.
type boxResult struct {
	Lines []ocrmerge.Line `json:"lines"`
	Raw   struct {
		Polys [][][]float64 `json:"dt_polys"`
	} `json:"raw"`
}

// textResult is the part of a PaddleOCR-VL result the merge reads. The markdown of
// that engine leaves footnote and page number blocks out, and `parsing_res_list`
// does not, so the merge reads the blocks and never the markdown.
type textResult struct {
	Blocks []textBlock `json:"parsing_res_list"`
}

type textBlock struct {
	Box  ocrmerge.Box `json:"block_bbox"`
	Text string       `json:"block_content"`
}

// MergeOf joins the line boxes of the PP-OCRv6 result with the text of the
// PaddleOCR-VL result. It reads both engine results and changes neither.
func MergeOf(boxData, textData []byte) (ocrmerge.Result, error) {
	var boxes boxResult
	if err := json.Unmarshal(boxData, &boxes); err != nil {
		return ocrmerge.Result{}, fmt.Errorf("the %s result: %w", ModeV6, err)
	}

	var text textResult
	if err := json.Unmarshal(textData, &text); err != nil {
		return ocrmerge.Result{}, fmt.Errorf("the %s result: %w", ModeVL, err)
	}

	// Before the merge, and not after it. The merge cuts the text of a block at
	// the line boundaries, so a fragment can straddle two lines of the result,
	// and a rule that reads one line cannot see the whole fragment.
	blocks := make([]ocrmerge.Block, len(text.Blocks))
	for i, block := range text.Blocks {
		blocks[i] = ocrmerge.Block{Box: block.Box, Text: latex.Clean(block.Text)}
	}

	return ocrmerge.Merge(ocrmerge.Input{
		Lines:      ocrmerge.WithCorners(boxes.Lines, boxes.Raw.Polys),
		Blocks:     blocks,
		BoxEngine:  ModeV6,
		TextEngine: ModeVL,
	}), nil
}

// minTextShare is the share of the lines of a merged result that must carry the
// text engine's own text before the merge counts as a reading of that page.
//
// It guards against an answer that is no transcription at all: a model that
// describes the page instead of reading it produces a block that aligns with no
// line, the merge falls back to the box engine's text for every line, and the
// result would otherwise be stored as a better reading holding none of its own
// text. Measured over the 11 pages of books 17, 18 and 24 that hold a
// transcription: ten place text on 95% to 100% of their lines, and the one
// narrated answer places it on none.
const minTextShare = 0.5

// MergeOfText joins the line boxes of the PP-OCRv6 result with plain text that
// carries no geometry, through one block covering the whole page.
//
// A text engine that reports no region leaves the merge nothing to place its text
// by, so the block is the page and the merge reads the printed rows from the
// PP-OCRv6 boxes alone, which is what it does for any block. Measured over the
// 293 read pages of books 17, 18 and 24: no page loses a box to the disaster
// path, two-column pages and pages with small type at the foot included.
//
// The second answer is false when the text reached too few lines to be a reading
// of that page.
func MergeOfText(boxData []byte, text string, width, height int) (ocrmerge.Result, bool, error) {
	var boxes boxResult
	if err := json.Unmarshal(boxData, &boxes); err != nil {
		return ocrmerge.Result{}, false, fmt.Errorf("the %s result: %w", ModeV6, err)
	}

	lines := ocrmerge.WithCorners(boxes.Lines, boxes.Raw.Polys)

	merged := ocrmerge.Merge(ocrmerge.Input{
		Lines: lines,
		Blocks: []ocrmerge.Block{{
			Box:  pageBlock(lines, width, height),
			Text: latex.Clean(text),
		}},
		BoxEngine:  ModeV6,
		TextEngine: ModeGLM,
	})

	if merged.Counts.Lines == 0 {
		return merged, false, nil
	}
	share := float64(merged.Counts.FromText) / float64(merged.Counts.Lines)
	return merged, share >= minTextShare, nil
}

// ReadingText answers the text a reading engine wrote for a page, in full.
//
// The markdown of PaddleOCR-VL is not that text: it leaves out the footnote and
// the page number blocks, which `parsing_res_list` holds. A judge that read the
// markdown would call every page carrying a footnote truncated, and every one of
// them would be a page that engine read whole.
//
// An engine that answers text and no blocks wrote what it wrote, so its text is
// itself.
func ReadingText(engine, txt string, data []byte) string {
	if engine != ModeVL {
		return txt
	}

	var text textResult
	if err := json.Unmarshal(data, &text); err != nil || len(text.Blocks) == 0 {
		return txt
	}

	blocks := make([]string, 0, len(text.Blocks))
	for _, block := range text.Blocks {
		blocks = append(blocks, block.Text)
	}
	return strings.Join(blocks, "\n\n")
}

// MergedText is the text of a merged result: one line per line, in page order.
func MergedText(merged ocrmerge.Result) string {
	lines := make([]string, len(merged.Lines))
	for i, line := range merged.Lines {
		lines[i] = line.Text
	}
	return strings.Join(lines, "\n")
}

// pageBlock is the box of the one block a text engine with no geometry merges
// through: the stored size of the page, widened to hold every line the box engine
// read.
//
// A block owns only the lines that lie inside it, so a block that failed to cover
// a line would leave that line to the box engine and the whole page would read as
// no transcription of itself. The stored size is the size of the image the engines
// read, so it covers them already — unless a page carries a size that does not
// describe its image, and then the boxes are the better evidence of where the page
// is.
func pageBlock(lines []ocrmerge.Line, width, height int) ocrmerge.Box {
	box := ocrmerge.Box{0, 0, width, height}
	for _, line := range lines {
		if line.Box[0] < box[0] {
			box[0] = line.Box[0]
		}
		if line.Box[1] < box[1] {
			box[1] = line.Box[1]
		}
		if line.Box[2] > box[2] {
			box[2] = line.Box[2]
		}
		if line.Box[3] > box[3] {
			box[3] = line.Box[3]
		}
	}
	return box
}
