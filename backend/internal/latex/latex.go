// Package latex reads the text a visual OCR engine wrote with LaTeX fragments
// in it, and answers the plain text of it.
//
// PaddleOCR-VL writes two shapes on a printed book. It wraps a footnote marker
// as `$ ^{33} $`. It spells a Greek word as LaTeX Greek commands, wrapped in
// dollar signs, sometimes with Greek runes among them and sometimes not.
//
// Those dollar signs are not the delimiters of a formula. The engine wraps
// single words in them, including words of the surrounding language, and it
// writes an odd number of them on a text that holds several. Nothing here pairs
// them, because no pairing can read them.
//
// A known Greek command is the whole of the evidence. The runes beside it say
// nothing: the engine writes `éóv` with a Latin e, a Latin o and a Latin v for a
// word the book prints `ἐόν`, so a block that reads as Greek to the eye can hold
// no Greek rune at all.
package latex

import (
	"regexp"
	"strings"
)

var greek = map[string]rune{
	"alpha": 'α', "beta": 'β', "gamma": 'γ', "delta": 'δ',
	"epsilon": 'ε', "varepsilon": 'ε', "zeta": 'ζ', "eta": 'η',
	"theta": 'θ', "vartheta": 'ϑ', "iota": 'ι', "kappa": 'κ',
	"lambda": 'λ', "mu": 'μ', "nu": 'ν', "xi": 'ξ',
	"omicron": 'ο', "pi": 'π', "varpi": 'ϖ', "rho": 'ρ',
	"varrho": 'ϱ', "sigma": 'σ', "varsigma": 'ς', "tau": 'τ',
	"upsilon": 'υ', "phi": 'φ', "varphi": 'ϕ', "chi": 'χ',
	"psi": 'ψ', "omega": 'ω',

	// Only the capitals a reader cannot mistake for a Latin letter. The others
	// are the Latin letter on the printed page, and the engine writes them so.
	"Gamma": 'Γ', "Delta": 'Δ', "Theta": 'Θ', "Lambda": 'Λ',
	"Xi": 'Ξ', "Pi": 'Π', "Sigma": 'Σ', "Upsilon": 'Υ',
	"Phi": 'Φ', "Psi": 'Ψ', "Omega": 'Ω',
}

// What the engine writes for a Greek letter it did not read as one. `\circ` is
// the circle it saw where the book prints an omicron.
//
// These are not evidence. `\circ` means the ring operator in a text that is
// really mathematics, so a block earns the reading below through a command in
// `greek` and never through one of these.
var misread = map[string]rune{
	"circ": 'ο',
}

// The engine puts an accent on a letter it could not place. The accent it picks
// is its own guess and a wrong one, so the letter under it is all this keeps.
var accent = regexp.MustCompile(
	`\\(?:ddot|dot|bar|hat|tilde|acute|grave|breve|check|vec|mathring)\{([^{}]*)\}`)

var superscript = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
	'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
}

var subscript = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄',
	'5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
}

var (
	// The whole shape at once, so an unpaired dollar sign elsewhere in the text
	// cannot reach it and a price cannot start one. The body is digits alone:
	// every digit has a Unicode script and most other characters do not, and a
	// marker this cannot spell in full is better left whole than half converted.
	marker = regexp.MustCompile(`[ \t]*\$\s*([\^_])\{([0-9]+)\}\s*\$`)

	command     = regexp.MustCompile(`\\([A-Za-z]+)`)
	spaceBefore = regexp.MustCompile(`[ \t]+([,.;:!?»])`)
	spaceAfter  = regexp.MustCompile(`([«¿¡])[ \t]+`)
	spaceRun    = regexp.MustCompile(`[ \t]{2,}`)
)

// Clean answers the plain text of one block.
//
// A block it has no rule for comes back rune for rune. The answer of a block it
// does change answers itself, so a text cleaned twice reads as a text cleaned
// once.
func Clean(text string) string {
	// Before the rescue below, which would take the dollar signs of a marker
	// away and strand the `^{33}` between them.
	text = marker.ReplaceAllStringFunc(text, script)

	// The dollar signs go only on evidence that this block is one the engine
	// mangled: a Greek command it wrote for a letter the book prints. A book
	// prints no Greek command, so the engine wrote every one this finds.
	if !holdsGreekCommand(text) {
		return text
	}

	text = accent.ReplaceAllString(text, "$1")
	text = command.ReplaceAllStringFunc(text, letter)
	// A space, not nothing: the markup separated words that must stay separate.
	text = strings.ReplaceAll(text, "$", " ")
	text = spaceBefore.ReplaceAllString(text, "$1")
	text = spaceAfter.ReplaceAllString(text, "$1")
	text = spaceRun.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func script(match string) string {
	parts := marker.FindStringSubmatch(match)

	table := superscript
	if parts[1] == "_" {
		table = subscript
	}

	var plain strings.Builder
	// The body matched `[0-9]+`, so the table holds every rune of it.
	for _, digit := range parts[2] {
		plain.WriteRune(table[digit])
	}
	return plain.String()
}

func letter(match string) string {
	if found, known := greek[match[1:]]; known {
		return string(found)
	}
	if found, known := misread[match[1:]]; known {
		return string(found)
	}
	return match
}

func holdsGreekCommand(text string) bool {
	for _, found := range command.FindAllStringSubmatch(text, -1) {
		if _, known := greek[found[1]]; known {
			return true
		}
	}
	return false
}
