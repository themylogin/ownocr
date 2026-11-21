import { expect, test, type Page as BrowserPage } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
import {
  apiURL,
  getBook,
  imageSet,
  ingest,
  reset,
  resetDewarp,
  startSplit,
  waitForSettled,
  type BookDetail,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

// split answers the source book and the complete copy the Split action made.
// The `spread` fixture holds spreads alone, so every page of the copy is a half
// and records its cut. The `portrait` fixture holds none, so every page of that
// copy came through whole.
async function split(
  request: APIRequestContext,
  fixture: 'spread' | 'portrait',
  name = 'Atlas',
): Promise<{ source: BookDetail; copy: BookDetail }> {
  const source = await ingest(request, name, imageSet(fixture))
  const started = await startSplit(request, source.id)
  const settled = await waitForSettled(request, started.id)
  expect(settled.state).toBe('complete')
  return { source, copy: await getBook(request, started.id) }
}

async function openGrid(page: BrowserPage, id: number) {
  await page.goto(`/books/${id}`)
  await expect(page.getByTestId('page-grid')).toBeVisible()
}

async function openMenu(page: BrowserPage, index: number) {
  await page.getByTestId(`page-cell-${index}`).click({ button: 'right' })
  await expect(page.getByTestId('page-context-menu')).toBeVisible()
}

async function openEditor(page: BrowserPage, index: number) {
  await openMenu(page, index)
  await page.getByTestId('page-context-cut').click()
  await expect(page.getByTestId('page-cut-image')).toBeVisible()
}

async function columnOf(page: BrowserPage): Promise<number> {
  const value = await page.getByTestId('page-cut-bar').getAttribute('data-column')
  return Number(value)
}

// dragTo moves the bar to a share of the image width, and answers the column it
// left the bar on.
async function dragTo(page: BrowserPage, share: number): Promise<number> {
  const handle = await page.getByTestId('page-cut-handle').boundingBox()
  const image = await page.getByTestId('page-cut-image').boundingBox()
  if (!handle || !image) {
    throw new Error('the editor is not on screen')
  }

  const middle = handle.y + handle.height / 2
  await page.mouse.move(handle.x + handle.width / 2, middle)
  await page.mouse.down()
  await page.mouse.move(image.x + image.width * share, middle, { steps: 10 })
  await page.mouse.up()

  return columnOf(page)
}

test.describe('a right click on a page cut out of a spread offers Edit', () => {
  test('Edit follows the image items, above Delete', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-cut')).toHaveText('Edit')
    await expect(page.getByTestId('page-context-delete')).toHaveText('Delete')

    const items = page.getByTestId('page-context-menu').locator('.n-dropdown-option')
    await expect(items).toHaveCount(4)
    await expect(items.nth(0)).toHaveText('Open in new tab')
    await expect(items.nth(1)).toHaveText('Download')
    await expect(items.nth(2)).toHaveText('Edit')
    await expect(items.nth(3)).toHaveText('Delete')
  })

  test('a page the split copied whole offers no Edit', async ({ page, request }) => {
    const { copy } = await split(request, 'portrait')
    expect(copy.pages.every((entry) => entry.cutSide === null)).toBe(true)

    await openGrid(page, copy.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-delete')).toHaveText('Delete')
    await expect(page.getByTestId('page-context-cut')).toHaveCount(0)
  })

  test('an uploaded page offers no Edit', async ({ page, request }) => {
    const { source } = await split(request, 'spread')

    await openGrid(page, source.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-cut')).toHaveCount(0)
  })

  test('the reader offers the same menu', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="2"]').click({ button: 'right' })
    await expect(page.getByTestId('page-context-cut')).toBeVisible()
  })
})

test.describe('the cut editor shows the parent image and the cut', () => {
  test('the left page greys what follows the bar', async ({ page, request }) => {
    const { source, copy } = await split(request, 'spread')

    expect(copy.pages[0].cutSide).toBe('left')

    await openGrid(page, copy.id)
    await openEditor(page, 1)

    // The editor opens on the page this one was cut out of, not on the page
    // itself: the spread is wider than its half.
    const parent = await page.getByTestId('page-cut-image').boundingBox()
    const shape = source.pages[0].width / source.pages[0].height
    expect(parent!.width / parent!.height).toBeCloseTo(shape, 1)

    const bar = await page.getByTestId('page-cut-bar').boundingBox()
    const shade = await page.getByTestId('page-cut-shade').boundingBox()
    expect(shade!.x).toBeCloseTo(bar!.x + bar!.width / 2, 0)
    expect(shade!.x + shade!.width).toBeCloseTo(parent!.x + parent!.width, 0)

    expect(await columnOf(page)).toBe(copy.pages[0].width)
  })

  test('the right page greys what stands before the bar', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    expect(copy.pages[1].cutSide).toBe('right')

    await openGrid(page, copy.id)
    await openEditor(page, 2)

    const parent = await page.getByTestId('page-cut-image').boundingBox()
    const bar = await page.getByTestId('page-cut-bar').boundingBox()
    const shade = await page.getByTestId('page-cut-shade').boundingBox()

    expect(shade!.x).toBeCloseTo(parent!.x, 0)
    expect(shade!.x + shade!.width).toBeCloseTo(bar!.x + bar!.width / 2, 0)
  })

  test('the image fits the window and is never enlarged', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    await openEditor(page, 1)

    const image = page.getByTestId('page-cut-image').locator('img')
    const box = await image.boundingBox()
    const window = page.viewportSize()!

    expect(box!.width).toBeLessThanOrEqual(window.width)
    expect(box!.height).toBeLessThanOrEqual(window.height)

    const natural = await image.evaluate((node: HTMLImageElement) => node.naturalWidth)
    expect(box!.width).toBeLessThanOrEqual(natural + 1)
  })

  test('the bar follows the pointer', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    await openEditor(page, 1)

    const before = await columnOf(page)
    const bar = page.getByTestId('page-cut-bar')
    const start = await bar.boundingBox()

    const moved = await dragTo(page, 0.25)
    expect(moved).toBeLessThan(before)

    const now = await bar.boundingBox()
    expect(now!.x).toBeLessThan(start!.x)

    // The bar never leaves the image, whatever the pointer does.
    const image = await page.getByTestId('page-cut-image').boundingBox()
    expect(now!.x).toBeGreaterThanOrEqual(image!.x - 2)
  })
})

