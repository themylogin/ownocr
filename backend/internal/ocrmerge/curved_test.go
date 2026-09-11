package ocrmerge

import "testing"

// pageFourThirtyTwoMerged is what the merge answers for page 432 of book 4015,
// line for line.
//
// The lines of this page fall by most of a row across the column. PP-OCRv6 broke
// seven rows of the first block into a left box and a right box, and read neither
// half of eight more. Every one of those eight comes back here, and so does the
// text of the four half rows whose other half the detector missed.
//
// Line 16 is what the filling costs. The printed page breaks `podemos` as `pode-`
// and `mos`, the text engine wrote the word whole, and the filler never breaks a
// word, so the whole of it stands one line lower than the printer put it.
//
// Lines 47 and 48 read `no importa qu` and `ase «sentido último»`. That is what
// PaddleOCR-VL wrote, and the merge changes no engine's text.
var pageFourThirtyTwoMerged = []struct {
	text   string
	filled bool
}{
	{"tematización descriptiva (por ejemplo: el estudio experimental de las", false},
	{"«propiedades químicas», la taxonomía vegetal y animal, etc.)", false},
	{"tenga una", false},
	{"entidad muy considerable y unas exigencias técnicas propias. Así, por", true},
	{"ejemplo, «la ciencia» tiene más o menos implícita (pero", false},
	{"tiene en todo", false},
	{"caso) la seguridad de que todo lo que constituye la vida", false},
	{"de un animal", false},
	{"sería en último análisis una maraña de procesos", false},
	{"físico-químicos, y de", false},
	{"que los procesos químicos serían físico-matemáticamente explicables en", true},
	{"cuanto se llegase a un refinamiento suficiente del análisis (como de", true},
	{"hecho ha ocurrido en cuanto los procesos químicos han podido ser", true},
	{"tratados a escala atómica, bien que ello ha provocado una modificación", true},
	{"en las estructuras de la propia física matemática). Por todo esto,", true},
	{"podemos adoptar la palabra «Física» como denominación de la ciencia de", true},
	{"la naturaleza en su conjunto; la constitución esencial de esa ciencia", true},
	{"(aquello que es a la vez su norma, su estructura y su lenguaje) es la", false},
	{"matemática.", false},
	{"La «sociología» de Comte sigue siendo una especie de «física» (cuyo", false},
	{"carácter de tal es concebido de un modo más o menos confuso). Si una", false},
	{"consideración positivista de la historia es posible, la ausencia de tal", false},
	{"consideración en Comte se hace patente en el hecho de que, cuando", false},
	{"Comte habla de algo así como la historia, emplea nociones que de", false},
	{"«positivismo» no tienen otra cosa que la banalidad que sin duda tienen:", false},
	{"el «progreso», el «perfeccionamiento» de la humanidad y «creciente", false},
	{"predominio de las tendencias más nobles de nuestra naturaleza», etc.,", false},
	{"todo ello cosas que no son en absoluto de naturaleza «positiva» y que", false},
	{"son simplemente producto de una metafísica barata.", false},
	{"Sin embargo, a mediados del siglo XIX, y precisamente en Alemania,", false},
	{"el ideal positivista penetra en campos que, al menos en principio, no", false},
	{"son el de «la naturaleza». Es por entonces cuando G. Th. Fechner", false},
	{"funda la psicología experimental; pero lo que Fechner fundamental", false},
	{"mente hace es establecer, sobre la base de ciertos convenios, una ma", false},
	{"nera coherente de atribuir cantidad a las sensaciones; por lo tanto,", false},
	{"su obra se sitúa en el camino de la reducción de toda ciencia al modo", false},
	{"de objetividad regido por la matemática. Mucha más importancia", false},
	{"(desde el punto de vista de lo que ahora nos ocupa) tiene la aparición", false},
	{"de una especie de positivismo en la investigación histórica; es la lla", false},
	{"mada «escuela histórica» (citemos a Leopold von Ranke), cuya at", false},
	{"mosfera respira Dilthey.", false},
	{"El propio Dilthey pertenece en cierta manera al ámbito del positi", false},
	{"vismo; más exactamente: pertenece sólo en la medida en que se siente", false},
	{"liberado, gracias al positivismo, de las cadenas metafísicas que hasta", false},
	{"entonces hacían imposible una verdadera ciencia histórica; así:", false},
	{"1. Dilthey rechaza toda «metafísica de la historia», entendiendo por", false},
	{"tal toda noción de un «sentido último» de la historia, no importa qu", false},
	{"ase «sentido último» tenga el carácter de un «fin» o el de un proceso", false},
	{"subyacente o el de cualquier otra cosa.", false},
	{"2. Dilthey rechaza la aplicación a la historia de nociones suprahis", false},
	{"tóricas. Si se admite, por ejemplo, «la naturaleza humana» como algo", false},
	{"que tiene que ser común al hombre del siglo XIX, al griego arcaico y al", false},
	{"432", false},
}

func TestPageFourThirtyTwoReadsEveryRowOfACurvedPage(t *testing.T) {
	result := Merge(fixtureOf(t, "page0432").input())

	if len(result.Lines) != len(pageFourThirtyTwoMerged) {
		t.Fatalf("page 432 answered %d lines, want %d",
			len(result.Lines), len(pageFourThirtyTwoMerged))
	}
	for i, want := range pageFourThirtyTwoMerged {
		if got := result.Lines[i].Text; got != want.text {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, want.text)
		}
		if got := result.Lines[i].Estimated; got != want.filled {
			t.Errorf("line %d reports filled=%v, want %v", i, got, want.filled)
		}
	}
}

// Before the rows existed the merge dropped 374 runes of this page, which is
// eight printed rows the text engine read and no box carried.
func TestPageFourThirtyTwoDropsNothing(t *testing.T) {
	result := Merge(fixtureOf(t, "page0432").input())

	if result.Counts.DroppedChars != 0 {
		t.Errorf("dropped %d runes, want none: %v",
			result.Counts.DroppedChars, result.Dropped)
	}
	if result.Counts.Estimated != 8 {
		t.Errorf("filled %d lines, want the 8 rows v6 never read",
			result.Counts.Estimated)
	}
	if result.Counts.FromBoxes != 0 {
		t.Errorf("%d lines fell back to the box engine, want none",
			result.Counts.FromBoxes)
	}
}

// The halves of a broken row must keep their own text. A tail box that swallowed
// the row under it would draw that row over the row above it.
func TestPageFourThirtyTwoLeavesEachHalfRowItsOwnText(t *testing.T) {
	result := Merge(fixtureOf(t, "page0432").input())

	halves := map[Box]string{
		{1411, 298, 1608, 381}: "tenga una",
		{1331, 380, 1607, 469}: "tiene en todo",
		{1336, 427, 1607, 510}: "de un animal",
		{1186, 456, 1609, 558}: "físico-químicos, y de",
	}

	found := 0
	for _, line := range result.Lines {
		want, held := halves[line.Box]
		if !held {
			continue
		}
		found++
		if line.Text != want {
			t.Errorf("the box %v reads %q, want %q", line.Box, line.Text, want)
		}
	}
	if found != len(halves) {
		t.Errorf("matched %d of %d half rows", found, len(halves))
	}
}
