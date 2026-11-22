package openrouter

import (
	"context"
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
		path          string
		authorization string
		body          request
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.path = r.URL.Path
		seen.authorization = r.Header.Get("Authorization")
		payload, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(payload, &seen.body); err != nil {
			t.Errorf("the request body is unreadable: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices": [{"finish_reason": "stop",
			"message": {"role": "assistant", "content": "  la palabra ἀρχή  "}}],
			"usage": {"prompt_tokens": 4040, "completion_tokens": 804}}`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

	text, usage, err := client.ReadPage(context.Background(), []byte("jpeg bytes"), "image/jpeg")
	if err != nil {
		t.Fatalf("ReadPage: %v", err)
	}

	if text != "la palabra ἀρχή" {
		t.Errorf("the page reads %q", text)
	}
	if usage.InputTokens != 4040 || usage.OutputTokens != 804 {
		t.Errorf("usage = %+v", usage)
	}

	if seen.path != "/v1/chat/completions" {
		t.Errorf("the request went to %q", seen.path)
	}
	if seen.authorization != "Bearer secret-key" {
		t.Errorf("the key reached the API as %q", seen.authorization)
	}
	if seen.body.Model != "z-ai/glm-5.3-flash" {
		t.Errorf("the request names the model %q", seen.body.Model)
	}
	if seen.body.Reasoning.Effort != "minimal" {
		t.Errorf("the request names the reasoning effort %q", seen.body.Reasoning.Effort)
	}

	if len(seen.body.Messages) != 1 || len(seen.body.Messages[0].Content) != 2 {
		t.Fatalf("the request holds %+v", seen.body.Messages)
	}
	content := seen.body.Messages[0].Content

	if content[0].Type != "image_url" || content[0].ImageURL == nil {
		t.Fatalf("the first part is %+v", content[0])
	}
	if content[0].ImageURL.URL != "data:image/jpeg;base64,anBlZyBieXRlcw==" {
		t.Errorf("the image reached the API as %q", content[0].ImageURL.URL)
	}

	if content[1].Type != "text" || !strings.Contains(content[1].Text, "Transcribe this page") {
		t.Errorf("the second part is %+v", content[1])
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
		io.WriteString(w, `{"error": {"code": 429, "message": "slow down"}}`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

	_, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if err == nil {
		t.Fatal("an API error was accepted")
	}
	for _, want := range []string{"OpenRouter", "429", "slow down"} {
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

func TestReadPageNamesAnAnswerWithNoChoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error": {"code": 502, "message": "the provider went away"}}`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

	_, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if err == nil {
		t.Fatal("an answer with no choice was accepted")
	}
	for _, want := range []string{"OpenRouter", "the provider went away"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if errors.Is(err, ErrNotRead) {
		t.Error("an answer with no choice reads as a page the model would not read")
	}
}

func TestReadPageReportsARefusalAsAPageNotRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices": [{"finish_reason": "content_filter",
			"message": {"role": "assistant", "content": ""}}],
			"usage": {"prompt_tokens": 4040, "completion_tokens": 0}}`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

	_, usage, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if !errors.Is(err, ErrNotRead) {
		t.Fatalf("a refusal answered %v, want ErrNotRead", err)
	}
	if usage.InputTokens != 4040 {
		t.Errorf("a refusal lost its usage: %+v", usage)
	}
}

// A transcription cut off mid-page would read as a page the engine abandoned, so
// it counts as a page not read rather than a short reading.
func TestReadPageRefusesATruncatedTranscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices": [{"finish_reason": "length",
			"message": {"role": "assistant", "content": "the page begins"}}],
			"usage": {"completion_tokens": 8192}}`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

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
		io.WriteString(w, `{"choices": [{"finish_reason": "stop",
			"message": {"role": "assistant", "content": "   "}}], "usage": {}}`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

	if _, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg"); !errors.Is(err, ErrNotRead) {
		t.Fatalf("an empty answer answered %v, want ErrNotRead", err)
	}
}

func TestReadPageNamesAnUnreadableAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `not json at all`)
	}))
	defer server.Close()

	client := NewClient("OpenRouter", server.URL, "secret-key")

	_, _, err := client.ReadPage(context.Background(), []byte("x"), "image/jpeg")
	if err == nil {
		t.Fatal("an unreadable answer was accepted")
	}
	for _, want := range []string{"OpenRouter", "unreadable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}
