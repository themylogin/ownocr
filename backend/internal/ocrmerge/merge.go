package ocrmerge

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Merge fills the line boxes of the box engine with the text of the text engine.
//
// It answers a line for every line of the box engine, in that order, carrying
// that box and that confidence. A line it cannot fill keeps the text it came
// with, so the merge never leaves a line the box engine read empty. Every
// character of the text engine either reaches a line or appears in Dropped.
//
// It answers more lines than that where the box engine missed lines the text
// engine read. Those carry Estimated and a box the filler placed, and they stand
// between the lines they belong between. Nothing else moves.
func Merge(in Input) Result {
	result := Result{
		Lines:   make([]MergedLine, len(in.Lines)),
		Dropped: []Dropped{},
	}
	for i, line := range in.Lines {
		result.Lines[i] = MergedLine{
			Box:    line.Box,
			Text:   line.Text,
			Source: in.BoxEngine,
			Score:  line.Score,
		}
	}

	groups := groupLines(in.Lines, in.Blocks)

	// The rows of every block, and the blocks whose rows are not the rows of the
	// page, both before any block is cut. A block the merge cannot order must not
	// reach the pitch that the filler and the interpolation stand on, and the
	// pitch cannot be measured until every block has been judged.
	rows := make(map[int][][]int, len(groups))
	for owner, members := range groups {
		rows[owner] = rowsOf(in.Lines, members)
	}

	// The pitch of the page is needed to judge a block, and the judgement is
	// needed to measure the page. The cycle breaks on the pitch measured over
	// every block: a block whose rows are wrong reports steps shorter than the
	// page, and shorter steps do not move a median that rests on every block.
	pitch := measure(in.Lines, rows, nil).pitch
	unusable := make(map[int]bool, len(groups))
	for owner, members := range groups {
		if hasText(in.Blocks[owner].Text) &&
			unusableBoxes(in.Lines, members, in.Blocks[owner], pitch) {
			unusable[owner] = true
		}
	}

	ref := measure(in.Lines, rows, unusable)
	fill := filler{pitch: ref.pitch}
	plan := laid{
		around:  map[int]*insert{},
		instead: map[int][]MergedLine{},
		drop:    map[int]bool{},
	}

	for _, owner := range sortedKeys(groups) {
		block := in.Blocks[owner]
		if !hasText(block.Text) {
			continue
		}

		if unusable[owner] {
			result.Dropped = replaceBlock(&plan, in, ref, owner, groups[owner], result.Dropped)
			continue
		}

		blockRows := rows[owner]
		members := flatten(blockRows, len(groups[owner]))
		pieces, ok := cutBlock(in.Lines, members, block.Text)
		if !ok {
			result.Dropped = append(result.Dropped, Dropped{block.Text, ReasonUnaligned})
			continue
		}

		height := medianHeightOf(in.Lines, members)
		density := blockDensity(in.Lines, members)
		at := 0
		for r, row := range blockRows {
			for k, index := range row {
				piece := pieces[at]
				at++
				if !hasText(piece.text) {
					continue
				}

				// Only the last box of a row can hold the text of the rows under
				// it. The cut hands each member every rune up to the match of the
				// next, so text of a row the box engine never read lands in the
				// piece of the box that precedes it.
				var above, under []int
				if hasText(piece.lead) {
					above = fill.before(block.Box, anchorOf(in.Lines, row), height)
				}
				if k == len(row)-1 {
					under = fill.below(block.Box, in.Lines, blockRows, r, height)
				}

				// Three things say the piece holds more than its own line. A lead
				// is text standing above it that matched none of it. A piece
				// holding a whole row more, with room under the row to put it, is
				// the same thing below. A piece far too long for its line says it
				// without geometry, and is what answers for a page the filler can
				// measure no pitch on.
				//
				// None of the three is proof. The filler answers nothing unless
				// the geometry agrees and the line accepts its own share.
				room := roomIn(in.Lines[index], density)
				fits := accept(in.Lines[index].Text, room, piece)
				spills := spillsOver(in.Lines[index], block.Box, piece, len(under))
				if !fits || hasText(piece.lead) || spills {
					if filled := fillPiece(in.Lines[index], block.Box, above, under,
						height, room, in.TextEngine, piece); filled != nil {
						result.Lines[index].Text = filled.own
						result.Lines[index].Source = in.TextEngine
						plan.around[index] = filled.insert
						continue
					}
				}

				if !fits {
					result.Dropped = append(result.Dropped, Dropped{piece.text, ReasonLine})
					continue
				}

				// A box the box engine drew short of the next box of its row leaves
				// ink between the two that no box covers. The cut hands that text to
				// the box before the gap, which then holds more than it was drawn
				// around, and a renderer squeezes it in.
				if k+1 < len(row) {
					beside := in.Lines[row[k+1]].Box
					if head, tail, ok := shareAcrossGap(piece.text, in.Lines[index].Box,
						beside, room, density); ok {
						result.Lines[index].Text = head
						result.Lines[index].Source = in.TextEngine
						plan.around[index] = &insert{after: []MergedLine{{
							Box:       gapBox(in.Lines[index].Box, beside),
							Text:      tail,
							Source:    in.TextEngine,
							Estimated: true,
						}}}
						continue
					}
				}

				result.Lines[index].Text = collapseSpace(piece.text)
				result.Lines[index].Source = in.TextEngine
			}
		}
	}

	for k, block := range in.Blocks {
		if _, owned := groups[k]; !owned && hasText(block.Text) {
			result.Dropped = append(result.Dropped, Dropped{block.Text, ReasonBlock})
		}
	}

	result.Lines = spliceIn(result.Lines, plan)
	result.Counts = countOf(result, in.TextEngine, len(plan.drop))
	return result
}

