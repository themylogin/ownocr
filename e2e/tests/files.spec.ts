import { readFileSync, readdirSync } from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import { expect, test } from '@playwright/test'
import { baseHost, basePort, dataDir, fixture, ingest, reset } from './helpers'

// Sends the request target byte for byte, with no URL normalization.
function rawGet(target: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const socket = net.connect(basePort, baseHost, () => {
      socket.write(`GET ${target} HTTP/1.1\r\nHost: ${baseHost}:${basePort}\r\nConnection: close\r\n\r\n`)
    })

    const chunks: Buffer[] = []
    socket.on('data', (chunk) => chunks.push(chunk))
    socket.on('end', () => resolve(Buffer.concat(chunks).toString('latin1')))
    socket.on('error', reject)
    socket.setTimeout(15_000, () => {
      socket.destroy()
      reject(new Error('raw request timed out'))
    })
  })
}

test.beforeEach(async () => {
  await reset()
})

test.describe('stored page files are readable over HTTP', () => {
  test('a browser loads a thumbnail', async ({ request }) => {
    const book = await ingest(request, 'Served', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const response = await request.get(book.pages[0].thumbnailUrl)

    expect(response.status()).toBe(200)
    expect(response.headers()['content-type']).toContain('image/jpeg')
  })

  test('all three renditions are served', async ({ request }) => {
    const book = await ingest(request, 'All Three', [
      { filename: 'page1.jpg', path: fixture('landscape', 'page1.jpg') },
    ])

    const page = book.pages[0]
    for (const url of [page.originalUrl, page.displayUrl, page.thumbnailUrl]) {
      const response = await request.get(url)
      expect(response.status(), url).toBe(200)
      expect((await response.body()).length).toBeGreaterThan(0)
    }
  })

  test('a request for an unknown file', async ({ request }) => {
    const response = await request.get('/files/nosuchbook/nosuchpage.480.jpg')

    expect(response.status()).toBe(404)
  })

  test('a directory is not listed', async ({ request }) => {
    const book = await ingest(request, 'No Listing', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const dir = book.pages[0].thumbnailUrl.replace(/[^/]+$/, '')
    const response = await request.get(dir)

    expect(response.status()).toBe(404)
  })

  // Every standards-compliant HTTP client resolves "../" and even "%2e%2e"
  // before the request leaves it, so the server never sees a traversal from
  // one. A raw socket sends the path untouched, which is the only way to test
  // what the server itself does with it.
  //
  // Both servers resolve dot segments before they choose a route, encoded or
  // not, so a traversal lands on an unknown route instead of on the file store.
  // What each then answers differs by design: nginx serves the app shell, the
  // dev server answers 404. Neither is a leak, so the assertion is on the bytes
  // rather than on the status.
  test('a raw traversal never serves a stored file', async ({ request }) => {
    await ingest(request, 'Guarded Raw', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const sourceDir = readdirSync(path.join(dataDir, 'sources'))[0]
    const sourceFile = readdirSync(path.join(dataDir, 'sources', sourceDir))[0]
    const real = readFileSync(path.join(dataDir, 'sources', sourceDir, sourceFile))

    for (const target of [
      `/files/../sources/${sourceDir}/${sourceFile}`,
      `/files/../../sources/${sourceDir}/${sourceFile}`,
      `/files/%2e%2e/sources/${sourceDir}/${sourceFile}`,
      `/files/..%2fsources/${sourceDir}/${sourceFile}`,
      '/files/../../../etc/passwd',
      '/files/%2e%2e/%2e%2e/etc/passwd',
    ]) {
      const raw = await rawGet(target)

      expect(raw, target).not.toContain('root:x:')
      expect(raw, target).not.toContain(real.toString('latin1').slice(0, 64))
      expect(raw.toLowerCase(), target).not.toContain('content-type: image/')
    }
  })

  // A plain "../" never reaches /files: the HTTP client resolves it away first.
  // What must hold is that no source byte is reachable either way.
  test('no source file is reachable through the served root', async ({ request }) => {
    await ingest(request, 'Guarded Bytes', [
      { filename: 'page1.jpg', path: fixture('portrait', 'page1.jpg') },
    ])

    const sources = readdirSync(path.join(dataDir, 'sources'))
    const sourceDir = sources[0]
    const sourceFile = readdirSync(path.join(dataDir, 'sources', sourceDir))[0]
    const real = readFileSync(path.join(dataDir, 'sources', sourceDir, sourceFile))

    const attempts = [
      `/files/../sources/${sourceDir}/${sourceFile}`,
      `/files/../../sources/${sourceDir}/${sourceFile}`,
      `/sources/${sourceDir}/${sourceFile}`,
      '/files/../../etc/passwd',
    ]

    for (const url of attempts) {
      const response = await request.get(url, { maxRedirects: 0 })
      const body = await response.body()

      expect(body.equals(real), `${url} served the source file`).toBe(false)
      expect(response.headers()['content-type'] ?? '', url).not.toContain('image/')
      expect(body.toString('utf8'), url).not.toContain('root:x:')
    }
  })
})
