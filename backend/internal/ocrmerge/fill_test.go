package ocrmerge

import (
	"strings"
	"testing"
)

// pageFiftyTwoMerged is what the merge answers for page 52 of book 3313, line for
// line. Five of these lines PP-OCRv6 never detected, and the merge dropped all
// five before the filler existed.
//
// Line 4 is what the filling costs. The printed page breaks `versatilidad` as
// `versati-` and `lidad`, and the text engine wrote the word whole, so the filler
// cannot break it and the whole word falls to the line below.
var pageFiftyTwoMerged = []struct {
	text   string
	filled bool
}{
	{"52", false},
	{"LA COSA Y EL RELATO", false},
	{"aspecto, es entre nosotros (i. e.: en Atenas) donde el mismo", true},
	{"hombre puede en mayor diversidad de formas, y en todas", true},
	{"ellas con gracia (kháris)², con la mayor flexibilidad o", false},
	{"versatilidad mostrar su cuerpo de manera autónoma» (II,41,1).", true},
	{"Nuevamente estamos ante la alusión a un espacio que deja", true},
	{"aparecer cada cosa que aparece, y precisamente cada cosa en", true},
	{"su irreductibilidad, esto es, en su ser; y también aquí afora", false},
	{"el que precisamente ese reconocimiento genera un espacio", false},
	{"uniforme, por lo tanto un ilimitado y la tendencia reduc", false},
	{"tora; y también aquí esto, como el substraerse que es inheren", false},
	{"tente a la comparecencia o tener lugar de lo siempre ya", false},
	{"supuesto, se expresa, como ya desde el comienzo mismo del", false},
	{"presente discurso de Pericles (cfr. nuestro capítulo 4),", false},
	{"mediante la contraposición de érgon a lógos (II,41,2), incluso", false},
	{"empleando de nuevo cierto recurso ya antes asociado a esa", false},
	{"contraposición y que ya conocemos: en efecto, el que la", false},
	{"cuestión escapa a la determinación y tiene que ver con espa", false},
	{"cio uniforme y con tendencia a la ilimitación se expresa", false},
	{"remitiendo de lógos a otro término, diciendo, pues, que lo", false},
	{"peculiar o atrevido de ello no es hinchazón o énfasis (kómpos,", false},
	{"ibid.) dentro del propio ámbito lógoi, sino que es lo otro, lo", false},
	{"cual, además de ser caracterizado como el ámbito érgos, es", false},
	{"designado como alétheia (ibid.), tal como ya ocurria -y así lo", false},
	{"vimos- en el comienzo del mismo discurso de Pericles y pre", false},
	{"cisamente a propósito de la cuestión de si procedía decir un", false},
	{"decir en vez de remitir todo a érgos. Nuevo, en comparación", false},
	{"con la presencia anterior de esto mismo en el mismo dis", false},
	{"curso de Pericles, es el que ahora con ello se relaciona expre", false},
	{"samente lo que en nuestra precedente exposición ha apare-", false},
	{"22 A propósito de las conexiones de esta palabra, cfr. mi El decir griego, en parti", true},
	{"cular capítulo 4 con referencia a los capítulos 1 y 2.", false},
}

func TestPageFiftyTwoFillsTheLinesTheBoxEngineMissed(t *testing.T) {
	result := Merge(fixtureOf(t, "page0052").input())

	if len(result.Lines) != len(pageFiftyTwoMerged) {
		t.Fatalf("page 52 answered %d lines, want %d",
			len(result.Lines), len(pageFiftyTwoMerged))
	}
	for i, want := range pageFiftyTwoMerged {
		if got := result.Lines[i].Text; got != want.text {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, want.text)
		}
		if got := result.Lines[i].Estimated; got != want.filled {
			t.Errorf("line %d reports filled=%v, want %v", i, got, want.filled)
		}
	}
	if result.Counts.DroppedChars != 0 {
		t.Errorf("page 52 dropped %d runes, want none: every one has a line now",
			result.Counts.DroppedChars)
	}
}

