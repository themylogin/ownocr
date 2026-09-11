package ocrmerge

import (
	"strings"
	"testing"
)

// rows lays out lines one under another, so a test states text and nothing else.
func rows(texts ...string) []Line {
	lines := make([]Line, len(texts))
	for i, text := range texts {
		top := 100 + i*50
		lines[i] = Line{Box: Box{100, top, 900, top + 40}, Text: text, Score: 0.9}
	}
	return lines
}

// covering answers a block box that holds every line of a page built by rows.
func covering(count int) Box { return Box{90, 90, 910, 100 + count*50} }

func mergeRows(text string, texts ...string) Result {
	lines := rows(texts...)
	return Merge(Input{
		Lines:      lines,
		Blocks:     []Block{{Box: covering(len(lines)), Text: text}},
		BoxEngine:  "boxes",
		TextEngine: "text",
	})
}

func TestCutPartitionsTheBlockExactly(t *testing.T) {
	// The block carries runes before its first line match and after its last, so
	// both ends of the partition are exercised.
	block := ">>> alpha beta gamma <<<"
	result := mergeRows(block, "alpha", "beta", "gamma")

	var joined strings.Builder
	for _, line := range result.Lines {
		joined.WriteString(line.Text)
	}
	if got, want := nonSpaceOf(joined.String()), nonSpaceOf(block); got != want {
		t.Errorf("the lines read %q, want every rune of %q", got, want)
	}
	if len(result.Dropped) != 0 {
		t.Errorf("dropped %v, want nothing", result.Dropped)
	}
}

func TestCutKeepsAHyphenatedWordSplitWithoutItsHyphen(t *testing.T) {
	result := mergeRows("(φιλοσοφία, philosophia) desde",
		"(φiλoσo-", "φíα, philosophia) desde")

	if got, want := result.Lines[0].Text, "(φιλοσο"; got != want {
		t.Errorf("the first line reads %q, want %q", got, want)
	}
	if got, want := result.Lines[1].Text, "φία, philosophia) desde"; got != want {
		t.Errorf("the second line reads %q, want %q", got, want)
	}
	if strings.Contains(result.Lines[0].Text, "-") {
		t.Error("the hyphen survived, and the block holds none")
	}
}

func TestCutCollapsesWhitespaceInsideAPiece(t *testing.T) {
	result := mergeRows("alpha\n\n  beta", "alpha", "beta")

	for i, line := range result.Lines {
		if strings.TrimSpace(line.Text) != line.Text {
			t.Errorf("line %d reads %q, want no whitespace at its ends", i, line.Text)
		}
		if strings.ContainsAny(line.Text, "\n\r\t") {
			t.Errorf("line %d reads %q, want one line", i, line.Text)
		}
	}
	if got, want := result.Lines[1].Text, "beta"; got != want {
		t.Errorf("the second line reads %q, want %q", got, want)
	}
}

func TestMergeKeepsTextEngineMarkup(t *testing.T) {
	result := mergeRows("misma $ ^{1} $ está vinculada una",
		"misma1 está vinculada una")

	if !strings.Contains(result.Lines[0].Text, "$ ^{1} $") {
		t.Errorf("the line reads %q, want the markup kept", result.Lines[0].Text)
	}
}

func TestMergeRefusesALineTheEnginesDidNotReadAlike(t *testing.T) {
	boxes := "lado-de-otro de justamente dos y precisamente esos dos térmi-"
	result := mergeRows(strings.Repeat("ἐπὶν αἰς ἐν ἀγαθίας, ", 4), boxes)

	if got := result.Lines[0].Text; got != boxes {
		t.Errorf("the line reads %q, want the box engine text", got)
	}
	if result.Lines[0].Source != "boxes" {
		t.Errorf("the line names %q, want boxes", result.Lines[0].Source)
	}
	if len(result.Dropped) != 1 || result.Dropped[0].Reason != ReasonLine {
		t.Fatalf("dropped %v, want one entry with reason %q", result.Dropped, ReasonLine)
	}
}

