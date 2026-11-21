import { expect, test } from '@playwright/test'
import {
  failDewarp,
  fixture,
  imageSet,
  ingest,
  reset,
  resetDewarp,
  startDewarp,
  startSplit,
  uploadBook,
  waitForSettled,
} from './helpers'

test.beforeEach(async ({ request }) => {
  await reset()
  await resetDewarp(request)
})

test.describe('stages appear as tabs', () => {
  test('the app opens', async ({ page }) => {
    await page.goto('/')

    await expect(page).toHaveURL(/\/stage\/uploaded$/)
    const tabs = page.getByTestId('stage-tabs').locator('a')
    await expect(tabs).toHaveText(['Uploaded', 'Dewarped', 'Split', 'OCR'])
    await expect(page.getByTestId('stage-tab-uploaded')).toHaveClass(/selected/)
  })

  test('the selected tab survives a reload', async ({ page }) => {
    await page.goto('/stage/dewarped')
    await expect(page.getByTestId('stage-tab-dewarped')).toHaveClass(/selected/)

    await page.reload()

    await expect(page).toHaveURL(/\/stage\/dewarped$/)
    await expect(page.getByTestId('stage-tab-dewarped')).toHaveClass(/selected/)
  })

  test('an unknown stage is requested', async ({ page }) => {
    await page.goto('/stage/nonsense')

    await expect(page).toHaveURL(/\/stage\/uploaded$/)
    await expect(page.getByTestId('stage-tab-uploaded')).toHaveClass(/selected/)
  })
})

