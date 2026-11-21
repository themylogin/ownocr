package health

import (
	"strings"
	"testing"
)

// A page of running prose, long enough for the wording and the length to be
// measured. The two readings of a sound page differ in the letters neither
// engine is sure of and in nothing else.
const linePage = `El sentido de la palabra no reside en la cosa nombrada sino en
el uso que de ella hacemos. Aristoteles llama a esto la significacion primera,
y distingue entre aquello que se dice de suyo y aquello que se dice por
accidente. La cuestion aparece tratada en el libro cuarto de la Metafisica,
donde el filosofo sostiene que ninguna demostracion alcanza a los principios.
Aquel que niega el principio de contradiccion no puede argumentar, porque todo
argumento lo supone. Por eso la refutacion no procede contra la tesis sino
contra quien la sostiene. El adversario habla y al hablar concede lo que
pretende negar. Tal es el argumento que los comentadores llaman elenctico, y
que Alejandro de Afrodisia expone con alguna extension en su parafrasis.`

func TestACleanReadingAgrees(t *testing.T) {
	// One reading spells a few letters differently and wraps its lines its own
	// way, which is what two sound readings of one page look like.
	answered := strings.ReplaceAll(linePage, "significacion", "significación")
	answered = strings.ReplaceAll(answered, "\n", " ")

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if verdict.Disagrees {
		t.Errorf("a sound reading disagrees, faults %v", verdict.Faults)
	}
}

func TestAReadingThatAbandonedThePageDisagrees(t *testing.T) {
	answered := "El sentido de la palabra no reside en la cosa nombrada sino en"

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if !verdict.Disagrees {
		t.Fatal("a reading holding a tenth of the page agrees")
	}
	if !holds(verdict.Faults, FaultTruncated) {
		t.Errorf("faults are %v, want %q among them", verdict.Faults, FaultTruncated)
	}
}

func TestAReadingThatLostTheWordingDisagrees(t *testing.T) {
	// As long as the line reading and no shorter, so the length test cannot be
	// what catches it: the wording has to.
	answered := strings.Repeat("quamquam ceterum deinde igitur postea namque ", 24)

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if !verdict.Disagrees {
		t.Fatal("a reading sharing none of the wording agrees")
	}
	if !holds(verdict.Faults, FaultWording) {
		t.Errorf("faults are %v, want %q among them", verdict.Faults, FaultWording)
	}
}

func TestALoopingReadingDisagrees(t *testing.T) {
	answered := linePage[:400] + strings.Repeat(" el filosofo sostiene que ninguna", 40)

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if !verdict.Disagrees {
		t.Fatal("a reading that repeats one phrase to the foot of the page agrees")
	}
	if !holds(verdict.Faults, FaultRepetition) {
		t.Errorf("faults are %v, want %q among them", verdict.Faults, FaultRepetition)
	}
}

// A plate, a half title or a blank leaf holds too little text for a share of it
// to mean anything, so only the repetition and the script judge it.
func TestAPageOfLittleTextIsJudgedByRepetitionAndScriptAlone(t *testing.T) {
	const plate = "LAMINA IV"

	long := Judge(Page{Answered: "Lamina cuarta: vista del teatro desde la ladera " +
		"oriental, tomada por Eugene Sellers en mil novecientos doce, antes de que " +
		"los trabajos descubrieran el muro sur.", Line: plate})
	if long.Disagrees {
		t.Errorf("a reading longer than a plate disagrees, faults %v", long.Faults)
	}

	short := Judge(Page{Answered: "L", Line: plate})
	if short.Disagrees {
		t.Errorf("a reading shorter than a plate disagrees, faults %v", short.Faults)
	}

	// The two tests that survive still run on such a page.
	looping := Judge(Page{Answered: strings.Repeat("LAMINA IV LAMINA IV ", 20), Line: plate})
	if !holds(looping.Faults, FaultRepetition) {
		t.Errorf("a looping plate reports %v, want %q", looping.Faults, FaultRepetition)
	}
	foreign := Judge(Page{Answered: "ЛАМИНА ЧЕТВЕРТАЯ IV", Line: plate})
	if !holds(foreign.Faults, FaultScript) {
		t.Errorf("a plate read in another script reports %v, want %q",
			foreign.Faults, FaultScript)
	}
}

func TestAPageWithNoLineReadingNeverDisagrees(t *testing.T) {
	verdict := Judge(Page{Answered: strings.Repeat("ЛАМИНА ", 200), Line: ""})
	if verdict.Disagrees {
		t.Errorf("a page with nothing to judge it against disagrees, faults %v", verdict.Faults)
	}
}