test.describe('the cut editor saves or cancels', () => {
  test('Cancel changes nothing', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    const before = await page.getByTestId('page-cell-1').locator('img').getAttribute('src')

    await openEditor(page, 1)
    await dragTo(page, 0.3)
    await page.getByTestId('page-cut-cancel').click()

    await expect(page.getByTestId('page-cut')).toHaveCount(0)
    await expect(page.getByTestId('page-cell-1').locator('img')).toHaveAttribute('src', before!)

    const reloaded = await getBook(request, copy.id)
    expect(reloaded.pages[0].width).toBe(copy.pages[0].width)
  })

  test('Save re-cuts this page alone', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    const before = await page.getByTestId('page-cell-1').locator('img').getAttribute('src')

    await openEditor(page, 1)
    const column = await dragTo(page, 0.3)
    expect(column).not.toBe(copy.pages[0].width)

    await page.evaluate(() => {
      ;(window as unknown as { staysLoaded: boolean }).staysLoaded = true
    })
    await page.getByTestId('page-cut-save').click()
    await expect(page.getByTestId('page-cut')).toHaveCount(0)

    // The grid shows the new image, and the view was never reloaded.
    await expect(page.getByTestId('page-cell-1').locator('img')).not.toHaveAttribute(
      'src',
      before!,
    )
    expect(
      await page.evaluate(() => (window as unknown as { staysLoaded?: boolean }).staysLoaded),
    ).toBe(true)

    const reloaded = await getBook(request, copy.id)
    expect(reloaded.pages[0].width).toBe(column)
    expect(reloaded.pageCount).toBe(copy.pageCount)

    // The other half of the spread keeps the cut it holds.
    expect(reloaded.pages[1].width).toBe(copy.pages[1].width)
    expect(reloaded.pages[1].thumbnailUrl).toBe(copy.pages[1].thumbnailUrl)
  })

  test('a right page saves the column and every column after it', async ({ page, request }) => {
    const { source, copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    await openEditor(page, 2)
    const column = await dragTo(page, 0.35)
    await page.getByTestId('page-cut-save').click()
    await expect(page.getByTestId('page-cut')).toHaveCount(0)

    const reloaded = await getBook(request, copy.id)
    expect(reloaded.pages[1].width).toBe(source.pages[0].width - column)
  })

  test('a second edit cuts out of the spread again', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)

    await openEditor(page, 1)
    const narrow = await dragTo(page, 0.2)
    await page.getByTestId('page-cut-save').click()
    await expect(page.getByTestId('page-cut')).toHaveCount(0)
    expect((await getBook(request, copy.id)).pages[0].width).toBe(narrow)

    // The editor still opens on the whole spread, so the columns the first cut
    // dropped are still there to take back.
    await openEditor(page, 1)
    expect(await columnOf(page)).toBe(narrow)
    const wide = await dragTo(page, 0.45)
    expect(wide).toBeGreaterThan(narrow)

    await page.getByTestId('page-cut-save').click()
    await expect(page.getByTestId('page-cut')).toHaveCount(0)

    expect((await getBook(request, copy.id)).pages[0].width).toBe(wide)
  })

  test('the editor shows a refusal and stays open', async ({ page, request }) => {
    const { copy } = await split(request, 'spread')

    await openGrid(page, copy.id)
    await openEditor(page, 1)
    await dragTo(page, 0.3)

    // Another client removes the page while this editor is open, so the save
    // has nothing left to cut.
    const removed = await request.post(`${apiURL}/api/books/${copy.id}/pages/1/delete`)
    expect(removed.ok()).toBe(true)

    await page.getByTestId('page-cut-save').click()
    await expect(page.getByTestId('page-cut-error')).toBeVisible()
    await expect(page.getByTestId('page-cut-image')).toBeVisible()

    // The other half of the spread is untouched, whatever the editor tried.
    const reloaded = await getBook(request, copy.id)
    expect(reloaded.pages[0].width).toBe(copy.pages[1].width)
  })
})
