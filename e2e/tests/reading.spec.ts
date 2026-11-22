import { expect, test } from '@playwright/test'
import type { APIRequestContext, Locator, Page } from '@playwright/test'
import { Client } from 'pg'
import {
  getBook,
  imageSet,
  ingest,
  objectKeys,
  pageLines,
  pageResults,
  rescanPages,
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

// readWithV6 makes a book read by PP-OCRv6 alone: every page holds one reading,
// which is the state a rescan adds to.
async function readWithV6(request: APIRequestContext, name = 'Choice'): Promise<Book> {
  const book = await ingest(request, name, imageSet('portrait'))
  const copy = await startOCR(request, book.id, 'v6')
  await waitForSettled(request, copy.id)
  return copy
}

test.describe('a page menu lists the readings of its page', () => {
  test('a page read once lists that reading and marks it', async ({ page, request }) => {
    const copy = await readWithV6(request)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })

    const reading = page.getByTestId('page-context-reading:v6')
    await expect(reading).toBeVisible()
    await expect(reading).toContainText('Normal (PP-OCRv6)')

    // The mark sits on the reading that answers.
    await expect(reading.locator('.n-dropdown-option-body__prefix')).toHaveText('✓')
  })

  test('a page read twice lists both readings and marks the better one', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })

    await expect(page.getByTestId('page-context-reading:merged')).toContainText(
      'Extra (PaddleOCR-VL)',
    )
    await expect(page.getByTestId('page-context-reading:v6')).toContainText('Normal (PP-OCRv6)')

    // The raw visual reading holds no lines, so it is not offered.
    await expect(page.getByTestId('page-context-reading:vl')).toHaveCount(0)
  })

  // The read half is about the reading: the readings it can answer, why that
  // reading is doubted, then the rescan. Normal, Extra, Ultra — the order the
  // dialog that starts a rescan offers them in.
  test('the read half orders the readings Normal, Extra, Ultra', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)
    await rescanPages(request, copy.id, [1], 'glm')
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })

    const tops = await page.evaluate(() =>
      ['v6', 'merged', 'glm_merged', 'rescan'].map((key) => {
        const element = document.querySelector(
          key === 'rescan'
            ? '[data-testid="page-context-rescan"]'
            : `[data-testid="page-context-reading:${key}"]`,
        )
        return element ? element.getBoundingClientRect().top : -1
      }),
    )
    expect(tops.every((top) => top >= 0)).toBe(true)
    expect(tops[0]).toBeLessThan(tops[1])
    expect(tops[1]).toBeLessThan(tops[2])
    expect(tops[2]).toBeLessThan(tops[3])

    // And the page half holds none of it.
    await page.locator('[data-page="1"] img.plain').click({ button: 'right' })
    await expect(page.getByTestId('page-context-reading:v6')).toHaveCount(0)
    await expect(page.getByTestId('page-context-rescan')).toHaveCount(0)
    await expect(page.getByTestId('page-context-open')).toBeVisible()
  })

  test('a page of a book that has not been read lists no reading', async ({ page, request }) => {
    const book = await ingest(request, 'Unread', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="1"]').click({ button: 'right' })

    await expect(page.getByTestId('page-context-open')).toBeVisible()
    await expect(page.getByTestId('page-context-reading:v6')).toHaveCount(0)
  })
})

