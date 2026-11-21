## 1. The detector moves into the sidecar

- [x] 1.1 Add `sidecar/split.py` holding the detector from `output/split_gutter.py`: the ridge energy map, the search over angle, the band centroids, the robust quadratic fit, and the coverage gate. Verify by importing it in a Python shell in the dev sidecar and detecting one page of `output/full/input`.
- [x] 1.2 Remove every option from the ported code. `--mask`, `--straighten`, `--overlap`, `--min-prominence`, `--quality`, `--debug`, `--json` and the CLI itself all go, along with `fill_beyond`, `paper_tone` and `shear_to_vertical`. Verify no `argparse` import and no `mask`, `straighten` or `percentile` identifier remains in `sidecar/split.py`.
- [x] 1.3 Expose one function that takes a path and returns `None` or a pair of columns, computing the pair as the extremes of the fitted curve plus the constant margin. Verify it returns `None` for `output/full/input/page001.jpg` and a pair for `page010.jpg`.
- [x] 1.4 Run the ported detector over all 68 images in `output/full/input` and confirm it reports 67 spreads and 1 single page, with cut columns matching `output/full/report.json` for every page. This is the regression check that the port changed nothing; do it before task 6.1 deletes those samples.

## 2. The sidecar offers the method

- [x] 2.1 Add `POST /split` to `sidecar/app.py`. It takes a list of paths and returns a list of the same length, each entry `null` or a pair of whole numbers. Keep the handler a `def`, not an `async def`, as the existing comment in `app.py` requires. Verify with a request against the dev sidecar on port 8000.
- [x] 2.2 Make the method refuse the whole request when it cannot read any image, naming the path it failed on, and return no partial answer. Verify a request holding one missing path and one real path is refused and names the missing path.
- [x] 2.3 Verify an empty list returns an empty list, and that entry `n` of a twenty-image answer describes image `n` of the request.
- [x] 2.4 Add `pytest` to `sidecar/requirements.txt` and add `sidecar/test_split.py` covering: a synthetic spread returns a pair, a synthetic single page returns `None`, the first column is greater than or equal to the second, both columns lie inside the image, and an unreadable path is refused. Verify `pytest` passes in the dev sidecar.

## 3. The backend reaches the sidecar

- [x] 3.1 Add `SIDECAR_URL` to `internal/config`, defaulting to `http://sidecar:8000`, trimming a trailing slash as `DEWARP_API_URL` does. Verify the config test covers the default and the trim.
- [x] 3.2 Add a sidecar client to `internal/split` that posts a batch of paths and decodes the answer into `[]*[2]int` or an equivalent that distinguishes `null` from a pair. Verify a unit test against an `httptest` server covering a `null` entry, a pair entry, a refusal, and a length mismatch between request and answer.
- [x] 3.3 Wire the client into the worker's construction in `cmd/server` alongside the existing store, files and hub. Verify the server still starts in the dev stack and `/api/health` answers.

## 4. The worker cuts

- [x] 4.1 Add a rendition writer to `internal/ingest` that takes a decoded `image.Image` instead of a source path and encodes the original rendition as JPEG. Verify a unit test writes all three renditions and reports the cropped dimensions.
- [x] 4.2 Add a crop helper that returns a sub-image for a rectangle, falling back to drawing into a new RGBA for an image type without `SubImage`. Verify a unit test covers both paths.
- [x] 4.3 Replace `pagesPerSource` in `internal/split/worker.go` with a pass that asks the sidecar in batches of 20 for the source pages, then writes one page for a `null` answer and two for a pair. Keep the existing sequential numbering, the source page reference and the rebuild that clears the copy first. Verify the package tests pass.
- [x] 4.4 Make an uncut page keep the existing path-based `WriteRenditions`, so its stored bytes stay identical to the source page and a PNG stays a PNG. Verify a test compares the original rendition of an uncut copy page byte for byte with its source.
- [x] 4.5 Report progress after each batch rather than on a fixed page interval, so the reported page count of the copy grows while the job runs. Verify a test on a multi-batch book observes the count rising.
- [x] 4.6 Make a refused batch fail the job, and confirm the copy is marked failed only after the last attempt with a reason that names the sidecar. Verify a test drives a failing sidecar through all three attempts.

## 5. Fixtures and the end-to-end suite

- [x] 5.1 Add a synthetic spread to `backend/tools/fixturegen`: two blocks of dark rows separated by a tall vertical band of a different tone, drawn with the standard library alone. Verify the sidecar detects it as a spread and the existing single-page fixtures still detect as single pages.
- [x] 5.2 Regenerate the fixtures with `go run ./tools/fixturegen -out /app/internal/ingest/testdata` and add the spread to the `e2e/fixtures` set. Verify the generated files are deterministic across two runs.
- [x] 5.3 Update `e2e/tests/split.spec.ts` to assert a real cut: a book of spread fixtures produces a copy with twice the page count, and a book holding one non-spread page produces one page for it. Verify with `./run-e2e.sh split.spec.ts`.
- [x] 5.4 Run the whole suite with `./run-e2e.sh` and verify no other spec regressed, in particular the page delete chain and the reader, which now meet a copy with more pages than its source.

## 6. Cleanup and documentation

- [x] 6.1 Delete `output/` once task 1.4 has passed. Verify the directory is gone and nothing in `backend/`, `sidecar/` or `e2e/` refers to it.
- [x] 6.2 Add `SIDECAR_URL` to `docker-compose.yml` and `docker-compose.dev.yml` for the backend service, and make the backend wait on the sidecar's existing health check. Verify both stacks come up and a split job succeeds in the dev stack.
- [x] 6.3 Update the `.env.example` variable table and the `README.md` environment table with `SIDECAR_URL` and its default. Verify the two tables agree.
- [x] 6.4 Rewrite the Split flow section of `README.md`: it currently says splitting is not implemented and that Split copies pages unchanged. Describe the sidecar call, the coverage gate, the overlapping cut and the strip of the facing page each half keeps. Verify no sentence in `README.md` still claims Split copies pages.
- [x] 6.5 Update the "Add real OCR" section of `README.md`, which names Split as an unimplemented example, and the Layout section, which does not list `sidecar/`. Verify both read correctly against the tree.