// One engine cannot disagree with itself.
func TestAPageAnsweringItsLineReadingNeverDisagrees(t *testing.T) {
	verdict := Judge(Page{Answered: linePage, Line: linePage, SameEngine: true})
	if verdict.Disagrees {
		t.Errorf("a page answering its own line reading disagrees, faults %v", verdict.Faults)
	}
}

// The script test reads the two readings of the one page and nothing else.
func TestAPageWhoseLineReadingLacksTheScriptDisagrees(t *testing.T) {
	answered := strings.Replace(linePage, "significacion primera",
		"значение первое значение", 1)

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if !verdict.Disagrees {
		t.Fatal("a reading holding a script the line reading lacks agrees")
	}
	if !holds(verdict.Faults, FaultScript) {
		t.Errorf("faults are %v, want %q among them", verdict.Faults, FaultScript)
	}
}

func TestAPageWhoseLineReadingHoldsTheScriptAgrees(t *testing.T) {
	line := linePage + "\nВопрос этот разбирается в четвертой книге."
	answered := line + " Еще одно предложение того же письма."

	verdict := Judge(Page{Answered: answered, Line: line})
	if verdict.Disagrees {
		t.Errorf("a page written in Cyrillic disagrees on the script, faults %v", verdict.Faults)
	}
}

// A letter or two of another script is a letter the reading engine mistook for a
// letter of another alphabet. The merge drops such a line for disagreeing with the
// line reading, so the page reads correctly and calling it doubted is noise.
func TestAFewSubstitutedLettersDoNotDisagree(t *testing.T) {
	answered := strings.Replace(linePage, "Aristoteles", "Aristotelеs", 1)
	if answered == linePage {
		t.Fatal("the test substituted nothing")
	}

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if holds(verdict.Faults, FaultScript) {
		t.Errorf("one substituted letter disagrees: %v", verdict.Faults)
	}
}

// A reading that turned to another script turns a passage, and that is the fault.
func TestAPassageInAnotherScriptDisagrees(t *testing.T) {
	answered := strings.Replace(linePage, "Aristoteles llama a esto",
		"Аристотель называет это", 1)

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if !holds(verdict.Faults, FaultScript) {
		t.Errorf("faults are %v, want %q among them", verdict.Faults, FaultScript)
	}
}

// A reading that turned a few characters of each of two scripts is as broken as one
// that turned wholly, so the counts are read together.
func TestScriptsCountTogether(t *testing.T) {
	answered := strings.Replace(linePage, "Aristoteles llama",
		"Аристо 谓之 нача", 1)

	verdict := Judge(Page{Answered: answered, Line: linePage})
	if !holds(verdict.Faults, FaultScript) {
		t.Errorf("faults are %v, want %q among them: counts %v",
			verdict.Faults, FaultScript, verdict.Counts)
	}
	if len(verdict.Foreign) != 2 {
		t.Errorf("the page reports %v, want both scripts", verdict.Foreign)
	}
}

// A reading holds markup the engine wrote and no reader speaks. All of it
// repeats, so the system removes it before it measures anything.
func TestTheMarkupOfACoverIsNotAFault(t *testing.T) {
	answered := strings.Repeat(`<div style="text-align: center"><img src="x.jpg"></div>`, 12) +
		" BIBLIOTECA CLASICA"

	verdict := Judge(Page{Answered: answered, Line: "BIBLIOTECA CLASICA"})
	if verdict.Disagrees {
		t.Errorf("a cover disagrees on its markup alone, faults %v", verdict.Faults)
	}
}

// The engine writes Greek as LaTeX. That is the page read, not a script the page
// is not written in, so it must survive the reading of the text.
func TestLatexGreekIsNotAForeignScript(t *testing.T) {
	line := linePage + "\nla voz ἀρχή aparece dos veces"
	answered := strings.Replace(line, "ἀρχή", `$\alpha\rho\chi\acute{\eta}$`, 1)

	verdict := Judge(Page{Answered: answered, Line: line})
	if holds(verdict.Faults, FaultScript) {
		t.Errorf("LaTeX Greek reads as a foreign script, faults %v", verdict.Faults)
	}
}

func TestJudgementsCountsTheDisagreeingPages(t *testing.T) {
	pages := map[int64]Page{
		1: {Answered: linePage, Line: linePage, SameEngine: true},
		2: {Answered: "El sentido de la palabra", Line: linePage},
		3: {Answered: strings.ReplaceAll(linePage, "\n", " "), Line: linePage},
	}

	verdicts := Judgements(pages)
	if len(verdicts) != len(pages) {
		t.Fatalf("judged %d pages of %d", len(verdicts), len(pages))
	}
	if count := Disagreeing(verdicts); count != 1 {
		t.Errorf("%d pages disagree, want 1", count)
	}
	if !verdicts[2].Disagrees {
		t.Error("the truncated page agrees")
	}

	// A page judged in a book and judged alone can never differ.
	for id, page := range pages {
		if Judge(page).Disagrees != verdicts[id].Disagrees {
			t.Errorf("page %d is judged differently alone", id)
		}
	}
}