// The filled lines stand on the pitch of the page, between the lines they belong
// between, and they never run backwards.
func TestFilledLinesStandInOrder(t *testing.T) {
	result := Merge(fixtureOf(t, "page0052").input())

	block := result.Lines[2:31]
	for i := 1; i < len(block); i++ {
		if centreOf(block[i].Box) <= centreOf(block[i-1].Box) {
			t.Errorf("line %d sits at %d, not below line %d at %d",
				i, centreOf(block[i].Box), i-1, centreOf(block[i-1].Box))
		}
	}
}

func TestFilledLinesCarryNoScore(t *testing.T) {
	result := Merge(fixtureOf(t, "page0052").input())

	filled := 0
	for _, line := range result.Lines {
		if !line.Estimated {
			continue
		}
		filled++
		if line.Score != 0 {
			t.Errorf("a filled line scores %v, want zero: no engine scored it", line.Score)
		}
		if line.Source == "v6" {
			t.Error("a filled line names the box engine, which never read it")
		}
	}
	if filled != 6 {
		t.Errorf("filled %d lines, want 6", filled)
	}
}

// A page the box engine read whole must come back untouched. The filler answering
// anything here would be inventing lines.
func TestTheFillerAddsNothingToAPageWithNoGaps(t *testing.T) {
	for _, page := range []string{"page0010", "page0011", "page0014"} {
		in := fixtureOf(t, page).input()
		result := Merge(in)

		if result.Counts.Estimated != 0 {
			t.Errorf("%s gained %d filled lines, want none",
				page, result.Counts.Estimated)
		}
		if len(result.Lines) != len(in.Lines) {
			t.Errorf("%s answered %d lines for %d, want them equal",
				page, len(result.Lines), len(in.Lines))
		}
	}
}