test.describe('a reader chooses which reading a page answers', () => {
  test('choosing another reading changes the drawn text and survives a reload', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    // Page 2 is rescanned too, so a choice made on page 1 has a neighbour that
    // could wrongly follow it.
    await rescanPages(request, copy.id, [1, 2])
    await waitForSettled(request, copy.id)

    const merged = (await pageLines(request, copy.id, 1)).lines.map((line) => line.text)
    const lineReading = (await pageResults(1, copy.id)).v6.txt.split('\n')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const drawn = page.locator('[data-page="1"]').getByTestId('ocr-line')
    await expect(drawn.first()).toHaveText(merged[0])

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-reading:v6').click()

    // The page draws the line reading it now answers.
    await expect(drawn.first()).toHaveText(lineReading[0])

    // And it still does after a reload, because the choice is stored.
    await page.reload()
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await expect(page.locator('[data-page="1"]').getByTestId('ocr-line').first()).toHaveText(
      lineReading[0],
    )

    // The mark moved with it.
    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await expect(
      page.getByTestId('page-context-reading:v6').locator('.n-dropdown-option-body__prefix'),
    ).toHaveText('✓')

    // The page the reader did not choose for still answers the better reading.
    expect((await pageLines(request, copy.id, 2)).model).toBe('merged')
    expect((await pageLines(request, copy.id, 1)).model).toBe('v6')
  })

  test('a better reading clears the choice a reader made', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-reading:v6').click()
    await expect(page.getByTestId('page-context-menu')).toHaveCount(0)

    expect((await pageLines(request, copy.id, 1)).model).toBe('v6')

    await rescanPages(request, copy.id, [1], 'glm')
    await waitForSettled(request, copy.id)

    // A reader picks a reading because it is the best the page holds, so a better
    // one arriving answers the same want: the choice goes and the page answers the
    // best it holds.
    expect((await pageLines(request, copy.id, 1)).model).toBe('glm_merged')

    const results = await pageResults(1, copy.id)
    expect(Object.keys(results)).toContain('glm_merged')

    // And the reader can still choose the line reading again.
    await page.reload()
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await page.getByTestId('page-context-reading:v6').click()
    expect((await pageLines(request, copy.id, 1)).model).toBe('v6')
  })
})

test.describe('an Ultra rescan reads a page on the metered API', () => {
  test('the page gains the Ultra readings, answers the merged one, and no object is uploaded', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    const detail = await getBook(request, copy.id)

    await rescanPages(request, copy.id, [1], 'glm')
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('complete')
    expect(settled.pageCount).toBe(detail.pages.length)

    const results = await pageResults(1, copy.id)
    expect(Object.keys(results).sort()).toEqual(['glm', 'glm_merged', 'v6'])
    expect(results.glm.txt).not.toBe('')

    // The merged Ultra reading carries the boxes of the line reading.
    const drawn = await pageLines(request, copy.id, 1)
    expect(drawn.model).toBe('glm_merged')
    const boxes = (await pageResults(1, copy.id)).v6.data as { lines: Array<{ bbox: number[] }> }
    expect(drawn.lines.map((line) => line.box)).toEqual(boxes.lines.map((line) => line.bbox))

    // It reaches no bucket: the page image is already on the server's disk.
    expect(await objectKeys(request, 'ocr/')).toEqual([])

    // A page the rescan did not name gains nothing.
    expect(Object.keys(await pageResults(2, copy.id))).toEqual(['v6'])

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })

    const ultra = page.getByTestId('page-context-reading:glm_merged')
    await expect(ultra).toContainText('Ultra (GLM-5.3 Flash)')
    await expect(ultra.locator('.n-dropdown-option-body__prefix')).toHaveText('✓')
  })

  test('the rescan dialog offers Ultra and the OCR dialog does not', async ({ page, request }) => {
    const copy = await readWithV6(request)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()
    await page.getByTestId('rescan-marked').click()

    await expect(page.getByTestId('rescan-dialog')).toBeVisible()
    await expect(page.getByTestId('rescan-profile-v6')).toBeVisible()
    await expect(page.getByTestId('rescan-profile-vl')).toBeVisible()
    await expect(page.getByTestId('rescan-profile-glm')).toContainText('Ultra')
    await expect(page.getByTestId('rescan-profile-glm')).toContainText('GLM-5.3 Flash')
    await expect(page.getByTestId('rescan-profile-glm')).toContainText(
      'Frontier visual model. Use when even PaddleOCR-VL fails.',
    )

    // This book was read with PP-OCRv6, so the dialog opens on the model after it.
    await expect(page.getByTestId('rescan-profile-vl').locator('input')).toBeChecked()
  })

  // A book already read with PaddleOCR-VL opens on Ultra: the model that read it has
  // failed the page, and Ultra is the only one left to try.
  test('the dialog of a book read with PaddleOCR-VL opens on Ultra', async ({
    page,
    request,
  }) => {
    const book = await ingest(request, 'Read with vl', imageSet('portrait'))
    const copy = await startOCR(request, book.id, 'vl')
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()
    await page.getByTestId('rescan-marked').click()

    await expect(page.getByTestId('rescan-profile-glm').locator('input')).toBeChecked()
    await expect(page.getByTestId('rescan-profile-vl').locator('input')).not.toBeChecked()
  })

  test('a normal rescan keeps the Ultra text and rebuilds its merged reading', async ({
    request,
  }) => {
    const copy = await readWithV6(request)

    await rescanPages(request, copy.id, [1], 'glm')
    await waitForSettled(request, copy.id)
    const transcription = (await pageResults(1, copy.id)).glm.txt

    await rescanPages(request, copy.id, [1], 'v6')
    await waitForSettled(request, copy.id)

    const results = await pageResults(1, copy.id)
    expect(Object.keys(results).sort()).toEqual(['glm', 'glm_merged', 'v6'])
    expect(results.glm.txt).toBe(transcription)
  })
})