func holds(faults []string, want string) bool {
	for _, fault := range faults {
		if fault == want {
			return true
		}
	}
	return false
}

// A page can repeat itself. Front matter naming three texts published by the same
// press in three years repeats a third of itself, and the line reading of it
// repeats just as much, so the repetition is printed and not a fault.
func TestAPageThatRepeatsItselfIsNotALoop(t *testing.T) {
	const advertencia = `ADVERTENCIA

El texto En torno al nacimiento del titulo «filosofia» se publico en la revista
«Anales de filosofia» de la Universidad de Murcia, vol. I, 1983.

El texto El sentido y lo no-pensado (Apuntes para el tema «Heidegger y los
griegos») fue publicado por el Secretariado de Publicaciones de la Universidad
de Murcia en 1985.

El texto Heraclito-Parmenides (Bases para una lectura) fue publicado por el
Secretariado de Publicaciones de la Universidad de Murcia en 1987.`

	verdict := Judge(Page{Answered: advertencia, Line: advertencia})
	if holds(verdict.Faults, FaultRepetition) {
		t.Errorf("a page that repeats itself reads as a loop: repeat %.2f, line %.2f",
			verdict.Repeat, verdict.LineRepeat)
	}
	if verdict.Repeat < 0.2 {
		t.Errorf("the page repeats %.2f, too little for this test to prove anything",
			verdict.Repeat)
	}
}

// A reading that looped repeats what its line reading does not.
func TestALoopIsTheExcessOverTheLineReading(t *testing.T) {
	looping := strings.Repeat(" el filosofo sostiene que ninguna", 40)

	verdict := Judge(Page{Answered: linePage + looping, Line: linePage})
	if !holds(verdict.Faults, FaultRepetition) {
		t.Errorf("a looping reading reads as a page: repeat %.2f, line %.2f",
			verdict.Repeat, verdict.LineRepeat)
	}
	if verdict.LineRepeat > maxExcessRepeat {
		t.Errorf("the line reading repeats %.2f, so the test proves nothing",
			verdict.LineRepeat)
	}
}

// A reader asks a page what is wrong with it, so the answer says what was
// measured and not which test fired.
func TestReasonsSayWhatWasMeasured(t *testing.T) {
	truncated := Judge(Page{Answered: "El sentido de la palabra no reside", Line: linePage})
	reasons := strings.Join(truncated.Reasons(), " | ")
	for _, want := range []string{"% of the text", "% of words"} {
		if !strings.Contains(reasons, want) {
			t.Errorf("the reasons %q do not hold %q", reasons, want)
		}
	}

	foreign := Judge(Page{
		Answered: strings.Replace(linePage, "Aristoteles llama a esto",
			"Аристотель называет это", 1),
		Line: linePage,
	})
	if got := strings.Join(foreign.Reasons(), " | "); !strings.Contains(got, "unexpected Cyrillic characters") {
		t.Errorf("the reasons %q do not name the script and its count", got)
	}

	looping := Judge(Page{Answered: linePage + strings.Repeat(" el filosofo sostiene", 40),
		Line: linePage})
	if got := strings.Join(looping.Reasons(), " | "); !strings.Contains(got, "hallucinations") {
		t.Errorf("the reasons %q do not name the repetition", got)
	}
}

// A page that does not disagree has nothing to explain.
func TestACleanPageAnswersNoReason(t *testing.T) {
	verdict := Judge(Page{Answered: strings.ReplaceAll(linePage, "\n", " "), Line: linePage})
	if len(verdict.Reasons()) != 0 {
		t.Errorf("a clean page answers the reasons %v", verdict.Reasons())
	}
}

// A page judged by the repetition and the script alone reports no share of the
// wording or the length, because neither was measured.
func TestAPlateAnswersNoShareOfTheWording(t *testing.T) {
	verdict := Judge(Page{Answered: "ЛАМИНА ЧЕТВЕРТАЯ IV", Line: "LAMINA IV"})
	for _, reason := range verdict.Reasons() {
		if strings.Contains(reason, "% of words") || strings.Contains(reason, "% of the text") {
			t.Errorf("a plate answers %q", reason)
		}
	}
}
