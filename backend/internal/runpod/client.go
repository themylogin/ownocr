package runpod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/themylogin/ownocr/backend/internal/books"
)

const (
	defaultPollInterval = 5 * time.Second
	pollErrorTolerance  = 5
	// pollHeartbeat logs one line every this many unchanged polls, so a long
	// GPU job leaves a trail.
	pollHeartbeat  = 12
	requestTimeout = 60 * time.Second
	maxErrorBody   = 4096
)

type Client struct {
	name    string
	baseURL string
	token   string
	http    *http.Client

	PollInterval time.Duration
}

func NewClient(name, baseURL, token string) *Client {
	return &Client{
		name:         name,
		baseURL:      baseURL,
		token:        token,
		http:         &http.Client{Timeout: requestTimeout},
		PollInterval: defaultPollInterval,
	}
}

type runResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type statusResponse struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Output json.RawMessage `json:"output"`
	Error  string          `json:"error"`
}

func (c *Client) Run(ctx context.Context, input any) (string, error) {
	body, err := json.Marshal(map[string]any{"input": input})
	if err != nil {
		return "", err
	}

	var answer runResponse
	if err := c.call(ctx, http.MethodPost, "/run", body, &answer); err != nil {
		return "", err
	}
	if answer.ID == "" {
		return "", fmt.Errorf("the %s endpoint %s returned no job id", c.name, c.baseURL)
	}

	return answer.ID, nil
}

func (c *Client) Wait(ctx context.Context, jobID string, log *slog.Logger,
	onStatus func(status string)) (json.RawMessage, error) {
	ticker := time.NewTicker(c.PollInterval)
	defer ticker.Stop()

	failures := 0
	polls := 0
	status := ""
	started := time.Now()

	for {
		var answer statusResponse
		err := c.call(ctx, http.MethodGet, "/status/"+jobID, nil, &answer)
		polls++

		if err != nil {
			failures++
			log.Warn("poll failed", "job", jobID, "failures", failures,
				"tolerance", pollErrorTolerance, "error", err)
			if failures > pollErrorTolerance {
				return nil, err
			}
		} else {
			failures = 0

			if answer.Status != status {
				status = answer.Status
				log.Info("endpoint job status", "job", jobID, "status", status,
					"waited", time.Since(started).Round(time.Second))
				if onStatus != nil {
					onStatus(status)
				}
			} else if polls%pollHeartbeat == 0 {
				log.Info("endpoint job still running", "job", jobID, "status", status,
					"waited", time.Since(started).Round(time.Second))
			}

			switch answer.Status {
			case "COMPLETED":
				if len(answer.Output) == 0 {
					return nil, fmt.Errorf("the %s endpoint reported success without a result",
						c.name)
				}
				return answer.Output, nil
			case "IN_QUEUE", "IN_PROGRESS":
			default:
				return nil, fmt.Errorf("the %s endpoint reported %s: %s",
					c.name, answer.Status, reasonOf(answer))
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func PhaseOfStatus(status string) (string, bool) {
	switch status {
	case "IN_QUEUE":
		return books.PhaseWaitingAI, true
	case "IN_PROGRESS":
		return books.PhaseProcessing, true
	default:
		return "", false
	}
}

func reasonOf(answer statusResponse) string {
	if answer.Error != "" {
		return answer.Error
	}
	if len(answer.Output) > 0 {
		return string(answer.Output)
	}
	return "no reason given"
}

func (c *Client) call(ctx context.Context, method, path string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("the %s endpoint %s did not answer: %w", c.name, c.baseURL, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("the %s endpoint %s: %w", c.name, c.baseURL, err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("the %s endpoint %s answered %d: %s",
			c.name, c.baseURL, response.StatusCode, trim(payload))
	}

	return json.Unmarshal(payload, out)
}

func trim(payload []byte) string {
	if len(payload) > maxErrorBody {
		return string(payload[:maxErrorBody])
	}
	return string(payload)
}
