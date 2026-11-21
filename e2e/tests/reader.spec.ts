import { expect, test } from '@playwright/test'
import { fixture, imageSet, ingest, reset } from './helpers'

test.beforeEach(async () => {
  await reset()
})

test.describe('selecting a page opens the reader at that page', () => {
  test('a user selects a page from the grid', async ({ page, request }) => {
    const book = await ingest(request, 'Ten Pager', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    await page.goto(`/books/${book.id}`)
    await page.getByTestId('page-cell-7').click()

    await expect(page).toHaveURL(new RegExp(`/books/${book.id}/pages/7$`))
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const inView = await page.locator('[data-page="7"]').evaluate((node) => {
      const box = node.getBoundingClientRect()
      return box.top < window.innerHeight && box.bottom > 0
    })
    expect(inView).toBe(true)
  })

  test('the reader URL is shared', async ({ page, request }) => {
    const book = await ingest(request, 'Direct', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    await page.goto(`/books/${book.id}/pages/9`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const inView = await page.locator('[data-page="9"]').evaluate((node) => {
      const box = node.getBoundingClientRect()
      return box.top < window.innerHeight && box.bottom > 0
    })
    expect(inView).toBe(true)
  })
})

test.describe('the reader fits a page to the content column', () => {
  test('pages stack in order below one another', async ({ page, request }) => {
    const book = await ingest(request, 'Ordered', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const tops = await page
      .getByTestId('reader-pages')
      .locator('figure')
      .evaluateAll((nodes) => nodes.map((node) => node.getBoundingClientRect().top))

    expect(tops).toHaveLength(3)
    for (let i = 1; i < tops.length; i++) {
      expect(tops[i]).toBeGreaterThan(tops[i - 1])
    }

    const indexes = await page
      .getByTestId('reader-pages')
      .locator('figure')
      .evaluateAll((nodes) => nodes.map((node) => node.getAttribute('data-page')))
    expect(indexes).toEqual(['1', '2', '3'])
  })

  test('a page fits the content column', async ({ page, request }) => {
    const book = await ingest(request, 'Fitted', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const measured = await page.evaluate(() => {
      const image = document.querySelector('[data-page="1"] img') as HTMLImageElement
      const column = document.querySelector('[data-testid="reader-pages"]') as HTMLElement
      return {
        image: Math.round(image.getBoundingClientRect().width),
        column: Math.round(column.getBoundingClientRect().width),
        viewport: window.innerWidth,
      }
    })

    // The page fits the column, and the column is narrower than the window. A
    // page wider than the column is scaled down to it.
    expect(measured.image).toBeLessThanOrEqual(measured.column)
    expect(measured.image).toBeLessThanOrEqual(book.pages[0].width)
    expect(measured.column).toBeLessThan(measured.viewport)
    expect(measured.image).toBe(measured.column)
  })

  test('a page narrower than the column keeps its own width', async ({ page, request }) => {
    const book = await ingest(request, 'Small Page', [
      { filename: 'page1.jpg', path: fixture('small', 'page1.jpg') },
    ])

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const measured = await page.evaluate(() => {
      const image = document.querySelector('[data-page="1"] img') as HTMLImageElement
      const column = document.querySelector('[data-testid="reader-pages"]') as HTMLElement
      return {
        image: Math.round(image.getBoundingClientRect().width),
        column: Math.round(column.getBoundingClientRect().width),
      }
    })

    expect(measured.image).toBe(book.pages[0].width)
    expect(measured.image).toBeLessThan(measured.column)
  })

  test('the reader shows no page number', async ({ page, request }) => {
    const book = await ingest(request, 'No Numbers', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    await expect(page.getByTestId('reader-pages').locator('figcaption')).toHaveCount(0)
  })

  test('a large book does not load every image at once', async ({ page, request }) => {
    const book = await ingest(request, 'Sixty Pager', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])
    expect(book.pages).toHaveLength(60)

    const requested = new Set<string>()
    page.on('request', (request) => {
      if (request.resourceType() === 'image') {
        requested.add(request.url())
      }
    })

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await page.waitForTimeout(2500)

    // The spec describes this with a 500 page book. The property under test is
    // the same at 60 pages: images load as they approach the viewport.
    expect(requested.size).toBeLessThan(book.pages.length)
  })

  test('a user leaves the reader', async ({ page, request }) => {
    const book = await ingest(request, 'Back Out', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/2`)
    await page.getByTestId('crumb-1').click()

    await expect(page).toHaveURL(new RegExp(`/books/${book.id}$`))
    await expect(page.getByTestId('page-grid')).toBeVisible()
  })

  test('the reader uses the display rendition', async ({ page, request }) => {
    const book = await ingest(request, 'Display', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()

    const src = await page.locator('[data-page="1"] img').getAttribute('src')
    expect(src).toMatch(/\.1280\.jpg$/)
  })
})

test.describe('breadcrumbs show the path to the current view', () => {
  test('the book view names its stage and the book', async ({ page, request }) => {
    const book = await ingest(request, 'To Stage', imageSet('portrait'))

    await page.goto(`/books/${book.id}`)

    const crumbs = page.getByTestId('breadcrumbs')
    await expect(crumbs).toContainText('Uploaded')
    await expect(crumbs).toContainText('To Stage')
    await expect(page.getByTestId('crumb-2')).toHaveCount(0)
  })

  test('the reader adds a third step', async ({ page, request }) => {
    const book = await ingest(request, 'Read Me', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)

    await expect(page.getByTestId('crumb-0')).toContainText('Uploaded')
    await expect(page.getByTestId('crumb-1')).toContainText('Read Me')
    await expect(page.getByTestId('crumb-2')).toContainText('View')
  })

  test('the first step returns to the stage list', async ({ page, request }) => {
    const book = await ingest(request, 'To Stage', imageSet('portrait'))

    await page.goto(`/books/${book.id}`)
    await page.getByTestId('crumb-0').click()

    await expect(page).toHaveURL(/\/stage\/uploaded$/)
    await expect(page.getByTestId(`book-card-${book.id}`)).toBeVisible()
  })
})
