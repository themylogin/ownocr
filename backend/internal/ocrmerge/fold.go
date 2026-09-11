package ocrmerge

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// fold answers the form of a rune that comparison uses: compatibility
// decomposition, then every combining mark removed, then lower case.
//
// Folding is what makes the merge safe rather than reckless. A box engine that
// read the letters of a word and missed its accents still folds onto what the
// text engine wrote, so a line the text engine merely improved is never mistaken
// for a line the two engines disagree about.
func fold(r rune) string {
	decomposed := norm.NFKD.String(string(r))

	var b strings.Builder
	b.Grow(len(decomposed))
	for _, d := range decomposed {
		if unicode.Is(unicode.Mn, d) {
			continue
		}
		b.WriteRune(unicode.ToLower(d))
	}

	folded := b.String()
	if folded == "" {
		// A rune that folds away entirely, such as a lone combining mark, still
		// has to compare unequal to every other rune.
		return string(r)
	}
	return folded
}

// foldIDs numbers the folded forms of runes, so the alignment compares two ints
// per cell instead of decomposing a rune per cell. The caller shares one table
// across both streams, which is what makes equal ids mean equal folded forms.
func foldIDs(runes []rune, table map[string]int) []int {
	ids := make([]int, len(runes))
	for i, r := range runes {
		key := fold(r)
		id, ok := table[key]
		if !ok {
			id = len(table)
			table[key] = id
		}
		ids[i] = id
	}
	return ids
}
