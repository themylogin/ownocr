// Package anthropic reads the text of a page image with Claude Opus 5.
//
// It is hand rolled over net/http, in the shape of internal/runpod: a client
// carrying a name, a base URL and a token. The engines the OCR endpoint runs
// answer over one HTTP call each, and so does this, so the error and timeout
// idiom of this system carries over and no module dependency is added.
package anthropic

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
	// DefaultBaseURL is the API this reads. It is a field of the client so a test
	// can point it at an httptest server.
	DefaultBaseURL = "https://api.anthropic.com"

	// Model is the reading model. Ultra exists because the endpoint engines
	// failed the page, so nothing cheaper belongs here.
	Model = "claude-opus-5"

	// version is the dated API contract this speaks.
	version = "2023-06-01"

	// maxTokens bounds one page. A dense page of this library reads as about
	// 1500 tokens, so this leaves room for the longest page and still stops a
	// reading that runs away.
	maxTokens = 8192

	// requestTimeout is one page. A page reads in 20 to 40 seconds, and the
	// worker reads the pages of a rescan one after another.
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

// prompt asks for the page and nothing else. Every rule in it answers a way the
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
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string  `json:"role"`
	Content []block `json:"content"`
}

type block struct {
	Type   string  `json:"type"`
	Text   string  `json:"text,omitempty"`
	Source *source `json:"source,omitempty"`
}

type source struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type response struct {
	Content    []block `json:"content"`
	StopReason string  `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
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
		Messages: []message{{
			Role: "user",
			Content: []block{
				// The image stands before the question, which is what the API
				// asks for on a vision request.
				{Type: "image", Source: &source{
					Type:      "base64",
					MediaType: mediaType,
					Data:      base64.StdEncoding.EncodeToString(image),
				}},
				{Type: "text", Text: prompt},
			},
		}},
	})
	if err != nil {
		return "", Usage{}, err
	}

	var answer response
	if err := c.call(ctx, "/v1/messages", body, &answer); err != nil {
		return "", Usage{}, err
	}

	usage := Usage{
		InputTokens:  answer.Usage.InputTokens,
		OutputTokens: answer.Usage.OutputTokens,
	}

	switch answer.StopReason {
	case "refusal":
		return "", usage, fmt.Errorf("%w: it refused the page", ErrNotRead)
	case "max_tokens":
		// A transcription cut off mid-page is worse than none: it would read as
		// a page the engine abandoned.
		return "", usage, fmt.Errorf("%w: it ran out of room mid-page", ErrNotRead)
	}

	var text strings.Builder
	for _, part := range answer.Content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}

	transcription := strings.TrimSpace(text.String())
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
	request.Header.Set("x-api-key", c.token)
	request.Header.Set("anthropic-version", version)
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
