package ocr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

// TestMergeOfDecodesWhatTheEnginesWrite reads the stored result of page 10 of book
// 3292, exactly as the endpoint wrote it, and checks that both shapes decode.
func TestMergeOfDecodesWhatTheEnginesWrite(t *testing.T) {
	boxData, textData := fixturePageTen(t)

	merged, err := MergeOf(boxData, textData)
	if err != nil {
		t.Fatalf("mergeOf: %v", err)
	}

	if len(merged.Lines) != 22 {
		t.Fatalf("the merge answered %d lines, want 22 from the v6 result", len(merged.Lines))
	}
	if merged.Lines[0].Box != (ocrmerge.Box{164, 926, 1359, 972}) {
		t.Errorf("line 0 carries the box %v, want the box of the v6 line", merged.Lines[0].Box)
	}
	if merged.Lines[0].Score == 0 {
		t.Error("line 0 carries no score, want the confidence of the v6 line")
	}
	if merged.Counts.FromText != 22 {
		t.Errorf("%d lines took vl text, want all 22", merged.Counts.FromText)
	}

	// The Greek the box engine wrote with Latin lookalikes is the whole point.
	if !strings.Contains(MergedText(merged), "φιλοσοφία") {
		t.Error("the merged text does not hold the Greek the vl result read")
	}
}

// The quads live beside the lines in what the engine writes, so the merge has to
// pair them by index. Without them it cannot measure how far a line slopes, and a
// block whose boxes name the wrong printed rows looks like any other block.
func TestMergeOfReadsTheCornersTheEngineFound(t *testing.T) {
	boxData, _ := fixturePageTen(t)

	var stored struct {
		Raw struct {
			Polys [][][]float64 `json:"dt_polys"`
		} `json:"raw"`
		Lines []json.RawMessage `json:"lines"`
	}
	if err := json.Unmarshal(boxData, &stored); err != nil {
		t.Fatalf("the v6 fixture: %v", err)
	}
	if len(stored.Raw.Polys) != len(stored.Lines) {
		t.Fatalf("the fixture holds %d quads for %d lines, so it cannot pair them",
			len(stored.Raw.Polys), len(stored.Lines))
	}

	var boxes boxResult
	if err := json.Unmarshal(boxData, &boxes); err != nil {
		t.Fatalf("the v6 fixture: %v", err)
	}
	lines := ocrmerge.WithCorners(boxes.Lines, boxes.Raw.Polys)

	for i, poly := range stored.Raw.Polys {
		if len(lines[i].Corners) != 4 {
			t.Fatalf("line %d carries %d corners, want 4", i, len(lines[i].Corners))
		}
		for k, point := range poly {
			want := ocrmerge.Point{int(point[0] + 0.5), int(point[1] + 0.5)}
			if got := lines[i].Corners[k]; got != want {
				t.Errorf("line %d corner %d is %v, want %v", i, k, got, want)
			}
		}
	}
}

// A result stored before the merge read quads holds none, and a result whose two
// lists disagree in length cannot be paired by index at all. Both leave every
// line level, so such a page merges as it did before quads existed.
func TestWithCornersLeavesALineAloneWhenItCannotPairThem(t *testing.T) {
	lines := []ocrmerge.Line{{Box: ocrmerge.Box{0, 0, 10, 10}}, {Box: ocrmerge.Box{0, 10, 10, 20}}}

	if got := ocrmerge.WithCorners(lines, nil); got[0].Corners != nil {
		t.Error("a result with no quads gave a line corners")
	}

	short := [][][]float64{{{0, 0}, {10, 0}, {10, 10}, {0, 10}}}
	if got := ocrmerge.WithCorners(lines, short); got[0].Corners != nil {
		t.Error("one quad for two lines was paired by index anyway")
	}

	wrong := [][][]float64{
		{{0, 0}, {10, 0}, {10, 10}},
		{{0, 10}, {10, 10}, {10, 20}, {0}},
	}
	got := ocrmerge.WithCorners(lines, wrong)
	if got[0].Corners != nil {
		t.Error("a quad of three points became corners")
	}
	if got[1].Corners != nil {
		t.Error("a quad holding a point of one coordinate became corners")
	}
}

