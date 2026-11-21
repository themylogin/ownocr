## 1. The profile reaches the worker

- [x] 1.1 Add `rescanModes` and `RescanModesOf` to `backend/internal/ocr/endpoint.go`,
  mapping `v6` to `[v6]` and `vl` to `[vl]`, and verify a unit test that each
  profile answers its one engine and an unknown profile answers false
- [x] 1.2 Add `Profile string` to `ocr.RescanArgs` and verify `go build ./...`
  passes
- [x] 1.3 Add `profile` to the rescan request body and check it with
  `ocr.KnownProfile` before `startRescan` locks the chain, and verify a handler
  test that a request naming no profile and one naming `xx` are both refused with
  the reason and leave the book `complete`
- [x] 1.4 Drop the `PageIndexesWithModel` lookup and the already read refusal from
  `checkPages`, and verify a handler test that a rescan of a page holding a `vl`
  result is accepted and queues one job
- [x] 1.5 Pass the profile into the queued `RescanArgs` and verify a handler test
  that the inserted job carries the profile the request named, and that the book
  still records the profile it recorded before

## 2. The store removes a reading

- [x] 2.1 Add `ReplacePageOCR(ctx, result, remove []string)` to
  `backend/internal/books/store.go`, running the upsert and the delete in one
  transaction, and verify a store test on a scratch database that a page holding
  `v6`, `vl` and `merged` ends holding the new `v6` alone
- [x] 2.2 Verify a store test that `ReplacePageOCR` with a `remove` list naming
  models the page does not hold still stores the result and removes nothing else
- [x] 2.3 Verify a store test that a failing write leaves every earlier result of
  the page in place, so the page never loses a reading it did not replace

## 3. The worker runs the engines of its profile

- [x] 3.1 Read `RescanModesOf(job.Args.Profile)` in `RescanWorker.Work`, abandon
  the job when the profile is unknown, and verify a worker test that the book
  returns to `complete` and no page changes
- [x] 3.2 Send the engines of the profile in `JobInput.Modes` and key `byInput` by
  the mode the job asked for, and verify worker tests that a `vl` rescan asks for
  `[vl]` and a `v6` rescan asks for `[v6]`
- [x] 3.3 Split `storePage` by profile: keep today's path for `vl`, and add a `v6`
  path that downloads the text and the JSON, then calls `ReplacePageOCR` with
  `remove = [vl, merged]`, and verify a worker test that a page holding three
  results ends holding the new `v6` alone
- [x] 3.4 Verify a worker test that a `v6` rescan of a page holding a `v6` result
  alone replaces it and removes nothing
- [x] 3.5 Verify a worker test that a page the endpoint failed keeps every result
  it held, under both profiles, and that the other named pages still gain theirs
- [x] 3.6 Verify a worker test that a `vl` rescan of a page that already holds a
  `vl` result replaces it and rebuilds the merged result from the stored `v6`
- [x] 3.7 Verify existing worker tests still pass: the book returns to `complete`,
  the page count survives, the prefix is cleared before and after, and the source
  book is untouched

## 4. The book detail drops `hasVisualText`

- [x] 4.1 Remove `HasVisualText` from `pageView`, drop the `visualText` parameter
  of `detailOf`, and drop the `PageIndexesWithModel` call from `getBook`, and
  verify `go build ./...` passes
- [x] 4.2 Remove `hasVisualText` from the `Page` interface in
  `frontend/src/api/client.ts` and verify `npx vue-tsc --noEmit` reports no error
- [x] 4.3 Verify an httpapi test that the book detail answers the same pages it
  did, without the field

## 5. The interface asks for a model

- [x] 5.1 Add the radio group of `OcrDialog.vue` to `RescanDialog.vue`, with the
  same two choices in the same words, `Extra` preselected, keeping its title, its
  body text and its "Rescan" button, and emit `start(profile)`
- [x] 5.2 Pass the profile from `ReaderView.startRescan` to `api.rescanPages`, and
  add the profile to `rescanPages` in `frontend/src/api/client.ts`
- [x] 5.3 Rename the menu item to "Rescan" and make `offersRescan` answer
  `props.rescan` alone in `PageMenu.vue`, so every page of a book that offers the
  rescan holds the item and opens the read half menu
- [x] 5.4 Rename the top bar control to "Rescan N page(s)" in `ReaderView.vue`
- [ ] 5.5 Verify by eye in the running app that the right half of a page holding a
  `vl` result opens the one item menu, and that the dialog opens with "Extra"
  selected

## 6. End to end

- [ ] 6.1 Add the profile to `rescanPages` and `rescanRefusal` in
  `e2e/tests/helpers.ts`, and update every call in `e2e/tests/rescan.spec.ts`
- [ ] 6.2 Update the e2e expectations that a rescanned page offers no rescan, and
  that `hasVisualText` reports the reading, and verify they assert the new
  behaviour instead
- [ ] 6.3 Add an e2e test that a `v6` rescan of a page read with the `vl` profile
  leaves that page holding a `v6` result alone, and that the page lines answer the
  model `v6`
- [ ] 6.4 Add an e2e test that a rescan request naming no profile is refused and
  leaves the book `complete`
- [ ] 6.5 Run the whole Go suite on a scratch database and verify it passes
- [ ] 6.6 Run `./run-e2e.sh rescan.spec.ts ocr.spec.ts ocrreader.spec.ts` and
  verify it passes
- [ ] 6.7 Run `openspec validate --strict rescan-any-page-with-a-chosen-model` and
  verify it passes

Note: `go test ./...` truncates `book CASCADE` on the development database. Follow
the scratch-database recipe before running the backend suite.
