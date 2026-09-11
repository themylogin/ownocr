package ocrmerge

import (
	"strings"
	"testing"
)

// TestEveryTextEngineRuneIsAccountedFor is the invariant the merge rests on. For
// every page, the runes of every block equal the runes of the lines that took
// text engine text plus the runes reported as dropped. A leak here means the
// merge lost text without saying so.
func TestEveryTextEngineRuneIsAccountedFor(t *testing.T) {
	for _, f := range loadFixtures(t) {
		in := f.input()
		result := Merge(in)

		blocks := 0
		for _, block := range in.Blocks {
			blocks += nonSpaceCount(block.Text)
		}

		rendered := 0
		for _, line := range result.Lines {
			if line.Source == in.TextEngine {
				rendered += nonSpaceCount(line.Text)
			}
		}

		dropped := 0
		for _, entry := range result.Dropped {
			dropped += nonSpaceCount(entry.Text)
		}

		if blocks != rendered+dropped {
			t.Errorf("page %d: %d block runes against %d rendered and %d dropped, leaking %d",
				f.Page, blocks, rendered, dropped, blocks-rendered-dropped)
		}
		if dropped != result.Counts.DroppedChars {
			t.Errorf("page %d: counts report %d dropped runes, found %d",
				f.Page, result.Counts.DroppedChars, dropped)
		}
	}
}

// TestTheAnswerFollowsTheBoxEngine holds the merge to the lines it was given.
// Taking the lines the merge placed itself out of the answer has to leave the box
// engine lines, in order, with the box and the score they came with.
//
// The merge may leave a line out, and only for a block whose boxes say nothing
// about where its printed rows are. So the answer is a subsequence of the input
// rather than the whole of it, and the lines it skips have to be exactly as many
// as the counts report discarded.
func TestTheAnswerFollowsTheBoxEngine(t *testing.T) {
	for _, f := range loadFixtures(t) {
		in := f.input()
		result := Merge(in)

		read := readLines(result)
		if len(read)+result.Counts.Discarded != len(in.Lines) {
			t.Fatalf("page %d: %d lines answered and %d discarded, for %d box engine lines",
				f.Page, len(read), result.Counts.Discarded, len(in.Lines))
		}

		at := 0
		for i, given := range in.Lines {
			if at < len(read) && read[at].Box == given.Box {
				line := read[at]
				at++
				if line.Score != given.Score {
					t.Errorf("page %d line %d: score %v, want %v",
						f.Page, i, line.Score, given.Score)
				}
				if line.Source != in.BoxEngine && line.Source != in.TextEngine {
					t.Errorf("page %d line %d: source %q names neither engine",
						f.Page, i, line.Source)
				}
				if strings.TrimSpace(given.Text) != "" && line.Text == "" {
					t.Errorf("page %d line %d: came out empty for %q",
						f.Page, i, given.Text)
				}
			}
		}
		if at != len(read) {
			t.Errorf("page %d: %d answered lines follow no box engine line, so the order broke",
				f.Page, len(read)-at)
		}

		if result.Counts.Lines != len(result.Lines) {
			t.Errorf("page %d: counts report %d lines, want %d",
				f.Page, result.Counts.Lines, len(result.Lines))
		}
		kept := len(in.Lines) - result.Counts.Discarded
		if result.Counts.FromText+result.Counts.FromBoxes != kept {
			t.Errorf("page %d: counts %+v do not add up to the %d box engine lines kept",
				f.Page, result.Counts, kept)
		}
		if result.Counts.Estimated != len(result.Lines)-kept {
			t.Errorf("page %d: counts report %d placed lines, want %d",
				f.Page, result.Counts.Estimated, len(result.Lines)-kept)
		}
	}
}

// TestNoFilledLineRepeatsALineAnEngineRead guards the failure a wrong reading
// order produces. Where the order puts the first line of a block last, the cut
// hands the piece of the second line a lead holding the text of the first, the
// filler places that lead above it, and the page carries the same sentence twice:
// once in the box the engine read it in, once in a box beside it.
func TestNoFilledLineRepeatsALineAnEngineRead(t *testing.T) {
	for _, f := range loadFixtures(t) {
		result := Merge(f.input())

		read := map[string]bool{}
		for _, line := range result.Lines {
			if !line.Estimated && len([]rune(line.Text)) > 8 {
				read[collapseSpace(line.Text)] = true
			}
		}
		for i, line := range result.Lines {
			if !line.Estimated {
				continue
			}
			if read[collapseSpace(line.Text)] {
				t.Errorf("page %d line %d repeats a line an engine read: %q",
					f.Page, i, line.Text)
			}
		}
	}
}