// A v6 result that carries no quads still merges, and merges to the same lines.
func TestMergeOfWithoutCornersMergesTheSamePage(t *testing.T) {
	boxData, textData := fixturePageTen(t)

	var trimmed map[string]json.RawMessage
	if err := json.Unmarshal(boxData, &trimmed); err != nil {
		t.Fatalf("the v6 fixture: %v", err)
	}
	delete(trimmed, "raw")
	cornerless, err := json.Marshal(trimmed)
	if err != nil {
		t.Fatal(err)
	}

	with, err := MergeOf(boxData, textData)
	if err != nil {
		t.Fatalf("with corners: %v", err)
	}
	without, err := MergeOf(cornerless, textData)
	if err != nil {
		t.Fatalf("without corners: %v", err)
	}

	if MergedText(with) != MergedText(without) {
		t.Error("the page reads differently with and without corners")
	}
	if with.Counts != without.Counts {
		t.Errorf("counts %+v with corners against %+v without", with.Counts, without.Counts)
	}
	if with.Counts.Discarded != 0 {
		t.Errorf("page 10 discarded %d boxes, want none", with.Counts.Discarded)
	}
}

func TestMergedTextIsOneLinePerLine(t *testing.T) {
	boxData, textData := fixturePageTen(t)

	merged, err := MergeOf(boxData, textData)
	if err != nil {
		t.Fatalf("mergeOf: %v", err)
	}

	text := MergedText(merged)
	if got := len(strings.Split(text, "\n")); got != len(merged.Lines) {
		t.Errorf("the text holds %d lines, want %d", got, len(merged.Lines))
	}
	if strings.HasSuffix(text, "\n") {
		t.Error("the text ends with a newline, want one line per line and no more")
	}
}

// The text engine writes a footnote marker as LaTeX, and the reader reads the
// merged result. Page 10 carries two of them.
func TestMergeOfCleansTheMarkupOfTheTextEngine(t *testing.T) {
	boxData, textData := fixturePageTen(t)

	merged, err := MergeOf(boxData, textData)
	if err != nil {
		t.Fatalf("mergeOf: %v", err)
	}

	for i, line := range merged.Lines {
		if strings.Contains(line.Text, "$") {
			t.Errorf("line %d still holds the markup: %q", i, line.Text)
		}
	}

	const want = "Platón «socrático». Con la palabra misma¹ está vinculada una"
	if got := merged.Lines[5].Text; got != want {
		t.Errorf("line 5\n got %q\nwant %q", got, want)
	}
	if got := merged.Lines[7].Text; !strings.Contains(got, "platónica)²,") {
		t.Errorf("line 7 reads %q, want the second marker converted", got)
	}
}

// The merged result is derived. The system writes it again from the two engine
// results alone, so the merge must leave both exactly as it read them.
func TestMergeOfChangesNeitherEngineResult(t *testing.T) {
	boxData, textData := fixturePageTen(t)

	boxBefore := string(boxData)
	textBefore := string(textData)

	if _, err := MergeOf(boxData, textData); err != nil {
		t.Fatalf("mergeOf: %v", err)
	}

	if string(boxData) != boxBefore {
		t.Error("the merge changed the v6 result it read")
	}
	if string(textData) != textBefore {
		t.Error("the merge changed the vl result it read")
	}
	if !strings.Contains(textBefore, `$ ^{1} $`) {
		t.Error("the fixture holds no markup, so this test proves nothing")
	}
}

