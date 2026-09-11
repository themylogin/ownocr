package ocrmerge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is one page of a real book, holding the result of both engines exactly
// as `page_ocr.data` holds it.
//
// Name is the file it came from, and it is what a test asks for. Two books can
// both have a page 116, and both are here.
type fixture struct {
	Name string
	Book int    `json:"book"`
	Page int    `json:"page"`
	Note string `json:"note"`
	V6   struct {
		Lines []Line `json:"lines"`
	} `json:"v6"`
	VL struct {
		Blocks []struct {
			Box   Box    `json:"block_bbox"`
			Label string `json:"block_label"`
			Text  string `json:"block_content"`
		} `json:"parsing_res_list"`
	} `json:"vl"`
}

// What each fixture page is there to prove.
var fixturePurpose = map[string]string{
	"page0010":         "clean page, a hyphenated line break, a footnote, Greek v6 wrote with Latin lookalikes",
	"page0011":         "margin lines outside every block, lines out of reading order, two lines on one row",
	"page0014":         "a Greek quotation v6 destroyed",
	"page0052":         "v6 missed five lines VL read, above and inside a block and in a footnote",
	"page0116":         "the text engine repeats itself",
	"page0119":         "the text engine answers too little",
	"page0432":         "lines that fall across the column, broken in half by v6, with eight rows missing",
	"book0017page0116": "v6 read two footnote rows as noise, and the length gate refused the reading that is right",
}

func (f fixture) input() Input {
	blocks := make([]Block, len(f.VL.Blocks))
	for i, b := range f.VL.Blocks {
		blocks[i] = Block{Box: b.Box, Text: b.Text}
	}
	return Input{Lines: f.V6.Lines, Blocks: blocks, BoxEngine: "v6", TextEngine: "vl"}
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()

	paths, err := filepath.Glob("testdata/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures in testdata: %v", err)
	}

	loaded := make([]fixture, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		f.Name = strings.TrimSuffix(filepath.Base(path), ".json")
		loaded = append(loaded, f)
	}
	return loaded
}

func TestFixturesParse(t *testing.T) {
	fixtures := loadFixtures(t)
	if len(fixtures) != len(fixturePurpose) {
		t.Fatalf("got %d fixtures, want %d", len(fixtures), len(fixturePurpose))
	}

	for _, f := range fixtures {
		if _, named := fixturePurpose[f.Name]; !named {
			t.Errorf("%s has no recorded purpose", f.Name)
		}
		if f.Note == "" {
			t.Errorf("page %d carries no note", f.Page)
		}
		if len(f.V6.Lines) == 0 {
			t.Errorf("page %d decoded no v6 line", f.Page)
		}
	}
}

func TestFixturePageTenShape(t *testing.T) {
	f := fixtureOf(t, "page0010")

	if got := len(f.V6.Lines); got != 22 {
		t.Errorf("page 10 has %d v6 lines, want 22", got)
	}
	if got := len(f.VL.Blocks); got != 6 {
		t.Errorf("page 10 has %d vl blocks, want 6", got)
	}
}

func fixtureOf(t *testing.T, name string) fixture {
	t.Helper()
	for _, f := range loadFixtures(t) {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no fixture named %s", name)
	return fixture{}
}
