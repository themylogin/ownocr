import { expect, test } from '@playwright/test'
import { existsSync } from 'node:fs'
import {
  fixture,
  getBook,
  imageSet,
  ingest,
  listBooks,
  rejectBook,
  reset,
  resetDewarp,
  startDewarp,
  startSplit,
  storedPath,
  uploadBook,
  waitForSettled,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

test.describe('the context menu offers Reject', () => {
  test('Reject follows the actions, after a divider', async ({ page, request }) => {
    const book = await ingest(request, 'Atlas', imageSet('portrait'))

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })

    const menu = page.getByTestId('context-menu')
    await expect(menu).toBeVisible()
    await expect(menu.locator('.n-dropdown-divider')).toHaveCount(1)

    const items = menu.locator('.n-dropdown-option-body__label')
    await expect(items).toHaveText(['Dewarp', 'Split', 'OCR', 'Reject'])
  })

  test('Reject is red', async ({ page, request }) => {
    const book = await ingest(request, 'Atlas', imageSet('portrait'))

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${book.id}`).click({ button: 'right' })

    const reject = page.getByTestId('context-reject')
    const dewarp = page.getByTestId('context-dewarp')
    const colour = (node: typeof reject) => node.evaluate((el) => getComputedStyle(el).color)

    expect(await colour(reject)).not.toBe(await colour(dewarp))
    expect(await colour(reject)).toMatch(/rgb\(2\d\d, \d+, \d+\)/)
  })

  test('a running book can be rejected', async ({ page, request }) => {
    const created = await uploadBook(request, 'Sixty Plates', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${created.id}`).click({ button: 'right' })

    await expect(page.getByTestId('context-reject')).toBeVisible()
    await expect(page.getByTestId('context-dewarp')).toHaveCount(0)
  })
})

test.describe('rejecting a book destroys it and frees its source', () => {
  test('the dialog names what it destroys, and cancel keeps it', async ({ page, request }) => {
    const source = await ingest(request, 'Atlas', imageSet('portrait'))
    const copy = await startDewarp(request, source.id)
    await waitForSettled(request, copy.id)

    await page.goto('/stage/dewarped')
    await page.getByTestId(`book-card-${copy.id}`).click({ button: 'right' })
    await page.getByTestId('context-reject').click()

    const dialog = page.getByTestId('reject-dialog')
    await expect(dialog).toBeVisible()
    await expect(page.getByTestId('reject-count')).toContainText('1 book')
    await expect(page.getByTestId('reject-list').locator('li')).toHaveCount(1)
    await expect(page.getByTestId('reject-freed')).toBeVisible()

    await page.getByRole('button', { name: 'Cancel' }).click()

    await expect(dialog).toHaveCount(0)
    await expect(page.getByTestId(`book-card-${copy.id}`)).toBeVisible()
    expect(await listBooks(request)).toHaveLength(2)
  })

  test('the copy goes and its source is ready again', async ({ page, request }) => {
    const source = await ingest(request, 'Atlas', imageSet('portrait'))
    const copy = await startDewarp(request, source.id)
    const settled = await waitForSettled(request, copy.id)

    await page.goto('/stage/uploaded')
    await expect(page.getByTestId(`book-card-${source.id}`)).toHaveAttribute('data-group', 'done')

    // The reject runs in the Dewarped tab, and the Uploaded tab must follow it.
    const second = await page.context().newPage()
    await second.goto('/stage/dewarped')
    await second.getByTestId(`book-card-${copy.id}`).click({ button: 'right' })
    await second.getByTestId('context-reject').click()
    await second.getByTestId('reject-confirm').click()

    await expect(second.getByTestId(`book-card-${copy.id}`)).toHaveCount(0)
    await expect(second.getByTestId('stage-empty')).toBeVisible()

    // No reload here: the first tab learns it from the event stream.
    await expect(page.getByTestId(`book-card-${source.id}`)).toHaveAttribute('data-group', 'ready')
    await expect(page.getByTestId(`book-mark-${source.id}`)).toHaveCount(0)
    await expect(page.getByTestId(`book-select-${source.id}`)).toBeVisible()

    const books = await listBooks(request)
    expect(books).toHaveLength(1)
    expect(books[0].id).toBe(source.id)

    const freed = await getBook(request, source.id)
    expect(freed.derivedState).toBeNull()

    // The pages of the copy are gone; the source keeps its own.
    expect(existsSync(storedPath(settled.pages[0].originalUrl))).toBe(false)
    expect(existsSync(storedPath(source.pages[0].originalUrl))).toBe(true)

    await second.close()
  })

  test('the source can take its action again', async ({ request }) => {
    const source = await ingest(request, 'Atlas', imageSet('portrait'))
    const first = await startDewarp(request, source.id)
    await waitForSettled(request, first.id)

    await rejectBook(request, first.id)

    const second = await startDewarp(request, source.id)
    expect(second.id).not.toBe(first.id)
    const settled = await waitForSettled(request, second.id)
    expect(settled.state).toBe('complete')
  })
})

