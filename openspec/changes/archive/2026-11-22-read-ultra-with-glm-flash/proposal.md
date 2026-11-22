## Why

The Ultra rescan reads a page with Claude Opus 5, and its content policy refuses
pages of the library. A reading model that refuses a book cannot be the model of
last resort.

GLM-5.3 Flash is the newest GLM Flash model on OpenRouter, and it reads images.
It read page 111 of a Spanish book on Plato in 6 seconds, every polytonic Greek
word correct, for $0.0008. The same page costs about $0.06 with Claude Opus 5.

The user wants every trace of Claude gone from the system, not hidden behind a
new default.

## What Changes

- The Ultra rescan reads a page with GLM-5.3 Flash through the OpenRouter API.
  It asks for the text of the page with the same rules as before.
- **BREAKING**: The Ultra profile is `glm` instead of `claude`. Its stored
  results are `glm` and `glm_merged` instead of `claude` and `claude_merged`.
- **BREAKING**: The server requires `OPENROUTER_API_KEY` instead of
  `CLAUDE_API_KEY`. `OPENROUTER_API_URL` replaces `CLAUDE_API_URL` for the end to
  end stand-in.
- A migration renames every stored `claude` result to `glm` and every
  `claude_merged` result to `glm_merged`. Every page keeps its readings and the
  reading a reader chose.
- The rescan dialog and the page menu name the Ultra engine "GLM-5.3 Flash".
  The description of Ultra does not change.
- The Anthropic client, its tests, the Anthropic stand-in in `fakerunpod`, and
  every Claude name in code, configuration, tests and comments go.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `book-ocr`: the third profile is `glm`, its merged result is `glm_merged`, and
  the text-only merge names GLM-5.3 Flash.
- `page-rescan`: the `glm` profile sends the page to the OpenRouter API, and a
  request accepts `glm`.
- `book-library`: the dialog, the page menu and the Ultra border name
  GLM-5.3 Flash.

## Impact

- Backend: `internal/anthropic` becomes `internal/openrouter`. `internal/config`,
  `internal/ocr`, `cmd/server`, `tools/ocrmerge` and `tools/fakerunpod` change
  with it.
- Database: migration `0012` renames the stored Claude results to GLM.
- Frontend: `api/client.ts`, `RescanDialog.vue` and `ReaderView.vue` name `glm`.
- Configuration: `docker-compose.yml`, `docker-compose.dev.yml`, both end to end
  compose files and `.env.example` name the OpenRouter variables.
- Tests: Go unit tests and `e2e/tests/reading.spec.ts` name the new profile,
  results and engine.
- Cost: an Ultra page costs about $0.001 on OpenRouter instead of about $0.06.
- Out of scope: `CLAUDE.md` and the `.claude` line of `.gitignore` belong to the
  coding agent, not the OCR system. Git history is not rewritten.
