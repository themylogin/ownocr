// Package health judges the reading a page answers against the line reading of
// that same page.
//
// The line engine cannot loop, cannot invent a script, and cannot abandon a page
// after two sentences. A reading engine can do all three, and its text reads
// fluently while it does, so nothing about a broken page looks broken. Where the
// two readings of one page part company, the reading engine is the suspect.
//
// Every test is page-local. What the other pages of the book hold never enters
// it: a page carrying a script its neighbours do not carry is a normal page of a
// mixed book, not a fault.
package health

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/themylogin/ownocr/backend/internal/latex"
)

// The measured thresholds. Every one comes from the 287 read pages of books 17,
// 18 and 24, where 17 pages carry a fault confirmed by hand.
const (
	// minWordShare is the share of the long words of the line reading that the
	// answering reading must keep. The 15 quiet failures of the measured books
	// keep 5% to 45%; a sound reading keeps 66% to 97%.
	minWordShare = 0.60

	// minWordLetters is the length a word must reach to be counted. Below it a
	// word is an article or a preposition, which two readings of one page share
	// whatever either got wrong.
	minWordLetters = 4

	// minLengthShare is the share of the characters of the line reading that the
	// answering reading must hold. It is what catches a reading that stopped
	// early rather than one that read the page wrongly.
	minLengthShare = 0.60

	// maxExcessRepeat is how much more of itself a reading may repeat than the
	// line reading of the same page repeats, before it is a loop and not a page.
	//
	// The excess and not the share: a page can repeat itself. An "ADVERTENCIA"
	// naming three texts published by the same press in three years repeats a
	// third of itself, and so does the line reading of it, because the repetition
	// is printed on the page. A reading that degenerated repeats what its line
	// reading does not.
	//
	// Over the 27 flagged pages of books 17, 18 and 24 the excess is bimodal: the
	// two real loops stand at 0.75 and 0.84, and every other page at 0.05 or
	// below, the repetitive front matter among them. Every value between answers
	// those pages alike.
	maxExcessRepeat = 0.25

	// runLength is the window the repetition is measured over, in characters. It
	// is longer than a word and shorter than a line, so a repeated phrase counts
	// and a repeated word does not.
	runLength = 15

	// minForeign is the characters of scripts the page is not written in that a
	// reading must hold before the script decides anything.
	//
	// A reading that turned to another script turns a passage, not a letter. One or
	// two stray characters are a letter the reading engine mistook for a letter of
	// another alphabet, and the merge drops such a line anyway for disagreeing with
	// the line reading — so the page reads correctly and calling it doubted is
	// noise. Over the read pages of books 17, 18 and 24 the stray readings hold 1 to
	// 8 such characters and the collapsed ones hold 26, 30, 94 and 119.
	minForeign = 10

	// minLetters is the letters a line reading must hold before a share of it
	// means anything. A plate, a half title or a blank leaf holds too little
	// text for the wording and the length to say anything, and is judged by the
	// repetition and the script alone.
	minLetters = 400
)

// The faults a page can carry. A page disagrees when it carries any of them.
const (
	FaultTruncated  = "truncated"
	FaultWording    = "wording"
	FaultRepetition = "repetition"
	FaultScript     = "script"
)

// Page is the two readings of one page.
type Page struct {
	// Answered is the text of the reading the page answers, and Line the text of
	// the PP-OCRv6 reading of that page.
	Answered string
	Line     string

	// SameEngine marks a page whose answering reading is its line reading. One
	// engine cannot disagree with itself, so such a page never disagrees.
	SameEngine bool
}

// Verdict says whether a page disagrees and which faults it carries. The faults
// are for a reader of a log or a report; nothing in the system branches on them.
type Verdict struct {
	Disagrees bool
	Faults    []string

	// Foreign names the scripts the reading holds that the line reading of the
	// page holds none of, and Counts how many characters of each it holds. The
	// count reports the fault; it never decides it.
	Foreign []string
	Counts  map[string]int

	// Repeat is the share of the reading that repeats itself, and LineRepeat the
	// share of the line reading of the same page. Both are reported so a run over
	// real pages can be read off.
	Repeat     float64
	LineRepeat float64

	// WordShare is the share of the long words of the line reading the answering
	// reading keeps, and LengthShare the share of its characters. Both are 1 for a
	// page judged by the repetition and the script alone.
	WordShare   float64
	LengthShare float64
}

