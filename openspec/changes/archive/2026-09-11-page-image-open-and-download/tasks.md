## 1. The page menu

- [x] 1.1 Add a `bookName` prop to `PageMenu.vue`, and pass `book.name` from
  `BookView.vue` and `ReaderView.vue`. Verify by typecheck.
- [x] 1.2 Put "Open in new tab" and "Download" at the head of `menuOptions`,
  above a divider, and keep the conditional item and "Delete" below it. Give
  them the testids `page-context-open` and `page-context-download`. Verify by
  right clicking a page of an uploaded book: the menu holds four items.
- [x] 1.3 Open `page.originalUrl` in a new tab from `onSelect`, with no `await`
  before the call, so the click gesture stays live and no popup blocker fires.
  Verify the new tab loads the original, not the thumbnail.
- [x] 1.4 Download `page.originalUrl` through an anchor that carries the
  `download` name. Build the name from the book name, the page index in four
  digits and the extension read off `originalUrl`. Verify a download of the
  seventh page of a book named `Vergil` saves `Vergil - 0007.jpg`.
- [x] 1.5 Update the comment above `menuOptions`: "Delete" is no longer the item
  that guarantees a menu.

## 2. The OCR reader

- [x] 2.1 In `ReaderView.vue`, move the right click handler off the `.spread`
  div and onto the `img.plain` of the left half. Verify a right click on the
  left half still opens the page menu.
- [x] 2.2 Leave a bare `@contextmenu.prevent` on the `.spread` div, so the right
  half and the gap open no browser menu either. Verify a right click on the
  right half opens nothing at all.
- [x] 2.3 Verify the `v-else` branch is untouched: a right click on a page of a
  `split` book in the reader still opens the page menu.
- [x] 2.4 Verify a drag selection of the drawn text still copies by keyboard.

## 3. Tests

- [x] 3.1 Raise the menu item count at `e2e/tests/cut.spec.ts:86` from 2 to 4,
  and assert "Open in new tab" is the first item instead of "Edit". Verify the
  cut spec passes.
- [x] 3.2 Raise the menu item count at `e2e/tests/pages.spec.ts:69`, and update
  the uploaded page assertion that expects no divider. Verify the pages spec
  passes.
- [x] 3.3 Add an e2e test that selects "Open in new tab" and asserts the new page
  URL ends in the original rendition path of that page.
- [x] 3.4 Add an e2e test that selects "Download" and asserts the suggested file
  name of the Playwright download event.
- [x] 3.5 Add an e2e test in `e2e/tests/ocr.spec.ts` that right clicks the
  `ocr-overlay` of a read page and asserts no `page-context-menu` appears.
- [ ] 3.6 Run the whole e2e suite with `./run-e2e.sh` and verify it passes.
  Partly done: `./run-e2e.sh pages.spec.ts cut.spec.ts ocr.spec.ts` passed 58 of
  58. The rest of the suite has not run against this change.
