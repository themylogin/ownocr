package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadPageAnswersTheTranscription(t *testing.T) {
	var seen struct {
		key     string
		version string
		body    request
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.key = r.Header.Get("x-api-key")
		seen.version = r.Header.Get("anthropic-version")
		payload, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(payload, &seen.body); err != nil {
			t.Errorf("the request body is unreadable: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"content": [{"type": "text", "text": "  la palabra ἀρχή  "}],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 4784, "output_tokens": 612}}`)
	}))
	defer server.Close()

	client := NewClient("Claude", server.URL, "secret-key")

	text, usage, err := client.ReadPage(context.Background(), []byte("jpeg bytes"), "image/jpeg")
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}

	if text != "la palabra ἀρχή" {
		t.Errorf("the page reads %q", text)
	}
	if usage.InputTokens != 4784 || usage.OutputTokens != 612 {
		t.Errorf("usage = %+v", usage)
	}

	if seen.key != "secret-key" {
		t.Errorf("the key reached the API as %q", seen.key)
	}
	if seen.version != version {
		t.Errorf("the request names the API version %q, want %q", seen.version, version)
	}
	if seen.body.Model != Model {
		t.Errorf("the request names the model %q, want %q", seen.body.Model, Model)
	}

	if len(seen.body.Messages) != 1 || len(seen.body.Messages[0].Content) != 2 {
		t.Fatalf("the request holds %+v", seen.body.Messages)
	}
	content := seen.body.Messages[0].Content

	// The image stands before the question, which is what a vision request asks
	// for.
	if content[0].Type != "image" || content[0].Source == nil {
		t.Fatalf("the first block is %+v", content[0])
	}
	if content[0].Source.MediaType != "image/jpeg" {
		t.Errorf("the image is sent as %q", content[0].Source.MediaType)
	}
	sent, err := base64.StdEncoding.DecodeString(content[0].Source.Data)
	if err != nil || string(sent) != "jpeg bytes" {
		t.Errorf("the image reached the API as %q (%v)", sent, err)
	}

	if content[1].Type != "text" || !strings.Contains(content[1].Text, "Transcribe this page") {
		t.Errorf("the second block is %+v", content[1])
	}
}

// The prompt has to ask for the page and nothing else, because a model that
// describes the page instead of reading it spoils the reading of that page.
func TestThePromptAsksForTheTranscriptionAlone(t *testing.T) {
	for _, want := range []string{
		"Output only the transcription",
		"polytonic Greek",
		"output nothing at all",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not hold %q", want)
		}
	}
}

func TestReadPageNamesAnAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error": {"type": "rate_limit_error", "message": "slow down"}}`)
	}))
	defer server.Close()

	client := NewClient("Claude", server.URL, "secret-key")

	_, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if err == nil {
		t.Fatal("an API error was accepted")
	}
	for _, want := range []string{"Claude", "429", "slow down"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	// An API error is a failure of the request, not a page the model would not
	// read, so the job may retry it.
	if errors.Is(err, ErrNotRead) {
		t.Error("an API error reads as a page the model would not read")
	}
}

func TestReadPageReportsARefusalAsAPageNotRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"content": [], "stop_reason": "refusal",
			"usage": {"input_tokens": 4784, "output_tokens": 0}}`)
	}))
	defer server.Close()

	client := NewClient("Claude", server.URL, "secret-key")

	_, usage, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if !errors.Is(err, ErrNotRead) {
		t.Fatalf("a refusal answered %v, want ErrNotRead", err)
	}
	if usage.InputTokens != 4784 {
		t.Errorf("a refusal lost its usage: %+v", usage)
	}
}

// A transcription cut off mid-page would read as a page the engine abandoned, so
// it counts as a page not read rather than a short reading.
func TestReadPageRefusesATruncatedTranscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"content": [{"type": "text", "text": "the page begins"}],
			"stop_reason": "max_tokens", "usage": {"output_tokens": 8192}}`)
	}))
	defer server.Close()

	client := NewClient("Claude", server.URL, "secret-key")

	text, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if !errors.Is(err, ErrNotRead) {
		t.Fatalf("a truncated reading answered %v, want ErrNotRead", err)
	}
	if text != "" {
		t.Errorf("a truncated reading answered the text %q", text)
	}
}

func TestReadPageReportsAnEmptyAnswerAsAPageNotRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"content": [{"type": "text", "text": "   "}],
			"stop_reason": "end_turn", "usage": {}}`)
	}))
	defer server.Close()

	client := NewClient("Claude", server.URL, "secret-key")

	if _, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg"); !errors.Is(err, ErrNotRead) {
		t.Fatalf("an empty answer answered %v, want ErrNotRead", err)
	}
}

func TestReadPageNamesAnUnreadableAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `not json at all`)
	}))
	defer server.Close()

	client := NewClient("Claude", server.URL, "secret-key")

	_, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if err == nil {
		t.Fatal("an unreadable answer was accepted")
	}
	if !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("error %q does not say the answer was unreadable", err)
	}
}