// The share cannot catch a text engine that began a line correctly and then ran
// on, because the correct opening is what it measures. The length can.
func TestMergeRefusesAPieceFarLongerThanItsLine(t *testing.T) {
	boxes := "dualidad de ser y no ser. ¿ De dónde sale esta condición, que la"
	// Long enough to pass maxPieceRatio, short enough that the block itself stays
	// within maxLengthRatio, so this tests the piece and not the block.
	runOn := "dualidad de ser y no ser. " + strings.Repeat("ἐπὶν αἰς ἐν ἀγαθίας, ", 20)

	result := mergeRows(runOn, boxes)

	if got := result.Lines[0].Text; got != boxes {
		t.Errorf("the line reads %q, want the box engine text", got)
	}
	if result.Lines[0].Source != "boxes" {
		t.Errorf("the line names %q, want boxes", result.Lines[0].Source)
	}
	if len(result.Dropped) != 1 || result.Dropped[0].Reason != ReasonLine {
		t.Fatalf("dropped %v, want one entry with reason %q", result.Dropped, ReasonLine)
	}

	// The share alone would have accepted it, which is why the length test exists.
	pieces, ok := cutBlock(rows(boxes), []int{0}, runOn)
	if !ok {
		t.Fatal("cutBlock refused the block")
	}
	if pieces[0].agree < minAgreement {
		t.Errorf("the piece agrees on %.2f, want the share to have accepted it",
			pieces[0].agree)
	}
}

// A correct piece can still be much longer than its line. The text engine writes
// a Greek word as a formula, which runs to about 2.4 times the line.
func TestMergeAcceptsAPieceLongerThanItsLineButNotTooLong(t *testing.T) {
	boxes := "(no 9εωρητiκ), en tanto que la 9εωρíα es la capacidad de"
	block := `(no  $ \epsilon \omega \rho \eta \tau \iota \kappa \eta $), en tanto ` +
		`que la  $ \epsilon \omega \rho \iota \alpha $  es la capacidad de`

	result := mergeRows(block, boxes)

	if result.Lines[0].Source != "text" {
		t.Errorf("the line names %q, want the text engine", result.Lines[0].Source)
	}
	if len(result.Dropped) != 0 {
		t.Errorf("dropped %v, want nothing", result.Dropped)
	}
}

// The allowance in characters is what covers a short line, so this gate needs no
// exemption of its own.
func TestMergeAcceptsAShortLineWithALongerPiece(t *testing.T) {
	result := mergeRows("2(30)(24)MAK", "SR")

	if got, want := result.Lines[0].Text, "2(30)(24)MAK"; got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
	if result.Lines[0].Source != "text" {
		t.Errorf("the line names %q, want the text engine", result.Lines[0].Source)
	}
}

func TestMergeAcceptsAShortLineTheEnginesReadDifferently(t *testing.T) {
	// Nine runes carry too little evidence to judge, so the text engine wins.
	result := mergeRows("δοχοῦντα.", "δoxovτα.")

	if got, want := result.Lines[0].Text, "δοχοῦντα."; got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
	if result.Lines[0].Source != "text" {
		t.Errorf("the line names %q, want text", result.Lines[0].Source)
	}
}

func TestMergeAcceptsALongGreekLineTheBoxEngineFailedAt(t *testing.T) {
	boxes := "ν τò σοφν μονον λγεσ9αι ο 9λει χα 9λει Zην"
	block := "ἐν τὸ σοφὸν μοῦνον λέγεσθαι οὐκ ἐθέλει καὶ ἐθέλει Ζηνὸς"

	result := mergeRows(block, boxes)

	if result.Lines[0].Source != "text" {
		t.Fatalf("the line names %q, want text: a Greek fix must not be refused",
			result.Lines[0].Source)
	}
	if got, want := result.Lines[0].Text, block; got != want {
		t.Errorf("the line reads %q, want %q", got, want)
	}
}

func TestMergeKeepsBoxTextForALineNoBlockCovers(t *testing.T) {
	lines := rows("inside the block", "far away in the margin")
	lines[1].Box = Box{2000, 2000, 2100, 2040}

	result := Merge(Input{
		Lines:      lines,
		Blocks:     []Block{{Box: Box{90, 90, 910, 160}, Text: "inside the block, rewritten"}},
		BoxEngine:  "boxes",
		TextEngine: "text",
	})

	if result.Lines[1].Text != "far away in the margin" {
		t.Errorf("the margin line reads %q, want its box engine text", result.Lines[1].Text)
	}
	if result.Lines[1].Source != "boxes" {
		t.Errorf("the margin line names %q, want boxes", result.Lines[1].Source)
	}
}

