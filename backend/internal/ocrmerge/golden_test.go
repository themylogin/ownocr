package ocrmerge

import "testing"

// pageTenMerged is what the merge answers for page 10 of book 3292, line for line.
//
// It holds the three things the cut has to get right. Lines 4 and 5 split
// `φιλοσοφία` where the printed page breaks it, and the hyphen the box engine
// reported is gone because the text engine never wrote one. Lines 6 and 8 keep
// the superscript markup of the text engine. Lines 16 to 22 are the footnotes and
// the page number, which the markdown of the text engine leaves out and its full
// result does not.
var pageTenMerged = []string{
	"I. LA EXÉGESIS SOCRÁTICO-PLATÓNICA DEL TÍTULO",
	"«FILOSOFÍA». NECESIDAD DE IR MÁS ALLÁ DE ESA",
	"EXÉGESIS",
	"La filosofía se designa a sí misma con esta palabra (φιλοσο",
	"φία, philosophia) desde Platón, más concretamente ya desde el",
	"Platón «socrático». Con la palabra misma $ ^{1} $ está vinculada una",
	"cierta exégesis también platónica (o, si se prefiere, socrático",
	"platónica) $ ^{2} $, que resumimos a continuación:",
	"En la medida en que algo quepa decir de los dioses, de ellos",
	"cabrá decir que son sabios; frente a ellos, el hombre tiene el",
	"privilegio de no-saber. Ahora bien, mientras que las piedras y",
	"los árboles simplemente son lo que son, para el hombre todo",
	"aquello que constituye su propio ser es posibilidad, algo que",
	"puede ser asumido o no serlo, y, así, también el mencionado",
	"no-saber se encuentra en tal situación: el hombre puede ignorar",
	"1 φιλοσοφία es abstraci) de φιλόσοφος, tema adjetival que, a su vez, es",
	"compuesto de φίλος y σοφος. El adjetivo aparece ya en Heráclito, pero segura",
	"mente no con el carácter de autodesignación expresa de la filosofía misma como",
	"tal. Cfr. II.",
	"2 El resumen que se hace a región seguido se apoya fundamentalmente en",
	"elementos de la Apología y del Symposium.",
	"13",
}

func TestPageTenMergesToTheRecordedText(t *testing.T) {
	in := fixtureOf(t, "page0010").input()
	result := Merge(in)

	if len(result.Lines) != len(pageTenMerged) {
		t.Fatalf("page 10 answered %d lines, want %d", len(result.Lines), len(pageTenMerged))
	}
	for i, want := range pageTenMerged {
		if got := result.Lines[i].Text; got != want {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, want)
		}
		if result.Lines[i].Source != in.TextEngine {
			t.Errorf("line %d names %q, want the text engine", i, result.Lines[i].Source)
		}
	}
	if len(result.Dropped) != 0 {
		t.Errorf("page 10 dropped %v, want nothing", result.Dropped)
	}
}
