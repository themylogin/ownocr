import { existsSync } from 'node:fs'
import { expect, test, type Page as BrowserPage } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
import {
  getBook,
  imageSet,
  ingest,
  reset,
  resetDewarp,
  startDewarp,
  startSplit,
  storedPath,
  waitForSettled,
  type BookDetail,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

async function seed(request: APIRequestContext, name = 'Atlas'): Promise<BookDetail> {
  return ingest(request, name, imageSet('portrait'))
}

// dewarped answers the source book and its complete dewarped copy. The fake
// endpoint shrinks every page, so the two differ in size and a rollback shows.
async function dewarped(
  request: APIRequestContext,
  name = 'Atlas',
): Promise<{ source: BookDetail; copy: BookDetail }> {
  const source = await seed(request, name)
  const started = await startDewarp(request, source.id)
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

test.describe('a right click on a page offers the page menu', () => {
  test('an uploaded page offers the image items and Delete', async ({ page, request }) => {
    const book = await seed(request)

    await openGrid(page, book.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-open')).toHaveText('Open in new tab')
    await expect(page.getByTestId('page-context-download')).toHaveText('Download')
    await expect(page.getByTestId('page-context-delete')).toHaveText('Delete')
    await expect(page.getByTestId('page-context-original')).toHaveCount(0)
    await expect(page.getByTestId('page-context-cut')).toHaveCount(0)

    const menu = page.getByTestId('page-context-menu')
    await expect(menu.locator('.n-dropdown-option')).toHaveCount(3)
    await expect(menu.locator('.n-dropdown-divider')).toHaveCount(1)
  })

  test('a dewarped page offers Show original first', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-original')).toHaveText('Show original')
    await expect(page.getByTestId('page-context-delete')).toHaveText('Delete')

    const items = await page.getByTestId('page-context-menu').locator('.n-dropdown-option').count()
    expect(items).toBe(4)
  })

  test('Delete is not the colour of the other items', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)

    const danger = await page.getByTestId('page-context-delete').evaluate(
      (node) => getComputedStyle(node).color,
    )
    const plain = await page.getByTestId('page-context-original').evaluate(
      (node) => getComputedStyle(node).color,
    )
    expect(danger).not.toBe(plain)
  })

  test('the reader offers the same menu', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await page.goto(`/books/${copy.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="2"]').click({ button: 'right' })
    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    await expect(page.getByTestId('page-context-original')).toBeVisible()
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
  })

  test('Open in new tab opens the original, not the thumbnail', async ({ page, request }) => {
    const book = await seed(request)
    const first = book.pages[0]
    expect(first.originalUrl).not.toBe(first.thumbnailUrl)

    await openGrid(page, book.id)
    await openMenu(page, 1)

    const opened = page.context().waitForEvent('page')
    await page.getByTestId('page-context-open').click()
    const tab = await opened

    expect(new URL(tab.url()).pathname).toBe(first.originalUrl)
    await expect(page.getByTestId('page-grid')).toBeVisible()
  })

  test('the reader opens the original, not the display rendition', async ({ page, request }) => {
    const book = await seed(request)
    const second = book.pages[1]

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="2"]').click({ button: 'right' })
    await expect(page.getByTestId('page-context-menu')).toBeVisible()

    const opened = page.context().waitForEvent('page')
    await page.getByTestId('page-context-open').click()
    const tab = await opened

    expect(new URL(tab.url()).pathname).toBe(second.originalUrl)
    expect(new URL(tab.url()).pathname).not.toBe(second.displayUrl)
  })

  test('Download saves the original under the book name and page index', async ({
    page,
    request,
  }) => {
    const book = await seed(request, 'Vergil')

    await openGrid(page, book.id)
    await openMenu(page, 2)

    const started = page.waitForEvent('download')
    await page.getByTestId('page-context-download').click()
    const download = await started

    expect(download.suggestedFilename()).toBe('Vergil - 0002.jpg')
    expect(new URL(download.url()).pathname).toBe(book.pages[1].originalUrl)
  })

  test('a second right click moves the menu', async ({ page, request }) => {
    const book = await seed(request)

    await openGrid(page, book.id)
    await openMenu(page, 1)
    const first = await page.getByTestId('page-context-menu').boundingBox()

    await page.getByTestId('page-cell-3').click({ button: 'right' })
    await expect(page.getByTestId('page-context-menu')).toBeVisible()
    const second = await page.getByTestId('page-context-menu').boundingBox()

    expect(second?.x).not.toBe(first?.x)
  })

  test('Escape closes the menu and runs nothing', async ({ page, request }) => {
    const book = await seed(request)

    await openGrid(page, book.id)
    await openMenu(page, 1)
    await page.keyboard.press('Escape')

    await expect(page.getByTestId('page-context-menu')).toHaveCount(0)
    await expect(page.getByTestId('page-cell-1')).toBeVisible()
  })
})

test.describe('deleting a page asks before it removes anything', () => {
  test('the confirmation names every book that loses a page', async ({ page, request }) => {
    const { source } = await dewarped(request)

    await openGrid(page, source.id)
    await openMenu(page, 2)
    await page.getByTestId('page-context-delete').click()

    await expect(page.getByTestId('page-delete-dialog')).toBeVisible()
    await expect(page.getByTestId('page-delete-count')).toContainText('2 books')
    await expect(page.getByTestId('page-delete-count')).toContainText('cannot be undone')
    await expect(page.getByTestId('page-delete-list').locator('li')).toHaveCount(2)
  })

  // A split copy makes two pages from one, so deleting one source page takes two
  // pages out of the copy. The copy is still one book, and is named once.
  test('a copy that loses several pages is named once', async ({ page, request }) => {
    // The spread set is what makes two copy pages from one source page.
    const source = await ingest(request, 'Atlas', imageSet('spread'))
    const started = await startSplit(request, source.id)
    const copy = await waitForSettled(request, started.id)
    expect(copy.state).toBe('complete')
    expect(copy.pageCount).toBe(source.pages.length * 2)

    await openGrid(page, source.id)
    await openMenu(page, 2)
    await page.getByTestId('page-context-delete').click()

    await expect(page.getByTestId('page-delete-dialog')).toBeVisible()
    await expect(page.getByTestId('page-delete-count')).toContainText('2 books')
    await expect(page.getByTestId('page-delete-list').locator('li')).toHaveCount(2)

    await page.getByTestId('page-delete-confirm').click()
    await expect(page.getByTestId('page-delete-dialog')).toHaveCount(0)

    const reloadedSource = await getBook(request, source.id)
    expect(reloadedSource.pages).toHaveLength(source.pages.length - 1)

    const reloadedCopy = await getBook(request, copy.id)
    expect(reloadedCopy.pages).toHaveLength(copy.pages.length - 2)
    expect(reloadedCopy.pageCount).toBe(copy.pages.length - 2)

    // Both pages made from the deleted source page are gone, so the copy names
    // one source page fewer, and still names each of them twice.
    const named = reloadedCopy.pages.map((entry) => entry.sourcePageId)
    expect(new Set(named).size).toBe(source.pages.length - 1)
    for (const id of new Set(named)) {
      expect(named.filter((entry) => entry === id)).toHaveLength(2)
    }
  })

  test('dismissing the confirmation removes nothing', async ({ page, request }) => {
    const book = await seed(request)

    await openGrid(page, book.id)
    await openMenu(page, 2)
    await page.getByTestId('page-context-delete').click()
    await expect(page.getByTestId('page-delete-dialog')).toBeVisible()

    await page.getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByTestId('page-delete-dialog')).toHaveCount(0)

    await expect(page.getByTestId('page-grid').locator('li')).toHaveCount(3)
    expect((await getBook(request, book.id)).pages).toHaveLength(3)
  })

  test('a confirmed delete removes the page from the whole chain', async ({ page, request }) => {
    const { source, copy } = await dewarped(request)
    const doomed = copy.pages.find((entry) => entry.index === 2)
    expect(doomed).toBeDefined()

    await openGrid(page, source.id)
    await openMenu(page, 2)
    await page.getByTestId('page-context-delete').click()
    await page.getByTestId('page-delete-confirm').click()

    await expect(page.getByTestId('page-delete-dialog')).toHaveCount(0)
    await expect(page.getByTestId('page-cell-2')).toHaveCount(0)
    await expect(page.getByTestId('page-grid').locator('li')).toHaveCount(2)

    const reloadedSource = await getBook(request, source.id)
    expect(reloadedSource.pages.map((entry) => entry.index)).toEqual([1, 3])
    expect(reloadedSource.pageCount).toBe(2)

    const reloadedCopy = await getBook(request, copy.id)
    expect(reloadedCopy.pages.map((entry) => entry.index)).toEqual([1, 3])
    expect(reloadedCopy.pageCount).toBe(2)

    expect(existsSync(storedPath(doomed!.displayUrl))).toBe(false)
  })

  test('the page index keeps its gap', async ({ page, request }) => {
    const book = await seed(request)

    await openGrid(page, book.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-delete').click()
    await page.getByTestId('page-delete-confirm').click()

    await expect(page.getByTestId('page-cell-1')).toHaveCount(0)
    await expect(page.getByTestId('page-cell-2')).toBeVisible()
    await expect(page.getByTestId('page-cell-3')).toBeVisible()

    const reloaded = await getBook(request, book.id)
    expect(reloaded.pages.map((entry) => entry.index)).toEqual([2, 3])
    expect(reloaded.coverUrl).not.toBeNull()
  })

  test('a delete from the reader leaves the stack', async ({ page, request }) => {
    const book = await seed(request)

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await page.locator('[data-page="2"]').click({ button: 'right' })
    await page.getByTestId('page-context-delete').click()
    await page.getByTestId('page-delete-confirm').click()

    await expect(page.locator('[data-page="2"]')).toHaveCount(0)
    await expect(page.locator('[data-page="1"]')).toBeVisible()
    await expect(page.getByTestId('reader-pages')).toBeVisible()
  })

  test('the confirmation refuses while a copy runs', async ({ page, request }) => {
    const book = await seed(request)
    await startSplit(request, book.id)

    await openGrid(page, book.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-delete').click()

    await expect(page.getByTestId('page-delete-refusal')).toBeVisible()
    await expect(page.getByTestId('page-delete-confirm')).toBeDisabled()
  })

  test('a page of a copy leaves the source book alone', async ({ page, request }) => {
    const { source, copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-delete').click()
    await expect(page.getByTestId('page-delete-count')).toContainText('1 book')
    await page.getByTestId('page-delete-confirm').click()

    await expect(page.getByTestId('page-grid').locator('li')).toHaveCount(2)
    expect((await getBook(request, source.id)).pages).toHaveLength(3)
  })
})

test.describe('the comparison shows the two pages side by side', () => {
  test('both stages are named, original on the left', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()

    await expect(page.getByTestId('page-compare')).toBeVisible()
    await expect(page.getByTestId('compare-source')).toContainText('Uploaded')
    await expect(page.getByTestId('compare-current')).toContainText('Dewarped')

    const left = await page.getByTestId('compare-source').boundingBox()
    const right = await page.getByTestId('compare-current').boundingBox()
    expect(left!.x).toBeLessThan(right!.x)
  })

  test('the pages fill the window and are never enlarged', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()
    await expect(page.getByTestId('page-compare')).toBeVisible()

    const viewport = page.viewportSize()!
    const images = page.getByTestId('page-compare').locator('img')
    await expect(images).toHaveCount(2)

    const boxes = await images.evaluateAll((nodes) =>
      nodes.map((node) => {
        const image = node as HTMLImageElement
        const box = image.getBoundingClientRect()
        return { width: box.width, height: box.height, natural: image.naturalWidth }
      }),
    )

    for (const box of boxes) {
      expect(box.width).toBeLessThanOrEqual(viewport.width)
      expect(box.height).toBeLessThanOrEqual(viewport.height)
      // Never larger than the stored display rendition.
      expect(box.width).toBeLessThanOrEqual(box.natural + 1)
    }

    // The two panes fill a fair part of the width rather than sitting small.
    const used = boxes.reduce((total, box) => total + box.width, 0)
    expect(used).toBeGreaterThan(viewport.width * 0.4)
  })

  test('Close changes nothing', async ({ page, request }) => {
    const { copy } = await dewarped(request)
    const before = await getBook(request, copy.id)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()
    await page.getByTestId('compare-close').click()

    await expect(page.getByTestId('page-compare')).toHaveCount(0)
    const after = await getBook(request, copy.id)
    expect(after.pages).toEqual(before.pages)
  })
})

test.describe('using the original asks before it replaces the page', () => {
  test('dismissing the second confirmation changes nothing', async ({ page, request }) => {
    const { copy } = await dewarped(request)
    const before = await getBook(request, copy.id)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()
    await page.getByTestId('use-original').click()

    await expect(page.getByTestId('use-original-dialog')).toBeVisible()
    await expect(page.getByTestId('use-original-dialog')).toContainText('cannot be undone')

    await page.getByTestId('use-original-dialog').getByRole('button', { name: 'Cancel' }).click()
    await expect(page.getByTestId('use-original-dialog')).toHaveCount(0)
    await expect(page.getByTestId('page-compare')).toBeVisible()

    expect((await getBook(request, copy.id)).pages).toEqual(before.pages)
  })

  test('a confirmed rollback installs the uploaded page', async ({ page, request }) => {
    const { source, copy } = await dewarped(request)

    const uploaded = source.pages.find((entry) => entry.index === 1)!
    const before = copy.pages.find((entry) => entry.index === 1)!
    expect(before.width).toBeLessThan(uploaded.width)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()
    await page.getByTestId('use-original').click()
    await page.getByTestId('use-original-confirm').click()

    await expect(page.getByTestId('page-compare')).toHaveCount(0)

    // The grid shows the new image without a reload: the URL changed with the
    // storage key.
    const shown = page.getByTestId('page-cell-1').locator('img')
    await expect(shown).not.toHaveAttribute('src', before.thumbnailUrl)

    const after = await getBook(request, copy.id)
    const page1 = after.pages.find((entry) => entry.index === 1)!
    expect(page1.width).toBe(uploaded.width)
    expect(page1.height).toBe(uploaded.height)
    expect(page1.sourcePageId).toBe(before.sourcePageId)
    expect(after.pageCount).toBe(3)

    // The copy owns its bytes: its file is its own, and the source keeps hers.
    expect(page1.displayUrl).not.toBe(uploaded.displayUrl)
    expect(existsSync(storedPath(page1.displayUrl))).toBe(true)
    expect(existsSync(storedPath(uploaded.displayUrl))).toBe(true)
    expect(existsSync(storedPath(before.displayUrl))).toBe(false)
  })

  test('a rolled back page offers the rollback no more', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()
    await page.getByTestId('use-original').click()
    await page.getByTestId('use-original-confirm').click()
    await expect(page.getByTestId('page-compare')).toHaveCount(0)

    // The menu of that page loses the item, without a reload.
    await openMenu(page, 1)
    await expect(page.getByTestId('page-context-original')).toHaveCount(0)
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
    await page.keyboard.press('Escape')

    // Every other page still offers it.
    await openMenu(page, 2)
    await expect(page.getByTestId('page-context-original')).toBeVisible()
    await page.keyboard.press('Escape')

    // And it stays gone after a reload.
    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await expect(page.getByTestId('page-context-original')).toHaveCount(0)
  })

  test('the confirmation stacks over the comparison', async ({ page, request }) => {
    const { copy } = await dewarped(request)

    await openGrid(page, copy.id)
    await openMenu(page, 1)
    await page.getByTestId('page-context-original').click()
    await page.getByTestId('use-original').click()

    // Both are on screen, the confirmation on top.
    await expect(page.getByTestId('page-compare')).toBeVisible()
    await expect(page.getByTestId('use-original-dialog')).toBeVisible()

    const layers = await page.evaluate(() => {
      const layerOf = (node: Element | null) => {
        for (let at = node; at; at = at.parentElement) {
          const index = getComputedStyle(at).zIndex
          if (index !== 'auto') {
            return Number(index)
          }
        }
        return 0
      }
      return {
        compare: layerOf(document.querySelector('[data-testid="page-compare"]')),
        confirm: layerOf(document.querySelector('[data-testid="use-original-dialog"]')),
      }
    })
    expect(layers.confirm).toBeGreaterThan(layers.compare)
  })

  test('a book that already has a copy offers no rollback', async ({ page, request }) => {
    const { copy } = await dewarped(request)
    const split = await startSplit(request, copy.id)
    await waitForSettled(request, split.id)

    await openGrid(page, copy.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-original')).toHaveCount(0)
    await expect(page.getByTestId('page-context-delete')).toBeVisible()
  })

  test('a split copy offers no rollback', async ({ page, request }) => {
    const book = await seed(request)
    const split = await startSplit(request, book.id)
    await waitForSettled(request, split.id)

    await openGrid(page, split.id)
    await openMenu(page, 1)

    await expect(page.getByTestId('page-context-original')).toHaveCount(0)
  })
})