// insert holds the lines the filler placed around one line of the box engine.
type insert struct{ before, after []MergedLine }

// laid is what the merge decided to do with the lines of the box engine.
//
// around holds the lines the filler placed around one of them. instead holds the
// lines that stand in place of a whole block, keyed by the first line of that
// block. drop holds the lines that leave the answer, which are the lines of every
// block the interpolation replaced.
type laid struct {
	around  map[int]*insert
	instead map[int][]MergedLine
	drop    map[int]bool
}

// replaceBlock puts the text of a block whose boxes cannot say where its printed
// rows are into rows of the merge's own, and takes the boxes of that block away.
//
// It answers the dropped text it added. A block the page cannot count the rows of
// keeps its boxes and its box engine text, and its text reaches no line: once the
// merge knows the order is wrong, cutting the block in that order would write the
// text of one printed row into the box of another on purpose.
func replaceBlock(plan *laid, in Input, ref reference, owner int, members []int,
	dropped []Dropped) []Dropped {
	block := in.Blocks[owner]

	var placed []MergedLine
	if rows, ok := rowCount(ref, block.Box, blockSlope(in.Lines, members),
		block.Text); ok {
		placed = interpolate(block, rows, in.TextEngine)
	}
	if placed == nil {
		return append(dropped, Dropped{block.Text, ReasonUnordered})
	}

	// members comes from groupLines, which walks the lines in order, so its first
	// entry is the first line of the block. The replacement stands where that line
	// stood, and every line of the block leaves the answer.
	plan.instead[members[0]] = placed
	for _, index := range members {
		plan.drop[index] = true
	}
	return dropped
}

// spliceIn lays the filled lines around the lines they belong to, puts a
// replacement where the block it replaces stood, and leaves out the lines of a
// replaced block. The lines of the box engine that survive keep their order, so
// the answer still reads as that engine read the page.
func spliceIn(lines []MergedLine, plan laid) []MergedLine {
	if len(plan.around) == 0 && len(plan.instead) == 0 {
		return lines
	}
	laid := make([]MergedLine, 0, len(lines)+2*len(plan.around))
	for i, line := range lines {
		if at, ok := plan.instead[i]; ok {
			laid = append(laid, at...)
		}
		if plan.drop[i] {
			continue
		}
		if at, ok := plan.around[i]; ok {
			laid = append(laid, at.before...)
			laid = append(laid, line)
			laid = append(laid, at.after...)
			continue
		}
		laid = append(laid, line)
	}
	return laid
}

type filledPiece struct {
	own    string
	insert *insert
}

// fillPiece answers the piece read as the several lines it holds, or nil when the
// geometry does not say how many lines that is.
//
// The lines it answers stand on the pitch of the page and carry no score, so a
// reader can tell them from a line an engine read. The line the box engine did
// read has to accept its share of the text, or the whole piece is refused as
// before: a piece that fails on its own text is a piece the two engines disagree
// about, and no amount of geometry makes it fit.
func fillPiece(line Line, block Box, above, below []int, height, room int,
	engine string, p piece) *filledPiece {
	if height <= 0 || (len(above) == 0 && len(below) == 0) {
		return nil
	}
	if hasText(p.lead) && len(above) == 0 {
		return nil
	}

	lead := wrapInto(p.lead, len(above))
	if hasText(p.lead) && lead == nil {
		return nil
	}

	own, rest, divides := shareOut(p, line.Box, block, len(below))
	if !divides {
		return nil
	}
	if !accept(line.Text, room, piece{text: own, agree: agreementWith(line.Text, own)}) {
		return nil
	}

	return &filledPiece{
		own: own,
		insert: &insert{
			before: linesAt(block, above, height, lead, engine),
			after:  linesAt(block, below, height, rest, engine),
		},
	}
}