// pageFourteenMerged is what MergeOf answers for page 14 of book 3292, line for
// line, with the cleanup in the path.
//
// The page is a Greek quotation PP-OCRv6 destroyed, and the text engine wrote no
// dollar sign on it. It is the guard that the cleanup leaves a Greek page alone
// when it holds no markup to convert.
//
// Two of its lines end in `^{7}` and `^{8}`: a footnote marker the engine wrote
// with no dollar signs around it. The cleanup converts the dollar-wrapped shape
// alone, so the bare shape survives here. See design.md — Open Questions.
var pageFourteenMerged = []string{
	"y no de que haya en general cosas reconocibles como bellas,",
	"sino de que la belleza se manifieste como tal únicamente a su",
	"propia luz.",
	"II. HERÁCLITO: τὸ σοφόν Υ φιλόσοφοι ἄνδρες",
	"En todo lo hasta aquí visto, σοφοός aparece como un adjetivo",
	"y σοφία como el abstracto de ese adjetivo, designando ambos",
	"una cualidad posible del hombre. Ello en el sentido siguiente: es",
	"del hombre de quien se discute si él es o no es σοφοός; la refe",
	"rencia que hemos registrado a «los dioses» no hace más que",
	"confirmar esto, pues, a fin de cuentas, da por supuesto que un",
	"dios no es algo de lo que se discuta si es σοφοός o no.",
	"Hay, sin embargo, algunos textos griegos arcaicos, impor",
	"tantes en la historia de la filosofía, que no se conforman con ese",
	"esquema.",
	"Así, por una parte, Heráclito, B 32:",
	"ἐν τὸ σοφὸν μοῦνον λέγεσθαι οὐκ ἐθέλει καὶ ἐθέλει Ζηνὸς",
	"ὁνομα^{7}.",
	"Aquí τὸ σοφόν no es ni dios ni hombre alguno; la palabra",
	"debe designar algo uno, único y singular; e incluso la referencia",
	"a Zeus se hace de manera que excluye tajantemente que σοφός",
	"designe una cualidad o atributo del dios; no se dice que Zeus sea",
	"ό σοφός, sino que τὸ σοφόν [nótese la diferencia morfológica^{8}]",
	"puede en cierta manera ser «dicho» en cuanto que es dicho el",
	"nombre de Zeus, si bien, por otra parte, no puede τὸ σοφόν ser",
	"dicho así.",
	"Igualmente, Heráclito, B 108:",
	"7 Diels, H.: Die Fragmente der Vorsokratiker, 6. Auflage, hrsg. von W.",
	"Kranz, 1951 (y reimpresiones posteriores de la misma edición, en adelante",
	"citado simplemente como «Diels-Kranz»), Herakl. B 32 (t. I, pág. 159).",
	"8 El llamado «neutro singular» es la forma que se emplea para el uso abso",
	"luto (sin vinculación atributiva ni predicativa) del adjetivo.",
	"17",
}

func TestMergeOfReadsPageFourteenAsRecorded(t *testing.T) {
	boxData, textData := fixturePage(t, "page0014")

	merged, err := MergeOf(boxData, textData)
	if err != nil {
		t.Fatalf("mergeOf: %v", err)
	}

	if len(merged.Lines) != len(pageFourteenMerged) {
		t.Fatalf("page 14 answered %d lines, want %d",
			len(merged.Lines), len(pageFourteenMerged))
	}
	for i, want := range pageFourteenMerged {
		if got := merged.Lines[i].Text; got != want {
			t.Errorf("line %d:\n got %q\nwant %q", i, got, want)
		}
	}
}

func TestMergeOfRefusesAnUnreadableResult(t *testing.T) {
	if _, err := MergeOf([]byte("not json"), []byte(dataOf(ModeVL))); err == nil {
		t.Error("an unreadable v6 result was accepted")
	} else if !strings.Contains(err.Error(), ModeV6) {
		t.Errorf("error %q does not name the engine", err)
	}

	if _, err := MergeOf([]byte(dataOf(ModeV6)), []byte("not json")); err == nil {
		t.Error("an unreadable vl result was accepted")
	} else if !strings.Contains(err.Error(), ModeVL) {
		t.Errorf("error %q does not name the engine", err)
	}
}

func TestMergeOfKeepsEveryLineWhenTheTextEngineFoundNoBlock(t *testing.T) {
	merged, err := MergeOf([]byte(dataOf(ModeV6)), []byte(dataWithoutBlocks))
	if err != nil {
		t.Fatalf("mergeOf: %v", err)
	}

	if merged.Counts.FromBoxes != len(merged.Lines) {
		t.Errorf("counts are %+v, want every line from the v6 result", merged.Counts)
	}
	for i, line := range merged.Lines {
		if line.Text == "" {
			t.Errorf("line %d came out empty", i)
		}
		if line.Source != ModeV6 {
			t.Errorf("line %d names %q, want %q", i, line.Source, ModeV6)
		}
	}
}