test.describe('the reader reports the pages that disagree', () => {
  test('a book whose readings agree says nothing about disagreement', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await expect(page.getByTestId('reader-disagreement')).toHaveCount(0)
  })

  test('a page whose reading engine abandoned it is counted and marked', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    await seedAbandonedReading(copy.id, 1)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await expect(page.getByTestId('reader-disagreement')).toHaveText('1 page disagrees')

    // The count reads as part of the trail: it stands right of the last crumb and
    // left of everything else in the bar.
    const lastCrumb = await page.getByTestId('crumb-2').boundingBox()
    const count = await page.getByTestId('reader-disagreement').boundingBox()
    const actions = await page.getByTestId('action-reject').boundingBox()
    expect(count!.x).toBeGreaterThan(lastCrumb!.x)
    expect(count!.x).toBeLessThan(actions!.x)
    expect(count!.x - (lastCrumb!.x + lastCrumb!.width)).toBeLessThan(24)

    // They read as one line, so they stand on one baseline and not on two.
    //
    // The baseline is measured with a probe of inline text in each, because a
    // crumb is an inline-flex box: a range over its contents answers the rect of
    // that box and says nothing about where its text sits.
    const baselines = await page.evaluate(() => {
      const of = (selector: string) => {
        const element = document.querySelector(selector)!
        const probe = document.createElement('span')
        probe.textContent = 'x'
        element.appendChild(probe)
        const bottom = probe.getBoundingClientRect().bottom
        probe.remove()
        return bottom
      }
      return {
        crumb: of('[data-testid="crumb-2"]'),
        count: of('[data-testid="reader-disagreement"]'),
      }
    })
    expect(Math.abs(baselines.crumb - baselines.count)).toBeLessThan(1.5)

    // The mark sits on the read half of that page and on no other.
    const marked = page.locator('[data-page="1"]').getByTestId('ocr-overlay')
    await expect(marked).toHaveClass(/disagrees/)
    await expect(page.locator('[data-page="2"]').getByTestId('ocr-overlay')).not.toHaveClass(
      /disagrees/,
    )
  })

  // The border is drawn inside the box of the read half, so paint containment
  // cannot clip an edge of it whatever the window is.
  for (const width of [1280, 1920, 4000]) {
    test(`the border shows on every edge in a window ${width} wide`, async ({ page, request }) => {
      await page.setViewportSize({ width, height: 900 })

      const copy = await readWithV6(request)
      await rescanPages(request, copy.id, [1])
      await waitForSettled(request, copy.id)
      await seedAbandonedReading(copy.id, 1)

      await page.goto(`/books/${copy.id}/pages/1`)
      await expect(page.getByTestId('reader-pages')).toBeVisible()

      const ring = await ringOf(page, '[data-page="1"] [data-testid="ocr-overlay"]')
      expect(ring).toContain('inset')

      // The row holds both halves with nothing hanging out of it. A row measured
      // from the window instead of from itself is too wide by the scrollbar the
      // pages bring, and the halves then overflow by that much — which is a border
      // clipped on the edge that overflowed, and only until the window is resized.
      const row = await page.evaluate(() => {
        const element = document.querySelector('[data-testid="reader-pages"]')!
        const spread = element.querySelector('[data-page="1"]')!
        const halves = [...spread.children].map((child) => child.getBoundingClientRect().width)
        return {
          scrollWidth: element.scrollWidth,
          clientWidth: element.clientWidth,
          halves,
        }
      })
      expect(row.scrollWidth).toBeLessThanOrEqual(row.clientWidth)

      // The two halves and the gap between them fill the row and no more. A row
      // taken from the window has to guess the padding of the content column and
      // whether a scrollbar is there; a row measured from itself cannot be wrong
      // about either, and a guess that is wrong by either shows up here.
      const filled = row.halves[0] + row.halves[1] + 16
      expect(filled).toBeLessThanOrEqual(row.clientWidth)
      if (row.halves[0] < 2000) {
        // The page is wider than half the row, so the halves were scaled to fit it
        // exactly rather than capped at their stored size.
        expect(filled).toBeGreaterThan(row.clientWidth - 3)
      }

      // Nothing of the half reaches outside the box its border is clipped to, so
      // every edge of that border is inside the clip and none of it is lost.
      const spread = (await page.locator('[data-page="1"]').boundingBox())!
      const half = (await page.locator('[data-page="1"]').getByTestId('ocr-overlay').boundingBox())!
      expect(half.x).toBeGreaterThanOrEqual(spread.x)
      expect(half.y).toBeGreaterThanOrEqual(spread.y)
      expect(half.x + half.width).toBeLessThanOrEqual(spread.x + spread.width)
      expect(half.y + half.height).toBeLessThanOrEqual(spread.y + spread.height)
    })
  }

  test('a page that is marked and disagrees carries both borders', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 1)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const half = page.locator('[data-page="1"]').getByTestId('ocr-overlay')
    const selector = '[data-page="1"] [data-testid="ocr-overlay"]'

    const doubt = await ringOf(page, selector)

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    await expect(half).toHaveClass(/marked/)
    await expect(half).toHaveClass(/disagrees/)

    // Two rings, one inside the other, in two colours: neither hides the other.
    const both = await ringOf(page, selector)
    expect(both.match(/inset/g)).toHaveLength(2)
    expect(both).not.toBe(doubt)

    const colours = both.match(/rgb\([^)]*\)/g) ?? []
    expect(colours).toHaveLength(2)
    expect(colours[0]).not.toBe(colours[1])

    // The control that starts the rescan wears the colour of the marks.
    await expect(page.getByTestId('rescan-marked')).toHaveClass(/n-button--success-type/)
  })

  // A page holding an Ultra reading wears a third border, and three reasons make
  // three rings that none of them hides.
  test('a page that is marked, disagrees and holds an Ultra reading wears three borders', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)
    await rescanPages(request, copy.id, [1], 'glm')
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 1)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const selector = '[data-page="1"] [data-testid="ocr-overlay"]'

    // The Ultra reading alone already borders the page, and page 2 holds none.
    const ultra = await ringOf(page, selector)
    expect(ultra.match(/inset/g)).toHaveLength(2)
    expect(await ringOf(page, '[data-page="2"] [data-testid="ocr-overlay"]')).not.toContain('inset')

    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    const all = await ringOf(page, selector)
    expect(all.match(/inset/g)).toHaveLength(3)

    // Three colours, each distinct, and each ring wider than the one inside it, so
    // none of them is covered.
    const colours = all.match(/rgb\([^)]*\)/g) ?? []
    expect(new Set(colours).size).toBe(3)

    // The spread of each ring, which is the fourth length of its layer. A layer
    // reads `rgb(...) 0px 0px 0px 3px inset`, so the offsets are lengths too.
    const spreads = all
      .split(/,(?![^(]*\))/)
      .map((layer) => [...layer.matchAll(/(-?[\d.]+)px/g)].map((match) => Number(match[1]))[3])
    expect(spreads).toEqual([...spreads].sort((a, b) => a - b))
    expect(new Set(spreads).size).toBe(3)
  })
})