// shareOut divides a piece between the line the box engine read and the lines the
// filler placed under it. It answers the text of that line, one string per filled
// line, and whether the piece divides at all.
//
// Each line takes the share of the text that its width is of the whole. wrapInto
// reads a column back from a line count, which holds only where every line is as
// wide as every other. The line the box engine read need not be: on a page whose
// lines do not lie level the detector reports the tail of a row on its own, and a
// tail holds a fraction of the runes a whole row holds. The filled lines under it
// do span the block, so they divide what is left between them evenly.
func shareOut(p piece, line, block Box, filled int) (string, []string, bool) {
	body := strings.TrimPrefix(p.text, p.lead)
	if filled == 0 {
		return collapseSpace(body), nil, hasText(body)
	}

	width, row := line[2]-line[0], block[2]-block[0]
	if width <= 0 || row <= 0 {
		return "", nil, false
	}

	total := width + filled*row
	mine := (utf8.RuneCountInString(body)*width + total - 1) / total
	head := wrapAt(strings.Fields(body), mine)
	if len(head) < 2 {
		return "", nil, false
	}

	rest := wrapInto(strings.Join(head[1:], " "), filled)
	if rest == nil {
		return "", nil, false
	}
	return head[0], rest, true
}

// spillsOver answers whether a piece holds a whole row more than the line it was
// cut for, with room under that line to put it.
//
// A piece far too long for its line says so without geometry, and accept refuses
// it. A piece holding a row or two more than its line can still pass accept, and
// then says nothing at all: the line takes the lot, and the rows the box engine
// missed are drawn over the row above them. This asks the geometry instead.
func spillsOver(line Line, block Box, p piece, filled int) bool {
	width, row := line.Box[2]-line.Box[0], block[2]-block[0]
	read := utf8.RuneCountInString(line.Text)
	if filled == 0 || width <= 0 || row <= 0 || read == 0 {
		return false
	}

	held := utf8.RuneCountInString(p.text) - utf8.RuneCountInString(p.lead)
	return held >= read+read*row/width
}

func linesAt(block Box, centres []int, height int, texts []string, engine string) []MergedLine {
	lines := make([]MergedLine, len(texts))
	for i, text := range texts {
		lines[i] = MergedLine{
			Box:       box(block, centres[i], height),
			Text:      text,
			Source:    engine,
			Estimated: true,
		}
	}
	return lines
}

// piece is the text of a block that belongs to one line, and how much of that
// line survived folding onto it.
//
// lead is the head of text that matched no rune of the line. Only the first
// member of a block can carry one, because every later piece starts where its
// line matched. It is text of the lines above, which the filler needs and the
// cut cannot leave out: a rune in no piece is a rune lost.
type piece struct {
	text  string
	lead  string
	agree float64
}

// cutBlock answers the piece of the block that belongs to each member, in member
// order. The pieces partition the block exactly, so reading them in order reads
// the block character for character.
//
// It refuses a block whose length cannot be one reading of the region its lines
// cover, before it allocates anything. That refusal is what keeps a text engine
// repeating itself without end from stalling a whole book on one page.
func cutBlock(lines []Line, members []int, text string) ([]piece, bool) {
	block := []rune(text)
	joined, bounds := joinLines(lines, members)

	slack := shortLine * maxLengthRatio
	if len(block) > maxLengthRatio*len(joined)+slack ||
		len(joined) > maxLengthRatio*len(block)+slack {
		return nil, false
	}

	aligned := align(joined, block)
	if aligned == nil {
		return nil, false
	}

	// Two boundaries do not follow from the alignment and have to be stated. The
	// first piece starts at the head of the block, or every rune the block holds
	// before its first line match would vanish. Each piece ends where the next
	// one starts, or every rune that aligned against a line separator would
	// vanish. Both leaks are silent, and both break the partition.
	pieces := make([]piece, len(members))
	for at := range members {
		low := 0
		if at > 0 {
			low = aligned.cut[bounds[at].start]
		}
		high := len(block)
		if at+1 < len(members) {
			high = aligned.cut[bounds[at+1].start]
		}
		if high < low {
			high = low
		}
		match := aligned.cut[bounds[at].start]
		if match < low || match > high {
			match = low
		}
		pieces[at] = piece{
			text:  string(block[low:high]),
			lead:  string(block[low:match]),
			agree: agreementOf(aligned, bounds[at]),
		}
	}
	return pieces, true
}

