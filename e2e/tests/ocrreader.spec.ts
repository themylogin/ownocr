import { expect, test } from '@playwright/test'
import {
  fixture,
  imageSet,
  ingest,
  pageResults,
  reset,
  resetDewarp,
  startOCR,
  startSplit,
  waitForSettled,
  type V6Result,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

test.describe('the reader of an OCR book shows the text beside the page', () => {
  test('every page appears twice, the right one veiled and drawn on', async ({ page, request }) => {
    const book = await ingest(request, 'Read Me', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    const stored = (await pageResults(1, copy.id)).merged.data as V6Result

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const first = page.locator('[data-page="1"]')
    await expect(first.locator('img')).toHaveCount(2)
    await expect(first.getByTestId('ocr-overlay')).toBeVisible()
    await expect(first.getByTestId('ocr-line')).toHaveCount(stored.lines.length)

    // The veil is white and lets the page through.
    const veil = first.getByTestId('ocr-overlay').locator('.veil')
    await expect(veil).toHaveCSS('opacity', '0.5')
    await expect(veil).toHaveCSS('background-color', 'rgb(255, 255, 255)')

    for (const [index, line] of stored.lines.entries()) {
      await expect(first.getByTestId('ocr-line').nth(index)).toHaveText(line.text)
    }

    expect(settled.pages).toHaveLength(book.pages.length)
  })

  test('every ground is drawn below every line', async ({ page, request }) => {
    const book = await ingest(request, 'Layered', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    const stored = (await pageResults(1, copy.id)).merged.data as V6Result

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const overlay = page.locator('[data-page="1"]').getByTestId('ocr-overlay')
    await expect(overlay.getByTestId('ocr-line')).toHaveCount(stored.lines.length)

    // SVG paints in document order, so a ground that follows a line would cover
    // the glyphs of that line which reach out of their own box. One path holds
    // every ground of the page, so the whole ground stands before the first
    // glyph and one subpath answers for each line.
    const painted = await overlay.locator('svg').evaluate((node) =>
      Array.from(node.querySelectorAll('path, text')).map((child) => child.tagName.toLowerCase()),
    )

    expect(painted).toHaveLength(stored.lines.length + 1)
    expect(painted.lastIndexOf('path')).toBeLessThan(painted.indexOf('text'))

    const subpaths = await overlay
      .locator('.grounds path')
      .evaluate((node) => (node.getAttribute('d') ?? '').split('Z').filter(Boolean).length)

    expect(subpaths).toBe(stored.lines.length)
  })

  test('a drawn line sits where its stored box says', async ({ page, request }) => {
    const book = await ingest(request, 'Placed', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    const stored = (await pageResults(1, copy.id)).merged.data as V6Result

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const overlay = page.locator('[data-page="1"]').getByTestId('ocr-overlay')
    const frame = await overlay.boundingBox()
    if (!frame) {
      throw new Error('the overlay has no box')
    }

    const scale = frame.width / settled.pages[0].width
    for (const [index, line] of stored.lines.entries()) {
      const drawn = await overlay.getByTestId('ocr-line').nth(index).boundingBox()
      if (!drawn) {
        throw new Error(`line ${index} has no box`)
      }

      const [left, top, right, bottom] = line.bbox
      expect(drawn.x - frame.x).toBeCloseTo(left * scale, 0)
      expect(drawn.width).toBeCloseTo((right - left) * scale, 0)
      expect(drawn.y - frame.y).toBeGreaterThanOrEqual(top * scale - 2)
      expect(drawn.y - frame.y + drawn.height).toBeLessThanOrEqual(bottom * scale + 2)
    }
  })

  test('both halves fit the window and take one scale', async ({ page, request }) => {
    const book = await ingest(request, 'Fitted', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    // Two stored pages are wider than this window, so both halves scale down.
    await page.setViewportSize({ width: 700, height: 800 })
    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const row = page.locator('[data-page="1"]')
    const plain = await row.locator('img.plain').boundingBox()
    const overlay = await row.getByTestId('ocr-overlay').boundingBox()
    if (!plain || !overlay) {
      throw new Error('a half has no box')
    }

    expect(plain.width).toBeCloseTo(overlay.width, 0)
    expect(plain.height).toBeCloseTo(overlay.height, 0)
    expect(plain.width + overlay.width).toBeLessThanOrEqual(700)
    expect(plain.width).toBeLessThan(settled.pages[0].width)

    const narrowLine = await row.getByTestId('ocr-line').first().boundingBox()

    // The page is never enlarged past its stored size, however wide the window.
    await page.setViewportSize({ width: 1900, height: 800 })
    await expect
      .poll(async () => (await row.locator('img.plain').boundingBox())?.width ?? 0)
      .toBeGreaterThan(plain.width)

    const wider = await row.locator('img.plain').boundingBox()
    const widerLine = await row.getByTestId('ocr-line').first().boundingBox()
    if (!wider || !narrowLine || !widerLine) {
      throw new Error('a half or a line has no box')
    }

    expect(wider.width).toBeGreaterThan(plain.width)
    expect(wider.width).toBeLessThanOrEqual(settled.pages[0].width)

    // The boxes follow the image, so the drawn text grows by the same factor.
    expect(widerLine.width / narrowLine.width).toBeCloseTo(wider.width / plain.width, 1)
  })

  test('the OCR reader uses the whole window', async ({ page, request }) => {
    const book = await ingest(request, 'Wide', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    await page.setViewportSize({ width: 1900, height: 800 })

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    const column = await page.getByTestId('content').boundingBox()

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    const whole = await page.getByTestId('content').boundingBox()

    if (!column || !whole) {
      throw new Error('the content has no box')
    }
    expect(column.width).toBeLessThanOrEqual(72 * 16)
    expect(whole.width).toBeGreaterThan(column.width)

    // Leaving the reader gives the column back.
    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    const again = await page.getByTestId('content').boundingBox()
    expect(again?.width).toBeCloseTo(column.width, 0)
  })

  test('the drawn text is the text of the lines', async ({ page, request }) => {
    const book = await ingest(request, 'Selectable', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    const stored = (await pageResults(1, copy.id)).merged.data as V6Result

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const row = page.locator('[data-page="1"]')
    const selected = await row.evaluate((node) => {
      const range = document.createRange()
      range.selectNodeContents(node)
      const selection = window.getSelection()
      selection?.removeAllRanges()
      selection?.addRange(range)
      return selection?.toString() ?? ''
    })

    for (const line of stored.lines) {
      expect(selected).toContain(line.text)
    }

    // The left image carries no alt text, so a selection holds text alone.
    await expect(row.locator('img.plain')).toHaveAttribute('alt', '')
  })

  test('a copy read by one engine draws the engine lines', async ({ page, request }) => {
    const book = await ingest(request, 'Normal', imageSet('portrait'))
    const copy = await startOCR(request, book.id, 'v6')
    await waitForSettled(request, copy.id)

    const results = await pageResults(1, copy.id)
    expect(Object.keys(results)).toEqual(['v6'])
    const stored = results.v6.data as V6Result

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const first = page.locator('[data-page="1"]')
    await expect(first.getByTestId('ocr-line')).toHaveCount(stored.lines.length)
    await expect(first.getByTestId('ocr-line').first()).toHaveText(stored.lines[0].text)
  })

  test('a book of another stage keeps the single column', async ({ page, request }) => {
    const book = await ingest(request, 'Split Only', imageSet('spread'))
    const split = await startSplit(request, book.id)
    const settled = await waitForSettled(request, split.id)
    expect(settled.stage).toBe('split')

    await page.goto(`/books/${split.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await expect(page.locator('[data-page="1"] img')).toHaveCount(1)
    await expect(page.getByTestId('ocr-overlay')).toHaveCount(0)
    await expect(page.getByTestId('ocr-line')).toHaveCount(0)
  })
})

test.describe('the OCR reader holds the text of every page', () => {
  test('a long book draws every page, and asks for the book once', async ({ page, request }) => {
    const book = await ingest(request, 'Sixty Pager', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])
    expect(book.pages).toHaveLength(60)

    const copy = await startOCR(request, book.id, 'v6')
    const settled = await waitForSettled(request, copy.id, 180_000)
    expect(settled.state).toBe('complete')

    const asked: string[] = []
    page.on('request', (request) => {
      if (request.url().includes('/ocr')) {
        asked.push(request.url())
      }
    })

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await expect(page.getByTestId('ocr-overlay').first()).toBeVisible()
    await page.waitForTimeout(2000)

    // One request for the book, and none for a page.
    expect(asked).toHaveLength(1)
    expect(asked[0]).toContain(`/api/books/${copy.id}/ocr`)

    const drawnPages = () =>
      page.evaluate(
        () =>
          new Set(
            Array.from(document.querySelectorAll('[data-testid="ocr-line"]')).map((node) =>
              node.closest('[data-page]')?.getAttribute('data-page'),
            ),
          ).size,
      )

    expect(await drawnPages()).toBe(settled.pages.length)

    // A page far below the window holds its text before anyone scrolls to it.
    await expect(page.locator('[data-page="60"]').getByTestId('ocr-line').first()).toHaveCount(1)

    await page.locator('[data-page="40"]').scrollIntoViewIfNeeded()
    await expect(page.locator('[data-page="40"]').getByTestId('ocr-line').first()).toBeVisible()
    await page.waitForTimeout(2000)

    // The pages left behind keep their text, and nothing was asked for again.
    await expect(page.locator('[data-page="1"]').getByTestId('ocr-line').first()).toHaveCount(1)
    expect(await drawnPages()).toBe(settled.pages.length)
    expect(asked).toHaveLength(1)
  })

  test('a page off screen reserves the size it takes', async ({ page, request }) => {
    const book = await ingest(request, 'Reserved', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])
    const copy = await startOCR(request, book.id, 'v6')
    const settled = await waitForSettled(request, copy.id, 180_000)
    expect(settled.state).toBe('complete')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const reserved = await page.locator('[data-page="60"]').evaluate((node) => {
      const style = getComputedStyle(node)
      return {
        visibility: style.getPropertyValue('content-visibility'),
        size: style.getPropertyValue('contain-intrinsic-size'),
        height: node.getBoundingClientRect().height,
      }
    })

    expect(reserved.visibility).toBe('auto')
    expect(reserved.size).not.toBe('')
    expect(reserved.size).not.toBe('none')
    expect(reserved.height).toBeGreaterThan(0)
  })

  test('the lines of the book do not arrive', async ({ page, request }) => {
    const book = await ingest(request, 'Silent', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    await page.route(`**/api/books/${copy.id}/ocr`, (route) =>
      route.fulfill({ status: 500, body: '{"error":"nope"}' }),
    )

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    // The pages keep their veiled images, and the reader stays quiet.
    await expect(page.locator('[data-page="1"]').getByTestId('ocr-overlay')).toBeVisible()
    await expect(page.locator('[data-page="1"]').locator('img')).toHaveCount(2)
    await expect(page.getByTestId('ocr-line')).toHaveCount(0)
    await expect(page.getByTestId('reader-error')).toHaveCount(0)
  })

  test('the reader opens at a late page with its text drawn', async ({ page, request }) => {
    const book = await ingest(request, 'Late Start', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])
    const copy = await startOCR(request, book.id, 'v6')
    const settled = await waitForSettled(request, copy.id, 180_000)
    expect(settled.state).toBe('complete')

    await page.goto(`/books/${copy.id}/pages/55`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const target = page.locator('[data-page="55"]')
    await expect(target.getByTestId('ocr-line').first()).toBeVisible()

    // The reader landed on the page the route named, not at the top.
    const placed = await target.evaluate((node) => ({
      top: node.getBoundingClientRect().top,
      viewport: window.innerHeight,
    }))
    expect(Math.abs(placed.top)).toBeLessThan(placed.viewport)
  })

  // The rule that finds a trim is proven on real pages in the Go tests. This
  // proves the reader draws what the server answers: the strip covered, and no
  // glyph of it drawn.
  test('a page carrying a strip of the page beside it shows it covered', async ({
    page,
    request,
  }) => {
    const book = await ingest(request, 'Trimmed', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    const settled = await waitForSettled(request, copy.id)
    const first = settled.pages[0]

    await page.route(`**/api/books/${copy.id}/ocr`, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          pages: [
            {
              index: 1,
              model: 'v6',
              width: first.width,
              height: first.height,
              trim: { side: 'left', column: 122 },
              lines: [
                { box: [182, 285, 1805, 347], text: 'a line of the page itself' },
                { box: [182, 361, 1805, 423], text: 'a second line of the page' },
              ],
            },
          ],
        }),
      }),
    )

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const target = page.locator('[data-page="1"]')
    const trim = target.getByTestId('ocr-trim')
    await expect(trim).toHaveCount(1)
    await expect(trim).toHaveAttribute('x', '0')
    await expect(trim).toHaveAttribute('width', '122')
    await expect(trim).toHaveAttribute('height', String(first.height))

    // Opaque, unlike the veil: it covers print of another page.
    await expect(trim).toHaveCSS('fill', 'rgb(255, 255, 255)')

    // The server dropped the fragments, so the reader draws the page alone.
    await expect(target.getByTestId('ocr-line')).toHaveCount(2)
    for (const box of await target.getByTestId('ocr-line').all()) {
      expect(Number(await box.getAttribute('x'))).toBeGreaterThanOrEqual(122)
    }

    // The plain half beside it is the untouched photo and carries no trim.
    await expect(target.locator('img')).toHaveCount(2)
    await expect(target.locator('.plain')).toBeVisible()
  })

  test('a page that carries no strip draws no trim', async ({ page, request }) => {
    const book = await ingest(request, 'Untrimmed', imageSet('portrait'))
    const copy = await startOCR(request, book.id)
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const target = page.locator('[data-page="1"]')
    await expect(target.getByTestId('ocr-line').first()).toBeVisible()
    await expect(target.getByTestId('ocr-trim')).toHaveCount(0)
  })
})
