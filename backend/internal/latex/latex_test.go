package latex

import (
	"strings"
	"testing"
	"unicode"
)

// mangledParagraph is one block of a page the engine read badly, joined as the
// engine writes a block. The book prints `ἀλήθεια`, and the engine answers a
// Greek rune, three LaTeX commands, another Greek rune and another command, with
// dollar signs around the pieces.
//
// It is the whole reason for the rescue. Its dollar signs are odd in number, so
// no pairing can read them as delimited spans.
const mangledParagraph = "Pues bien, también en la otra contraposición el término que " +
	"se define (o, si nos permitimos decirlo así, el término «positivo») " +
	`es $ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ o $ τὸ $ $ ἐóν $, pero $ ἱ $ $ cómo $ se llama entonces el término ` +
	"«otro» o «negativo»? Lo cierto es que no tiene nombre, y todo " +
	"parece indicar que es esencial que no lo tenga. Por referencia a " +
	"δὲν $ εἴναι $, eso «otro» es aludido en fórmulas como μὴ $ ἐὶν $, μὴ $ " +
	"εἴναι $, esto es: mediante una no-designación. Por referencia a " +
	`$ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ no hay nombre alguno para lo «otro» de la contraposición, ` +
	"si bien, vista esta ausencia de nombre y visto el modo en " +
	"que se no-nombra lo otro que $ δν $ $ εἴναι $, adquiere relevancia el " +
	`hecho de que la propia palabra $ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ connota un cierto rechazo de algo, no en virtud de`

// everyCase is every text the tests below read, so the repeatability test covers
// all of them without repeating one.
var everyCase = []string{
	mangledParagraph,
	`Con la palabra misma $ ^{1} $ está vinculada una`,
	`que en su lugar se describen $ ^{50} $. Dentro de`,
	`se ofrece$^{51}$ del rasgo`,
	`el agua $ _{2} $ hierve`,
	`la nota $ ^{a} $ del pie`,
	`it costs $5 and I paid $12`,
	`un texto con τὸ ἐόν pero $ sin $ comandos`,
	`la relectura $ \alpha $ del rasgo, sin griego`,
	`el término $ τὸ $ \etax aparte`,
	`the map $ f \circ g $ and the angle $ 30 \circ $`,
	`contraposición de $ \lambda\eta\theta\epsilon\iota\eta $ a $ \delta\delta\xi\alpha $ ` +
		`o de $ \tau\ddot{\alpha} $ éóv a $ \tau\ddot{\alpha}\delta\kappa\circ\bar{\upsilon}\nu\tau\alpha $.`,
	`es $ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ o $ τὸ $ $ ἐóν $, pero $ ἱ $ $ cómo $ se llama`,
	`es $ τὸ $ \eta la nota $ ^{12} $ y sigue`,
	`el término $ τὸ $ \eta y \foobar aparte`,
	`Pues bien, también en la otra contraposición`,
	`guardado en C:\temp\newfile.txt`,
	`la razón $\frac{a+b}{2}$ del texto`,
	``,
}

func TestCleanConvertsAFootnoteMarker(t *testing.T) {
	for _, one := range []struct{ read, want string }{
		{`Con la palabra misma $ ^{1} $ está vinculada una`,
			`Con la palabra misma¹ está vinculada una`},
		{`que en su lugar se describen $ ^{50} $. Dentro de`,
			`que en su lugar se describen⁵⁰. Dentro de`},
		{`se ofrece$^{51}$ del rasgo`,
			`se ofrece⁵¹ del rasgo`},
		{`el agua $ _{2} $ hierve`,
			`el agua₂ hierve`},
	} {
		if got := Clean(one.read); got != one.want {
			t.Errorf("Clean(%q)\n got %q\nwant %q", one.read, got, one.want)
		}
	}
}

func TestCleanLeavesAMarkerItCannotSpell(t *testing.T) {
	for _, read := range []string{
		`la nota $ ^{a} $ del pie`,
		`it costs $5 and I paid $12`,
	} {
		if got := Clean(read); got != read {
			t.Errorf("Clean(%q) answered %q, want the text it read", read, got)
		}
	}
}

// The engine spells a whole line as commands and writes the runes beside them
// with Latin lookalikes: `éóv` is a Latin e, a Latin o and a Latin v for a word
// the book prints `ἐόν`. The block holds no Greek rune at all, so a rune can be
// no part of the evidence.
func TestCleanRescuesALineOfCommandsWithNoGreekRune(t *testing.T) {
	read := `contraposición de $ \lambda\eta\theta\epsilon\iota\eta $ a $ \delta\delta\xi\alpha $ ` +
		`o de $ \tau\ddot{\alpha} $ éóv a $ \tau\ddot{\alpha}\delta\kappa\circ\bar{\upsilon}\nu\tau\alpha $.`
	want := `contraposición de ληθειη a δδξα o de τα éóv a ταδκουντα.`

	for _, found := range read {
		if unicode.Is(unicode.Greek, found) {
			t.Fatalf("the fixture holds the Greek rune %q, so it proves nothing", found)
		}
	}

	if got := Clean(read); got != want {
		t.Errorf("Clean a line of commands\n got %q\nwant %q", got, want)
	}
}

