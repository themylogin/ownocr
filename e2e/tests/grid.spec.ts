import { expect, test, type Page } from '@playwright/test'
import { fixture, imageSet, ingest, reset } from './helpers'

test.beforeEach(async () => {
  await reset()
})

async function cellBoxes(page: Page) {
  return page.getByTestId('page-grid').locator('li').evaluateAll((nodes) =>
    nodes.map((node) => {
      const box = node.getBoundingClientRect()
      return { width: Math.round(box.width), height: Math.round(box.height) }
    }),
  )
}

// The cell takes its width from the container and its shape from the server.
// The pixel size therefore follows the window, the ratio does not.
async function expectShape(page: Page, cell: { width: number; height: number }) {
  const boxes = await cellBoxes(page)
  expect(boxes.length).toBeGreaterThan(0)

  const want = cell.width / cell.height
  for (const box of boxes) {
    expect(box).toEqual(boxes[0])
    expect(box.width / box.height).toBeCloseTo(want, 1)
  }
}

// The number of cells in the first row of the grid.
async function cellsPerRow(page: Page) {
  const boxes = await page.getByTestId('page-grid').locator('li').evaluateAll((nodes) =>
    nodes.map((node) => Math.round(node.getBoundingClientRect().top)),
  )
  return boxes.filter((top) => top === boxes[0]).length
}

async function openGrid(page: Page, id: number) {
  await page.goto(`/books/${id}`)
  await expect(page.getByTestId('page-grid')).toBeVisible()
}

test.describe('the grid cell size derives from the page dimensions', () => {
  test('every page is landscape', async ({ page, request }) => {
    const book = await ingest(request, 'Landscape', imageSet('landscape'))
    expect(book.gridCell).toEqual({ width: 480, height: 320 })

    await openGrid(page, book.id)

    expect(await cellBoxes(page)).toHaveLength(3)
    await expectShape(page, { width: 480, height: 320 })
  })

  test('every page is portrait', async ({ page, request }) => {
    const book = await ingest(request, 'Portrait', imageSet('portrait'))
    expect(book.gridCell).toEqual({ width: 320, height: 480 })

    await openGrid(page, book.id)

    await expectShape(page, { width: 320, height: 480 })
  })

  test('pages mix orientations', async ({ page, request }) => {
    const book = await ingest(request, 'Mixed', imageSet('mixed'))
    expect(book.gridCell).toEqual({ width: 480, height: 480 })

    await openGrid(page, book.id)

    await expectShape(page, { width: 480, height: 480 })
  })

  test('a page is square', async ({ page, request }) => {
    const book = await ingest(request, 'Square', imageSet('square'))
    expect(book.gridCell).toEqual({ width: 480, height: 480 })

    await openGrid(page, book.id)

    await expectShape(page, { width: 480, height: 480 })
  })

  test('pages are extremely wide', async ({ page, request }) => {
    const book = await ingest(request, 'Extreme', imageSet('extreme'))
    expect(book.gridCell).toEqual({ width: 480, height: 240 })

    await openGrid(page, book.id)

    await expectShape(page, { width: 480, height: 240 })
  })

  test('the book has one page', async ({ page, request }) => {
    const book = await ingest(request, 'Single', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    expect(book.gridCell).toEqual({ width: 320, height: 480 })

    await openGrid(page, book.id)

    expect(await cellBoxes(page)).toHaveLength(1)
    await expectShape(page, { width: 320, height: 480 })
  })
})

test.describe('opening a book shows its pages in a regular grid', () => {
  test('a user opens a book', async ({ page, request }) => {
    const book = await ingest(request, 'Ten Pager', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    await openGrid(page, book.id)

    const cells = page.getByTestId('page-grid').locator('li')
    await expect(cells).toHaveCount(10)

    const boxes = await cellBoxes(page)
    const first = boxes[0]
    for (const box of boxes) {
      expect(box).toEqual(first)
    }

    expect(await cellsPerRow(page)).toBe(5)

    for (let index = 1; index <= 10; index++) {
      await expect(page.getByTestId(`page-cell-${index}`)).toHaveCount(1)
    }
  })

  test('every row holds five cells at any width', async ({ page, request }) => {
    const book = await ingest(request, 'Ten Pager', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    for (const width of [1280, 1024, 800]) {
      await page.setViewportSize({ width, height: 900 })
      await openGrid(page, book.id)

      expect(await cellsPerRow(page), `at ${width}px`).toBe(5)

      const boxes = await cellBoxes(page)
      expect(boxes[0].width / boxes[0].height).toBeCloseTo(
        book.gridCell.width / book.gridCell.height,
        1,
      )
    }
  })

  test('a page is smaller than its cell', async ({ page, request }) => {
    const book = await ingest(request, 'Tiny', [
      { filename: 'page1.jpg', path: fixture('small', 'page1.jpg') },
    ])

    await openGrid(page, book.id)

    const image = page.getByTestId('page-cell-1').locator('img')
    await expect(image).toBeVisible()

    const fit = await image.evaluate((node) => getComputedStyle(node).objectFit)
    expect(fit).toBe('contain')

    const rendered = await image.evaluate((img: HTMLImageElement) => ({
      natural: img.naturalWidth,
      box: Math.round(img.getBoundingClientRect().width),
    }))
    expect(rendered.natural).toBe(300)
    expect(rendered.box).toBeGreaterThan(0)
  })

  test('thumbnails load', async ({ page, request }) => {
    const book = await ingest(request, 'Loads', imageSet('portrait'))

    await openGrid(page, book.id)

    const images = page.getByTestId('page-grid').locator('img')
    await expect(images).toHaveCount(3)
    for (let i = 0; i < 3; i++) {
      await expect(images.nth(i)).toBeVisible()
      await expect
        .poll(() => images.nth(i).evaluate((img: HTMLImageElement) => img.naturalWidth), {
          timeout: 15_000,
        })
        .toBeGreaterThan(0)
    }
  })
})