test.describe('a stage tab lists every book of that stage in one ordered list', () => {
  test('books wait in the Uploaded stage', async ({ page, request }) => {
    const names = ['First Book', 'Second Book', 'Third Book']
    for (const name of names) {
      await ingest(request, name, [{ filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') }])
    }

    await page.goto('/stage/uploaded')

    const cards = page.getByTestId('stage-books').locator('li')
    await expect(cards).toHaveCount(3)
    for (const name of names) {
      await expect(page.getByText(name, { exact: true })).toBeVisible()
    }

    const covers = page.getByTestId('stage-books').locator('img')
    await expect(covers).toHaveCount(3)
    for (let i = 0; i < 3; i++) {
      await expect(covers.nth(i)).toHaveAttribute('src', /^\/files\/.+\.480\.jpg$/)
    }
  })

  test('every cover actually loads', async ({ page, request }) => {
    await ingest(request, 'Cover Check', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    await page.goto('/stage/uploaded')
    const cover = page.getByTestId('stage-books').locator('img').first()
    await expect(cover).toBeVisible()

    // The cover is lazily loaded, so decoding finishes after it is visible.
    await expect
      .poll(() => cover.evaluate((img: HTMLImageElement) => img.naturalWidth), { timeout: 15_000 })
      .toBeGreaterThan(0)
  })

  test('no books wait in a stage', async ({ page }) => {
    await page.goto('/stage/uploaded')

    await expect(page.getByTestId('stage-empty')).toBeVisible()
    await expect(page.getByTestId('stage-empty')).toContainText('Upload')
  })

  test('a book belongs to a different stage', async ({ page, request }) => {
    const book = await ingest(request, 'Only Uploaded', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    expect(book.stage).toBe('uploaded')

    await page.goto('/stage/split')
    await expect(page.getByTestId('stage-empty')).toBeVisible()

    // `ocr` is a stage of its own now, so an unknown stage needs another name.
    const other = await request.get('/api/books?stage=nonsense')
    expect(other.status()).toBe(422)
  })

  // The OCR tab holds the end of the pipeline. Until an OCR job runs, it must
  // stay visible, empty and harmless.
  test('the OCR tab is empty until an OCR job runs', async ({ page, request }) => {
    const source = await ingest(request, 'Split Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    const copy = await startSplit(request, source.id)
    const settled = await waitForSettled(request, copy.id)
    expect(settled.stage).toBe('split')

    await page.goto('/stage/ocr')
    await expect(page.getByTestId('stage-tab-ocr')).toBeVisible()
    await expect(page.getByTestId('stage-empty')).toBeVisible()
    await expect(page.getByTestId(`book-card-${copy.id}`)).toHaveCount(0)
    await expect(page.getByTestId('select-all')).toHaveCount(0)
  })

  // The five state groups, in the order the list must show them.
  test('the list orders the state groups', async ({ page, request }) => {
    const failed = await uploadBook(request, 'Failed Book', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])
    await waitForSettled(request, failed.id)

    const ready = await ingest(request, 'Ready Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    const readyNewer = await ingest(request, 'Newer Ready Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const working = await ingest(request, 'Working Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    const done = await ingest(request, 'Done Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const doneCopy = await startSplit(request, done.id)
    await waitForSettled(request, doneCopy.id)

    // An endpoint that always fails holds this copy out of the done group, so
    // the order does not turn on how fast a job runs.
    await failDewarp(request, 'job')
    await startDewarp(request, working.id)

    await page.goto('/stage/uploaded')

    const cards = page.getByTestId('stage-books').locator('li')
    await expect(cards).toHaveCount(5)

    const ordered = [failed.id, readyNewer.id, ready.id, working.id, done.id]
    for (const [position, id] of ordered.entries()) {
      await expect(cards.nth(position)).toHaveAttribute('data-testid', `book-card-${id}`)
    }

    await expect(page.getByTestId(`book-card-${failed.id}`)).toHaveAttribute('data-group', 'failed')
    await expect(page.getByTestId(`book-card-${ready.id}`)).toHaveAttribute('data-group', 'ready')
    await expect(page.getByTestId(`book-card-${working.id}`)).toHaveAttribute('data-group', 'working')
    await expect(page.getByTestId(`book-card-${done.id}`)).toHaveAttribute('data-group', 'done')
  })

  test('a card is marked by the state of its copy', async ({ page, request }) => {
    const working = await ingest(request, 'Working Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    const done = await ingest(request, 'Done Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    const plain = await ingest(request, 'Plain Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const copy = await startSplit(request, done.id)
    await waitForSettled(request, copy.id)

    // An endpoint that always fails keeps this copy out of the done group for
    // the whole test, whatever the job timing.
    await failDewarp(request, 'job')
    const stuck = await startDewarp(request, working.id)

    await page.goto('/stage/uploaded')

    await expect(
      page.getByTestId(`book-mark-${done.id}`).locator('[data-testid="mark-check"]'),
    ).toBeVisible()
    await expect(
      page.getByTestId(`book-mark-${working.id}`).locator('[data-testid="mark-spinner"]'),
    ).toBeVisible()
    await expect(page.getByTestId(`book-mark-${plain.id}`)).toHaveCount(0)

    // The card with a finished copy steps back further than the one still
    // working, and both step back from a card that took no action.
    const opacity = async (id: number) =>
      Number(
        await page
          .getByTestId(`book-card-${id}`)
          .locator('.face')
          .evaluate((node) => getComputedStyle(node).opacity),
      )
    expect(await opacity(done.id)).toBeLessThan(await opacity(working.id))
    expect(await opacity(working.id)).toBeLessThan(await opacity(plain.id))

    // A copy that failed leaves its source in the same group, with a warning.
    const settled = await waitForSettled(request, stuck.id)
    expect(settled.state).toBe('failed')

    await expect(
      page.getByTestId(`book-mark-${working.id}`).locator('[data-testid="mark-warning"]'),
    ).toBeVisible()
    await expect(page.getByTestId(`book-card-${working.id}`)).toHaveAttribute(
      'data-group',
      'working',
    )
  })

  test('a dewarped book reaches the Dewarped stage', async ({ page, request }) => {
    const book = await ingest(request, 'Crosses Over', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const copy = await startDewarp(request, book.id)
    const settled = await waitForSettled(request, copy.id)
    expect(settled.stage).toBe('dewarped')

    await page.goto('/stage/dewarped')

    const card = page.getByTestId(`book-card-${copy.id}`)
    await expect(card).toBeVisible()
    await expect(card.getByText('Crosses Over', { exact: true })).toBeVisible()
    await expect(card.locator('img')).toHaveAttribute('src', /^\/files\/.+\.480\.jpg$/)
  })
})

test.describe('a card shows the state of its book', () => {
  test('a book is ingesting, then completes without a reload', async ({ page, request }) => {
    await page.goto('/stage/uploaded')
    await expect(page.getByTestId('stage-empty')).toBeVisible()

    const created = await uploadBook(request, 'Watch Me', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    const card = page.getByTestId(`book-card-${created.id}`)
    await expect(card).toBeVisible()
    await expect(card).toHaveAttribute('data-group', 'running')
    await expect(card.locator('[data-testid="mark-spinner"]')).toBeVisible()
    await expect(card.locator('img')).toHaveCount(0)
    await expect(card.getByText('Watch Me', { exact: true })).toBeVisible()

    // The phase text is whatever the job reports, without a page reload.
    await expect(page.getByTestId(`book-phase-${created.id}`)).toHaveText(
      /Waiting in local queue|Converting/,
    )

    await expect(card).toHaveAttribute('data-group', 'ready', { timeout: 90_000 })
    await expect(card.locator('img')).toHaveCount(1)
  })

  test('ingest fails', async ({ page, request }) => {
    await page.goto('/stage/uploaded')

    const created = await uploadBook(request, 'Broken', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])
    await waitForSettled(request, created.id)

    const card = page.getByTestId(`book-card-${created.id}`)
    await expect(card).toBeVisible({ timeout: 60_000 })
    await expect(card).toHaveAttribute('data-group', 'failed')
    await expect(card.locator('[data-testid="mark-warning"]')).toBeVisible()
    await expect(page.getByTestId(`book-phase-${created.id}`)).toContainText('corrupt.jpg')
  })

  test('a failed book heads the list', async ({ page, request }) => {
    const ready = await ingest(request, 'Fine Book', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])
    const broken = await uploadBook(request, 'Broken Book', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])
    await waitForSettled(request, broken.id)

    await page.goto('/stage/uploaded')

    const cards = page.getByTestId('stage-books').locator('li')
    await expect(cards).toHaveCount(2)
    await expect(cards.nth(0)).toHaveAttribute('data-testid', `book-card-${broken.id}`)
    await expect(cards.nth(1)).toHaveAttribute('data-testid', `book-card-${ready.id}`)
  })
})

test.describe('book lists update live', () => {
  test('another user uploads a book', async ({ page, browser, request }) => {
    await page.goto('/stage/uploaded')
    await expect(page.getByTestId('stage-tabs')).toBeVisible()

    const second = await browser.newContext()
    const other = await second.newPage()
    await other.goto('/stage/uploaded')

    const created = await uploadBook(request, 'From Elsewhere', imageSet('portrait'))
    await waitForSettled(request, created.id)

    await expect(page.getByTestId(`book-card-${created.id}`)).toBeVisible({ timeout: 60_000 })
    await expect(other.getByTestId(`book-card-${created.id}`)).toBeVisible({ timeout: 60_000 })

    await second.close()
  })

  // A card must not jump under the pointer while only its phase changes.
  test('a book changes state group without a reload', async ({ page, request }) => {
    const first = await ingest(request, 'Alpha', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    await page.goto('/stage/uploaded')
    await expect(page.getByTestId(`book-card-${first.id}`)).toHaveAttribute('data-group', 'ready')

    await startDewarp(request, first.id)

    await expect(page.getByTestId(`book-card-${first.id}`)).toHaveAttribute('data-group', 'working')
    await expect(page.getByTestId(`book-card-${first.id}`)).toHaveAttribute(
      'data-group',
      'done',
      { timeout: 90_000 },
    )
  })

  test('the connection state is visible', async ({ page }) => {
    await page.goto('/stage/uploaded')

    await expect(page.getByText(/^ws: /)).toBeVisible()
    await expect(page.getByText('ws: open')).toBeVisible()
  })
})
