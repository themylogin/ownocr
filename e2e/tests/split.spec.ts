import { expect, test } from '@playwright/test'
import {
  getBook,
  imageSet,
  ingest,
  listBooks,
  reset,
  resetDewarp,
  startDewarp,
  startSplit,
  storedPath,
  waitForSettled,
} from './helpers'
import { existsSync } from 'node:fs'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

test.describe('the Split action leads to the Split stage', () => {
  test('a user splits an uploaded book', async ({ page, request }) => {
    const book = await ingest(request, 'Straight Through', imageSet('portrait'))

    await page.goto('/stage/split')
    await expect(page.getByTestId('stage-empty')).toBeVisible()

    const copy = await startSplit(request, book.id)
    expect(copy.stage).toBe('split')

    const card = page.getByTestId(`book-card-${copy.id}`)
    await expect(card).toBeVisible()
    await expect(card).toHaveAttribute('data-group', 'running')

    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')
    expect(settled.stage).toBe('split')
    await expect(card).toHaveAttribute('data-group', 'ready', { timeout: 60_000 })
  })

  test('a user splits a dewarped book', async ({ request }) => {
    const book = await ingest(request, 'Two Steps', imageSet('portrait'))

    const dewarped = await startDewarp(request, book.id)
    await waitForSettled(request, dewarped.id)

    const copy = await startSplit(request, dewarped.id)
    expect(copy.stage).toBe('split')

    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')
    expect(await listBooks(request)).toHaveLength(3)
  })
})

test.describe('splitting cuts the spreads of the source book', () => {
  test('a book of spreads is split', async ({ request }) => {
    const source = await ingest(request, 'Copied', imageSet('spread'))

    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    // Every fixture of the spread set carries a gutter, so each makes two pages.
    expect(settled.pages).toHaveLength(source.pages.length * 2)
    expect(settled.pageCount).toBe(source.pages.length * 2)

    for (let i = 0; i < settled.pages.length; i++) {
      expect(settled.pages[i].index).toBe(i + 1)

      // The two pages made from one source page sit next to each other. Each
      // one is a half, so it keeps the full height and roughly half the width.
      const origin = source.pages[Math.floor(i / 2)]
      expect(settled.pages[i].height).toBe(origin.height)
      expect(settled.pages[i].width).toBeGreaterThan(origin.width * 0.4)
      expect(settled.pages[i].width).toBeLessThan(origin.width * 0.6)
    }

    const sourceIds = settled.pages.map((page) => page.sourcePageId)
    expect(new Set(sourceIds).size).toBe(source.pages.length)
    for (let i = 0; i < sourceIds.length; i += 2) {
      expect(sourceIds[i]).not.toBeNull()
      expect(sourceIds[i]).toBe(sourceIds[i + 1])
    }
  })

  test('a landscape page with no gutter is cut at its middle', async ({ request }) => {
    // The landscape fixture is 3000 by 2000 and carries no gutter, so the
    // sidecar finds nothing and the shape of the page decides the cut.
    const source = await ingest(request, 'No Gutter', imageSet('landscape'))

    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('complete')
    expect(settled.pages).toHaveLength(source.pages.length * 2)
    expect(settled.pageCount).toBe(source.pages.length * 2)

    for (let i = 0; i < settled.pages.length; i++) {
      const origin = source.pages[Math.floor(i / 2)]
      const half = Math.floor(origin.width / 2)

      // The two pages meet at the middle: no column is in both, none is lost.
      expect(settled.pages[i].width).toBe(i % 2 === 0 ? half : origin.width - half)
      expect(settled.pages[i].height).toBe(origin.height)
    }

    // The two halves of one page name that page, one on each side of the cut.
    for (let i = 0; i < settled.pages.length; i += 2) {
      expect(settled.pages[i].sourcePageId).toBe(settled.pages[i + 1].sourcePageId)
      expect(settled.pages[i].sourcePageId).not.toBeNull()
      expect(settled.pages[i].cutSide).toBe('left')
      expect(settled.pages[i + 1].cutSide).toBe('right')
    }
  })

  test('a portrait page with no gutter passes through whole', async ({ request }) => {
    const source = await ingest(request, 'Upright', imageSet('portrait'))

    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    // The rule reads the shape of the page, so an upright one is left alone.
    expect(settled.pages).toHaveLength(source.pages.length)
    for (const page of settled.pages) {
      expect(page.cutSide).toBeNull()
    }
  })

  test('a page that is not a spread passes through whole', async ({ request }) => {
    const source = await ingest(request, 'Single Pages', imageSet('portrait'))

    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.pages).toHaveLength(source.pages.length)
    expect(settled.pageCount).toBe(source.pages.length)

    for (let i = 0; i < settled.pages.length; i++) {
      expect(settled.pages[i].index).toBe(i + 1)
      expect(settled.pages[i].width).toBe(source.pages[i].width)
      expect(settled.pages[i].height).toBe(source.pages[i].height)
      expect(settled.pages[i].sourcePageId).not.toBeNull()
    }

    // One page each, so no source page is named twice.
    const sourceIds = settled.pages.map((page) => page.sourcePageId)
    expect(new Set(sourceIds).size).toBe(source.pages.length)
  })

  test('the pages are independent', async ({ request }) => {
    const source = await ingest(request, 'Independent', imageSet('spread'))

    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    const sourceUrls = new Set(source.pages.map((page) => page.originalUrl))
    for (const page of settled.pages) {
      expect(sourceUrls.has(page.originalUrl)).toBe(false)
      expect(existsSync(storedPath(page.originalUrl))).toBe(true)
    }
  })

  test('every rendition of a split page loads', async ({ request }) => {
    const source = await ingest(request, 'Renditions', imageSet('spread'))
    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    const page = settled.pages[0]
    for (const url of [page.originalUrl, page.displayUrl, page.thumbnailUrl]) {
      const response = await request.get(url)
      expect(response.status(), url).toBe(200)
      expect(response.headers()['content-type']).toContain('image/')
    }
  })

  test('the source book is untouched', async ({ request }) => {
    const source = await ingest(request, 'Untouched', imageSet('portrait'))

    const copy = await startSplit(request, source.id)
    await waitForSettled(request, copy.id)

    const unchanged = await getBook(request, source.id)
    expect(unchanged.state).toBe('complete')
    expect(unchanged.stage).toBe('uploaded')
    expect(unchanged.pages).toHaveLength(source.pages.length)
    expect(unchanged.derivedState).toBe('complete')
  })
})

test.describe('splitting needs no external configuration', () => {
  test('a book reaches the Split stage and opens there', async ({ page, request }) => {
    const source = await ingest(request, 'Finished', imageSet('portrait'))
    const copy = await startSplit(request, source.id)
    await waitForSettled(request, copy.id)

    await page.goto('/stage/split')

    const card = page.getByTestId(`book-card-${copy.id}`)
    await expect(card).toBeVisible()
    await expect(card.getByText('Finished', { exact: true })).toBeVisible()
    await expect(card.locator('img')).toHaveAttribute('src', /^\/files\/.+\.480\.jpg$/)

    // OCR comes after Split, so the copy is ready for it.
    await expect(page.getByTestId(`book-select-${copy.id}`)).toBeVisible()
    await expect(page.getByTestId('select-all')).toBeVisible()
  })
})