// Reasons answers why a page disagrees, in words and with the numbers that
// decided it. A reader asks the page what is wrong with it, so the answer says
// what was measured and not which test fired.
//
// It answers nothing for a page that does not disagree.
func (v Verdict) Reasons() []string {
	if !v.Disagrees {
		return nil
	}

	reasons := make([]string, 0, len(v.Faults))
	for _, fault := range v.Faults {
		switch fault {
		case FaultTruncated:
			reasons = append(reasons, fmt.Sprintf(
				"Contains only %.0f%% of the text", v.LengthShare*100))
		case FaultWording:
			reasons = append(reasons, fmt.Sprintf(
				"Contains only %.0f%% of words", v.WordShare*100))
		case FaultRepetition:
			reasons = append(reasons, fmt.Sprintf("%.0f%% hallucinations", v.Repeat*100))
		case FaultScript:
			for _, name := range v.Foreign {
				reasons = append(reasons, fmt.Sprintf("%d unexpected %s character%s",
					v.Counts[name], name, plural(v.Counts[name])))
			}
		}
	}
	return reasons
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// Judge answers whether the reading a page answers disagrees with its line
// reading.
func Judge(page Page) Verdict {
	if page.SameEngine {
		return Verdict{}
	}

	line := prose(page.Line)
	if strings.TrimSpace(line) == "" {
		// Nothing to judge it against.
		return Verdict{}
	}
	answered := prose(page.Answered)

	var faults []string

	repeat, lineRepeat := repeatShare(answered), repeatShare(line)
	if repeat-lineRepeat > maxExcessRepeat {
		faults = append(faults, FaultRepetition)
	}
	foreign, counts := foreignScripts(answered, line)
	if foreignCount(counts) >= minForeign {
		faults = append(faults, FaultScript)
	}

	// A page holding too little text for a share of it to mean anything is
	// judged by the two tests above alone.
	words, length := 1.0, 1.0
	if letters(line) >= minLetters {
		length = float64(len([]rune(answered))) / float64(len([]rune(line)))
		if length < minLengthShare {
			faults = append(faults, FaultTruncated)
		}
		words = wordShare(answered, line)
		if words < minWordShare {
			faults = append(faults, FaultWording)
		}
	}

	return Verdict{Disagrees: len(faults) > 0, Faults: faults,
		Foreign: foreign, Counts: counts, Repeat: repeat, LineRepeat: lineRepeat,
		WordShare: words, LengthShare: length}
}

// Disagrees answers the verdict of Judge alone.
func Disagrees(page Page) bool { return Judge(page).Disagrees }

// markup is what an engine wrote and no reader speaks: an HTML tag for a table
// or a plate, and a markdown image placeholder. None of it is reading, and all of
// it repeats, so it would answer the repetition test on its own.
var (
	htmlTag  = regexp.MustCompile(`<[^>]*>`)
	mdImage  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	repeated = regexp.MustCompile(`\s+`)
)

// prose answers the reading of a text: its LaTeX read as the characters it
// stands for, its markup gone, and its spacing flattened so one reading wrapping
// a line differently from the other cannot count as a difference.
func prose(text string) string {
	plain := latex.Clean(text)
	plain = mdImage.ReplaceAllString(plain, " ")
	plain = htmlTag.ReplaceAllString(plain, " ")
	return strings.TrimSpace(repeated.ReplaceAllString(plain, " "))
}

func letters(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return count
}

// wordShare answers the share of the long words of the line reading that the
// answering reading holds. It counts a word once however often either reading
// writes it, so a reading that loops one phrase cannot earn a share with it.
func wordShare(answered, line string) float64 {
	want := longWords(line)
	if len(want) == 0 {
		return 1
	}

	held := longWords(answered)
	found := 0
	for word := range want {
		if held[word] {
			found++
		}
	}
	return float64(found) / float64(len(want))
}

func longWords(text string) map[string]bool {
	words := make(map[string]bool)
	for _, field := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len([]rune(field)) >= minWordLetters {
			words[strings.ToLower(field)] = true
		}
	}
	return words
}

// repeatShare answers the share of the character runs of a text that the text
// holds more than once. It is measured on both readings of a page and never on
// one: the share alone cannot tell a reading that looped from a page that repeats,
// and the line reading says which of the two it is.
func repeatShare(text string) float64 {
	runes := []rune(text)
	if len(runes) <= runLength {
		return 0
	}

	seen := make(map[string]int, len(runes))
	for i := 0; i+runLength <= len(runes); i++ {
		seen[string(runes[i:i+runLength])]++
	}

	windows, repeats := 0, 0
	for _, count := range seen {
		windows += count
		if count > 1 {
			repeats += count
		}
	}
	if windows == 0 {
		return 0
	}
	return float64(repeats) / float64(windows)
}

// foreignScripts answers the scripts the answering reading holds that the line
// reading of the same page holds none of.
//
// Nothing here holds a list of forbidden scripts. A book written in a script is
// not a book failing to be written in another, so the line reading of the page is
// the whole of the evidence.
//
// The count of characters does not decide it. One substituted letter and a whole
// substituted page both name a script the page is not written in.
func foreignScripts(answered, line string) ([]string, map[string]int) {
	held := scriptsIn(line)

	var foreign []string
	counts := make(map[string]int)
	for name, count := range scriptsIn(answered) {
		if held[name] == 0 {
			foreign = append(foreign, name)
			counts[name] = count
		}
	}
	sort.Strings(foreign)
	return foreign, counts
}

// foreignCount is how many characters of scripts the page is not written in the
// reading holds, over every such script. A reading that turned to another script a
// few characters at a time is as broken as one that turned wholly, so the count is
// of all of them together and not of the worst.
func foreignCount(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func scriptsIn(text string) map[string]int {
	found := make(map[string]int)
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		if name, ok := scriptOf(r); ok {
			found[name]++
		}
	}
	return found
}

// scriptOf answers the Unicode script a letter belongs to. The tables are the
// whole of the knowledge: this package names no alphabet of its own.
//
// The answer is remembered, because finding it costs a walk of every script
// table and a book asks for the same few hundred letters over and over.
var scriptCache sync.Map

func scriptOf(r rune) (string, bool) {
	if name, found := scriptCache.Load(r); found {
		return name.(string), name.(string) != ""
	}

	name := ""
	for candidate, table := range unicode.Scripts {
		if unicode.Is(table, r) {
			name = candidate
			break
		}
	}

	scriptCache.Store(r, name)
	return name, name != ""
}

// Judgements judges every page of a book and answers the pages that disagree.
// Every page is judged on its own, so the answer for one page here and the
// answer for that page alone can never differ.
func Judgements(pages map[int64]Page) map[int64]Verdict {
	verdicts := make(map[int64]Verdict, len(pages))
	for id, page := range pages {
		verdicts[id] = Judge(page)
	}
	return verdicts
}

// Disagreeing counts the pages of a book that disagree.
func Disagreeing(verdicts map[int64]Verdict) int {
	count := 0
	for _, verdict := range verdicts {
		if verdict.Disagrees {
			count++
		}
	}
	return count
}