func TestMergeNeverEmptiesALine(t *testing.T) {
	t.Run("a block with no text", func(t *testing.T) {
		result := mergeRows("   \n  ", "read out of a figure", "and another")
		for i, line := range result.Lines {
			if line.Text == "" {
				t.Errorf("line %d came out empty", i)
			}
			if line.Source != "boxes" {
				t.Errorf("line %d names %q, want boxes", i, line.Source)
			}
		}
	})

	t.Run("a line the block never reached", func(t *testing.T) {
		result := mergeRows("alpha", "alpha", "beta gamma delta epsilon zeta eta")
		for i, line := range result.Lines {
			if line.Text == "" {
				t.Errorf("line %d came out empty", i)
			}
		}
	})
}

func TestMergeRefusesABlockFarLongerThanItsLines(t *testing.T) {
	block := strings.Repeat("x", 40000)
	boxes := strings.Repeat("a", 200)

	result := mergeRows(block, boxes)

	if result.Lines[0].Text != boxes {
		t.Error("the line took text from a block it could not be a reading of")
	}
	if len(result.Dropped) != 1 || result.Dropped[0].Reason != ReasonUnaligned {
		t.Fatalf("dropped %v, want one entry with reason %q", result.Dropped, ReasonUnaligned)
	}
	if result.Counts.DroppedChars != 40000 {
		t.Errorf("dropped %d runes, want 40000", result.Counts.DroppedChars)
	}
}

func TestMergeRefusesABlockFarShorterThanItsLines(t *testing.T) {
	texts := make([]string, 40)
	for i := range texts {
		texts[i] = strings.Repeat("a", 60)
	}

	result := mergeRows("tiny", texts...)

	if len(result.Dropped) != 1 || result.Dropped[0].Reason != ReasonUnaligned {
		t.Fatalf("dropped %v, want one entry with reason %q", result.Dropped, ReasonUnaligned)
	}
	for i, line := range result.Lines {
		if line.Source != "boxes" {
			t.Errorf("line %d names %q, want boxes", i, line.Source)
		}
	}
}

func TestMergeReportsABlockNoLineCovers(t *testing.T) {
	result := Merge(Input{
		Lines:      rows("a line the block covers"),
		Blocks:     []Block{{Box: Box{2000, 2000, 2100, 2040}, Text: "2023"}},
		BoxEngine:  "boxes",
		TextEngine: "text",
	})

	if len(result.Dropped) != 1 {
		t.Fatalf("dropped %v, want one entry", result.Dropped)
	}
	if result.Dropped[0] != (Dropped{"2023", ReasonBlock}) {
		t.Errorf("dropped %v, want 2023 with reason %q", result.Dropped[0], ReasonBlock)
	}
	if result.Lines[0].Source != "boxes" {
		t.Errorf("the line names %q, want boxes", result.Lines[0].Source)
	}
}

func TestMergeReportsEveryReason(t *testing.T) {
	seen := map[string]bool{}
	for _, result := range []Result{
		mergeRows(strings.Repeat("ἐπὶν αἰς ἐν ἀγαθίας, ", 4),
			"lado-de-otro de justamente dos y precisamente esos dos térmi-"),
		mergeRows(strings.Repeat("x", 40000), strings.Repeat("a", 200)),
		Merge(Input{
			Lines:      rows("a line"),
			Blocks:     []Block{{Box: Box{2000, 2000, 2100, 2040}, Text: "2023"}},
			BoxEngine:  "boxes",
			TextEngine: "text",
		}),
	} {
		for _, dropped := range result.Dropped {
			seen[dropped.Reason] = true
		}
	}

	for _, reason := range []string{ReasonLine, ReasonUnaligned, ReasonBlock} {
		if !seen[reason] {
			t.Errorf("no case produced the reason %q", reason)
		}
	}
}

// rowsWithAGap lays out lines one under another and leaves room for `missing`
// more between the last two. It is the shape of a page the detector read except
// for a run of lines in the middle of it.
func rowsWithAGap(missing int, texts ...string) []Line {
	lines := rows(texts...)
	shift := missing * 50
	lines[len(lines)-1].Box[1] += shift
	lines[len(lines)-1].Box[3] += shift
	return lines
}

