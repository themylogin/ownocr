package split

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// One batch reads and measures twenty full size pages inside OpenCV, which is
	// seconds, not minutes. The timeout covers a slow disk, not a slow algorithm.
	requestTimeout = 5 * time.Minute
	maxErrorBody   = 4096
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: requestTimeout},
	}
}

// Left is the column after the last column of the left page. Right is the column
// of the first column of the right page. Left is at or after Right, so the two
// halves overlap and a leaning gutter clips neither page.
type Cut struct {
	Left  int
	Right int
}

// Cuts asks the sidecar where to cut every path. Entry n of the answer describes
// path n. A nil entry names a page that is not a spread.
func (c *Client) Cuts(ctx context.Context, paths []string) ([]*Cut, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/split",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("the sidecar did not answer: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the sidecar refused the batch: %s: %s",
			response.Status, readError(response.Body))
	}

	// A page that is not a spread arrives as null, which decodes to a nil slice.
	var columns [][]int
	if err := json.NewDecoder(response.Body).Decode(&columns); err != nil {
		return nil, fmt.Errorf("the sidecar answered with no readable list: %w", err)
	}
	if len(columns) != len(paths) {
		return nil, fmt.Errorf("the sidecar answered for %d pages of %d", len(columns), len(paths))
	}

	cuts := make([]*Cut, len(columns))
	for index, pair := range columns {
		cut, err := cutOf(index, pair)
		if err != nil {
			return nil, err
		}
		cuts[index] = cut
	}
	return cuts, nil
}

func cutOf(index int, pair []int) (*Cut, error) {
	if pair == nil {
		return nil, nil
	}
	if len(pair) != 2 {
		return nil, fmt.Errorf("page %d: the sidecar named %d columns, want 2", index, len(pair))
	}

	cut := Cut{Left: pair[0], Right: pair[1]}
	if cut.Right < 0 {
		return nil, fmt.Errorf("page %d: the sidecar named the column %d", index, cut.Right)
	}
	if cut.Left < cut.Right {
		return nil, fmt.Errorf("page %d: the sidecar named the columns %d and %d, which do not overlap",
			index, cut.Left, cut.Right)
	}
	return &cut, nil
}

func readError(body io.Reader) string {
	message, err := io.ReadAll(io.LimitReader(body, maxErrorBody))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(message))
}
