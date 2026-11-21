import { expect, test } from '@playwright/test'
import type { APIRequestContext, Page } from '@playwright/test'
import {
  failDewarp,
  getBook,
  imageSet,
  ingest,
  objectKeys,
  pageLines,
  pageResults,
  rescanPages,
  rescanRefusal,
  reset,
  resetDewarp,
  startOCR,
  waitForSettled,
  type Book,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

// readWithV6 makes a book read by PP-OCRv6 alone, which is what a rescan adds
// to. Every page holds one result and no merged result.
async function readWithV6(request: APIRequestContext, name = 'Normal'): Promise<Book> {
  const book = await ingest(request, name, imageSet('portrait'))
  const copy = await startOCR(request, book.id, 'v6')
  await waitForSettled(request, copy.id)
  return copy
}

test.describe('a rescan reads chosen pages again with Extra effort', () => {
  test('the named pages gain the visual and merged results, and no other page does', async ({
    request,
  }) => {
    const copy = await readWithV6(request)
    const detail = await getBook(request, copy.id)
    expect(detail.pages.length).toBeGreaterThanOrEqual(3)

    await rescanPages(request, copy.id, [1, 2])
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('complete')
    expect(settled.pageCount).toBe(detail.pages.length)

    for (const index of [1, 2]) {
      const results = await pageResults(index, copy.id)
      expect(Object.keys(results).sort()).toEqual(['merged', 'v6', 'vl'])
      expect(results.merged.txt).not.toBe('')
    }

    const untouched = await pageResults(3, copy.id)
    expect(Object.keys(untouched)).toEqual(['v6'])
  })

  test('the box result of a rescanned page is unchanged', async ({ request }) => {
    const copy = await readWithV6(request)

    const before = (await pageResults(1, copy.id)).v6
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)
    const after = (await pageResults(1, copy.id)).v6

    expect(after.txt).toBe(before.txt)
    expect(after.data).toEqual(before.data)
  })

  test('the book keeps the v6 profile and every page still offers the rescan', async ({
    request,
  }) => {
    const copy = await readWithV6(request)

    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    const detail = await getBook(request, copy.id)
    expect(detail.ocrProfile).toBe('v6')
    expect(detail.pageRescan).toBe(true)

    // A page the visual engine has read is read again like any other.
    await rescanPages(request, copy.id, [1])
    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')

    const results = await pageResults(1, copy.id)
    expect(Object.keys(results).sort()).toEqual(['merged', 'v6', 'vl'])
  })

  test('a normal rescan replaces the reading and drops the visual one', async ({
    request,
  }) => {
    const book = await ingest(request, 'Extra', imageSet('portrait'))
    const copy = await startOCR(request, book.id, 'vl')
    await waitForSettled(request, copy.id)

    const before = await pageResults(1, copy.id)
    expect(Object.keys(before).sort()).toEqual(['merged', 'v6', 'vl'])

    await rescanPages(request, copy.id, [1], 'v6')
    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')

    // The page answers with the line engine alone. The merged result derives
    // from boxes this reading replaced, so it goes with the visual one.
    const after = await pageResults(1, copy.id)
    expect(Object.keys(after)).toEqual(['v6'])
    expect(after.v6.txt).not.toBe('')

    const lines = await pageLines(request, copy.id, 1)
    expect(lines.model).toBe('v6')

    // The book keeps the profile of the reading that built it, and the page the
    // rescan did not name keeps every result it held.
    const detail = await getBook(request, copy.id)
    expect(detail.ocrProfile).toBe('vl')

    const untouched = await pageResults(2, copy.id)
    expect(Object.keys(untouched).sort()).toEqual(['merged', 'v6', 'vl'])
  })

  test('a rescan removes its exchange objects', async ({ request }) => {
    const copy = await readWithV6(request)

    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    expect(await objectKeys(request, 'ocr/')).toHaveLength(0)
  })
})

