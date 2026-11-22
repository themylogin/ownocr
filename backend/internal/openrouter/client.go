// Package openrouter reads the text of a page image with GLM-5.3 Flash through
// the OpenRouter chat completions API.
//
// It is hand rolled over net/http, in the shape of internal/runpod: a client
// carrying a name, a base URL and a token. One page is one HTTP call, so the
// error and timeout idiom of this system carries over and no module dependency is
// added.
package openrouter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is a field of the client so a test can point it at an
	// httptest server.
	DefaultBaseURL = "https://openrouter.ai/api"

	// Model is pinned rather than the floating `~z-ai/glm-flash-latest`, because
	// the stored result records it and an alias would make that record lie after
	// the next release.
	Model = "z-ai/glm-5.3-flash"

	// effort is the lowest the model accepts: it refuses a request that turns
	// reasoning off. At its default effort it spent all of maxTokens on reasoning
	// for one page in two and answered no text.
	effort = "minimal"

	// maxTokens bounds one page. A dense page of this library reads as about
	// 1500 tokens, so this leaves room for the longest page and still stops a
	// reading that runs away.
	maxTokens = 8192

	// requestTimeout is one page. The worker reads the pages of a rescan one
	// after another.
	requestTimeout = 5 * time.Minute

	maxErrorBody = 4096
)

// ErrNotRead marks a page the model would not or could not read: it refused, it
// answered nothing, or it ran out of room mid-page. It is not a failure of the
// job. A rescan stores what it can and leaves such a page as it was.
var ErrNotRead = errors.New("the model did not read the page")

type Client struct {
	name    string
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(name, baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: requestTimeout},
	}
}

// prompt asks for the page and nothing else. Every rule in it answers a way a
// model otherwise spoils a transcription: it wraps the text in a fence, it
// describes the page instead of reading it, it silently modernises the
// orthography, or it renders a Greek word in the alphabet of the surrounding
// language.
const prompt = `Transcribe this page of a printed book.

Rules:
- Output only the transcription. No preamble, no commentary, no code fences.
- Preserve the orthography exactly, including every accent and breathing of
  polytonic Greek, and including spellings that look archaic or wrong.
- Keep the printed reading order. Transcribe a footnote where it is printed.
- Join a word a line break hyphenated into one word.
- Separate paragraphs with one blank line.
- Write ` + "`⟨?⟩`" + ` for a character you cannot read.
- If the page holds no text at all, output nothing at all.`

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Reasoning reasoning `json:"reasoning"`
	Messages  []message `json:"messages"`
}

type reasoning struct {
	Effort string `json:"effort"`
}

type message struct {
	Role    string `json:"role"`
	Content []part `json:"content"`
}

type part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type response struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Usage is what one page cost, for the log. Nothing branches on it.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ReadPage answers the text of one page image. mediaType is the type of the
// image bytes, such as `image/jpeg`.
//
// It answers ErrNotRead for a page the model would not read, which no other page
// of the rescan shares.
func (c *Client) ReadPage(ctx context.Context, image []byte,
	mediaType string) (string, Usage, error) {
	body, err := json.Marshal(request{
		Model:     Model,
		MaxTokens: maxTokens,
		Reasoning: reasoning{Effort: effort},
		Messages: []message{{
			Role: "user",
			Content: []part{
				{Type: "image_url", ImageURL: &imageURL{
					URL: "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(image),
				}},
				{Type: "text", Text: prompt},
			},
		}},
	})
	if err != nil {
		return "", Usage{}, err
	}

	var answer response
	if err := c.call(ctx, "/v1/chat/completions", body, &answer); err != nil {
		return "", Usage{}, err
	}

	usage := Usage{
		InputTokens:  answer.Usage.PromptTokens,
		OutputTokens: answer.Usage.CompletionTokens,
	}

	if len(answer.Choices) == 0 {
		reason := "no choice"
		if answer.Error != nil {
			reason = answer.Error.Message
		}
		return "", usage, fmt.Errorf("the %s API %s answered no reading: %s",
			c.name, c.baseURL, reason)
	}
	choice := answer.Choices[0]

	switch choice.FinishReason {
	case "content_filter":
		return "", usage, fmt.Errorf("%w: it refused the page", ErrNotRead)
	case "length":
		// A transcription cut off mid-page is worse than none: it would read as
		// a page the engine abandoned.
		return "", usage, fmt.Errorf("%w: it ran out of room mid-page", ErrNotRead)
	}

	transcription := strings.TrimSpace(choice.Message.Content)
	if transcription == "" {
		return "", usage, fmt.Errorf("%w: it answered nothing", ErrNotRead)
	}

	return transcription, usage, nil
}

func (c *Client) call(ctx context.Context, path string, body []byte, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("the %s API %s did not answer: %w", c.name, c.baseURL, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("the %s API %s: %w", c.name, c.baseURL, err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("the %s API %s answered %d: %s",
			c.name, c.baseURL, response.StatusCode, trim(payload))
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("the %s API %s returned an unreadable answer: %w",
			c.name, c.baseURL, err)
	}
	return nil
}

func trim(payload []byte) string {
	if len(payload) > maxErrorBody {
		return string(payload[:maxErrorBody])
	}
	return string(payload)
}
