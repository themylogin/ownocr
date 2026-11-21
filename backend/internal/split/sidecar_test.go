package split

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// answering serves one fixed body, so a test states the answer of the sidecar
// and nothing else.
func answering(t *testing.T, status int, body string) (*Client, *[]string) {
	t.Helper()

	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read the request: %v", err)
		}
		if err := json.Unmarshal(raw, &seen); err != nil {
			t.Errorf("the client sent %s, want a list of paths", raw)
		}

		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL), &seen
}

func TestCutsSendsThePathsAndKeepsTheOrder(t *testing.T) {
	client, seen := answering(t, http.StatusOK, `[null, [220, 180], null]`)

	cuts, err := client.Cuts(context.Background(),
		[]string{"/data/one.jpg", "/data/two.jpg", "/data/three.jpg"})
	if err != nil {
		t.Fatalf("Cuts: %v", err)
	}

	want := []string{"/data/one.jpg", "/data/two.jpg", "/data/three.jpg"}
	if len(*seen) != len(want) {
		t.Fatalf("the client sent %d paths, want %d", len(*seen), len(want))
	}
	for index, path := range want {
		if (*seen)[index] != path {
			t.Errorf("path %d = %q, want %q", index, (*seen)[index], path)
		}
	}

	if len(cuts) != 3 {
		t.Fatalf("got %d answers, want 3", len(cuts))
	}
	if cuts[0] != nil || cuts[2] != nil {
		t.Error("a null entry did not become a page that is not a spread")
	}
	if cuts[1] == nil || cuts[1].Left != 220 || cuts[1].Right != 180 {
		t.Errorf("entry 1 = %+v, want the columns 220 and 180", cuts[1])
	}
}

// The sidecar answers nothing for an empty list, so the client saves the call.
func TestCutsSkipsAnEmptyBatch(t *testing.T) {
	client, seen := answering(t, http.StatusOK, `[]`)

	cuts, err := client.Cuts(context.Background(), nil)
	if err != nil {
		t.Fatalf("Cuts: %v", err)
	}
	if len(cuts) != 0 {
		t.Errorf("got %d answers for an empty batch", len(cuts))
	}
	if *seen != nil {
		t.Error("the client called the sidecar for an empty batch")
	}
}

func TestCutsReportsARefusedBatch(t *testing.T) {
	client, _ := answering(t, http.StatusBadRequest,
		`{"detail":"cannot read /data/one.jpg"}`)

	_, err := client.Cuts(context.Background(), []string{"/data/one.jpg"})
	if err == nil {
		t.Fatal("Cuts accepted a refusal")
	}
	if !strings.Contains(err.Error(), "sidecar") {
		t.Errorf("the reason %q does not name the sidecar", err)
	}
	if !strings.Contains(err.Error(), "/data/one.jpg") {
		t.Errorf("the reason %q does not name the page", err)
	}
}

func TestCutsRefusesAnAnswerOfTheWrongLength(t *testing.T) {
	client, _ := answering(t, http.StatusOK, `[null]`)

	_, err := client.Cuts(context.Background(), []string{"/data/one.jpg", "/data/two.jpg"})
	if err == nil {
		t.Fatal("Cuts accepted an answer for one page of two")
	}
	if !strings.Contains(err.Error(), "1 pages of 2") {
		t.Errorf("the reason %q does not name the counts", err)
	}
}

func TestCutsRefusesAnAnswerItCannotRead(t *testing.T) {
	client, _ := answering(t, http.StatusOK, `not json`)

	if _, err := client.Cuts(context.Background(), []string{"/data/one.jpg"}); err == nil {
		t.Fatal("Cuts accepted an answer that is not a list")
	}
}

func TestCutsRefusesABadEntry(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"one column", `[[220]]`, "1 columns"},
		{"three columns", `[[220, 180, 200]]`, "3 columns"},
		// The first column is the end of the left page, the second the start of
		// the right one. The halves overlap, so the first is never the smaller.
		{"no overlap", `[[180, 220]]`, "do not overlap"},
		{"a column below zero", `[[220, -1]]`, "the column -1"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client, _ := answering(t, http.StatusOK, test.body)

			_, err := client.Cuts(context.Background(), []string{"/data/one.jpg"})
			if err == nil {
				t.Fatalf("Cuts accepted %s", test.body)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("the reason %q does not name %q", err, test.want)
			}
		})
	}
}

func TestCutsReportsASidecarThatDoesNotAnswer(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")

	_, err := client.Cuts(context.Background(), []string{"/data/one.jpg"})
	if err == nil {
		t.Fatal("Cuts accepted a sidecar that is not there")
	}
	if !strings.Contains(err.Error(), "the sidecar did not answer") {
		t.Errorf("the reason %q does not name the sidecar", err)
	}
}