// The extra profile runs both engines, so every page gains a third result derived
// from the two.
func TestWorkerWritesTheMergedResult(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 2)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	results := h.results(t, copyBook)
	for index := 1; index <= 2; index++ {
		if len(results[index]) != 3 {
			t.Fatalf("page %d holds %d results, want three", index, len(results[index]))
		}

		row, held := results[index][ModelMerged]
		if !held {
			t.Fatalf("page %d holds no %s result", index, ModelMerged)
		}

		var merged ocrmerge.Result
		if err := json.Unmarshal(row.Data, &merged); err != nil {
			t.Fatalf("page %d: the merged result does not decode: %v", index, err)
		}
		if len(merged.Lines) != 2 {
			t.Fatalf("page %d: the merged result holds %d lines, want 2",
				index, len(merged.Lines))
		}
		if merged.Lines[0].Source != ModeVL {
			t.Errorf("page %d: line 0 names %q, want the vl text",
				index, merged.Lines[0].Source)
		}
		if merged.Lines[0].Box != (ocrmerge.Box{100, 100, 900, 140}) {
			t.Errorf("page %d: line 0 carries %v, want the v6 box",
				index, merged.Lines[0].Box)
		}
		if row.Text != MergedText(merged) {
			t.Errorf("page %d: the stored text is not the merged lines", index)
		}

		// The engine results stay exactly what the engines wrote.
		if results[index][ModeV6].Text != textOf(InputKey(copyBook.StorageKey, index, "jpg"), ModeV6) {
			t.Errorf("page %d: the v6 text changed", index)
		}
		if results[index][ModeVL].Text != textOf(InputKey(copyBook.StorageKey, index, "jpg"), ModeVL) {
			t.Errorf("page %d: the vl text changed", index)
		}
	}
}

// The normal profile runs one engine, which leaves nothing to join.
func TestWorkerWritesNoMergedResultForOneEngine(t *testing.T) {
	h := newHarness(t)
	h.profile = ProfileNormal
	_, copyBook := h.seed(t, 2)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	results := h.results(t, copyBook)
	for index := 1; index <= 2; index++ {
		if len(results[index]) != 1 {
			t.Fatalf("page %d holds %d results, want one", index, len(results[index]))
		}
		if _, held := results[index][ModelMerged]; held {
			t.Errorf("page %d holds a %s result, want none", index, ModelMerged)
		}
	}
}

// A retry rebuilds the pages, so the merged result of the earlier attempt cannot
// survive beside the new one.
func TestWorkerRetryLeavesOneMergedResult(t *testing.T) {
	h := newHarness(t)
	_, copyBook := h.seed(t, 2)

	for attempt := 1; attempt <= 2; attempt++ {
		if err := h.run(copyBook.ID, attempt, 3); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
	}

	results := h.results(t, copyBook)
	if len(results) != 2 {
		t.Fatalf("the copy holds %d pages, want 2", len(results))
	}
	for index := 1; index <= 2; index++ {
		if len(results[index]) != 3 {
			t.Errorf("page %d holds %d results, want three", index, len(results[index]))
		}
	}
}

// A page the text engine read as empty still merges, and keeps every line the box
// engine read.
func TestWorkerMergesAPageTheTextEngineReadAsEmpty(t *testing.T) {
	h := newHarness(t)
	h.endpoint.blocklessPage = "0002"
	_, copyBook := h.seed(t, 2)

	if err := h.run(copyBook.ID, 1, 3); err != nil {
		t.Fatalf("Work: %v", err)
	}

	results := h.results(t, copyBook)
	row, held := results[2][ModelMerged]
	if !held {
		t.Fatal("page 2 holds no merged result")
	}

	var merged ocrmerge.Result
	if err := json.Unmarshal(row.Data, &merged); err != nil {
		t.Fatalf("the merged result does not decode: %v", err)
	}
	if merged.Counts.FromBoxes != len(merged.Lines) {
		t.Errorf("counts are %+v, want every line from the v6 result", merged.Counts)
	}
	for i, line := range merged.Lines {
		if line.Text == "" {
			t.Errorf("line %d came out empty", i)
		}
	}
}

// fixturePageTen answers the two engine results of page 10 of book 3292, as the
// ocrmerge fixtures hold them.
func fixturePageTen(t *testing.T) (boxData, textData []byte) {
	t.Helper()
	return fixturePage(t, "page0010")
}

func fixturePage(t *testing.T, name string) (boxData, textData []byte) {
	t.Helper()

	path := filepath.Join("..", "ocrmerge", "testdata", name+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	var fixture struct {
		V6 json.RawMessage `json:"v6"`
		VL json.RawMessage `json:"vl"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return fixture.V6, fixture.VL
}
