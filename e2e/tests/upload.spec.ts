import { expect, test } from '@playwright/test'
import { fixture, imageSet, postBook, reset, uploadBook, waitForSettled } from './helpers'

test.beforeEach(async () => {
  await reset()
})

test.describe('book upload accepts a PDF or an image set', () => {
  test('upload of a single PDF', async ({ request }) => {
    const response = await postBook(request, 'Ten Pager', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    expect(response.status()).toBe(201)
    const book = await response.json()
    expect(book.name).toBe('Ten Pager')
    expect(book.state).toBe('pending')
    expect(book.stage).toBe('uploaded')
  })

  test('upload of an image set', async ({ request }) => {
    const response = await postBook(request, 'Three Scans', imageSet('landscape'))

    expect(response.status()).toBe(201)
  })

  test('upload mixes a PDF with images', async ({ request }) => {
    const response = await postBook(request, 'Mixed', [
      { filename: 'book1.pdf', path: fixture('book1.pdf') },
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    expect(response.status()).toBe(422)
    expect((await response.json()).error).toContain('either one PDF or a set of images')
  })

  test('upload contains two PDF files', async ({ request }) => {
    const response = await postBook(request, 'Two PDFs', [
      { filename: 'book1.pdf', path: fixture('book1.pdf') },
      { filename: 'second.pdf', path: fixture('second.pdf') },
    ])

    expect(response.status()).toBe(422)
  })

  test('upload contains an unsupported file type', async ({ request }) => {
    const response = await postBook(request, 'Notes', [
      { filename: 'notes.txt', path: fixture('notes.txt') },
    ])

    expect(response.status()).toBe(422)
    expect((await response.json()).error).toContain('notes.txt')
  })

  test('upload contains no files', async ({ request }) => {
    const response = await postBook(request, 'Empty', [])

    expect(response.status()).toBe(422)
  })

  test('a declared content type cannot smuggle a text file through', async ({ request }) => {
    const response = await postBook(request, 'Liar', [
      { filename: 'notes.jpg', path: fixture('notes.txt') },
    ])

    expect(response.status()).toBe(422)
  })
})

test.describe('a book name is required', () => {
  test('name is missing', async ({ request }) => {
    const response = await postBook(request, null, [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    expect(response.status()).toBe(422)
    expect((await response.json()).error).toContain('name is required')
  })

  test('name is only whitespace', async ({ request }) => {
    const response = await postBook(request, '   ', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    expect(response.status()).toBe(422)
  })

  test('name has surrounding whitespace', async ({ request }) => {
    const book = await uploadBook(request, '  Moby Dick  ', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    expect(book.name).toBe('Moby Dick')
  })

  test('two books share a name', async ({ request }) => {
    const first = await uploadBook(request, 'Same Title', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])
    const second = await uploadBook(request, 'Same Title', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    expect(second.id).not.toBe(first.id)
    await waitForSettled(request, first.id)
    await waitForSettled(request, second.id)
  })
})

test.describe('the upload form prefills the name from a PDF file name', () => {
  test('user selects a PDF', async ({ page }) => {
    await page.goto('/stage/uploaded')
    await page.getByTestId('upload-button').click()
    await page.getByTestId('upload-files').setInputFiles(fixture('book10.pdf'))

    await expect(page.getByTestId('upload-name')).toHaveValue('book10')
  })

  test('user selects images', async ({ page }) => {
    await page.goto('/stage/uploaded')
    await page.getByTestId('upload-button').click()
    await page
      .getByTestId('upload-files')
      .setInputFiles(imageSet('landscape').map((file) => file.path))

    await expect(page.getByTestId('upload-name')).toHaveValue('')
    await expect(page.getByTestId('upload-submit')).toBeDisabled()
  })

  test('user edits the prefilled name', async ({ page, request }) => {
    await page.goto('/stage/uploaded')
    await page.getByTestId('upload-button').click()
    await page.getByTestId('upload-files').setInputFiles(fixture('second.pdf'))
    await expect(page.getByTestId('upload-name')).toHaveValue('second')

    await page.getByTestId('upload-name').fill('My Own Title')
    await page.getByTestId('upload-submit').click()
    await expect(page.getByTestId('upload-dialog')).toBeHidden()

    const listed = await request.get('/api/books')
    const books = await listed.json()
    expect(books[0].name).toBe('My Own Title')
  })
})

test.describe('upload is reachable from every stage tab', () => {
  test('user opens the upload dialog', async ({ page }) => {
    await page.goto('/stage/uploaded')
    await page.getByTestId('upload-button').click()

    await expect(page.getByTestId('upload-dialog')).toBeVisible()
    await expect(page.getByTestId('upload-name')).toBeVisible()
    await expect(page.getByTestId('upload-files')).toBeAttached()
  })
})

test.describe('the upload preserves the file order', () => {
  test('the interface shows what it will send', async ({ page }) => {
    await page.goto('/stage/uploaded')
    await page.getByTestId('upload-button').click()
    await page
      .getByTestId('upload-files')
      .setInputFiles([fixture('unordered', 'b.jpg'), fixture('unordered', 'a.jpg'), fixture('unordered', 'c.jpg')])

    const listed = page.getByTestId('upload-file-list').locator('li')
    await expect(listed).toHaveText(['b.jpg', 'a.jpg', 'c.jpg'])
  })
})

test.describe('upload size has a configured limit', () => {
  test('a large upload is accepted under the default limit', async ({ request }) => {
    const big = Buffer.alloc(12 * 1024 * 1024, 7)
    const response = await postBook(request, 'Big But Invalid', [
      { filename: 'big.bin', body: big },
    ])

    // 12 MB is far past the old 1 MB JSON cap and the old 25 MB nginx cap is
    // not in play here: the body is read, then rejected on type, not on size.
    expect(response.status()).toBe(422)
  })
})

test.describe('a new book is announced to connected clients', () => {
  test('a second browser is open', async ({ page, request }) => {
    await page.goto('/stage/uploaded')
    await expect(page.getByTestId('stage-tabs')).toBeVisible()

    const events = page.evaluate(
      () =>
        new Promise<string>((resolve) => {
          const socket = new WebSocket(
            (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/api/ws',
          )
          socket.onmessage = (message) => {
            const event = JSON.parse(message.data as string)
            if (event.type === 'book.created') {
              resolve(JSON.stringify(event.payload))
            }
          }
        }),
    )

    await page.waitForTimeout(500)
    await uploadBook(request, 'Broadcast Me', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const payload = JSON.parse(await events)
    expect(payload.name).toBe('Broadcast Me')
    expect(payload.state).toBe('pending')
    expect(payload.id).toBeGreaterThan(0)
  })
})
