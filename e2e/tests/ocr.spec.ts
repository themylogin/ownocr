import { existsSync, readFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import type { APIRequestContext, Page as PlaywrightPage } from '@playwright/test'
import {
  failDewarp,
  getBook,
  imageSet,
  ingest,
  objectKeys,
  pageResults,
  reset,
  resetDewarp,
  runAction,
  startOCR,
  startSplit,
  storedPath,
  waitForSettled,
  type BookDetail,
  type MergedResult,
  type OcrProfile,
  type V6Result,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

async function seedBook(request: APIRequestContext, name = 'Atlas'): Promise<BookDetail> {
  return ingest(request, name, imageSet('portrait'))
}

test.describe('the OCR action reads a book', () => {
  test('the copy lands in the OCR stage with one result per engine per page', async ({
    page,
    request,
  }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id)
    expect(copy.stage).toBe('ocr')
    expect(copy.ocrProfile).toBe('vl')

    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')
    expect(settled.stage).toBe('ocr')
    expect(settled.pageCount).toBe(book.pageCount)
    expect(settled.pages).toHaveLength(book.pages.length)

    for (const [index, copyPage] of settled.pages.entries()) {
      const sourcePage = book.pages[index]

      // OCR reads a page, it does not change it. The copy owns no file, so
      // every rendition of its page is the rendition of the source page.
      expect(copyPage.width).toBe(sourcePage.width)
      expect(copyPage.height).toBe(sourcePage.height)
      expect(copyPage.originalUrl).toBe(sourcePage.originalUrl)
      expect(copyPage.displayUrl).toBe(sourcePage.displayUrl)
      expect(copyPage.thumbnailUrl).toBe(sourcePage.thumbnailUrl)
      expect(readFileSync(storedPath(copyPage.originalUrl)).length).toBeGreaterThan(0)

      // Extra runs both engines, so the page holds a result for each, and the
      // merged result derived from the two.
      const results = await pageResults(index + 1, copy.id)
      expect(Object.keys(results).sort()).toEqual(['merged', 'v6', 'vl'])
      for (const model of ['v6', 'vl']) {
        expect(results[model].txt).toContain(`000${index + 1}.jpg`)
        expect(results[model].data).toBeTruthy()
      }
    }

    // A complete copy leaves nothing in the bucket.
    expect(await objectKeys(request, 'ocr/')).toHaveLength(0)

    const source = await getBook(request, book.id)
    expect(source.stage).toBe('uploaded')
    expect(source.derivedState).toBe('complete')

    await page.goto('/stage/ocr')
    await expect(page.getByTestId(`book-card-${copy.id}`)).toBeVisible()
  })

  // The copy shows the images of the source book, so it writes no file of its
  // own and its pages still render.
  test('the copy owns no files and still shows its pages', async ({ page, request }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id, 'v6')
    const settled = await waitForSettled(request, copy.id)

    expect(existsSync(storedPath(`/files/${copy.storageKey ?? 'none'}/`))).toBe(false)

    for (const copyPage of settled.pages) {
      expect(copyPage.originalUrl).not.toContain(String(copy.id))
      expect(existsSync(storedPath(copyPage.originalUrl))).toBe(true)
    }

    // The thumbnail is served out of the source book, and it decodes.
    await page.goto(`/books/${copy.id}`)
    const first = page.getByTestId('page-cell-1').locator('img')
    await expect(first).toBeVisible()
    await expect(first).toHaveAttribute('src', settled.pages[0].thumbnailUrl)
    await expect(first).not.toHaveJSProperty('naturalWidth', 0)
  })

  test('the normal profile stores one result a page', async ({ request }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id, 'v6')
    expect(copy.ocrProfile).toBe('v6')

    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')

    for (let index = 1; index <= settled.pages.length; index++) {
      const results = await pageResults(index, copy.id)
      expect(Object.keys(results)).toEqual(['v6'])
    }
  })

  test('a split book reaches the end of the pipeline', async ({ request }) => {
    const book = await seedBook(request)
    const split = await startSplit(request, book.id)
    await waitForSettled(request, split.id)

    const copy = await startOCR(request, split.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('complete')
    expect(settled.stage).toBe('ocr')
    expect(settled.pages.length).toBeGreaterThan(0)
  })

  test('a page of the copy offers no rollback', async ({ page, request }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}`)
    await page.getByTestId('page-cell-1').click({ button: 'right' })

    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
    await expect(page.getByTestId('page-context-original')).toHaveCount(0)
  })

  test('a read book offers no further action', async ({ request }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    const result = await runAction(request, 'ocr', [copy.id], { ocrProfile: 'v6' })
    expect(result.started).toHaveLength(0)
    expect(result.failed[0].error).toContain('ocr')
  })

  test('the request must name a model', async ({ request }) => {
    const book = await seedBook(request)

    const response = await request.post(`${process.env.E2E_API_URL ?? 'http://backend:8080'}/api/books/actions/ocr`, {
      data: { bookIds: [book.id] },
    })
    expect(response.status()).toBe(422)
    expect(await response.text()).toContain('ocrProfile')

    const source = await getBook(request, book.id)
    expect(source.derivedState).toBeNull()
  })
})

test.describe('the OCR dialog asks for a model', () => {
  test('it opens from the book page and starts the chosen model', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto(`/books/${book.id}`)
    await page.getByTestId('action-ocr').click()

    await expect(page.getByTestId('ocr-dialog')).toBeVisible()
    expect(await getBook(request, book.id).then((b) => b.derivedState)).toBeNull()

    await page.getByTestId('ocr-profile-vl').click()
    await page.getByTestId('ocr-start').click()

    await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()

    const source = await getBook(request, book.id)
    expect(source.derivedState).not.toBeNull()
  })

  test('cancelling starts nothing', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto(`/books/${book.id}`)
    await page.getByTestId('action-ocr').click()
    await expect(page.getByTestId('ocr-dialog')).toBeVisible()

    await page.getByTestId('ocr-cancel').click()
    await expect(page.getByTestId('ocr-dialog')).toHaveCount(0)

    const source = await getBook(request, book.id)
    expect(source.derivedState).toBeNull()
  })

  test('it opens from the card context menu', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })
    await page.getByTestId('context-ocr').click()

    await expect(page.getByTestId('ocr-dialog')).toBeVisible()
    await expect(page.getByTestId('ocr-dialog-count')).toContainText('this book')
  })

  test('it opens from a bulk action and names the count', async ({ page, request }) => {
    const first = await seedBook(request, 'One')
    const second = await seedBook(request, 'Two')

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-select-${first.id}`).click()
    await page.getByTestId(`book-select-${second.id}`).click()

    await page.getByTestId('bulk-actions').click()
    await page.getByTestId('bulk-ocr').click()

    await expect(page.getByTestId('ocr-dialog')).toBeVisible()
    await expect(page.getByTestId('ocr-dialog-count')).toContainText('these 2 books')

    await page.getByTestId('ocr-start').click()

    for (const book of [first, second]) {
      const source = await getBook(request, book.id)
      expect(source.derivedState).not.toBeNull()
    }
  })
})