// `\circ` is the ring operator of real mathematics. It earns no reading of its
// own, so a block that names it and no Greek letter keeps every dollar sign.
func TestCleanLeavesAMathBlockAlone(t *testing.T) {
	read := `the map $ f \circ g $ and the angle $ 30 \circ $`
	if got := Clean(read); got != read {
		t.Errorf("Clean(%q) answered %q, want the text it read", read, got)
	}
}

func TestTheGreekMapAnswersOneGreekRunePerEntry(t *testing.T) {
	if len(greek) != 41 {
		t.Errorf("the map holds %d commands, want 41", len(greek))
	}
	for name, found := range greek {
		if !unicode.Is(unicode.Greek, found) {
			t.Errorf(`\%s answers %q, which is not a Greek rune`, name, found)
		}
	}
	for _, name := range []string{
		"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta",
		"iota", "kappa", "lambda", "mu", "nu", "xi", "omicron", "pi", "rho",
		"sigma", "tau", "upsilon", "phi", "chi", "psi", "omega",
	} {
		if _, known := greek[name]; !known {
			t.Errorf(`the map does not know \%s`, name)
		}
	}
}

// The command runs to the end of its letters, so a longer name is another
// command and not a known one with a letter behind it.
func TestCleanReadsACommandToItsEnd(t *testing.T) {
	read := `el término $ τὸ $ \etax aparte`
	if got := Clean(read); got != read {
		t.Errorf(`Clean(%q) answered %q, so \etax was read as \eta`, read, got)
	}
}

// Greek runes the engine read correctly are not evidence that it mangled
// anything. Only a command it wrote is.
func TestCleanKeepsTheDollarSignsWithoutAGreekCommand(t *testing.T) {
	for _, read := range []string{
		`un texto con τὸ ἐόν pero $ sin $ comandos`,
		`it costs $5 and I paid $12`,
		`la razón $\frac{a+b}{2}$ del texto`,
	} {
		if got := Clean(read); got != read {
			t.Errorf("Clean(%q) answered %q, want the text it read", read, got)
		}
	}
}

// One command is the whole of the evidence, with no Greek rune anywhere near it.
func TestCleanRescuesOnOneCommandAlone(t *testing.T) {
	read := `la relectura $ \alpha $ del rasgo, sin griego`
	want := `la relectura α del rasgo, sin griego`

	if got := Clean(read); got != want {
		t.Errorf("Clean(%q)\n got %q\nwant %q", read, got, want)
	}
}

func TestCleanRescuesAGreekWord(t *testing.T) {
	read := `es $ ᄋλὴ $ \eta\theta\epsilon ῥ\eta $ o $ τὸ $ $ ἐóν $, pero $ ἱ $ $ cómo $ se llama`
	want := `es ᄋλὴ ηθε ῥη o τὸ ἐóν, pero ἱ cómo se llama`

	if got := Clean(read); got != want {
		t.Errorf("Clean the mangled Greek\n got %q\nwant %q", got, want)
	}
}

func TestCleanRescuesTheParagraphTheEngineMangled(t *testing.T) {
	if held := strings.Count(mangledParagraph, "$"); held != 27 {
		t.Fatalf("the fixture holds %d dollar signs, want the 27 the engine wrote", held)
	}
	if held := strings.Count(mangledParagraph, `\`); held != 12 {
		t.Fatalf("the fixture holds %d commands, want the 12 the engine wrote", held)
	}

	got := Clean(mangledParagraph)

	if strings.Contains(got, "$") {
		t.Errorf("the answer still holds a dollar sign:\n%s", got)
	}
	if strings.Contains(got, `\`) {
		t.Errorf("the answer still holds a command:\n%s", got)
	}
	if strings.Contains(got, "  ") {
		t.Errorf("the answer holds the spacing a dollar sign occupied:\n%s", got)
	}
	for _, want := range []string{"ᄋλὴ ηθε ῥη", "μὴ εἴναι", "τὸ ἐóν, pero"} {
		if !strings.Contains(got, want) {
			t.Errorf("the answer does not hold %q:\n%s", want, got)
		}
	}
}

// The marker runs first, or the rescue takes its dollar signs away and strands
// the script between them.
func TestCleanConvertsAMarkerInsideARescuedText(t *testing.T) {
	read := `es $ τὸ $ \eta la nota $ ^{12} $ y sigue`
	want := `es τὸ η la nota¹² y sigue`

	if got := Clean(read); got != want {
		t.Errorf("Clean a rescued text holding a marker\n got %q\nwant %q", got, want)
	}
}

func TestCleanLeavesAnUnknownCommandInARescuedText(t *testing.T) {
	read := `el término $ τὸ $ \eta y \foobar aparte`
	want := `el término τὸ η y \foobar aparte`

	if got := Clean(read); got != want {
		t.Errorf("Clean a rescued text holding an unknown command\n got %q\nwant %q", got, want)
	}
}

func TestCleanChangesNothingElse(t *testing.T) {
	for _, read := range []string{
		`Pues bien, también en la otra contraposición`,
		`guardado en C:\temp\newfile.txt`,
		`la razón $\frac{a+b}{2}$ del texto`,
		``,
	} {
		if got := Clean(read); got != read {
			t.Errorf("Clean(%q) answered %q, want the text it read", read, got)
		}
	}
}

func TestCleanIsRepeatable(t *testing.T) {
	for _, read := range everyCase {
		once := Clean(read)
		if twice := Clean(once); twice != once {
			t.Errorf("a second reading of %q\n got %q\nwant %q", read, twice, once)
		}
	}
}