test.describe('the reader holds a strip naming the pages worth returning to', () => {
  test('a doubted page and a marked page each get a mark, and both together show both', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1, 2, 3])
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 2)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    // Page 2 is doubted, so it alone carries a mark.
    const strip = page.getByTestId('page-minimap')
    await expect(strip).toBeVisible()
    await expect(strip.locator('.mark')).toHaveCount(1)
    await expect(page.getByTestId('page-mark-2')).toHaveClass(/disagrees/)

    // The strip stands inside the window, against its right edge.
    const box = (await strip.boundingBox())!
    const viewport = page.viewportSize()!
    expect(box.width).toBeLessThanOrEqual(16)
    expect(box.x + box.width).toBeCloseTo(viewport.width, 0)

    // Marking page 1 adds a mark of its own, above the other.
    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    await expect(strip.locator('.mark')).toHaveCount(2)
    await expect(page.getByTestId('page-mark-1')).toHaveClass(/marked/)
    const first = (await page.getByTestId('page-mark-1').boundingBox())!
    const second = (await page.getByTestId('page-mark-2').boundingBox())!
    expect(first.y).toBeLessThan(second.y)

    // A page carrying both reasons shows both, one inside the other.
    await page
      .locator('[data-page="2"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    const both = page.getByTestId('page-mark-2')
    await expect(both).toHaveClass(/marked/)
    await expect(both).toHaveClass(/disagrees/)
    const bands = await bandsOf(both)
    expect(bands.colours).toHaveLength(2)
    expect(bands.colours[0]).not.toBe(bands.colours[1])
  })

  // A page holding an Ultra reading carries a mark of its own, and a page carrying
  // every reason bands its mark between them with none of them hidden.
  test('a mark shows a band per reason', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1, 2])
    await waitForSettled(request, copy.id)
    await rescanPages(request, copy.id, [1], 'glm')
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 1)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    // Page 1 holds an Ultra reading and disagrees; page 2 holds neither.
    await expect(page.getByTestId('page-mark-1')).toHaveClass(/ultra/)
    await expect(page.getByTestId('page-mark-2')).toHaveCount(0)

    const two = await bandsOf(page.getByTestId('page-mark-1'))
    expect(two.colours).toHaveLength(2)

    await page.locator('[data-page="1"]').getByTestId('ocr-overlay').click({ button: 'right' })
    await page.getByTestId('page-context-rescan').click()

    const three = await bandsOf(page.getByTestId('page-mark-1'))
    expect(three.colours).toHaveLength(3)
    expect(new Set(three.colours).size).toBe(3)

    // The bands share the mark: each reaches further in than the one outside it, and
    // the innermost reaches its centre, so none of them is covered and no gap is left.
    expect(three.spreads).toEqual([...three.spreads].sort((a, b) => a - b))
    expect(new Set(three.spreads).size).toBe(3)
    expect(three.spreads[2]).toBe(6)
  })

  test('choosing a mark brings its page into view', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1, 2, 3])
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 3)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const before = await page.evaluate(() => window.scrollY)
    await page.getByTestId('page-mark-3').click()

    await expect
      .poll(() => page.evaluate(() => window.scrollY))
      .toBeGreaterThan(before)
  })

  test('a book with nothing to mark holds no strip', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await expect(page.getByTestId('page-minimap')).toHaveCount(0)
  })
})

