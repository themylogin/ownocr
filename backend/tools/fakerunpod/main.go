// Command fakerunpod answers like a RunPod serverless endpoint, without a GPU,
// and like the OpenRouter chat completions API, without spending money.
// It "dewarps" by copying every input object to the output prefix at half size,
// and it "reads" a page by writing a markdown object that names it. Both are
// enough to prove the server's half of the contract.
//
// The input prefix names the action: a job under `ocr/` reads pages, every
// other job dewarps them.
//
// It is a test tool. Nothing in production uses it.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/themylogin/ownocr/backend/internal/dewarp"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/ocr"
)

type server struct {
	token string
	// store lets a test read the bucket without signing a request itself. It
	// is nil when the tool runs without bucket settings.
	store *objectstore.Store

	mu sync.Mutex
	// jobs holds the answer of a finished job. The shape follows the action
	// the job ran, so it is kept as the value the handler wrote.
	jobs     map[string]any
	failures map[string]string
	next     int
	failMode string
}

func main() {
	addr := flag.String("addr", envOr("ADDR", ":9090"), "listen address")
	flag.Parse()

	api := &server{
		token: os.Getenv("DEWARP_API_TOKEN"),
		store: objectstore.New(objectstore.Settings{
			Endpoint:        os.Getenv("AWS_ENDPOINT_URL"),
			Region:          envOr("AWS_REGION", "us-east-1"),
			AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Bucket:          os.Getenv("AWS_BUCKET"),
		}),
		jobs:     map[string]any{},
		failures: map[string]string{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /run", api.run)
	mux.HandleFunc("POST /v1/chat/completions", api.readPage)
	mux.HandleFunc("GET /status/{id}", api.status)
	mux.HandleFunc("POST /control/fail", api.controlFail)
	mux.HandleFunc("POST /control/reset", api.controlReset)
	mux.HandleFunc("GET /control/objects", api.controlObjects)
	mux.HandleFunc("DELETE /control/objects", api.controlPurge)

	log.Printf("fake runpod listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (s *server) run(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}

	var body struct {
		Input jobInput `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.next++
	id := fmt.Sprintf("job-%d", s.next)
	mode := s.failMode
	s.mu.Unlock()

	if mode == "job" {
		s.mu.Lock()
		s.failures[id] = "the pod ran out of memory"
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "IN_QUEUE"})
		return
	}

	output, err := runPrefix(r.Context(), body.Input, mode == "page")
	if err != nil {
		s.mu.Lock()
		s.failures[id] = err.Error()
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "IN_QUEUE"})
		return
	}

	s.mu.Lock()
	s.jobs[id] = output
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "IN_QUEUE"})
}

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")

	s.mu.Lock()
	output, done := s.jobs[id]
	reason, failed := s.failures[id]
	s.mu.Unlock()

	switch {
	case failed:
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "FAILED", "error": reason})
	case done:
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "COMPLETED", "output": output})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "IN_PROGRESS"})
	}
}

// controlFail makes every job fail until it is called again with mode=off. The
// server retries a dewarp job three times, so a single failure would not show a
// failed book. mode=job fails the whole endpoint job. mode=page reports the
// first page as failed.
func (s *server) controlFail(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	switch mode {
	case "page", "off":
	default:
		mode = "job"
	}
	if mode == "off" {
		mode = ""
	}

	s.mu.Lock()
	s.failMode = mode
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"failMode": mode})
}

func (s *server) controlReset(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.jobs = map[string]any{}
	s.failures = map[string]string{}
	s.failMode = ""
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"status": "reset"})
}

func (s *server) controlObjects(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.List(r.Context(), r.URL.Query().Get("prefix"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (s *server) controlPurge(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "dewarp/"
	}

	if err := s.store.DeletePrefix(r.Context(), prefix); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"purged": prefix})
}

func (s *server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	return r.Header.Get("Authorization") == "Bearer "+s.token
}

// jobInput is every field either endpoint reads. The OCR endpoint takes the
// engines to run; the dewarp endpoint does not.
type jobInput struct {
	dewarp.JobInput
	Modes []string `json:"modes"`
}

// runPrefix runs the action the input prefix names.
func runPrefix(ctx context.Context, input jobInput, failFirstPage bool) (any, error) {
	store, err := bucketOf(input)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(input.InputPrefix, ocr.Root) {
		return readPrefix(ctx, store, input, failFirstPage)
	}
	return dewarpPrefix(ctx, store, input, failFirstPage)
}

func bucketOf(input jobInput) (*objectstore.Store, error) {
	if input.AccessKeyID == "" || input.SecretAccessKey == "" || input.Bucket == "" {
		return nil, fmt.Errorf("the job carried no bucket credentials")
	}

	return objectstore.New(objectstore.Settings{
		Endpoint:        input.EndpointURL,
		Region:          input.Region,
		AccessKeyID:     input.AccessKeyID,
		SecretAccessKey: input.SecretAccessKey,
		Bucket:          input.Bucket,
	}), nil
}

// readPrefix stands in for the OCR endpoint. It writes the objects of every
// engine the job names, one pair per engine per input object, holding the name
// of that object so a test can tell the pages of a copy apart.
//
// The real handler refuses a job that names no engine, so this one does too.
func readPrefix(ctx context.Context, store *objectstore.Store, input jobInput,
	failFirstPage bool) (ocr.Output, error) {
	if len(input.Modes) == 0 {
		return ocr.Output{}, fmt.Errorf("'modes' is required and must be a non-empty array")
	}
	for _, mode := range input.Modes {
		if mode != ocr.ModeV6 && mode != ocr.ModeVL {
			return ocr.Output{}, fmt.Errorf("unknown mode %q", mode)
		}
	}

	keys, err := store.List(ctx, input.InputPrefix)
	if err != nil {
		return ocr.Output{}, err
	}

	var output ocr.Output
	for index, key := range keys {
		if strings.HasSuffix(key, "/") {
			continue
		}

		if failFirstPage && index == 0 {
			output.Failed = append(output.Failed,
				ocr.OutputFailure{InputKey: key, Error: "cannot identify image file"})
			continue
		}

		lines, err := pageLines(ctx, store, key)
		if err != nil {
			return ocr.Output{}, err
		}

		stem := strings.TrimSuffix(path.Base(key), path.Ext(key))
		for _, mode := range input.Modes {
			page, err := writeEngineResult(ctx, store, input.OutputPrefix, stem, key, mode, lines)
			if err != nil {
				return ocr.Output{}, err
			}
			output.Pages = append(output.Pages, page)
		}
	}

	return output, nil
}

// fakeLine is one line of the page the fake reads: the text and the box it
// claims to have read it in. A renderer draws these boxes, so they must lie on
// the page and must differ from one another.
type fakeLine struct {
	Text string
	Box  [4]int
}

// pageLines lays three lines over the page the object holds. The boxes follow
// the size of that image, so a test can check that a drawn box lands where the
// line says.
func pageLines(ctx context.Context, store *objectstore.Store, key string) ([]fakeLine, error) {
	body, err := store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	config, _, err := image.DecodeConfig(body)
	if err != nil {
		return nil, err
	}

	width, height := config.Width, config.Height
	margin := max(1, width/10)
	lineHeight := max(1, height/20)
	texts := []string{path.Base(key), "the text of this page", "the second line of this page"}

	lines := make([]fakeLine, 0, len(texts))
	for index, text := range texts {
		top := height/10 + index*2*lineHeight
		lines = append(lines, fakeLine{
			Text: text,
			Box:  [4]int{margin, top, width - margin, min(top+lineHeight, height)},
		})
	}
	return lines, nil
}

// writeEngineResult writes the text and the full result of one engine, under
// the names that engine uses. The names are what tells one engine from the
// other in the answer.
//
// PP-OCRv6 reports one box per line. PaddleOCR-VL reports one block that holds
// the same lines reflowed, in a box that covers them, which is what gives the
// merge something to join.
func writeEngineResult(ctx context.Context, store *objectstore.Store,
	outputPrefix, stem, inputKey, mode string, lines []fakeLine) (ocr.OutputPage, error) {
	texts := make([]string, len(lines))
	for i, line := range lines {
		texts[i] = line.Text
	}

	textName, dataName := stem+".md", stem+".json"
	text := "# " + path.Base(inputKey) + "\n\n" + strings.Join(texts[1:], "\n") + "\n"
	data, err := blockData(lines)
	if err != nil {
		return ocr.OutputPage{}, err
	}

	if mode == ocr.ModeV6 {
		textName, dataName = stem+".v6.txt", stem+".v6.json"
		text = strings.Join(texts, "\n") + "\n"
		if data, err = lineData(lines); err != nil {
			return ocr.OutputPage{}, err
		}
	}

	for name, body := range map[string]string{textName: text, dataName: data} {
		if err := store.Put(ctx, outputPrefix+name, strings.NewReader(body)); err != nil {
			return ocr.OutputPage{}, err
		}
	}

	return ocr.OutputPage{
		InputKey:   inputKey,
		OutputKeys: []string{outputPrefix + textName, outputPrefix + dataName},
		Chars:      len(text),
		Seconds:    0.1,
	}, nil
}

func lineData(lines []fakeLine) (string, error) {
	type resultLine struct {
		Text  string  `json:"text"`
		Score float64 `json:"score"`
		Box   [4]int  `json:"bbox"`
	}

	result := struct {
		Raw   struct{}     `json:"raw"`
		Lines []resultLine `json:"lines"`
	}{}
	for _, line := range lines {
		result.Lines = append(result.Lines, resultLine{Text: line.Text, Score: 0.99, Box: line.Box})
	}

	data, err := json.Marshal(result)
	return string(data), err
}

func blockData(lines []fakeLine) (string, error) {
	type block struct {
		Label   string `json:"block_label"`
		Content string `json:"block_content"`
		Box     [4]int `json:"block_bbox"`
	}

	texts := make([]string, len(lines))
	box := lines[0].Box
	for i, line := range lines {
		texts[i] = line.Text
		box[0] = min(box[0], line.Box[0])
		box[1] = min(box[1], line.Box[1])
		box[2] = max(box[2], line.Box[2])
		box[3] = max(box[3], line.Box[3])
	}

	result := struct {
		Blocks []block `json:"parsing_res_list"`
	}{Blocks: []block{{Label: "text", Content: strings.Join(texts, " "), Box: box}}}

	data, err := json.Marshal(result)
	return string(data), err
}

func dewarpPrefix(ctx context.Context, store *objectstore.Store, input jobInput,
	failFirstPage bool) (dewarp.Output, error) {
	keys, err := store.List(ctx, input.InputPrefix)
	if err != nil {
		return dewarp.Output{}, err
	}

	var output dewarp.Output
	for index, key := range keys {
		if strings.HasSuffix(key, "/") {
			continue
		}

		if failFirstPage && index == 0 {
			output.Failed = append(output.Failed,
				dewarp.OutputFailure{InputKey: key, Error: "could not decode"})
			continue
		}

		body, err := store.Get(ctx, key)
		if err != nil {
			return dewarp.Output{}, err
		}
		raw, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			return dewarp.Output{}, err
		}

		shrunk, width, height, err := shrink(raw)
		if err != nil {
			output.Failed = append(output.Failed,
				dewarp.OutputFailure{InputKey: key, Error: err.Error()})
			continue
		}

		name := strings.TrimSuffix(path.Base(key), path.Ext(key)) + ".jpg"
		outputKey := input.OutputPrefix + name
		if err := store.Put(ctx, outputKey, bytes.NewReader(shrunk)); err != nil {
			return dewarp.Output{}, err
		}

		output.Pages = append(output.Pages, dewarp.OutputPage{
			InputKey: key, OutputKey: outputKey, Width: width, Height: height,
		})
	}

	return output, nil
}

// shrink stands in for dewarping. The result differs in size from its source,
// so a test can tell the two apart.
func shrink(raw []byte) ([]byte, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, err
	}

	bounds := img.Bounds()
	longest := max(bounds.Dx(), bounds.Dy())
	resized := ingest.Resize(img, max(1, longest/2))

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, resized, &jpeg.Options{Quality: 90}); err != nil {
		return nil, 0, 0, err
	}

	size := resized.Bounds()
	return buffer.Bytes(), size.Dx(), size.Dy(), nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// glmTranscription is what the stand-in model answers for every page. Its last
// two lines are the lines `pageLines` lays over every page, so the merge places
// the answer on the boxes of that page as it would place a real transcription.
// Its first line differs from the first line the box engine reads, so a test can
// see which reading a page draws.
const glmTranscription = "a page the frontier model read\n" +
	"the text of this page\n" +
	"the second line of this page"

// readPage answers like POST /v1/chat/completions of the OpenRouter API: one
// choice holding the transcription of the image the request carries.
//
// A request naming no image is answered with a refusal, so a test can prove the
// worker sends the page and not an empty body.
func (s *server) readPage(w http.ResponseWriter, r *http.Request) {
	if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); !ok || token == "" {
		http.Error(w, `{"error": {"message": "no api key"}}`, http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, `{"error": {"message": "unreadable request"}}`, http.StatusBadRequest)
		return
	}

	finish, text := "stop", glmTranscription
	if !bytes.Contains(body, []byte(`"type":"image_url"`)) &&
		!bytes.Contains(body, []byte(`"type": "image_url"`)) {
		finish, text = "content_filter", ""
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"choices": []map[string]any{{
			"finish_reason": finish,
			"message":       map[string]string{"role": "assistant", "content": text},
		}},
		"usage": map[string]int{"prompt_tokens": 4040, "completion_tokens": 240},
	})
}