test.describe('a rescan that cannot finish leaves the book as it was', () => {
  test('a page the endpoint refuses costs the pages it read nothing', async ({ request }) => {
    const copy = await readWithV6(request)

    // mode=page fails the first page of the job, which is page 1 here.
    await failDewarp(request, 'page')
    await rescanPages(request, copy.id, [1, 2])
    const settled = await waitForSettled(request, copy.id)
    await failDewarp(request, 'off')

    expect(settled.state).toBe('complete')
    expect(settled.error).toBeNull()

    const missed = await pageResults(1, copy.id)
    expect(Object.keys(missed)).toEqual(['v6'])

    const read = await pageResults(2, copy.id)
    expect(Object.keys(read).sort()).toEqual(['merged', 'v6', 'vl'])

    // Both pages offer the rescan again: the one that did not take and the one
    // that did.
    const detail = await getBook(request, copy.id)
    expect(detail.pageRescan).toBe(true)
  })

  test('a whole failure completes the book and offers no retry', async ({ request }) => {
    const copy = await readWithV6(request)
    const before = await getBook(request, copy.id)

    await failDewarp(request, 'job')
    await rescanPages(request, copy.id, [1, 2])
    const settled = await waitForSettled(request, copy.id)
    await failDewarp(request, 'off')

    expect(settled.state).toBe('complete')
    expect(settled.error).toBeNull()
    expect(settled.progress).toBeNull()
    expect(settled.pageCount).toBe(before.pages.length)

    // Every page holds what it held before, so nothing was lost.
    for (const page of before.pages) {
      const results = await pageResults(page.index, copy.id)
      expect(Object.keys(results)).toEqual(['v6'])
    }

    // A failed book would offer a retry, and that retry deletes every page of
    // the book. The card must not offer one.
    const detail = await getBook(request, copy.id)
    expect(detail.state).toBe('complete')
    expect(detail.error).toBeNull()
  })
})