func TestWrapIntoReadsTheColumnBackFromTheLineCount(t *testing.T) {
	text := "aspecto, es entre nosotros (i. e.: en Atenas) donde el mismo " +
		"hombre puede en mayor diversidad de formas, y en todas"

	got := wrapInto(text, 2)

	want := []string{
		"aspecto, es entre nosotros (i. e.: en Atenas) donde el mismo",
		"hombre puede en mayor diversidad de formas, y en todas",
	}
	if len(got) != len(want) {
		t.Fatalf("wrapped into %d lines, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

func TestWrapIntoKeepsEveryWord(t *testing.T) {
	text := "alpha beta gamma delta epsilon zeta eta theta iota kappa"

	for n := 1; n <= 5; n++ {
		got := wrapInto(text, n)
		if len(got) != n {
			t.Fatalf("wrapped into %d lines, want %d", len(got), n)
		}
		if joined := strings.Join(got, " "); joined != text {
			t.Errorf("into %d lines reads %q, want %q", n, joined, text)
		}
	}
}

func TestWrapIntoRefusesWhatItCannotSplit(t *testing.T) {
	if got := wrapInto("two words", 3); got != nil {
		t.Errorf("wrapped two words into %q, want nothing: there is no third line", got)
	}
	if got := wrapInto("alpha beta", 0); got != nil {
		t.Errorf("wrapped into no lines and answered %q, want nothing", got)
	}
}

func rowsAt(pitch int, count int) []Line {
	lines := make([]Line, count)
	for i := range lines {
		top := 100 + i*pitch
		lines[i] = Line{Box: Box{100, top, 900, top + 40}, Text: "a line of text"}
	}
	return lines
}

func fillerOf(lines []Line) filler {
	return newFiller(lines, map[int][]int{0: indexesOf(lines)})
}

func indexesOf(lines []Line) []int {
	out := make([]int, len(lines))
	for i := range lines {
		out[i] = i
	}
	return out
}

func TestFillerReadsAGapAsWholeLines(t *testing.T) {
	lines := rowsAt(50, 6)
	f := fillerOf(lines)

	if f.pitch != 50 {
		t.Fatalf("the pitch measures %d, want 50", f.pitch)
	}

	// Two boxes four pitches apart hold three lines between them.
	above := Box{100, 100, 900, 140}
	below := Box{100, 300, 900, 340}
	if got := f.between(above, below); len(got) != 3 {
		t.Errorf("read %d lines in the gap, want 3: %v", len(got), got)
	}
}

// A paragraph break is wider than a line and holds no line. Reading it as one
// would invent a line and tear a correct line in half to fill it.
func TestFillerRefusesAGapThatIsNoWholeNumberOfLines(t *testing.T) {
	f := fillerOf(rowsAt(50, 6))

	above := Box{100, 100, 900, 140}
	below := Box{100, 168, 900, 208}
	if got := f.between(above, below); got != nil {
		t.Errorf("read %v in a gap of 1.4 pitches, want nothing", got)
	}
}

// A pitch rests on the gaps between lines, and n lines hold n-1 gaps.
func TestFillerNeedsEnoughLinesToMeasureAPitch(t *testing.T) {
	lines := rowsAt(50, minPitchSamples+1)
	if f := fillerOf(lines); f.pitch == 0 {
		t.Errorf("%d lines measured no pitch, want one", len(lines))
	}

	lines = rowsAt(50, minPitchSamples)
	f := fillerOf(lines)
	if f.pitch != 0 {
		t.Errorf("%d lines measured a pitch of %d, want none", len(lines), f.pitch)
	}
	if got := f.between(Box{100, 100, 900, 140}, Box{100, 300, 900, 340}); got != nil {
		t.Errorf("a filler with no pitch answered %v, want nothing", got)
	}
}

// A detector reading a curved page breaks a line into a wide box and a small one
// beside it. Both stand at one height, so the step between them is no pitch. A
// filler that counted it would measure a pitch near zero and read every ordinary
// gap as dozens of missing lines.
func TestFillerIgnoresTwoBoxesOnOneRow(t *testing.T) {
	var lines []Line
	for i := 0; i < 8; i++ {
		top := 100 + i*50
		lines = append(lines,
			Line{Box: Box{100, top, 700, top + 40}, Text: "the left part of the line"},
			Line{Box: Box{690, top - 5, 900, top + 45}, Text: "the tail"})
	}

	f := fillerOf(lines)

	if f.pitch != 50 {
		t.Errorf("the pitch measures %d, want 50: the split boxes are no pitch", f.pitch)
	}
	// Two rows one pitch apart hold nothing between them.
	if got := f.between(Box{100, 100, 700, 140}, Box{100, 150, 700, 190}); got != nil {
		t.Errorf("read %d lines in a gap of one pitch, want none", len(got))
	}
}

// The pitch of page 432 has to be the pitch of its rows, whether it is measured
// on the blocks the detector read row by row or on the whole page, block 1
// included. Two boxes of one row there stand 0 to 35 pixels apart, so a pitch
// that counted them would come out far under a row and read every ordinary gap as
// several missing lines.
func TestThePitchOfACurvedPageIsTheRowPitch(t *testing.T) {
	in := fixtureOf(t, "page0432").input()
	groups := groupLines(in.Lines, in.Blocks)

	unbroken := newFiller(in.Lines, map[int][]int{1: groups[1], 2: groups[2]}).pitch
	whole := newFiller(in.Lines, groups).pitch

	if unbroken < 44 || unbroken > 47 {
		t.Errorf("the unbroken blocks measure a pitch of %d, want the row pitch of 46",
			unbroken)
	}
	if whole < unbroken-1 || whole > unbroken+1 {
		t.Errorf("the page measures a pitch of %d against %d for its unbroken blocks: "+
			"the block the detector broke moved it", whole, unbroken)
	}
}

// A row and the row two under it are two pitches apart, measured anchor to
// anchor. Measured from the tail of the first row they are 1.26 pitches apart,
// which is no whole number of lines and fills nothing.
func TestAGapIsMeasuredFromTheAnchorOfARowAndNotItsTail(t *testing.T) {
	in := fixtureOf(t, "page0432").input()
	groups := groupLines(in.Lines, in.Blocks)
	fill := newFiller(in.Lines, groups)
	rows := rowsOf(in.Lines, groups[0])

	under := fill.below(in.Blocks[0].Box, in.Lines, rows, 1, medianHeight(in.Lines, groups[0]))
	if len(under) != 1 {
		t.Fatalf("the anchors read %d lines under the second row, want 1", len(under))
	}

	tail := in.Lines[rows[1][len(rows[1])-1]].Box
	if got := fill.between(tail, anchorOf(in.Lines, rows[2])); len(got) != 0 {
		t.Errorf("the tail read %d lines, want none: it is why the anchor exists", len(got))
	}
}

// The eight rows v6 never read stand between two rows it did. They are one gap of
// 8.13 pitches, and it has to read as seven lines between them plus the one the
// row above spills.
func TestTheLongGapOfACurvedPageReadsAsSevenLines(t *testing.T) {
	in := fixtureOf(t, "page0432").input()
	groups := groupLines(in.Lines, in.Blocks)
	fill := newFiller(in.Lines, groups)
	rows := rowsOf(in.Lines, groups[0])

	under := fill.below(in.Blocks[0].Box, in.Lines, rows, 4, medianHeight(in.Lines, groups[0]))

	if len(under) != 7 {
		t.Fatalf("read %d lines in the gap, want 7", len(under))
	}
	for at := 1; at < len(under); at++ {
		if step := under[at] - under[at-1]; step < fill.pitch-2 || step > fill.pitch+2 {
			t.Errorf("line %d stands %d under line %d, want about %d",
				at, step, at-1, fill.pitch)
		}
	}
}

// shareOut divides by width. A box holding the tail of a row takes the share of
// the text its width is of the row, not an even share of it.
func TestShareOutGivesATailBoxOnlyItsOwnWords(t *testing.T) {
	p := piece{text: "tenga una entidad muy considerable y unas exigencias técnicas propias. Así, por "}

	own, rest, divides := shareOut(p, Box{1411, 298, 1608, 381}, Box{182, 235, 1614, 942}, 1)

	if !divides {
		t.Fatal("the piece would not divide")
	}
	if own != "tenga una" {
		t.Errorf("the tail box reads %q, want %q", own, "tenga una")
	}
	if len(rest) != 1 || rest[0] != "entidad muy considerable y unas exigencias técnicas propias. Así, por" {
		t.Errorf("the filled line reads %q, want the whole row", rest)
	}
}

// A line as wide as the block takes about an even share, which is what every page
// the detector read row by row needs. It takes a little under half rather than
// half: a line box is drawn around the ink and a filled box spans the block, so
// the filled line really is the wider of the two.
func TestShareOutGivesAFullWidthLineAboutAnEvenShare(t *testing.T) {
	text := "aspecto, es entre nosotros (i. e.: en Atenas) donde el mismo " +
		"hombre puede en mayor diversidad de formas, y en todas"

	own, rest, divides := shareOut(piece{text: text}, Box{100, 100, 900, 140},
		Box{90, 90, 910, 240}, 1)

	if !divides {
		t.Fatal("the piece would not divide")
	}
	if len(rest) != 1 {
		t.Fatalf("divided into %d filled lines, want 1", len(rest))
	}
	if joined := own + " " + rest[0]; joined != text {
		t.Errorf("the two lines read %q, want %q", joined, text)
	}

	even := len([]rune(text)) / 2
	if got := len([]rune(own)); got < even-12 || got > even {
		t.Errorf("the line took %d runes of %d, want a little under half",
			got, len([]rune(text)))
	}
}

func TestShareOutRefusesAPieceWithNothingToSpare(t *testing.T) {
	if _, _, divides := shareOut(piece{text: "alpha beta"}, Box{100, 100, 900, 140},
		Box{90, 90, 910, 240}, 3); divides {
		t.Error("two words divided into four lines, want a refusal")
	}
	if _, _, divides := shareOut(piece{text: "   "}, Box{100, 100, 900, 140},
		Box{90, 90, 910, 240}, 0); divides {
		t.Error("a piece of whitespace divided, want a refusal")
	}
}

// The filler runs on geometry, so a block with no room above its first line fills
// nothing and the old behaviour stands: the stray text joins the line below it.
func TestAStrayHeadWithNoRoomAboveItJoinsItsLine(t *testing.T) {
	result := mergeRows(">>> alpha beta gamma <<<", "alpha", "beta", "gamma")

	if result.Counts.Estimated != 0 {
		t.Errorf("filled %d lines in a block with no room, want none",
			result.Counts.Estimated)
	}
	if len(result.Dropped) != 0 {
		t.Errorf("dropped %v, want nothing", result.Dropped)
	}
	if !strings.Contains(result.Lines[0].Text, ">>>") {
		t.Errorf("the first line reads %q, want the head kept", result.Lines[0].Text)
	}
}
