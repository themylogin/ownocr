## Context

The Ultra rescan runs in the rescan worker. `internal/anthropic` sends one page
image per request to the Messages API. The worker merges the answer with the
stored PP-OCRv6 boxes and stores two results: the raw text and the merge. See
proposal.md for why Claude goes.

The change that added Ultra, `add-ultra-rescan-and-reading-choice`, is
archived, and the main specs hold its requirements. Its live check against the
Anthropic API stayed open. Task 6.3 here replaces it.

A merged result names an engine on every line: its `source` is `v6` or the
text engine. The raw result holds `{"model": "claude-opus-5", "text": ...}`.

OpenRouter speaks the OpenAI chat completions format. A probe sent page 111 of
a Spanish book on Plato to `z-ai/glm-5.3-flash` four times:

| Reasoning effort | Result | Time | Cost |
| --- | --- | --- | --- |
| default | full text, all Greek correct, 3,180 reasoning tokens | 28 s | $0.0018 |
| default | no text: 8,192 reasoning tokens, `finish_reason: length` | 33 s | $0.0047 |
| `minimal` | the same text as the first run, 1 reasoning token | 6 s | $0.0008 |
| disabled | refused: "Reasoning is mandatory for this endpoint" | — | — |

## Goals / Non-Goals

**Goals:**

- Ultra reads with GLM-5.3 Flash, and every other behavior of Ultra stays.
- No name, comment, variable, test or stand-in in the system names Claude or
  Anthropic.

**Non-Goals:**

- No change to the prompt, the merge, the rank of readings, the menu or the
  dialog layout.
- No choice of upstream provider on OpenRouter.
- No rename of the "Ultra" profile title.

## Decisions

### `internal/openrouter` replaces `internal/anthropic` in the same shape

The new package keeps the shape of the old one: a `Client` with a name, a base
URL and a token, over `net/http`. `ReadPage(ctx, image, mediaType)` keeps its
signature, `ErrNotRead` and `Usage`. The worker therefore changes its import and
names, not its flow.

The OpenAI Go SDK is the alternative. It adds a module dependency for one POST
request, and the RunPod client already sets the idiom.

### The model is pinned to `z-ai/glm-5.3-flash`

The stored raw result records the model that read the page. The alias
`~z-ai/glm-flash-latest` moves without notice, so that record would lie after
the next release. `z-ai/glm-5.3-flashx` is newer, but it costs 9 times as much
per input token and 16 times as much per output token.

### Reasoning effort is `minimal`

The model always reasons, and OpenRouter refuses a request that disables it. At
the default effort, one of two runs spent the whole budget on reasoning and
answered no text. At `minimal` the page reads in a fifth of the time, for less
than half the price, with the same text. `max_tokens` stays 8,192. It bounds a run that
still loops.

A larger `max_tokens` is the alternative. It raises the cost and the time of
every page and still sets no bound on reasoning.

### The wire format

The request is `POST {base}/v1/chat/completions` with `Authorization: Bearer`.
The default base is `https://openrouter.ai/api`. The body holds `model`,
`max_tokens`, `reasoning: {"effort": "minimal"}` and one user message. The
message holds the image as a `data:` URL part, then the prompt as a text part.

The worker reads `choices[0]`:

| `finish_reason` | Answer |
| --- | --- |
| `stop` with text | the trimmed text |
| `stop` with no text | `ErrNotRead`: it answered nothing |
| `length` | `ErrNotRead`: it ran out of room mid-page |
| `content_filter` | `ErrNotRead`: it refused the page |

A response with no choice is a job failure that names the error OpenRouter
returned. A status outside 2xx is a job failure, as before.

### The prompt does not change

The probe used the current prompt word for word. The answer held no fence, no
preamble and no commentary, and it kept every accent and breathing. It also held
the running head. The PP-OCRv6 result holds a box for that head, so the merge
places it.

### Names follow the engine for results and the vendor for the connection

The mode, the profile and the raw result are `glm`. The merge is `glm_merged`.
The Go names are `ModeGLM`, `ModelGLMMerged` and `ProfileUltra = "glm"`.

The configuration names the vendor: `config.OpenRouter`, `OPENROUTER_API_KEY`
(required) and `OPENROUTER_API_URL` (optional, for the stand-in). The settings
fields are `OpenRouterAPIURL` and `OpenRouterAPIToken`.

`ultra` as the profile value is the alternative. It would survive the next
engine swap. But `v6` and `vl` name engines, and a result named `ultra` would
not say which model wrote it.

### A migration renames the stored Claude results to GLM

Migration `0012` rewrites every Claude row in place:

- A `claude` row becomes `glm`, and its `data.model` becomes
  `z-ai/glm-5.3-flash`.
- A `claude_merged` row becomes `glm_merged`, and every line whose `source` is
  `claude` gets the source `glm`. A line with the source `v6` stays.

The row ids do not change, so `book_page.ocr_id` keeps every choice a reader
made. The `txt` columns hold no engine name and stay as they are. No `glm` row
exists before the migration, because the code that writes one ships with it.
The unique index on `(page_id, model)` therefore holds.

Deleting the rows is the alternative. It throws away paid readings and every
choice that names one.

### The stand-in speaks the chat completions format

`fakerunpod` serves `POST /v1/chat/completions` in place of `/v1/messages`. It
requires a bearer token. It answers `finish_reason: content_filter` with no text
for a request that holds no `image_url` part, and the fixed transcription for
any other.

## Risks / Trade-offs

- [OpenRouter sends requests to different providers, and their answers differ]
  → Accepted. Four runs reached InferenceNet, CoreWeave and Venice. The merge
  refuses an answer that places on too few lines, so a bad answer stores
  nothing.
- [GLM has a content filter of its own] → `content_filter` maps to
  `ErrNotRead`. The page keeps every result it held, and the rescan goes on.
- [One page is thin evidence] → The live check in tasks.md reads several pages
  of different books before the change is done.
- [A rescan job with the profile `claude` waits in the queue at deploy] → The
  worker treats it as an unknown profile. It retries, then abandons the job and
  returns the book to `complete`. No page loses a result.

## Migration Plan

1. Set `OPENROUTER_API_KEY` in the production `.env`, and delete
   `CLAUDE_API_KEY`. The server refuses to start without the new key.
2. Deploy. Migration `0012` runs at startup and renames the Claude results.

A rollback redeploys the previous image with `CLAUDE_API_KEY`. The previous
code knows no `glm` or `glm_merged` result, so the rollback must reverse the
rename by hand before that image starts.