test.describe('the reader marks pages and starts the rescan', () => {
  test('marking pages, confirming, and landing back in the stage list', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request, 'Marked')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()

    await expect(page.getByTestId('rescan-marked')).toHaveCount(0)

    // Mark page 1 from the read half, and page 2 from the page half.
    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    await expect(page.getByTestId('rescan-marked')).toHaveText(/Rescan 1 page/)
    await expect(page.locator('[data-page="1"][data-marked="true"]')).toHaveCount(1)

    await page
      .locator('[data-page="2"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    await expect(page.getByTestId('rescan-marked')).toHaveText(/Rescan 2 pages/)
    await expect(page.locator('[data-marked="true"]')).toHaveCount(2)

    // Cancelling starts nothing and keeps every mark.
    await page.getByTestId('rescan-marked').click()
    await expect(page.getByTestId('rescan-dialog')).toBeVisible()
    await page.getByTestId('rescan-cancel').click()
    await expect(page.getByTestId('rescan-dialog')).toHaveCount(0)
    await expect(page.locator('[data-marked="true"]')).toHaveCount(2)

    // The dialog asks for the model, and opens on Extra.
    await page.getByTestId('rescan-marked').click()
    await expect(page.getByTestId('rescan-profile-v6')).toBeVisible()
    await expect(page.getByTestId('rescan-profile-vl')).toBeVisible()
    await expect(page.getByTestId('rescan-profile-vl').locator('input')).toBeChecked()
    await page.getByTestId('rescan-start').click()

    // Starting the rescan leaves the reader for the list of the OCR stage.
    await expect(page).toHaveURL(/\/stage\/ocr$/)
    await expect(page.getByTestId('stage-books')).toBeVisible()

    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')

    for (const index of [1, 2]) {
      const results = await pageResults(index, copy.id)
      expect(Object.keys(results).sort()).toEqual(['merged', 'v6', 'vl'])
    }
  })

  test('the mark borders the read half alone', async ({ page, request }) => {
    const copy = await readWithV6(request, 'Outlined')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    const spread = page.locator('[data-page="1"]')
    await expect(spread).toHaveCSS('outline-style', 'none')

    // The border is drawn inside the read half, above the page. It cannot be an
    // outline offset outwards: the spread carries paint containment, which would
    // clip it on three edges of four.
    expect(await ringOf(page, '[data-page="1"] [data-testid="ocr-overlay"]')).toContain('inset')
    expect(await ringOf(page, '[data-page="1"] img.plain')).not.toContain('inset')
  })

  // ringOf answers the border of a half, which is painted by the ::after of that
  // element and so is out of reach of toHaveCSS.
  async function ringOf(page: Page, selector: string): Promise<string> {
    return page.evaluate((css) => {
      const element = document.querySelector(css)
      if (!element) {
        return ''
      }
      return getComputedStyle(element, '::after').boxShadow
    }, selector)
  }

  // The item says what choosing it does: it marks a page that is not marked and
  // takes the mark off one that is.
  test('the item takes a mark off a page that carries one', async ({ page, request }) => {
    const copy = await readWithV6(request, 'Toggled')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await expect(page.getByTestId('page-context-rescan')).toHaveText('Rescan')
    await page.getByTestId('page-context-rescan').click()

    await page
      .locator('[data-page="2"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()
    await expect(page.getByTestId('rescan-marked')).toHaveText(/Rescan 2 pages/)

    // The menu of a marked page offers to undo the mark, and nothing to make it
    // again.
    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await expect(page.getByTestId('page-context-unrescan')).toHaveText('Do not rescan')
    await expect(page.getByTestId('page-context-rescan')).toHaveCount(0)
    await page.getByTestId('page-context-unrescan').click()

    // That page loses its mark and its border, and the other keeps both.
    await expect(page.locator('[data-page="1"][data-marked="true"]')).toHaveCount(0)
    await expect(page.locator('[data-page="2"][data-marked="true"]')).toHaveCount(1)
    await expect(page.getByTestId('rescan-marked')).toHaveText(/Rescan 1 page/)

    // The read half offers the same item, and taking off the last mark takes the
    // control with it.
    await page.locator('[data-page="2"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await expect(page.getByTestId('page-context-unrescan')).toHaveText('Do not rescan')
    await page.getByTestId('page-context-unrescan').click()

    await expect(page.locator('[data-marked="true"]')).toHaveCount(0)
    await expect(page.getByTestId('rescan-marked')).toHaveCount(0)

    // And the page can be marked again afterwards.
    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await expect(page.getByTestId('page-context-rescan')).toHaveText('Rescan')
    await page.getByTestId('page-context-rescan').click()
    await expect(page.getByTestId('rescan-marked')).toHaveText(/Rescan 1 page/)
  })

  test('the control stands after Reject', async ({ page, request }) => {
    const copy = await readWithV6(request, 'Ordered')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    const reject = await page.getByTestId('action-reject').boundingBox()
    const rescan = await page.getByTestId('rescan-marked').boundingBox()
    expect(reject).not.toBeNull()
    expect(rescan).not.toBeNull()
    expect(rescan!.x).toBeGreaterThan(reject!.x)
  })

  // A page the visual engine has read offers the rescan like any other, from
  // both halves of the spread.
  test('a page read with Extra still opens the rescan menu', async ({ page, request }) => {
    const book = await ingest(request, 'Extra reader', imageSet('portrait'))
    const copy = await startOCR(request, book.id, 'vl')
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()

    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await expect(page.getByTestId('page-context-rescan')).toBeVisible()
    await page.keyboard.press('Escape')

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await expect(page.getByTestId('page-context-rescan')).toBeVisible()
  })

  test('the grid offers no rescan', async ({ page, request }) => {
    const copy = await readWithV6(request, 'Gridded')

    await page.goto(`/books/${copy.id}`)
    await expect(page.getByTestId('page-grid')).toBeVisible()

    await page.getByTestId('page-cell-1').click({ button: 'right' })

    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
    await expect(page.getByTestId('page-context-rescan')).toHaveCount(0)
  })
})

test.describe('the system refuses a rescan it cannot run', () => {
  test('a book that has not been read', async ({ request }) => {
    const book = await ingest(request, 'Unread', imageSet('portrait'))

    const refused = await rescanRefusal(request, book.id, [1])
    expect(refused.status).toBe(422)
    expect(refused.error).not.toBe('')
  })

  test('a page the book does not hold', async ({ request }) => {
    const copy = await readWithV6(request)

    const refused = await rescanRefusal(request, copy.id, [1, 900])
    expect(refused.status).toBe(422)
    expect(refused.error).toContain('900')

    // The whole request is refused, so page 1 is not read either.
    const results = await pageResults(1, copy.id)
    expect(Object.keys(results)).toEqual(['v6'])
  })

  test('no page at all', async ({ request }) => {
    const copy = await readWithV6(request)

    const refused = await rescanRefusal(request, copy.id, [])
    expect(refused.status).toBe(422)
    expect(refused.error).not.toBe('')
  })

  test('no profile at all', async ({ request }) => {
    const copy = await readWithV6(request)

    const refused = await rescanRefusal(request, copy.id, [1], null)
    expect(refused.status).toBe(422)
    expect(refused.error).not.toBe('')

    const settled = await getBook(request, copy.id)
    expect(settled.state).toBe('complete')

    const results = await pageResults(1, copy.id)
    expect(Object.keys(results)).toEqual(['v6'])
  })

  test('a profile the system does not know', async ({ request }) => {
    const copy = await readWithV6(request)

    const refused = await rescanRefusal(request, copy.id, [1], 'nonsense')
    expect(refused.status).toBe(422)
    expect(refused.error).toContain('nonsense')
  })
})