// TestEveryFilledLineStandsInABlockOnThePitch holds the filler to the page. A
// filled line carries no reading of its own, so the only thing that says it
// belongs on the page is where it stands: inside the block whose text it holds,
// and under the filled line before it.
func TestEveryFilledLineStandsInABlockOnThePitch(t *testing.T) {
	for _, f := range loadFixtures(t) {
		in := f.input()
		result := Merge(in)

		previous := -1
		for i, line := range result.Lines {
			if !line.Estimated {
				previous = -1
				continue
			}

			height := line.Box[3] - line.Box[1]
			slack := int(float64(height) * edgeSlack)
			held := false
			for _, block := range in.Blocks {
				if line.Box[1] >= block.Box[1]-slack && line.Box[3] <= block.Box[3]+slack &&
					line.Box[0] >= block.Box[0]-slack && line.Box[2] <= block.Box[2]+slack {
					held = true
					break
				}
			}
			if !held {
				t.Errorf("page %d line %d stands at %v, which is inside no block",
					f.Page, i, line.Box)
			}

			if previous >= 0 && centreOf(line.Box) <= previous {
				t.Errorf("page %d line %d sits at %d, not under the filled line at %d",
					f.Page, i, centreOf(line.Box), previous)
			}
			previous = centreOf(line.Box)
		}
	}
}

// readLines answers the lines an engine read, which is the answer without the
// lines the filler placed.
func readLines(result Result) []MergedLine {
	read := make([]MergedLine, 0, len(result.Lines))
	for _, line := range result.Lines {
		if !line.Estimated {
			read = append(read, line)
		}
	}
	return read
}

// TestTheTextEngineRepeatingItselfCostsOnlyItsOwnLines guards page 116, where the
// text engine degenerated after a correct prefix. The prefix must survive.
func TestTheTextEngineRepeatingItselfCostsOnlyItsOwnLines(t *testing.T) {
	result := Merge(fixtureOf(t, "page0116").input())

	if result.Counts.FromBoxes == 0 {
		t.Error("no line fell back to the box engine, so the repetition was accepted")
	}
	if result.Counts.FromText == 0 {
		t.Error("no line took text engine text, so the correct prefix was lost")
	}
	if result.Counts.DroppedChars < 1000 {
		t.Errorf("dropped %d runes, want the repetition reported",
			result.Counts.DroppedChars)
	}
}

// TestPageOneOneSixRendersNoRunOnPiece guards the one line of the book whose share
// accepts it and whose length must not. The text engine wrote its opening
// correctly and then repeated itself to the end of the block.
func TestPageOneOneSixRendersNoRunOnPiece(t *testing.T) {
	in := fixtureOf(t, "page0116").input()
	result := Merge(in)

	const runOn = "dualidad de ser y no ser. ¿ De dónde sale esta condición, que la"

	found := false
	for _, line := range result.Lines {
		if line.Text != runOn {
			continue
		}
		found = true
		if line.Source != in.BoxEngine {
			t.Errorf("the run-on line names %q, want the box engine", line.Source)
		}
	}
	if !found {
		t.Fatalf("no line reads %q, so the run-on piece was rendered", runOn)
	}

	// The refused piece has to appear in Dropped, or the invariant would leak.
	for _, entry := range result.Dropped {
		if strings.HasPrefix(entry.Text, "dualidad de ser y no ser.") {
			if entry.Reason != ReasonLine {
				t.Errorf("the run-on piece is dropped as %q, want %q",
					entry.Reason, ReasonLine)
			}
			if got := len([]rune(entry.Text)); got < 600 {
				t.Errorf("the dropped piece holds %d runes, want the whole run", got)
			}
			return
		}
	}
	t.Error("the run-on piece was not reported as dropped")
}

// No line of any fixture page keeps a piece the length gate should have refused.
// The gate measures against the room a box has, so this measures the same thing.
func TestNoRenderedLineIsFarLongerThanTheRoomItsBoxHas(t *testing.T) {
	for _, f := range loadFixtures(t) {
		in := f.input()
		result := Merge(in)

		// Keyed by box and not by index, because the merge may leave a line out
		// and the answer is then no longer index for index with the input.
		room := make(map[Box]int, len(in.Lines))
		for _, line := range in.Lines {
			room[line.Box] = len([]rune(line.Text))
		}
		for _, members := range groupLines(in.Lines, in.Blocks) {
			density := blockDensity(in.Lines, members)
			for _, index := range members {
				room[in.Lines[index].Box] = roomIn(in.Lines[index], density)
			}
		}

		for i, line := range readLines(result) {
			if line.Source != in.TextEngine {
				continue
			}
			held := room[line.Box]
			if got := len([]rune(line.Text)); got > maxPieceRatio*held+pieceSlack {
				t.Errorf("%s line %d: rendered %d runes for a box with room for %d",
					f.Name, i, got, held)
			}
		}
	}
}

// TestTheTextEngineAnsweringTooLittleLeavesTheRestAlone guards page 119, where the
// text engine returned a fraction of the region.
func TestTheTextEngineAnsweringTooLittleLeavesTheRestAlone(t *testing.T) {
	result := Merge(fixtureOf(t, "page0119").input())

	if result.Counts.FromBoxes < 15 {
		t.Errorf("%d lines fell back to the box engine, want most of the page",
			result.Counts.FromBoxes)
	}
	if result.Counts.FromText == 0 {
		t.Error("no line took text engine text, want the lines it did reach")
	}
}