// agreementOf answers the share of the runes of one line that paired with a rune
// of the block equal after folding.
//
// It counts an agreeing pair, not merely a pair. On a page where the text engine
// repeated one phrase, the alignment pairs that phrase one for one against
// correct text of the box engine and pairs everything, which says nothing.
// Requiring the pair to fold onto each other tells a line the text engine
// improved from a line the two engines never read alike.
func agreementOf(aligned *alignment, at bound) float64 {
	if at.end <= at.start {
		return 0
	}
	agreed := 0
	for _, ok := range aligned.agree[at.start:at.end] {
		if ok {
			agreed++
		}
	}
	return float64(agreed) / float64(at.end-at.start)
}

type bound struct{ start, end int }

// joinLines lays the text of the members out as one stream, separated by
// newlines, and reports where each member sits in it.
func joinLines(lines []Line, members []int) ([]rune, []bound) {
	texts := make([]string, len(members))
	for at, index := range members {
		texts[at] = lines[index].Text
	}

	bounds := make([]bound, len(members))
	at := 0
	for i, text := range texts {
		length := utf8.RuneCountInString(text)
		bounds[i] = bound{start: at, end: at + length}
		at += length + 1
	}
	return []rune(strings.Join(texts, "\n")), bounds
}

// accept answers whether the two engines read one line alike enough for the text
// engine to win it. Two tests refuse a piece, and either one is enough.
//
// The share answers whether the two engines read the same line. It cannot answer
// whether the text engine stopped at the end of it: a line that begins correctly
// agrees well, and the cut then hands it every rune up to the next line that
// aligned, which on a run-on is the whole run. The length answers that.
//
// holds is what the length is measured against: the runes the box has room for,
// not the runes the box engine got out of it. The two are the same on a line that
// engine read, and nothing alike on a line it failed at.
func accept(lineText string, holds int, p piece) bool {
	if utf8.RuneCountInString(p.text) > maxPieceRatio*holds+pieceSlack {
		return false
	}

	// A short line carries too little evidence to measure a share on.
	line := utf8.RuneCountInString(lineText)
	return line < shortLine || p.agree >= minAgreement
}

// roomIn answers the runes a line box has room for: what the box engine read in
// it, or what its width says at the density of its block, whichever is more.
//
// A box engine that gets nine runes out of a box wide enough for sixty has
// failed at that line, and those nine runes are the one number the length gate
// must not trust. Trusting them refuses the reading that is right, because a
// correct line is many times longer than the noise the engine left.
func roomIn(line Line, density float64) int {
	read := utf8.RuneCountInString(line.Text)
	width := line.Box[2] - line.Box[0]
	if density <= 0 || width <= 0 {
		return read
	}
	return max(read, int(float64(width)*density))
}

// blockDensity answers the runes a pixel of one block holds, as the median over
// its lines of what the box engine read against the width it read it in.
//
// A median and not a mean, because the lines this exists to catch are exactly
// the ones that would drag a mean down.
func blockDensity(lines []Line, members []int) float64 {
	rates := make([]float64, 0, len(members))
	for _, index := range members {
		width := lines[index].Box[2] - lines[index].Box[0]
		if width <= 0 {
			continue
		}
		rates = append(rates,
			float64(utf8.RuneCountInString(lines[index].Text))/float64(width))
	}
	if len(rates) == 0 {
		return 0
	}
	sort.Float64s(rates)
	return rates[len(rates)/2]
}

func countOf(result Result, textEngine string, discarded int) Counts {
	counts := Counts{Lines: len(result.Lines), Discarded: discarded}
	for _, line := range result.Lines {
		switch {
		case line.Estimated:
			counts.Estimated++
		case line.Source == textEngine:
			counts.FromText++
		default:
			counts.FromBoxes++
		}
	}
	for _, dropped := range result.Dropped {
		counts.DroppedChars += nonSpaceCount(dropped.Text)
	}
	return counts
}

func sortedKeys(groups map[int][]int) []int {
	keys := make([]int, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

func hasText(s string) bool { return strings.TrimSpace(s) != "" }

// collapseSpace answers the piece as one line. A run of whitespace becomes one
// space and both ends lose theirs, because a line of a page carries no line break
// of its own.
func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

func nonSpaceCount(s string) int {
	count := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}
