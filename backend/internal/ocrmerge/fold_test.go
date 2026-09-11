package ocrmerge

import "testing"

func TestFoldRemovesAccentsAndCase(t *testing.T) {
	same := []struct{ a, b rune }{
		{'í', 'i'},
		{'ó', 'o'},
		{'Φ', 'φ'},
		{'ὸ', 'ο'},
		{'ἐ', 'ε'},
		{'Á', 'a'},
		{'ñ', 'n'},
	}
	for _, pair := range same {
		if fold(pair.a) != fold(pair.b) {
			t.Errorf("fold(%q)=%q, fold(%q)=%q, want equal",
				pair.a, fold(pair.a), pair.b, fold(pair.b))
		}
	}

	// Folding removes marks and case. It must not pull two different letters
	// together, or the alignment would call a Greek omicron a Latin o and the
	// merge would lose the difference it exists to keep.
	differ := []struct{ a, b rune }{
		{'φ', 'o'},
		{'ο', 'o'},
		{'σ', 'c'},
		{'ν', 'v'},
		{'a', 'b'},
	}
	for _, pair := range differ {
		if fold(pair.a) == fold(pair.b) {
			t.Errorf("fold(%q) and fold(%q) both give %q, want different",
				pair.a, pair.b, fold(pair.a))
		}
	}
}

func TestFoldKeepsALoneMark(t *testing.T) {
	if fold('́') == "" {
		t.Error("a lone combining acute folds to nothing, so it would equal every empty fold")
	}
}

func TestFoldIDsShareOneTable(t *testing.T) {
	table := map[string]int{}
	left := foldIDs([]rune("ÍO"), table)
	right := foldIDs([]rune("io"), table)

	for i := range left {
		if left[i] != right[i] {
			t.Errorf("rune %d: id %d and %d differ, want equal", i, left[i], right[i])
		}
	}
}
