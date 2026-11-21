package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const renderTimeout = 5 * time.Minute

func PageCount(ctx context.Context, pdfPath string) (int, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "pdfinfo", pdfPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("pdfinfo: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	for _, line := range strings.Split(stdout.String(), "\n") {
		rest, found := strings.CutPrefix(line, "Pages:")
		if !found {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return 0, fmt.Errorf("pdfinfo reported an unreadable page count %q", strings.TrimSpace(rest))
		}
		if count < 1 {
			return 0, errors.New("the PDF has no pages")
		}
		return count, nil
	}

	return 0, errors.New("pdfinfo reported no page count")
}

// RenderPage rasterizes one page to outBase+".jpg". Rendering one page per
// call bounds memory and temp disk to a single page and isolates a failure to
// the page that caused it.
func RenderPage(ctx context.Context, pdfPath string, pageNumber, dpi int, outBase string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()

	page := strconv.Itoa(pageNumber)
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-jpeg", "-r", strconv.Itoa(dpi),
		"-f", page, "-l", page, "-singlefile",
		pdfPath, outBase)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("rendering page %d timed out", pageNumber)
		}
		return "", fmt.Errorf("pdftoppm page %d: %w: %s",
			pageNumber, err, strings.TrimSpace(stderr.String()))
	}

	return outBase + ".jpg", nil
}