test.describe('rejecting a book destroys every copy made from it', () => {
  test('a whole chain goes at once', async ({ page, request }) => {
    const source = await ingest(request, 'Atlas', imageSet('portrait'))
    const dewarped = await startDewarp(request, source.id)
    await waitForSettled(request, dewarped.id)
    const done = await startSplit(request, dewarped.id)
    await waitForSettled(request, done.id)

    expect(await listBooks(request)).toHaveLength(3)

    await page.goto('/stage/dewarped')
    await page.getByTestId(`book-card-${dewarped.id}`).click({ button: 'right' })
    await page.getByTestId('context-reject').click()

    await expect(page.getByTestId('reject-count')).toContainText('2 books')
    await expect(page.getByTestId('reject-list').locator('li')).toHaveCount(2)

    await page.getByTestId('reject-confirm').click()
    await expect(page.getByTestId('reject-dialog')).toHaveCount(0)

    const left = await listBooks(request)
    expect(left).toHaveLength(1)
    expect(left[0].id).toBe(source.id)
    expect(left[0].derivedState).toBeNull()

    await page.goto('/stage/split')
    await expect(page.getByTestId('stage-empty')).toBeVisible()
  })

  test('rejecting the first book of a chain empties every tab', async ({ page, request }) => {
    const source = await ingest(request, 'Atlas', imageSet('portrait'))
    const dewarped = await startDewarp(request, source.id)
    await waitForSettled(request, dewarped.id)
    const done = await startSplit(request, dewarped.id)
    await waitForSettled(request, done.id)

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-card-${source.id}`).click({ button: 'right' })
    await page.getByTestId('context-reject').click()

    await expect(page.getByTestId('reject-count')).toContainText('3 books')
    await expect(page.getByTestId('reject-freed')).toHaveCount(0)

    await page.getByTestId('reject-confirm').click()

    await expect(page.getByTestId('stage-empty')).toBeVisible()
    expect(await listBooks(request)).toHaveLength(0)
  })
})

test.describe('the reader offers the actions of the book', () => {
  test('the sticky bar keeps them on every page', async ({ page, request }) => {
    const book = await ingest(request, 'Sixty Plates', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])

    await page.goto(`/books/${book.id}/pages/1`)
    await expect(page.getByTestId('reader-pages')).toBeVisible()
    await expect(page.getByTestId('action-dewarp')).toBeVisible()
    await expect(page.getByTestId('action-split')).toBeVisible()
    await expect(page.getByTestId('action-reject')).toBeVisible()

    // Deep into the book, the same controls are still on screen.
    await page.locator('[data-page="40"]').scrollIntoViewIfNeeded()
    await expect(page.getByTestId('action-dewarp')).toBeInViewport()
    await expect(page.getByTestId('action-reject')).toBeInViewport()
    await expect(page.getByTestId('crumb-0')).toBeInViewport()
  })

  test('an action runs from the reader', async ({ page, request }) => {
    const book = await ingest(request, 'Atlas', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await page.getByTestId('action-split').click()

    await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()
    await expect(page.getByTestId('action-split')).toHaveCount(0)

    const copy = (await listBooks(request)).find((entry) => entry.sourceBookId === book.id)
    expect(copy?.action).toBe('split')
  })

  test('a book whose copy is done shows the mark, not the actions', async ({ page, request }) => {
    const book = await ingest(request, 'Atlas', imageSet('portrait'))
    const copy = await startSplit(request, book.id)
    await waitForSettled(request, copy.id)

    await page.goto(`/books/${book.id}/pages/1`)

    await expect(
      page.getByTestId(`book-mark-${book.id}`).locator('[data-testid="mark-check"]'),
    ).toBeVisible()
    await expect(page.getByTestId('action-split')).toHaveCount(0)
    await expect(page.getByTestId('action-reject')).toBeVisible()
  })

  test('rejecting from the reader returns to the stage', async ({ page, request }) => {
    const book = await ingest(request, 'Atlas', imageSet('portrait'))

    await page.goto(`/books/${book.id}/pages/1`)
    await page.getByTestId('action-reject').click()

    await expect(page.getByTestId('reject-count')).toContainText('1 book')
    await page.getByTestId('reject-confirm').click()

    await expect(page).toHaveURL(/\/stage\/uploaded$/)
    await expect(page.getByTestId('stage-empty')).toBeVisible()
    expect(await listBooks(request)).toHaveLength(0)
  })

  test('rejecting from the book view returns to the stage', async ({ page, request }) => {
    const book = await ingest(request, 'Atlas', imageSet('portrait'))

    await page.goto(`/books/${book.id}`)
    await page.getByTestId('action-reject').click()
    await page.getByTestId('reject-confirm').click()

    await expect(page).toHaveURL(/\/stage\/uploaded$/)
    expect(await listBooks(request)).toHaveLength(0)
  })
})

test.describe('rejecting a failed book removes it', () => {
  test('a broken upload can be cleared', async ({ page, request }) => {
    const broken = await uploadBook(request, 'Torn Scan', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])
    await waitForSettled(request, broken.id)

    await page.goto('/stage/uploaded')
    await expect(page.getByTestId(`book-card-${broken.id}`)).toHaveAttribute('data-group', 'failed')

    await page.getByTestId(`book-card-${broken.id}`).click({ button: 'right' })
    await page.getByTestId('context-reject').click()
    await page.getByTestId('reject-confirm').click()

    await expect(page.getByTestId(`book-card-${broken.id}`)).toHaveCount(0)
    expect(await listBooks(request)).toHaveLength(0)
  })

  test('an unknown book cannot be rejected', async ({ request }) => {
    const response = await request.post('/api/books/999999/reject')
    expect(response.status()).toBe(404)
  })
})