test.describe('a failed OCR keeps the copy', () => {
  test('the endpoint refuses the job', async ({ request }) => {
    const book = await seedBook(request)
    await failDewarp(request, 'job')

    const copy = await startOCR(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('failed')
    expect(settled.stage).toBe('ocr')
    expect(settled.error).toContain('the pod ran out of memory')
    expect(settled.sourceBookId).toBe(book.id)
  })

  test('a page the endpoint cannot read fails the copy, then a retry succeeds', async ({
    request,
  }) => {
    const book = await seedBook(request)
    await failDewarp(request, 'page')

    const copy = await startOCR(request, book.id)
    const failed = await waitForSettled(request, copy.id)
    expect(failed.state).toBe('failed')
    expect(failed.error).toContain('page 1')
    expect(failed.ocrProfile).toBe('vl')

    await failDewarp(request, 'off')

    // The retry names the other model. The copy keeps the one it was started
    // with, because the server reads the profile off the copy.
    await startOCR(request, book.id, 'v6')

    const retried = await waitForSettled(request, copy.id)
    expect(retried.id).toBe(copy.id)
    expect(retried.state).toBe('complete')
    expect(retried.ocrProfile).toBe('vl')
    expect(retried.pages).toHaveLength(book.pages.length)

    for (let index = 1; index <= retried.pages.length; index++) {
      const results = await pageResults(index, copy.id)
      expect(Object.keys(results).sort()).toEqual(['merged', 'v6', 'vl'])
    }
  })
})

test.describe('a read page holds a box per line', () => {
  test('the stored result places every line on the page', async ({ request }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    const results = await pageResults(1, copy.id)
    const lines = (results.v6.data as V6Result).lines
    expect(lines.length).toBeGreaterThan(1)

    const first = settled.pages[0]
    const tops = new Set<number>()
    for (const line of lines) {
      const [left, top, right, bottom] = line.bbox
      expect(left).toBeGreaterThanOrEqual(0)
      expect(top).toBeGreaterThanOrEqual(0)
      expect(right).toBeGreaterThan(left)
      expect(bottom).toBeGreaterThan(top)
      expect(right).toBeLessThanOrEqual(first.width)
      expect(bottom).toBeLessThanOrEqual(first.height)
      tops.add(top)
    }
    expect(tops.size).toBe(lines.length)
  })

  test('the merged result takes the text of the blocks', async ({ request }) => {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    const results = await pageResults(1, copy.id)
    const v6 = (results.v6.data as V6Result).lines
    const merged = (results.merged.data as MergedResult).lines

    expect(merged).toHaveLength(v6.length)
    for (const [index, line] of merged.entries()) {
      expect(line.bbox).toEqual(v6[index].bbox)
      expect(line.source).toBe('vl')
    }
  })
})

test.describe('the read half of the reader offers the rescan alone', () => {
  async function openReader(
    page: PlaywrightPage,
    request: APIRequestContext,
    profile: OcrProfile = 'vl',
  ) {
    const book = await seedBook(request)
    const copy = await startOCR(request, book.id, profile)
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()
  }

  // Playwright cannot see the menu of the browser, so the test watches the
  // event instead: a prevented default is the menu that never opens.
  function watchDefault(page: PlaywrightPage) {
    return page.evaluate(
      () =>
        new Promise<boolean>((resolve) => {
          document.addEventListener(
            'contextmenu',
            (event) => resolve(event.defaultPrevented),
            { once: true },
          )
        }),
    )
  }

  // A reading is a choice, not a rank. A page the visual engine has read offers
  // the rescan like any other, so its read half opens the same one item menu.
  test('a right click on an already read half offers the rescan', async ({ page, request }) => {
    await openReader(page, request, 'vl')

    const watching = watchDefault(page)
    await page.getByTestId('ocr-overlay').first().click({ button: 'right' })

    expect(await watching).toBe(true)
    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-rescan')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toHaveCount(0)
  })

  test('a right click on the read half of a v6 page offers the rescan alone', async ({
    page,
    request,
  }) => {
    await openReader(page, request, 'v6')

    const watching = watchDefault(page)
    await page.getByTestId('ocr-overlay').first().click({ button: 'right' })

    expect(await watching).toBe(true)
    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-rescan')).toBeVisible()
    await expect(page.getByTestId('page-context-open')).toHaveCount(0)
    await expect(page.getByTestId('page-context-download')).toHaveCount(0)
    await expect(page.getByTestId('page-context-delete')).toHaveCount(0)
  })

  // The page half reaches the image and changes the page. Everything about the
  // reading — which reading answers, why it is doubted, and the rescan that would
  // read it again — belongs to the read half.
  test('the page half of a v6 page holds nothing about the reading', async ({
    page,
    request,
  }) => {
    await openReader(page, request, 'v6')

    await page.locator('[data-page="1"] img.plain').click({ button: 'right' })

    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-open')).toBeVisible()
    await expect(page.getByTestId('page-context-download')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toBeVisible()

    await expect(page.getByTestId('page-context-rescan')).toHaveCount(0)
    await expect(page.getByTestId('page-context-reading:v6')).toHaveCount(0)
    await expect(page.getByTestId('page-context-disagreement')).toHaveCount(0)
  })

  test('the page half of an already read page holds no rescan either', async ({
    page,
    request,
  }) => {
    await openReader(page, request, 'vl')

    await page.locator('[data-page="1"] img.plain').click({ button: 'right' })

    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
    await expect(page.getByTestId('page-context-rescan')).toHaveCount(0)

    // The read half of the same page does hold it.
    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await expect(page.getByTestId('page-context-rescan')).toBeVisible()
  })

  test('a right click on the gap opens nothing at all', async ({ page, request }) => {
    await openReader(page, request)

    // The two halves are equal and centred, so the centre of the spread, which
    // is where a click lands by default, is the gap between them.
    const watching = watchDefault(page)
    await page.locator('[data-page="1"]').click({ button: 'right' })

    expect(await watching).toBe(true)
    await expect(page.getByTestId('page-context-menu')).toHaveCount(0)
  })

  test('a right click on the page half opens the page menu', async ({ page, request }) => {
    await openReader(page, request)

    await page.locator('[data-page="1"] img.plain').click({ button: 'right' })

    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-open')).toBeVisible()
    await expect(page.getByTestId('page-context-download')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
  })

  test('the drawn text stays selectable', async ({ page, request }) => {
    await openReader(page, request)

    const line = page.getByTestId('ocr-line').first()
    await expect(line).toBeVisible()
    const box = await line.boundingBox()
    expect(box).not.toBeNull()

    const middle = box!.y + box!.height / 2
    await page.mouse.move(box!.x + 1, middle)
    await page.mouse.down()
    await page.mouse.move(box!.x + box!.width - 1, middle, { steps: 8 })
    await page.mouse.up()

    const selected = await page.evaluate(() => window.getSelection()?.toString() ?? '')
    expect(selected.trim()).not.toBe('')
  })
})
