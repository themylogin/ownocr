package runpod

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/themylogin/ownocr/backend/internal/books"
)

type jobInput struct {
	Bucket       string `json:"bucket"`
	InputPrefix  string `json:"input_prefix"`
	OutputPrefix string `json:"output_prefix"`
}

type jobOutput struct {
	Pages []string `json:"pages"`
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewClient("dewarp", server.URL, "token")
	client.PollInterval = time.Millisecond
	return client
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func decode(t *testing.T, raw json.RawMessage) jobOutput {
	t.Helper()

	var output jobOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return output
}

func TestRunSendsTheJobInput(t *testing.T) {
	var seen jobInput

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/run" || r.Method != http.MethodPost {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}

		var body struct {
			Input jobInput `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		seen = body.Input

		writeJSON(w, map[string]any{"id": "job-7", "status": "IN_QUEUE"})
	}))

	id, err := client.Run(context.Background(), jobInput{
		Bucket:       "ownocr",
		InputPrefix:  "dewarp/abc/in/",
		OutputPrefix: "dewarp/abc/out/",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if id != "job-7" {
		t.Errorf("id = %q, want job-7", id)
	}
	if seen.Bucket != "ownocr" || seen.InputPrefix != "dewarp/abc/in/" ||
		seen.OutputPrefix != "dewarp/abc/out/" {
		t.Errorf("the endpoint received %+v", seen)
	}
}

func TestRunWithoutAJobID(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"status": "IN_QUEUE"})
	}))

	if _, err := client.Run(context.Background(), jobInput{}); err == nil {
		t.Fatal("a response without an id was accepted")
	}
}

func TestWaitPollsUntilTheJobEnds(t *testing.T) {
	polls := 0

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		polls++
		if polls < 3 {
			writeJSON(w, map[string]any{"id": "job-7", "status": "IN_PROGRESS"})
			return
		}
		writeJSON(w, map[string]any{"id": "job-7", "status": "COMPLETED",
			"output": jobOutput{Pages: []string{"out/0001.jpg"}}})
	}))

	raw, err := client.Wait(context.Background(), "job-7", testLogger(), nil)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if polls != 3 {
		t.Errorf("polled %d times, want 3", polls)
	}

	output := decode(t, raw)
	if len(output.Pages) != 1 || output.Pages[0] != "out/0001.jpg" {
		t.Errorf("output = %+v", output)
	}
}

func TestWaitReportsAFinalFailure(t *testing.T) {
	for _, status := range []string{"FAILED", "CANCELLED", "TIMED_OUT"} {
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"id": "job-7", "status": status, "error": "out of memory"})
		}))

		_, err := client.Wait(context.Background(), "job-7", testLogger(), nil)
		if err == nil {
			t.Fatalf("%s was treated as success", status)
		}
		if !strings.Contains(err.Error(), status) || !strings.Contains(err.Error(), "out of memory") {
			t.Errorf("%s gave %q", status, err)
		}
	}
}

func TestWaitNamesTheEndpoint(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"id": "job-7", "status": "FAILED", "error": "the pod died"})
	}))
	client.name = "OCR"

	_, err := client.Wait(context.Background(), "job-7", testLogger(), nil)
	if err == nil || !strings.Contains(err.Error(), "the OCR endpoint") {
		t.Fatalf("error %v does not name the endpoint", err)
	}
}

func TestWaitNeedsAResult(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"id": "job-7", "status": "COMPLETED"})
	}))

	if _, err := client.Wait(context.Background(), "job-7", testLogger(), nil); err == nil {
		t.Fatal("success without a result was accepted")
	}
}

func TestWaitToleratesABlipThenGivesUp(t *testing.T) {
	polls := 0

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		polls++
		if polls == 1 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{"id": "job-7", "status": "COMPLETED", "output": jobOutput{}})
	}))

	if _, err := client.Wait(context.Background(), "job-7", testLogger(), nil); err != nil {
		t.Fatalf("one failed poll ended the job: %v", err)
	}

	failing := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))

	if _, err := failing.Wait(context.Background(), "job-7", testLogger(), nil); err == nil {
		t.Fatal("an endpoint that never answers was tolerated forever")
	}
}

func TestCallNamesAnUnreachableEndpoint(t *testing.T) {
	client := NewClient("dewarp", "http://127.0.0.1:1", "token")
	client.http.Timeout = 500 * time.Millisecond

	_, err := client.Run(context.Background(), jobInput{})
	if err == nil {
		t.Fatal("an unreachable endpoint returned no error")
	}
	if !strings.Contains(err.Error(), "http://127.0.0.1:1") {
		t.Errorf("error %q does not name the endpoint", err)
	}
}

func TestWaitReportsEveryStatusChange(t *testing.T) {
	statuses := []string{"IN_QUEUE", "IN_QUEUE", "IN_PROGRESS", "IN_PROGRESS", "COMPLETED"}
	polls := 0

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		status := statuses[polls]
		polls++
		if status == "COMPLETED" {
			writeJSON(w, map[string]any{"id": "job-7", "status": status,
				"output": jobOutput{Pages: []string{"out/0001.jpg"}}})
			return
		}
		writeJSON(w, map[string]any{"id": "job-7", "status": status})
	}))

	var seen []string
	if _, err := client.Wait(context.Background(), "job-7", testLogger(), func(status string) {
		seen = append(seen, status)
	}); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	want := []string{"IN_QUEUE", "IN_PROGRESS", "COMPLETED"}
	if len(seen) != len(want) {
		t.Fatalf("callback saw %v, want %v", seen, want)
	}
	for index, status := range want {
		if seen[index] != status {
			t.Fatalf("callback saw %v, want %v", seen, want)
		}
	}
}

func TestPhaseOfStatus(t *testing.T) {
	cases := map[string]string{
		"IN_QUEUE":    books.PhaseWaitingAI,
		"IN_PROGRESS": books.PhaseProcessing,
	}
	for status, want := range cases {
		phase, ok := PhaseOfStatus(status)
		if !ok || phase != want {
			t.Errorf("PhaseOfStatus(%q) = %q, %v, want %q", status, phase, ok, want)
		}
	}

	if _, ok := PhaseOfStatus("COMPLETED"); ok {
		t.Error("COMPLETED became a phase")
	}
}
