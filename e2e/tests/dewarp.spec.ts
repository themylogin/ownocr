import { expect, test } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
import {
  failDewarp,
  fixture,
  getBook,
  imageSet,
  ingest,
  listBooks,
  objectKeys,
  reset,
  resetDewarp,
  runAction,
  startDewarp,
  startSplit,
  waitForSettled,
  type Book,
  type BookDetail,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

async function seedBook(request: APIRequestContext, name = 'Atlas'): Promise<BookDetail> {
  return ingest(request, name, imageSet('portrait'))
}

async function copyOf(request: APIRequestContext, sourceId: number): Promise<Book> {
  const books = await listBooks(request)
  const copy = books.find((book) => book.sourceBookId === sourceId)
  if (!copy) {
    throw new Error(`book ${sourceId} has no copy`)
  }
  return copy
}

test.describe('a stage offers the actions of the stages after it', () => {
  test('the Uploaded stage offers Dewarp, Split and OCR', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto(`/books/${book.id}`)

    await expect(page.getByTestId('action-dewarp')).toHaveText('Dewarp')
    await expect(page.getByTestId('action-split')).toHaveText('Split')
    await expect(page.getByTestId('action-ocr')).toHaveText('OCR')
    await expect(page.getByTestId(`book-mark-${book.id}`)).toHaveCount(0)
  })

  test('a user right clicks a book card', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })

    await expect(page.getByTestId('context-menu')).toBeVisible()
    await expect(page.getByTestId('context-dewarp')).toHaveText('Dewarp')
    await expect(page.getByTestId('context-split')).toHaveText('Split')
    await expect(page.getByTestId('context-ocr')).toHaveText('OCR')
  })

  test('the Dewarped stage offers Split and OCR', async ({ page, request }) => {
    const book = await seedBook(request)
    const copy = await startDewarp(request, book.id)
    const settled = await waitForSettled(request, copy.id)
    expect(settled.stage).toBe('dewarped')

    await page.goto(`/books/${copy.id}`)
    await expect(page.getByTestId('action-split')).toBeVisible()
    await expect(page.getByTestId('action-ocr')).toBeVisible()
    await expect(page.getByTestId('action-dewarp')).toHaveCount(0)

    await page.goto('/stage/dewarped')
    await page.getByTestId(`book-card-${copy.id}`).click({ button: 'right' })
    await expect(page.getByTestId('context-menu')).toBeVisible()
    await expect(page.getByTestId('context-dewarp')).toHaveCount(0)
    await expect(page.getByTestId('context-split')).toBeVisible()
    await expect(page.getByTestId('context-ocr')).toBeVisible()
  })

  test('the Split stage offers OCR alone', async ({ page, request }) => {
    const book = await seedBook(request)
    const copy = await startSplit(request, book.id)
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${copy.id}`)
    await expect(page.getByTestId('page-grid')).toBeVisible()
    await expect(page.getByTestId('action-ocr')).toBeVisible()
    await expect(page.getByTestId('action-split')).toHaveCount(0)
    await expect(page.getByTestId('action-dewarp')).toHaveCount(0)

    await page.goto('/stage/split')
    await page.getByTestId(`book-card-${copy.id}`).click({ button: 'right' })
    await expect(page.getByTestId('context-menu')).toBeVisible()
    await expect(page.getByTestId('context-split')).toHaveCount(0)
    await expect(page.getByTestId('context-dewarp')).toHaveCount(0)
    await expect(page.getByTestId('context-ocr')).toBeVisible()
    await expect(page.getByTestId('context-reject')).toBeVisible()
  })

  test('a user dismisses the context menu', async ({ page, request }) => {
    const book = await seedBook(request)
    await page.goto('/stage/uploaded')

    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })
    await expect(page.getByTestId('context-menu')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByTestId('context-menu')).toHaveCount(0)

    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })
    await expect(page.getByTestId('context-menu')).toBeVisible()
    await page.getByTestId('stage-books').click({ position: { x: 5, y: 5 } })
    await expect(page.getByTestId('context-menu')).toHaveCount(0)

    expect(await listBooks(request)).toHaveLength(1)
  })

  test('a book that already has a copy offers only Reject', async ({ page, request }) => {
    const book = await seedBook(request)
    await startDewarp(request, book.id)

    await page.goto(`/books/${book.id}`)
    await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()
    await expect(page.getByTestId('action-dewarp')).toHaveCount(0)
    await expect(page.getByTestId('action-split')).toHaveCount(0)
    await expect(page.getByTestId('action-ocr')).toHaveCount(0)

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })
    await expect(page.getByTestId('context-menu')).toBeVisible()
    await expect(page.getByTestId('context-dewarp')).toHaveCount(0)
    await expect(page.getByTestId('context-split')).toHaveCount(0)
    await expect(page.getByTestId('context-ocr')).toHaveCount(0)
    await expect(page.getByTestId('context-reject')).toBeVisible()
  })
})

test.describe('an action creates its copy at once', () => {
  test('a user runs the action', async ({ page, request }) => {
    const book = await seedBook(request, 'Atlas')

    await page.goto(`/books/${book.id}`)
    await page.getByTestId('action-dewarp').click()
    await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()

    const books = await listBooks(request)
    expect(books).toHaveLength(2)

    const copy = await copyOf(request, book.id)
    expect(copy.name).toBe('Atlas')
    expect(copy.sourceBookId).toBe(book.id)
    expect(copy.action).toBe('dewarp')
    expect(copy.derivedState).toBeNull()

    const source = books.find((entry) => entry.id === book.id)
    expect(source?.derivedState).not.toBeNull()
  })

  test('the copy carries its stage while it runs', async ({ request }) => {
    const book = await seedBook(request)

    const copy = await startDewarp(request, book.id)
    expect(copy.state).toBe('pending')
    expect(copy.stage).toBe('dewarped')
    expect(copy.progress).toBe('Waiting in local queue')
    expect(copy.sourceBookId).toBe(book.id)
  })

  test('the copy is listed in the stage it is going to', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto('/stage/dewarped')
    await expect(page.getByTestId('stage-empty')).toBeVisible()

    const copy = await startDewarp(request, book.id)

    const card = page.getByTestId(`book-card-${copy.id}`)
    await expect(card).toBeVisible()
    await expect(card).toHaveAttribute('data-group', 'running')

    await expect(card).toHaveAttribute('data-group', 'ready', { timeout: 90_000 })
  })

  test('the copy finishes', async ({ page, request }) => {
    const book = await seedBook(request)
    const copy = await startDewarp(request, book.id)

    const settled = await waitForSettled(request, copy.id)
    expect(settled.state).toBe('complete')
    expect(settled.stage).toBe('dewarped')

    await page.goto('/stage/dewarped')
    await expect(page.getByTestId(`book-card-${copy.id}`)).toBeVisible()
    await expect(page.getByText(book.name, { exact: true })).toBeVisible()
  })

  test('the action runs twice', async ({ request }) => {
    const book = await seedBook(request)
    await startDewarp(request, book.id)

    const second = await runAction(request, 'dewarp', [book.id])
    expect(second.started).toHaveLength(0)
    expect(second.failed[0].error).toContain('already has a copy')
    expect(await listBooks(request)).toHaveLength(2)
  })

  test('a second action after the first is refused', async ({ request }) => {
    const book = await seedBook(request)
    await startDewarp(request, book.id)

    const second = await runAction(request, 'split', [book.id])
    expect(second.started).toHaveLength(0)
    expect(second.failed[0].error).toContain('already has a copy')
    expect(await listBooks(request)).toHaveLength(2)
  })

  test('the stage does not offer the action', async ({ request }) => {
    const book = await seedBook(request)
    const copy = await startDewarp(request, book.id)
    await waitForSettled(request, copy.id)

    const result = await runAction(request, 'dewarp', [copy.id])
    expect(result.started).toHaveLength(0)
    expect(result.failed[0].error).toContain('does not offer Dewarp')
  })

  test('the book does not exist', async ({ request }) => {
    const result = await runAction(request, 'dewarp', [999999])
    expect(result.started).toHaveLength(0)
    expect(result.failed[0]).toEqual({ bookId: 999999, error: 'no such book' })
  })

  test('the action does not exist', async ({ request }) => {
    const response = await request.post('/api/books/actions/nonsense', { data: { bookIds: [1] } })
    expect(response.status()).toBe(404)
  })

  test('a book that is not complete is refused', async ({ request }) => {
    const created = await ingest(request, 'Broken', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])
    expect(created.state).toBe('failed')

    const result = await runAction(request, 'split', [created.id])
    expect(result.started).toHaveLength(0)
    expect(result.failed[0].error).toContain('not complete')
  })
})

test.describe('a card is marked by the state of its copy', () => {
  test('the user runs the action in the book view', async ({ page, request }) => {
    const book = await seedBook(request)
    await page.goto(`/books/${book.id}`)

    const loads = { count: 0 }
    page.on('load', () => {
      loads.count += 1
    })

    await page.getByTestId('action-dewarp').click()

    await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()
    await expect(page.getByTestId('action-dewarp')).toHaveCount(0)
    expect(loads.count).toBe(0)
  })

  test('another user runs the action', async ({ page, request }) => {
    const book = await seedBook(request)

    await page.goto('/stage/uploaded')
    await expect(page.getByTestId(`book-card-${book.id}`)).toBeVisible()
    await expect(page.getByTestId(`book-mark-${book.id}`)).toHaveCount(0)

    await startDewarp(request, book.id)

    await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()
    await expect(page.getByTestId(`book-card-${book.id}`)).toHaveAttribute('data-group', 'working')
  })

  test('the mark survives a reload', async ({ page, request }) => {
    const book = await seedBook(request)
    const copy = await startDewarp(request, book.id)
    await waitForSettled(request, copy.id)

    await page.goto('/stage/uploaded')
    await expect(
      page.getByTestId(`book-mark-${book.id}`).locator('[data-testid="mark-check"]'),
    ).toBeVisible()

    await page.reload()

    await expect(
      page.getByTestId(`book-mark-${book.id}`).locator('[data-testid="mark-check"]'),
    ).toBeVisible()
    await expect(page.getByTestId(`book-card-${book.id}`)).toHaveAttribute('data-group', 'done')
    await expect(page.getByTestId(`book-card-${book.id}`).locator('.face')).toHaveCSS(
      'opacity',
      '0.55',
    )
  })
})

test.describe('the dewarped pages become the pages of the copy', () => {
  test('every page arrives in order at its dewarped size', async ({ request }) => {
    const source = await seedBook(request)
    const copy = await startDewarp(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('complete')
    expect(settled.pages).toHaveLength(source.pages.length)

    for (let i = 0; i < source.pages.length; i++) {
      expect(settled.pages[i].index).toBe(i + 1)
      // The fake endpoint halves every page, so a page of the copy is smaller
      // than its source page.
      expect(settled.pages[i].width).toBeLessThan(source.pages[i].width)
      expect(settled.pages[i].height).toBeLessThan(source.pages[i].height)
    }

    expect(settled.gridCell.width).toBeGreaterThan(0)
  })

  test('every rendition of a dewarped page loads', async ({ request }) => {
    const source = await seedBook(request)
    const copy = await startDewarp(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    const page = settled.pages[0]
    for (const url of [page.originalUrl, page.displayUrl, page.thumbnailUrl]) {
      const response = await request.get(url)
      expect(response.status(), url).toBe(200)
      expect(response.headers()['content-type']).toContain('image/')
    }
  })

  test('the exchange objects are gone', async ({ request }) => {
    const source = await seedBook(request)
    const copy = await startDewarp(request, source.id)
    await waitForSettled(request, copy.id)

    expect(await objectKeys(request)).toEqual([])
  })
})

test.describe('a failed copy allows a retry', () => {
  test('the endpoint job fails', async ({ page, request }) => {
    const book = await seedBook(request)
    await failDewarp(request, 'job')

    const copy = await startDewarp(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('failed')
    expect(settled.error).toContain('out of memory')
    expect(settled.sourceBookId).toBe(book.id)
    expect(settled.stage).toBe('dewarped')

    const source = await getBook(request, book.id)
    expect(source.derivedState).toBe('failed')
    expect(source.stage).toBe('uploaded')

    // The failed copy heads the list of the stage it was going to.
    await page.goto('/stage/dewarped')
    const card = page.getByTestId(`book-card-${copy.id}`)
    await expect(card).toBeVisible()
    await expect(card).toHaveAttribute('data-group', 'failed')
    await expect(page.getByTestId(`book-phase-${copy.id}`)).toContainText('out of memory')

    // The source book carries a warning mark and offers no action.
    await page.goto('/stage/uploaded')
    await expect(
      page.getByTestId(`book-mark-${book.id}`).locator('[data-testid="mark-warning"]'),
    ).toBeVisible()

    await failDewarp(request, 'off')
    await page.goto('/stage/dewarped')
    await page.getByTestId(`retry-${copy.id}`).click()

    const retried = await waitForSettled(request, copy.id)
    expect(retried.state).toBe('complete')
    expect(retried.pages).toHaveLength(book.pages.length)
    expect(await listBooks(request)).toHaveLength(2)
  })

  test('one page fails', async ({ request }) => {
    const book = await seedBook(request)
    await failDewarp(request, 'page')

    const copy = await startDewarp(request, book.id)
    const settled = await waitForSettled(request, copy.id)

    expect(settled.state).toBe('failed')
    expect(settled.error).toContain('page 1')
    expect(settled.pages).toHaveLength(0)
  })
})
