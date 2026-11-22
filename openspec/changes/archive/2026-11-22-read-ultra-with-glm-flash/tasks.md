## 1. OpenRouter client

- [x] 1.1 Create `backend/internal/openrouter/client.go` in the shape of
  `internal/anthropic`: `Client`, `NewClient(name, baseURL, token)`,
  `DefaultBaseURL = "https://openrouter.ai/api"`, `Model = "z-ai/glm-5.3-flash"`,
  `ErrNotRead`, `Usage` and `ReadPage`. Keep the prompt word for word. Send the
  wire format and map `finish_reason` as design.md says. Verify
  `go build ./internal/openrouter/` in the dev backend succeeds
- [x] 1.2 Write `backend/internal/openrouter/client_test.go` against an httptest
  server. Cover: the request names `z-ai/glm-5.3-flash`, effort `minimal`, a
  bearer token, and a `data:image/jpeg;base64,` part before the prompt;
  `content_filter`, `length` and an empty answer each give `ErrNotRead`; a
  non-2xx status, a response with no choice and an unreadable body each give an
  error that names the API. Verify `go test ./internal/openrouter/` passes

## 2. Configuration and the rescan worker

- [x] 2.1 Replace `config.Claude` with `config.OpenRouter`. It reads
  `OPENROUTER_API_URL` as optional and `OPENROUTER_API_KEY` as required. Update
  `config_test.go` to the literal names. Verify `go test ./internal/config/`
  passes and reports a missing `OPENROUTER_API_KEY`
- [x] 2.2 In `internal/ocr`, rename `ModeClaude` to `ModeGLM` (`"glm"`) and
  `ModelClaudeMerged` to `ModelGLMMerged` (`"glm_merged"`). Set
  `ProfileUltra = "glm"` and the Ultra engine name to "GLM-5.3 Flash". Replace
  the settings fields with `OpenRouterAPIURL` and `OpenRouterAPIToken`, and wire
  them in `cmd/server/main.go`. Verify `go vet ./...` passes
- [x] 2.3 Move the rescan worker to `openrouter.Client`. Rename
  `readWithClaude`, `readPageWithClaude`, `storeClaudeReading` and `claudeData`
  to GLM names. The raw result records `openrouter.Model`. Rewrite every comment
  that names Claude or Anthropic. Verify `go build ./...` succeeds
- [x] 2.4 Rename `rescan_claude_test.go` to `rescan_glm_test.go` and
  `TestTheClaudeKeyReachesTheAPIAlone` to `TestTheOpenRouterKeyReachesTheAPIAlone`.
  Update `endpoint_test.go`, `lines_test.go`, `merge_text_test.go`,
  `books/reading_test.go` and `httpapi/reading_test.go` to the literals `glm`,
  `glm_merged` and "GLM-5.3 Flash". Verify those packages pass
- [x] 2.5 Move `tools/ocrmerge` to `ocr.ModeGLM` and `ocr.ModelGLMMerged`, and
  rewrite its comment on text-only engines. Verify `go build ./tools/...`
  succeeds
- [x] 2.6 Delete `backend/internal/anthropic/`. Verify
  `docker compose -f docker-compose.dev.yml exec backend go test ./...` passes

## 3. Stored Claude results

- [x] 3.1 Add `backend/internal/db/migrations/0012_rename_claude_results_to_glm.sql`.
  It renames `claude` rows to `glm` and sets `data.model` to
  `z-ai/glm-5.3-flash`. It renames `claude_merged` rows to `glm_merged` and sets
  the `source` of every `claude` line to `glm`. Verify on the dev database:
  before the new backend starts, insert a `claude` row and a `claude_merged` row
  with `claude` and `v6` lines, and make a page name the merged row through
  `ocr_id`. Start the backend. Confirm the new models, `data.model`, the line
  sources (`v6` lines unchanged), and that `ocr_id` still names the same row

## 4. Stand-in, compose files and docs

- [x] 4.1 Make `tools/fakerunpod` serve `POST /v1/chat/completions` in place of
  `/v1/messages`. It requires a bearer token. It answers
  `finish_reason: content_filter` with no text when no `image_url` part comes,
  and the fixed transcription otherwise. Rename `claudeTranscription`, and make
  the package comment name the OpenRouter chat completions API. Verify
  `go build ./tools/fakerunpod` succeeds
- [x] 4.2 In `docker-compose.yml` and `docker-compose.dev.yml`, replace
  `CLAUDE_API_KEY` with `OPENROUTER_API_KEY: ${OPENROUTER_API_KEY}`. In both end
  to end compose files, set `OPENROUTER_API_KEY: e2e-openrouter-key` and
  `OPENROUTER_API_URL: http://fakerunpod:9090`. Verify each
  `docker compose ... config` renders the OpenRouter variables and no `CLAUDE_`
  variable
- [x] 4.3 Add `OPENROUTER_API_KEY` to `.env.example` with a one-line comment.
  Add `OPENROUTER_API_KEY` (required) and `OPENROUTER_API_URL` (default
  `https://openrouter.ai/api`) to the README environment table. Add
  `internal/openrouter/` to the README layout. Verify the table renders as one
  table

## 5. Frontend and end to end tests

- [x] 5.1 Set `RescanProfile` to `OcrProfile | 'glm'` in `api/client.ts`. In
  `RescanDialog.vue`, the Ultra choice becomes value `glm` and engine
  "GLM-5.3 Flash", and the preselection names `glm`. The note stays as it is.
  `holdsUltra` in `ReaderView.vue` tests `glm_merged`. Verify
  `npm run typecheck` in the dev frontend passes
- [x] 5.2 Update `e2e/tests/reading.spec.ts` to the literals `glm`, `glm_merged`,
  `rescan-profile-glm`, "GLM-5.3 Flash" and "Ultra (GLM-5.3 Flash)". Verify
  `./run-e2e.sh reading.spec.ts` passes

## 6. Integration

- [x] 6.1 Run
  `grep -rniE 'claude|anthropic|opus' --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=openspec .`
  and verify that it finds only `CLAUDE.md`, the `.claude` line of `.gitignore`,
  and migration `0012`
- [x] 6.2 Run the whole Go suite and `./run-e2e.sh`, and verify both pass
- [x] 6.3 On a machine that holds the page images, run an Ultra rescan of the
  page behind `files/23f9f3171a329247a8b52819d02ac653/8ed3c7e909fbc97e5c6715e5ffc150ae.orig.jpg`
  and of two pages from other books. Verify by eye in the reader that the merged
  text draws on the `v6` boxes, the Greek is intact, and the log reports the
  tokens of each page

## Workflow follow-up

- Set `OPENROUTER_API_KEY` in the production `.env` and delete `CLAUDE_API_KEY`
  before deploying.
