package guttertrim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

// fixture is one page of a real book: the boxes the endpoint would answer for
// it, the geometry they count in, and the trim this package must find. `want` is
// null for a page that must get none.
//
// Model records which engine the boxes came from. The endpoint prefers the
// merged result and falls back to PP-OCRv6, and the fixtures were taken the same
// way, so both paths are covered.
type fixture struct {
	Name    string
	Book    int    `json:"book"`
	Page    int    `json:"page"`
	Note    string `json:"note"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	CutSide string `json:"cutSide"`
	Model   string `json:"model"`
	Want    *Trim  `json:"want"`
	Lines   []struct {
		Box  ocrmerge.Box `json:"bbox"`
		Text string       `json:"text"`
	} `json:"lines"`
}

func (f fixture) boxes() []ocrmerge.Box {
	boxes := make([]ocrmerge.Box, len(f.Lines))
	for i, line := range f.Lines {
		boxes[i] = line.Box
	}
	return boxes
}

func fixtures(t *testing.T) []fixture {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("testdata holds no fixture")
	}

	loaded := make([]fixture, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		var f fixture
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		f.Name = strings.TrimSuffix(filepath.Base(path), ".json")

		if f.Width <= 0 || f.CutSide == "" || len(f.Lines) == 0 {
			t.Fatalf("%s: the fixture holds no width, side or line", f.Name)
		}
		loaded = append(loaded, f)
	}
	return loaded
}

// The verdict table of design.md, page by page. A change to a constant that
// moves any one of these has to be argued for in the design first.
func TestTheFixturesReproduceTheVerdictTable(t *testing.T) {
	for _, f := range fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			trim, ok := Of(f.boxes(), f.Width, f.CutSide)

			if f.Want == nil {
				if ok {
					t.Fatalf("trimmed at %d against the %s edge, want no trim: %s",
						trim.Column, trim.Side, f.Note)
				}
				return
			}

			if !ok {
				t.Fatalf("no trim, want %d against the %s edge: %s",
					f.Want.Column, f.Want.Side, f.Note)
			}
			if trim != *f.Want {
				t.Errorf("trimmed at %d against the %s edge, want %d against the %s edge",
					trim.Column, trim.Side, f.Want.Column, f.Want.Side)
			}
		})
	}
}

// A trim covers the fragments and nothing else. The count is what the endpoint
// drops, so a fixture that gained a covered body line would fail here before it
// reached a reader.
func TestATrimCoversOnlyTheFragments(t *testing.T) {
	covered := map[string]int{
		"book0017page0029": 2,
		"book0017page0045": 18,
		"book0017page0053": 23,
		"book0019page0051": 20,
	}

	for _, f := range fixtures(t) {
		trim, ok := Of(f.boxes(), f.Width, f.CutSide)
		if !ok {
			continue
		}

		count := 0
		for _, line := range f.Lines {
			if !trim.Covers(line.Box, f.Width) {
				continue
			}
			count++

			near, far := insets(line.Box, f.Width, trim.Side)
			if far-near > int(fragmentShare*float64(f.Width)) {
				t.Errorf("%s: the trim covers a line %d wide holding %q",
					f.Name, far-near, line.Text)
			}
		}

		if want, named := covered[f.Name]; named && count != want {
			t.Errorf("%s: the trim covers %d lines, want %d", f.Name, count, want)
		}
	}
}
