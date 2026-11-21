import { expect, test } from '@playwright/test'
import type { APIRequestContext } from '@playwright/test'
import {
  fixture,
  ingest,
  listBooks,
  reset,
  resetDewarp,
  startDewarp,
  uploadBook,
  waitForSettled,
  type BookDetail,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

function seed(request: APIRequestContext, name: string): Promise<BookDetail> {
  return ingest(request, name, [{ filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') }])
}

test.describe('a book that is ready for an action carries a checkbox', () => {
  test('only a ready book carries one', async ({ page, request }) => {
    const ready = await seed(request, 'Ready')
    const working = await seed(request, 'Working')
    await startDewarp(request, working.id)

    const broken = await uploadBook(request, 'Broken', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])
    await waitForSettled(request, broken.id)

    await page.goto('/stage/uploaded')

    await expect(page.getByTestId(`book-select-${ready.id}`)).toBeVisible()
    await expect(page.getByTestId(`book-select-${working.id}`)).toHaveCount(0)
    await expect(page.getByTestId(`book-select-${broken.id}`)).toHaveCount(0)
  })

  test('selecting a book does not open it', async ({ page, request }) => {
    const book = await seed(request, 'Stay Here')

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-select-${book.id}`).click()

    await expect(page).toHaveURL(/\/stage\/uploaded$/)
    await expect(page.getByTestId('bulk-actions')).toContainText('1 selected')
  })
})

test.describe('the top bar holds the selection controls', () => {
  test('nothing is selected', async ({ page, request }) => {
    await seed(request, 'Alone')

    await page.goto('/stage/uploaded')

    await expect(page.getByTestId('select-all')).toBeVisible()
    await expect(page.getByTestId('deselect-all')).toHaveCount(0)
    await expect(page.getByTestId('bulk-actions')).toHaveCount(0)
  })

  test('select all covers every ready book', async ({ page, request }) => {
    const first = await seed(request, 'One')
    const second = await seed(request, 'Two')
    const working = await seed(request, 'Working')
    await startDewarp(request, working.id)

    await page.goto('/stage/uploaded')
    await expect(page.getByTestId(`book-card-${first.id}`)).toBeVisible()

    await page.getByTestId('select-all').click()

    await expect(page.getByTestId('bulk-actions')).toContainText('2 selected')
    await expect(page.getByTestId(`book-select-${first.id}`)).toBeChecked()
    await expect(page.getByTestId(`book-select-${second.id}`)).toBeChecked()
  })

  test('deselect all clears the selection', async ({ page, request }) => {
    await seed(request, 'One')

    await page.goto('/stage/uploaded')
    await page.getByTestId('select-all').click()
    await expect(page.getByTestId('deselect-all')).toBeVisible()

    await page.getByTestId('deselect-all').click()

    await expect(page.getByTestId('deselect-all')).toHaveCount(0)
    await expect(page.getByTestId('bulk-actions')).toHaveCount(0)
  })

  test('a tab change clears the selection', async ({ page, request }) => {
    await seed(request, 'One')

    await page.goto('/stage/uploaded')
    await page.getByTestId('select-all').click()
    await expect(page.getByTestId('bulk-actions')).toBeVisible()

    await page.getByTestId('stage-tab-dewarped').click()

    await expect(page.getByTestId('bulk-actions')).toHaveCount(0)
  })

  test('the last stage holds no selection control', async ({ page }) => {
    await page.goto('/stage/ocr')

    await expect(page.getByTestId('upload-button')).toBeVisible()
    await expect(page.getByTestId('select-all')).toHaveCount(0)
  })
})

test.describe('a bulk action applies to every selected book', () => {
  test('the menu holds the stages after the current one', async ({ page, request }) => {
    await seed(request, 'One')

    await page.goto('/stage/uploaded')
    await page.getByTestId('select-all').click()
    await page.getByTestId('bulk-actions').click()

    await expect(page.getByTestId('bulk-menu')).toBeVisible()
    const items = page.getByTestId('bulk-menu').locator('.n-dropdown-option-body__label')
    await expect(items).toHaveText(['Dewarp', 'Split', 'OCR'])
  })

  test('the menu on the middle tab holds Split and OCR', async ({ page, request }) => {
    const book = await seed(request, 'Middle')
    const copy = await startDewarp(request, book.id)
    await waitForSettled(request, copy.id)

    await page.goto('/stage/dewarped')
    await page.getByTestId('select-all').click()
    await page.getByTestId('bulk-actions').click()

    const items = page.getByTestId('bulk-menu').locator('.n-dropdown-option-body__label')
    await expect(items).toHaveText(['Split', 'OCR'])
  })

  test('a user runs a bulk action over three books', async ({ page, request }) => {
    const books = []
    for (const name of ['One', 'Two', 'Three']) {
      books.push(await seed(request, name))
    }

    await page.goto('/stage/uploaded')
    await expect(page.getByTestId(`book-card-${books[2].id}`)).toBeVisible()

    await page.getByTestId('select-all').click()
    await expect(page.getByTestId('bulk-actions')).toContainText('3 selected')

    await page.getByTestId('bulk-actions').click()
    await page.getByTestId('bulk-dewarp').click()

    for (const book of books) {
      await expect(page.getByTestId(`book-card-${book.id}`)).toHaveAttribute('data-group', 'working')
      await expect(page.getByTestId(`book-mark-${book.id}`)).toBeVisible()
    }

    // The selection is spent, so the controls go away.
    await expect(page.getByTestId('bulk-actions')).toHaveCount(0)
    await expect(page.getByTestId('deselect-all')).toHaveCount(0)

    expect(await listBooks(request)).toHaveLength(6)
  })

  test('one book of many is rejected', async ({ page, request }) => {
    const fine = await seed(request, 'Fine')
    const taken = await seed(request, 'Taken')
    await startDewarp(request, taken.id)

    await page.goto('/stage/uploaded')
    await page.getByTestId(`book-select-${fine.id}`).click()

    // The taken book has no checkbox, so the request is built by hand to show
    // what a stale selection would do.
    const result = await request.post('/api/books/actions/split', {
      data: { bookIds: [fine.id, taken.id] },
    })
    const answer = await result.json()
    expect(answer.started).toHaveLength(1)
    expect(answer.failed).toHaveLength(1)
    expect(answer.failed[0].bookId).toBe(taken.id)
  })

  test('the context menu runs the action on one book alone', async ({ page, request }) => {
    const first = await seed(request, 'One')
    const second = await seed(request, 'Two')

    await page.goto('/stage/uploaded')
    await page.getByTestId('select-all').click()
    await expect(page.getByTestId('bulk-actions')).toContainText('2 selected')

    await page.getByTestId(`book-card-${first.id}`).click({ button: 'right' })
    await page.getByTestId('context-split').click()

    await expect(page.getByTestId(`book-card-${first.id}`)).toHaveAttribute('data-group', 'working')
    await expect(page.getByTestId(`book-card-${second.id}`)).toHaveAttribute('data-group', 'ready')

    // A book that took its action leaves the selection, the other stays.
    await expect(page.getByTestId('bulk-actions')).toContainText('1 selected')
  })
})