func mergeLines(lines []Line, text string) Result {
	bottom := 0
	for _, line := range lines {
		bottom = max(bottom, line.Box[3])
	}
	return Merge(Input{
		Lines:      lines,
		Blocks:     []Block{{Box: Box{90, 90, 910, bottom + 10}, Text: text}},
		BoxEngine:  "boxes",
		TextEngine: "text",
	})
}

// A piece that holds two lines more than its own can still be short enough for
// accept, which measures a piece against its line and knows nothing of the page.
// The line then swallows both, and they are drawn over the line above them.
//
// This is what the room under the row answers. The gap here is three pitches, so
// two lines stand in it, and the text of those two lines belongs in them.
func TestALineIsNotLetSwallowTheRowsUnderIt(t *testing.T) {
	read := []string{
		"alpha alpha alpha alpha alpha alpha alpha alpha",
		"bravo bravo bravo bravo bravo bravo bravo bravo",
		"charlie charlie charlie charlie charlie charlie",
		"delta delta delta delta delta delta delta delta",
		"golf golf golf golf golf golf golf golf golf go",
	}
	missing := "echo echo echo echo echo echo echo echo echo " +
		"foxtrot foxtrot foxtrot foxtrot foxtrot foxtrot "
	lines := rowsWithAGap(2, read...)
	block := strings.Join(read[:4], " ") + " " + missing + read[4]

	result := mergeLines(lines, block)

	// accept alone would have taken the whole piece, which is why this is needed.
	pieces, ok := cutBlock(lines, readingOrder(lines, indexesOf(lines)), block)
	if !ok {
		t.Fatal("cutBlock refused the block")
	}
	if !accept(lines[3].Text, roomIn(lines[3], 0), pieces[3]) {
		t.Fatal("accept refused the piece, so this no longer tests the geometry")
	}

	if result.Counts.Estimated != 2 {
		t.Fatalf("filled %d lines in a gap of two, want 2", result.Counts.Estimated)
	}
	if len(result.Lines) != len(read)+2 {
		t.Fatalf("answered %d lines, want %d", len(result.Lines), len(read)+2)
	}
	if result.Counts.DroppedChars != 0 {
		t.Errorf("dropped %d runes, want none", result.Counts.DroppedChars)
	}

	filled := ""
	for _, line := range result.Lines {
		if line.Estimated {
			filled += " " + line.Text
		}
	}
	for _, word := range []string{"echo", "foxtrot"} {
		if !strings.Contains(filled, word) {
			t.Errorf("no filled line reads %q, so its line was swallowed: %q", word, filled)
		}
	}
}

// The other side of the same rule. Room under a row is not a reason to fill it:
// a paragraph that ends short of the block leaves room, and the text engine read
// no line to put there. Splitting the line above would tear a correct line in two.
func TestALineWithRoomUnderItAndNothingToSpareIsLeftWhole(t *testing.T) {
	read := []string{
		"alpha alpha alpha alpha alpha alpha alpha alpha",
		"bravo bravo bravo bravo bravo bravo bravo bravo",
		"charlie charlie charlie charlie charlie charlie",
		"delta delta delta delta delta delta delta delta",
		"golf golf golf golf golf golf golf golf golf go",
	}
	lines := rowsWithAGap(2, read...)

	result := mergeLines(lines, strings.Join(read, " "))

	if result.Counts.Estimated != 0 {
		t.Errorf("filled %d lines with no text to put in them, want none",
			result.Counts.Estimated)
	}
	for i, want := range read {
		if got := result.Lines[i].Text; got != want {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, want)
		}
	}
	if len(result.Dropped) != 0 {
		t.Errorf("dropped %v, want nothing", result.Dropped)
	}
}

func TestMergeCountsWhatItDid(t *testing.T) {
	result := mergeRows("alpha beta", "alpha", "beta")

	if result.Counts.Lines != 2 || result.Counts.FromText != 2 || result.Counts.FromBoxes != 0 {
		t.Errorf("counts are %+v, want 2 lines both from the text engine", result.Counts)
	}
	if result.Counts.DroppedChars != 0 {
		t.Errorf("dropped %d runes, want none", result.Counts.DroppedChars)
	}
}

func TestMergeWithNoBlocksKeepsEveryLine(t *testing.T) {
	lines := rows("one", "two", "three")
	result := Merge(Input{Lines: lines, BoxEngine: "boxes", TextEngine: "text"})

	if result.Counts.FromBoxes != 3 || result.Counts.FromText != 0 {
		t.Errorf("counts are %+v, want all three from the boxes", result.Counts)
	}
	for i, line := range result.Lines {
		if line.Text != lines[i].Text {
			t.Errorf("line %d reads %q, want %q", i, line.Text, lines[i].Text)
		}
	}
}