test.describe('the page menu says why a reading is doubted', () => {
  test('the foot of the menu names the fault and its number', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 1)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })

    const doubt = page.getByTestId('page-context-disagreement')
    await expect(doubt).toBeVisible()
    await expect(doubt).toContainText('Contains only')
    await expect(doubt).toContainText('% of the text')

    // It carries no heading, and it wears the colour of the border it explains.
    await expect(doubt).not.toContainText('doubted')
    const words = await doubt.evaluate((el) => getComputedStyle(el).color)
    const item = await page
      .getByTestId('page-context-rescan')
      .evaluate((el) => getComputedStyle(el).color)
    expect(words).not.toBe(item)

    // It stands between the readings and the rescan.
    const reading = (await page.getByTestId('page-context-reading:merged').boundingBox())!
    const rescan = (await page.getByTestId('page-context-rescan').boundingBox())!
    const explanation = (await doubt.boundingBox())!
    expect(explanation.y).toBeGreaterThan(reading.y)
    expect(explanation.y).toBeLessThan(rescan.y)

    // Its text carries the left padding of an item of the menu, so it lines up with
    // the item below it rather than sitting against the edge.
    const labels = await page.evaluate(() => {
      const left = (selector: string) =>
        document.querySelector(selector)!.getBoundingClientRect().left
      return {
        rescan: left('[data-testid="page-context-rescan"] .n-dropdown-option-body__label'),
        doubt: left('[data-testid="page-context-disagreement"] div'),
      }
    })
    expect(Math.abs(labels.doubt - labels.rescan)).toBeLessThan(4)

    // It is read and not chosen: clicking it runs nothing and closes nothing.
    await doubt.click()
    await expect(page.getByTestId('page-context-menu')).toBeVisible()
  })

  test('the menu of a page that agrees says nothing about a doubt', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page
      .locator('[data-page="1"]')
      .getByTestId('ocr-overlay')
      .click({ button: 'right' })
    await expect(page.getByTestId('page-context-rescan')).toBeVisible()
    await expect(page.getByTestId('page-context-disagreement')).toHaveCount(0)
  })
})

