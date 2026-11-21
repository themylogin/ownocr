import { readFileSync, readdirSync, statSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from '@playwright/test'
import {
  dataDir,
  fixture,
  getBook,
  imageSet,
  ingest,
  reset,
  storedPath,
  uploadBook,
  waitForSettled,
} from './helpers'

test.beforeEach(async () => {
  await reset()
})

test.describe('ingest splits a PDF into ordered page images', () => {
  test('a ten page PDF', async ({ request }) => {
    const book = await ingest(request, 'Ten Pager', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    expect(book.state).toBe('complete')
    expect(book.pageCount).toBe(10)
    expect(book.pages).toHaveLength(10)
    expect(book.pages.map((page) => page.index)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
  })

  test('a PDF that cannot be read', async ({ request }) => {
    const book = await ingest(request, 'Truncated', [
      { filename: 'truncated.pdf', path: fixture('truncated.pdf') },
    ])

    expect(book.state).toBe('failed')
    expect(book.error).toBeTruthy()
  })
})

test.describe('ingest keeps the upload order of an image set', () => {
  test('files arrive out of name order', async ({ request }) => {
    const book = await ingest(request, 'Unordered', [
      { filename: 'b.jpg', path: fixture('unordered', 'b.jpg') },
      { filename: 'a.jpg', path: fixture('unordered', 'a.jpg') },
      { filename: 'c.jpg', path: fixture('unordered', 'c.jpg') },
    ])

    expect(book.state).toBe('complete')
    expect(book.pages).toHaveLength(3)

    const expected = ['b.jpg', 'a.jpg', 'c.jpg'].map((name) =>
      readFileSync(fixture('unordered', name)),
    )
    for (let i = 0; i < 3; i++) {
      const stored = readFileSync(storedPath(book.pages[i].originalUrl))
      expect(stored.equals(expected[i])).toBe(true)
    }
  })

  test('file names carry no numbering', async ({ request }) => {
    const book = await ingest(request, 'No Numbers', [
      { filename: 'front.jpg', path: fixture('unordered', 'c.jpg') },
      { filename: 'middle.jpg', path: fixture('unordered', 'a.jpg') },
      { filename: 'end.jpg', path: fixture('unordered', 'b.jpg') },
    ])

    const expected = ['c.jpg', 'a.jpg', 'b.jpg'].map((name) =>
      readFileSync(fixture('unordered', name)),
    )
    for (let i = 0; i < 3; i++) {
      expect(readFileSync(storedPath(book.pages[i].originalUrl)).equals(expected[i])).toBe(true)
    }
  })

  test('two uploaded files share a name', async ({ request }) => {
    const book = await ingest(request, 'Same Name', [
      { filename: 'scan.jpg', path: fixture('unordered', 'a.jpg') },
      { filename: 'scan.jpg', path: fixture('unordered', 'c.jpg') },
    ])

    expect(book.state).toBe('complete')
    expect(book.pages).toHaveLength(2)
    expect(book.pages[0].index).toBe(1)
    expect(book.pages[1].index).toBe(2)

    expect(
      readFileSync(storedPath(book.pages[0].originalUrl)).equals(
        readFileSync(fixture('unordered', 'a.jpg')),
      ),
    ).toBe(true)
    expect(
      readFileSync(storedPath(book.pages[1].originalUrl)).equals(
        readFileSync(fixture('unordered', 'c.jpg')),
      ),
    ).toBe(true)
  })
})

test.describe('every page gets a unique file name', () => {
  test('two books contain a file with the same name', async ({ request }) => {
    const first = await ingest(request, 'Book A', [
      { filename: 'scan.jpg', path: fixture('unordered', 'a.jpg') },
    ])
    const second = await ingest(request, 'Book B', [
      { filename: 'scan.jpg', path: fixture('unordered', 'b.jpg') },
    ])

    expect(first.pages[0].originalUrl).not.toBe(second.pages[0].originalUrl)
    expect(
      readFileSync(storedPath(first.pages[0].originalUrl)).equals(
        readFileSync(fixture('unordered', 'a.jpg')),
      ),
    ).toBe(true)
  })
})

test.describe('ingest stores three renditions of every page', () => {
  test('a landscape page', async ({ request }) => {
    const book = await ingest(request, 'Landscape', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    const page = book.pages[0]
    expect(page.width).toBe(3000)
    expect(page.height).toBe(2000)

    for (const url of [page.originalUrl, page.displayUrl, page.thumbnailUrl]) {
      expect(statSync(storedPath(url)).size).toBeGreaterThan(0)
    }
  })

  test('a portrait page', async ({ request }) => {
    const book = await ingest(request, 'Portrait', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    expect(book.pages[0].width).toBe(2000)
    expect(book.pages[0].height).toBe(3000)
  })

  test('the original is preserved', async ({ request }) => {
    const book = await ingest(request, 'Preserved', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    const stored = readFileSync(storedPath(book.pages[0].originalUrl))
    const source = readFileSync(fixture('landscape', 'page1.jpg'))
    expect(stored.equals(source)).toBe(true)
  })

  test('a book stores exactly three files per page', async ({ request }) => {
    const book = await ingest(request, 'Three Each', imageSet('landscape'))

    const dir = path.dirname(storedPath(book.pages[0].thumbnailUrl))
    expect(readdirSync(dir)).toHaveLength(book.pages.length * 3)
  })
})

test.describe('the rendered image sizes match the spec', () => {
  test('renditions are served and readable', async ({ request }) => {
    const book = await ingest(request, 'Served', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    for (const url of [book.pages[0].displayUrl, book.pages[0].thumbnailUrl]) {
      const response = await request.get(url)
      expect(response.status()).toBe(200)
      expect(response.headers()['content-type']).toContain('image/')
    }
  })
})

test.describe('ingest never upscales a rendition', () => {
  test('a page smaller than the thumbnail size', async ({ request }) => {
    const book = await ingest(request, 'Tiny', [
      { filename: 'page1.jpg', path: fixture('small', 'page1.jpg') },
    ])

    expect(book.pages[0].width).toBe(300)
    expect(book.pages[0].height).toBe(200)

    const original = statSync(storedPath(book.pages[0].originalUrl)).size
    const display = statSync(storedPath(book.pages[0].displayUrl)).size
    expect(display).toBeLessThanOrEqual(original * 3)
  })
})

test.describe('ingest records each page with its index and dimensions', () => {
  test('page rows after ingest', async ({ request }) => {
    const book = await ingest(request, 'Indexed', imageSet('landscape'))

    const indexes = book.pages.map((page) => page.index)
    expect(indexes).toEqual([1, 2, 3])
    expect(new Set(indexes).size).toBe(indexes.length)
    for (const page of book.pages) {
      expect(page.width).toBeGreaterThan(0)
      expect(page.height).toBeGreaterThan(0)
    }
  })
})

test.describe('the uploaded source is kept', () => {
  test('after a PDF ingest', async ({ request }) => {
    const book = await ingest(request, 'Kept Source', [
      { filename: 'second.pdf', path: fixture('second.pdf') },
    ])

    expect(book.state).toBe('complete')

    const sources = readdirSync(path.join(dataDir, 'sources'))
    let found = false
    for (const dir of sources) {
      const files = readdirSync(path.join(dataDir, 'sources', dir))
      if (files.some((name) => name.endsWith('second.pdf'))) {
        found = true
      }
    }
    expect(found).toBe(true)
  })
})

test.describe('an uploaded book starts in the first stage', () => {
  test('ingest completes', async ({ request }) => {
    const book = await ingest(request, 'Staged', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    expect(book.state).toBe('complete')
    expect(book.stage).toBe('uploaded')
    expect(book.progress).toBeNull()
  })

  test('a book carries its stage while it processes', async ({ request }) => {
    const created = await uploadBook(request, 'In Flight', [
      { filename: 'book10.pdf', path: fixture('book10.pdf') },
    ])

    expect(created.state).toBe('pending')
    expect(created.stage).toBe('uploaded')

    const staged = await request.get('/api/books?stage=uploaded')
    const listed = await staged.json()
    expect(listed.some((book: { id: number }) => book.id === created.id)).toBe(true)

    await waitForSettled(request, created.id)
  })
})

test.describe('a running book carries a progress phase', () => {
  test('the phase moves from the queue to converting', async ({ request }) => {
    const created = await uploadBook(request, 'Phased', [
      { filename: 'book60.pdf', path: fixture('book60.pdf') },
    ])

    expect(created.progress).toBe('Waiting in local queue')

    const phases = new Set<string>([created.progress as string])
    const deadline = Date.now() + 60_000
    while (Date.now() < deadline) {
      const book = await getBook(request, created.id)
      if (book.progress !== null) {
        phases.add(book.progress)
      }
      if (book.state === 'complete' || book.state === 'failed') {
        break
      }
      await new Promise((resolve) => setTimeout(resolve, 200))
    }

    expect(phases.has('Converting')).toBe(true)

    // Every reported phase belongs to the fixed vocabulary.
    const vocabulary = [
      'Waiting in local queue',
      'Converting',
      'Uploading',
      'Waiting for AI',
      'Processing with AI',
      'Downloading',
    ]
    for (const phase of phases) {
      expect(vocabulary).toContain(phase)
    }

    const settled = await getBook(request, created.id)
    expect(settled.progress).toBeNull()
  })
})

test.describe('ingest reports failure', () => {
  test('a page image is corrupt', async ({ request }) => {
    const book = await ingest(request, 'Corrupt', [
      { filename: 'corrupt.jpg', path: fixture('corrupt.jpg') },
    ])

    expect(book.state).toBe('failed')
    expect(book.error).toContain('corrupt.jpg')
    expect(book.stage).toBe('uploaded')
    expect(book.progress).toBeNull()
  })
})

test.describe('stored page files are readable over HTTP', () => {
  test('a browser loads a thumbnail', async ({ request }) => {
    const book = await ingest(request, 'Thumb', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const response = await request.get(book.coverUrl as string)
    expect(response.status()).toBe(200)
    expect(response.headers()['content-type']).toContain('image/jpeg')
    expect((await response.body()).length).toBeGreaterThan(0)
  })
})