func nonSpaceOf(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// Book 17 page 116 sets three footnotes. PP-OCRv6 read two of their rows as
// noise: `4mbé,a,dlSd,` out of a 1528 pixel box and `auEll Mdr` out of a 1519
// pixel box. PaddleOCR-VL read both rows correctly.
//
// The length gate measures a piece against the runes the box engine got out of
// the line, which is the one number that cannot be trusted when the box engine
// failed. It therefore refuses the reading that is right and keeps the noise.
func TestALineTheBoxEngineFailedAtStillTakesTheTextEngineReading(t *testing.T) {
	result := Merge(fixtureOf(t, "book0017page0116").input())

	want := map[Box]string{
		{284, 2699, 1812, 2836}: "49 Ctr. también, a este respecto, mi El concepto de lo civil (Santiago de Chile,",
		{284, 2812, 1803, 2964}: "5º Cfr. para todo ello mi Pasión tranquila (Ensayo sobre la filosofía de Hume) (Madrid,",
	}

	found := 0
	for _, line := range result.Lines {
		text, held := want[line.Box]
		if !held {
			continue
		}
		found++
		if line.Text != text {
			t.Errorf("the box %v reads\n got %q\nwant %q", line.Box, line.Text, text)
		}
	}
	if found != len(want) {
		t.Errorf("matched %d of %d footnote rows", found, len(want))
	}
}

// Book 17 page 116 drops nothing once the gate measures the room a box has. The
// three footnotes of that page were 77 of the runes the merge lost over both
// books of the library.
func TestBookSeventeenPageOneOneSixDropsNothing(t *testing.T) {
	result := Merge(fixtureOf(t, "book0017page0116").input())

	if result.Counts.DroppedChars != 0 {
		t.Errorf("dropped %d runes, want none: %v",
			result.Counts.DroppedChars, result.Dropped)
	}
}

func TestRoomInReadsTheWidthWhereTheBoxEngineFailed(t *testing.T) {
	// The footnotes of book 17 page 116: two rows the engine read and two it
	// turned to noise, all four in boxes whose width says what they hold.
	lines := []Line{
		{Box: Box{284, 2699, 1812, 2836}, Text: "4mbé,a,dlSd,"},
		{Box: Box{375, 2768, 536, 2838}, Text: "2008)."},
		{Box: Box{284, 2812, 1803, 2964}, Text: "auEll Mdr"},
		{Box: Box{373, 2885, 538, 2953}, Text: "2009)."},
	}

	density := blockDensity(lines, []int{0, 1, 2, 3})

	if got := roomIn(lines[0], density); got < 50 {
		t.Errorf("a 1528 pixel box has room for %d runes, want a whole row's worth", got)
	}
	if got := roomIn(lines[2], density); got < 50 {
		t.Errorf("a 1519 pixel box has room for %d runes, want a whole row's worth", got)
	}
	if got := roomIn(lines[1], density); got != 6 {
		t.Errorf("a box the engine read holds %d runes, want the 6 it read", got)
	}
}

// A density read as a mean would be dragged down by the very lines this exists
// to catch, so it is read as a median.
func TestBlockDensityIsNotMovedByTheLinesItMustCatch(t *testing.T) {
	read := []Line{
		{Box: Box{100, 100, 1100, 160}, Text: "a row of fifty runes of ordinary body text here ok"},
		{Box: Box{100, 200, 1100, 260}, Text: "another row of fifty runes of ordinary body text ok"},
		{Box: Box{100, 300, 1100, 360}, Text: "a third row of fifty runes of ordinary body text ok"},
	}
	whole := blockDensity(read, []int{0, 1, 2})

	failed := append(append([]Line(nil), read...),
		Line{Box: Box{100, 400, 1100, 460}, Text: "xQ z"})
	withNoise := blockDensity(failed, []int{0, 1, 2, 3})

	if withNoise < whole*0.9 {
		t.Errorf("one failed line moved the density from %.5f to %.5f", whole, withNoise)
	}
	if got := roomIn(failed[3], withNoise); got < 40 {
		t.Errorf("the failed line's box has room for %d runes, want a row's worth", got)
	}
}