test.describe('the reader shows which lines the merge filled from the line reading', () => {
  test('a line of the line engine stands on a grey ground and keeps its colour', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1])
    await waitForSettled(request, copy.id)

    // The merge of the fake stack takes every line from the reading engine, so
    // one line is rewritten to stand for a line the two engines read differently.
    await markLineAsFromV6(copy.id, 1)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const lines = page.locator('[data-page="1"]').getByTestId('ocr-line')
    await expect(lines.first()).toHaveClass(/substituted/)
    await expect(lines.nth(1)).not.toHaveClass(/substituted/)

    // The text of such a line is the page as much as any other, so it keeps its
    // colour: the ground under it is what says where the text came from.
    const marked = await lines.first().evaluate((el) => getComputedStyle(el).fill)
    const plain = await lines.nth(1).evaluate((el) => getComputedStyle(el).fill)
    expect(marked).toBe(plain)

    const grounds = page.locator('[data-page="1"]').getByTestId('ocr-substituted-grounds')
    await expect(grounds).toHaveCount(1)

    const grey = await grounds.evaluate((el) => getComputedStyle(el).fill)
    const white = await page
      .locator('[data-page="1"] .grounds path:not(.substituted)')
      .evaluate((el) => getComputedStyle(el).fill)
    expect(grey).not.toBe(white)
    expect(white).toBe('rgb(255, 255, 255)')

    // Grey: one shade, and light enough to read black text on.
    const channels = grey.match(/\d+/g)!.map(Number)
    expect(channels[0]).toBe(channels[1])
    expect(channels[1]).toBe(channels[2])
    expect(channels[0]).toBeGreaterThan(180)
    expect(channels[0]).toBeLessThan(250)
  })

  test('a page that answers its line reading stands wholly on white', async ({
    page,
    request,
  }) => {
    const copy = await readWithV6(request)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await expect(page.locator('[data-page="1"] .substituted')).toHaveCount(0)
  })
})

// markLineAsFromV6 rewrites the first line of a merged reading to say the merge
// took its text from the line engine, which is what a line the two engines read
// differently records.
async function markLineAsFromV6(bookId: number, pageIndex: number) {
  const client = new Client({ connectionString: process.env.DATABASE_URL })
  await client.connect()
  try {
    await client.query(
      `UPDATE page_ocr SET data = jsonb_set(data, '{lines,0,source}', '"v6"')
       WHERE model = 'merged' AND page_id = (
         SELECT id FROM book_page WHERE book_id = $1 AND page_index = $2)`,
      [bookId, pageIndex],
    )
  } finally {
    await client.end()
  }
}

