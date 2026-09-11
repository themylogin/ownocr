package ocrmerge

import (
	"strings"
	"testing"
)

func TestAlignPairsGreekTheBoxEngineMangled(t *testing.T) {
	// The box engine wrote Latin lookalikes inside a Greek word and ended the
	// printed line with a hyphen the text engine does not carry.
	line := []rune("φiλoσo-")
	block := []rune("φιλοσοφ")

	aligned := align(line, block)
	if aligned == nil {
		t.Fatal("align refused two seven rune streams")
	}

	// Every one of the six letters pairs with the letter under it, so the cut
	// walks the block one rune at a time. Pairing is what keeps the line whole.
	for i := 0; i <= 6; i++ {
		if aligned.cut[i] != i {
			t.Errorf("rune %d sits at block position %d, want %d", i, aligned.cut[i], i)
		}
	}

	// Only the three Greek letters the box engine got right agree. A Latin `i`
	// is not a Greek iota and folding must not pretend otherwise, so the pairs at
	// 1, 3 and 5 are the disagreement this merge exists to resolve.
	want := []bool{true, false, true, false, true, false, false}
	for i, expected := range want {
		if aligned.agree[i] != expected {
			t.Errorf("rune %d (%q) agrees=%v, want %v", i, line[i], aligned.agree[i], expected)
		}
	}

	// Three of seven is 0.43, above minAgreement, so a line like this is accepted.
	if got := agreementOf(aligned, bound{0, 7}); got < minAgreement {
		t.Errorf("agreement is %.2f, want at least %.2f", got, minAgreement)
	}
}

func TestAlignPrefersASubstitutionOverTwoGaps(t *testing.T) {
	aligned := align([]rune("abc"), []rune("axc"))
	if aligned == nil {
		t.Fatal("align refused three rune streams")
	}

	// A gap pair costs 16 and a substitution costs 10, so `b` must pair with `x`
	// rather than both being dropped. That relation is what keeps a line the two
	// engines read differently aligned instead of torn apart.
	if aligned.cut[1] != 1 || aligned.cut[2] != 2 {
		t.Errorf("cuts are %v, want b paired with x", aligned.cut)
	}
}

func TestAlignPairsAnyWhitespaceWithAnyWhitespace(t *testing.T) {
	aligned := align([]rune("a\nb"), []rune("a b"))
	if aligned == nil {
		t.Fatal("align refused three rune streams")
	}

	// The newline of a line break pairs with the space the text engine reflowed
	// it into, at no cost, so the letters around it stay in step.
	for i := 0; i <= 3; i++ {
		if aligned.cut[i] != i {
			t.Errorf("rune %d sits at block position %d, want %d", i, aligned.cut[i], i)
		}
	}

	// It costs nothing to align and still does not agree. A space is evidence of
	// nothing, and counting it would inflate the share of every line alike.
	if aligned.agree[1] {
		t.Error("a newline agreed with a space, want whitespace left out of the share")
	}
	if !aligned.agree[0] || !aligned.agree[2] {
		t.Error("the letters around the break did not agree")
	}
}

func TestAlignRefusesStreamsTooLargeToTable(t *testing.T) {
	huge := []rune(strings.Repeat("x", 8192))
	if align(huge, huge) != nil {
		t.Error("align built a table of more than maxAlignCells")
	}
}

func TestAlignReportsTheEndOfTheBlock(t *testing.T) {
	aligned := align([]rune("ab"), []rune("abcdef"))
	if aligned == nil {
		t.Fatal("align refused two short streams")
	}
	if got := aligned.cut[2]; got != 6 {
		t.Errorf("the end of the line stream sits at %d, want 6", got)
	}
}