test.describe('the page grid marks the pages that disagree', () => {
  test('a disagreeing page is bordered and its cell keeps its size', async ({ page, request }) => {
    const copy = await readWithV6(request)
    await rescanPages(request, copy.id, [1, 2])
    await waitForSettled(request, copy.id)
    await seedAbandonedReading(copy.id, 1)

    await page.goto(`/books/${copy.id}`)
    await expect(page.getByTestId('page-grid')).toBeVisible()

    const flagged = page.getByTestId('page-cell-1').locator('img')
    const sound = page.getByTestId('page-cell-2').locator('img')

    await expect(page.getByTestId('book-disagreement')).toHaveText('1 page disagrees')

    // The count sits in the trail, on the baseline of the last crumb, as it does
    // in the reader.
    const crumb = await page.getByTestId('crumb-1').boundingBox()
    const count = await page.getByTestId('book-disagreement').boundingBox()
    expect(count!.x).toBeGreaterThan(crumb!.x)
    expect(count!.x - (crumb!.x + crumb!.width)).toBeLessThan(24)

    await expect(flagged).toHaveClass(/disagrees/)
    await expect(flagged).toHaveCSS('outline-style', 'solid')

    await expect(sound).not.toHaveClass(/disagrees/)
    await expect(sound).toHaveCSS('outline-style', 'none')

    // The border moves no thumbnail and changes no cell.
    const one = (await page.getByTestId('page-cell-1').boundingBox())!
    const two = (await page.getByTestId('page-cell-2').boundingBox())!
    expect(one.width).toBeCloseTo(two.width, 0)
    expect(one.height).toBeCloseTo(two.height, 0)
  })

  test('a grid of a book that has not been read marks nothing', async ({ page, request }) => {
    const book = await ingest(request, 'Unread grid', imageSet('portrait'))

    await page.goto(`/books/${book.id}`)
    await expect(page.getByTestId('page-grid')).toBeVisible()

    await expect(page.locator('.grid img.disagrees')).toHaveCount(0)
    await expect(page.getByTestId('book-disagreement')).toHaveCount(0)
  })
})

// bandsOf reads the colours of a mark of the strip and how far each reaches in. A
// band is one inset shadow, written `rgb(...) 0px 0px 0px 4px inset`, so the offsets
// are lengths too and the spread is the fourth of them.
async function bandsOf(mark: Locator) {
  const shadow = await mark.evaluate((el) => getComputedStyle(el).boxShadow)
  const layers = shadow.split(/,(?![^(]*\))/)

  return {
    colours: layers.map((layer) => (layer.match(/rgba?\([^)]*\)/) ?? [''])[0]),
    spreads: layers.map(
      (layer) => [...layer.matchAll(/(-?[\d.]+)px/g)].map((match) => Number(match[1]))[3],
    ),
  }
}

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

// seedAbandonedReading makes the visual reading of a page look like a reading
// that stopped after two words, and its line reading like a page of prose. The
// merged reading keeps its length, because the merge filled it out with line
// text, so only the raw reading shows the fault.
async function seedAbandonedReading(bookId: number, pageIndex: number) {
  const client = new Client({ connectionString: process.env.DATABASE_URL })
  await client.connect()
  try {
    const { rows } = await client.query(
      `SELECT id FROM book_page WHERE book_id = $1 AND page_index = $2`,
      [bookId, pageIndex],
    )
    const pageId = rows[0].id as number

    await client.query(`UPDATE page_ocr SET txt = $2 WHERE page_id = $1 AND model = 'v6'`, [
      pageId,
      longPage,
    ])
    await client.query(`UPDATE page_ocr SET txt = $2 WHERE page_id = $1 AND model = 'merged'`, [
      pageId,
      longPage,
    ])
    await client.query(`UPDATE page_ocr SET txt = $2 WHERE page_id = $1 AND model = 'vl'`, [
      pageId,
      'una palabra',
    ])
  } finally {
    await client.end()
  }
}

const longPage = `El sentido de la palabra no reside en la cosa nombrada sino en
el uso que de ella hacemos. Aristoteles llama a esto la significacion primera,
y distingue entre aquello que se dice de suyo y aquello que se dice por
accidente. La cuestion aparece tratada en el libro cuarto de la Metafisica,
donde el filosofo sostiene que ninguna demostracion alcanza a los principios.
Aquel que niega el principio de contradiccion no puede argumentar, porque todo
argumento lo supone. Por eso la refutacion no procede contra la tesis sino
contra quien la sostiene.`
