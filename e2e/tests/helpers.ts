import { readFileSync, readdirSync, rmSync } from 'node:fs'
import path from 'node:path'
import { Client } from 'pg'
import type { APIRequestContext } from '@playwright/test'

export const apiURL = process.env.E2E_API_URL ?? 'http://backend:8080'
export const dataDir = process.env.DATA_DIR ?? '/data'
export const fixtures = path.resolve('/e2e/fixtures')

export function fixture(...parts: string[]): string {
  return path.join(fixtures, ...parts)
}

export const fakeRunpodURL = process.env.FAKE_RUNPOD_URL ?? 'http://fakerunpod:9090'

export type BookState = 'pending' | 'processing' | 'complete' | 'failed'

export interface Book {
  id: number
  name: string
  state: BookState
  stage: string
  pageCount: number
  error: string | null
  coverUrl: string | null
  createdAt: string
  derivedState: BookState | null
  progress: string | null
  action: string | null
  ocrProfile: OcrProfile | null
  sourceBookId: number | null
}

export interface ActionResult {
  started: Book[]
  failed: Array<{ bookId: number; error: string }>
}

export interface Page {
  index: number
  width: number
  height: number
  originalUrl: string
  displayUrl: string
  thumbnailUrl: string
  sourcePageId: number | null
  rolledBack: boolean
  cutSide: 'left' | 'right' | null
}

export interface BookDetail extends Book {
  gridCell: { width: number; height: number }
  pageCut: boolean
  pages: Page[]
}

// Playwright's multipart option is a record, so it cannot repeat the "files"
// field. Several tests turn on the exact order of repeated parts, so the body
// is built by hand.
export function multipart(
  name: string | null,
  files: Array<{ field?: string; filename: string; path?: string; body?: Buffer; type?: string }>,
): { body: Buffer; contentType: string } {
  const boundary = '----ownocrE2E' + Math.random().toString(16).slice(2)
  const chunks: Buffer[] = []

  if (name !== null) {
    chunks.push(
      Buffer.from(
        `--${boundary}\r\nContent-Disposition: form-data; name="name"\r\n\r\n${name}\r\n`,
      ),
    )
  }

  for (const file of files) {
    const data = file.body ?? readFileSync(file.path as string)
    chunks.push(
      Buffer.from(
        `--${boundary}\r\n` +
          `Content-Disposition: form-data; name="${file.field ?? 'files'}"; filename="${file.filename}"\r\n` +
          `Content-Type: ${file.type ?? 'application/octet-stream'}\r\n\r\n`,
      ),
    )
    chunks.push(data)
    chunks.push(Buffer.from('\r\n'))
  }

  chunks.push(Buffer.from(`--${boundary}--\r\n`))

  return {
    body: Buffer.concat(chunks),
    contentType: `multipart/form-data; boundary=${boundary}`,
  }
}

export async function postBook(
  request: APIRequestContext,
  name: string | null,
  files: Array<{ filename: string; path?: string; body?: Buffer; field?: string }>,
) {
  const { body, contentType } = multipart(name, files)
  return request.post(`${apiURL}/api/books`, {
    headers: { 'Content-Type': contentType },
    data: body,
    timeout: 120_000,
  })
}

export async function uploadBook(
  request: APIRequestContext,
  name: string,
  files: Array<{ filename: string; path?: string; body?: Buffer }>,
): Promise<Book> {
  const response = await postBook(request, name, files)
  if (response.status() !== 201) {
    throw new Error(`upload failed: ${response.status()} ${await response.text()}`)
  }
  return (await response.json()) as Book
}

export async function getBook(request: APIRequestContext, id: number): Promise<BookDetail> {
  const response = await request.get(`${apiURL}/api/books/${id}`)
  if (!response.ok()) {
    throw new Error(`get book ${id}: ${response.status()}`)
  }
  return (await response.json()) as BookDetail
}

export async function waitForSettled(
  request: APIRequestContext,
  id: number,
  timeoutMs = 120_000,
): Promise<BookDetail> {
  const deadline = Date.now() + timeoutMs

  while (Date.now() < deadline) {
    const book = await getBook(request, id)
    if (book.state === 'complete' || book.state === 'failed') {
      return book
    }
    await new Promise((resolve) => setTimeout(resolve, 400))
  }

  throw new Error(`book ${id} never settled`)
}

export async function ingest(
  request: APIRequestContext,
  name: string,
  files: Array<{ filename: string; path?: string; body?: Buffer }>,
): Promise<BookDetail> {
  const book = await uploadBook(request, name, files)
  return waitForSettled(request, book.id)
}

export function imageSet(dir: string): Array<{ filename: string; path: string }> {
  return readdirSync(fixture(dir))
    .sort()
    .map((filename) => ({ filename, path: fixture(dir, filename) }))
}

export function storedPath(url: string): string {
  return path.join(dataDir, 'pages', url.replace(/^\/files\//, ''))
}

export async function reset() {
  const client = new Client({ connectionString: process.env.DATABASE_URL })
  await client.connect()
  try {
    await client.query('TRUNCATE book CASCADE')
    await client.query('TRUNCATE river_job')
  } finally {
    await client.end()
  }

  for (const dir of ['pages', 'sources', 'tmp']) {
    const root = path.join(dataDir, dir)
    for (const entry of readdirSync(root)) {
      rmSync(path.join(root, entry), { recursive: true, force: true })
    }
  }
}

const parsedBase = new URL(process.env.E2E_BASE_URL ?? 'http://frontend:5173')
export const baseHost = parsedBase.hostname
export const basePort = Number(parsedBase.port || (parsedBase.protocol === 'https:' ? 443 : 80))

export type OcrProfile = 'v6' | 'vl'

export interface ActionArguments {
  ocrProfile?: OcrProfile
}

export async function runAction(
  request: APIRequestContext,
  action: string,
  bookIds: number[],
  args: ActionArguments = {},
): Promise<ActionResult> {
  const response = await request.post(`${apiURL}/api/books/actions/${action}`, {
    data: { bookIds, ...args },
  })
  if (!response.ok()) {
    throw new Error(`${action} failed: ${response.status()} ${await response.text()}`)
  }
  return (await response.json()) as ActionResult
}

// start runs an action on one book and answers with the copy it made. It
// refuses a rejected book, so a test never carries on with nothing.
export async function start(
  request: APIRequestContext,
  action: string,
  sourceId: number,
  args: ActionArguments = {},
): Promise<Book> {
  const result = await runAction(request, action, [sourceId], args)
  if (result.started.length !== 1) {
    throw new Error(`${action} on ${sourceId} did not start: ${JSON.stringify(result.failed)}`)
  }
  return result.started[0]
}

export const startDewarp = (request: APIRequestContext, sourceId: number) =>
  start(request, 'dewarp', sourceId)

export const startSplit = (request: APIRequestContext, sourceId: number) =>
  start(request, 'split', sourceId)

// startOCR reads a book. Extra is the default because it runs both engines,
// which is the case that stores two results a page.
export const startOCR = (request: APIRequestContext, sourceId: number, profile: OcrProfile = 'vl') =>
  start(request, 'ocr', sourceId, { ocrProfile: profile })

export interface PageResult {
  model: string
  txt: string
  data: unknown
}

// A box is left, top, right, bottom, in the pixels of the stored page image.
export type Box = [number, number, number, number]

export interface V6Result {
  lines: Array<{ bbox: Box; text: string; score: number }>
}

export interface MergedResult {
  lines: Array<{ bbox: Box; text: string; source: string }>
}

// pageResults answers what every engine stored for a page, by engine. The text
// of a page lives in the database now, not beside its renditions.
export async function pageResults(pageIndex: number, bookId: number): Promise<Record<string, PageResult>> {
  const client = new Client({ connectionString: process.env.DATABASE_URL })
  await client.connect()
  try {
    const { rows } = await client.query(
      `SELECT r.model, r.txt, r.data FROM page_ocr r
       JOIN book_page p ON p.id = r.page_id
       WHERE p.book_id = $1 AND p.page_index = $2 ORDER BY r.model`,
      [bookId, pageIndex],
    )
    const byModel: Record<string, PageResult> = {}
    for (const row of rows) {
      byModel[row.model] = row as PageResult
    }
    return byModel
  } finally {
    await client.end()
  }
}

// pageLines answers the lines a reader draws over one page, and the model they
// come from.
export async function pageLines(
  request: APIRequestContext,
  bookId: number,
  pageIndex: number,
): Promise<{ model: string; lines: Array<{ box: Box; text: string }> }> {
  const response = await request.get(
    `${apiURL}/api/books/${bookId}/pages/${pageIndex}/ocr`,
  )
  if (!response.ok()) {
    throw new Error(`page lines failed: ${response.status()} ${await response.text()}`)
  }
  return (await response.json()) as { model: string; lines: Array<{ box: Box; text: string }> }
}

export async function rejectBook(request: APIRequestContext, id: number): Promise<number[]> {
  const response = await request.post(`${apiURL}/api/books/${id}/reject`)
  if (!response.ok()) {
    throw new Error(`reject failed: ${response.status()} ${await response.text()}`)
  }
  return ((await response.json()) as { destroyed: number[] }).destroyed
}

export async function listBooks(request: APIRequestContext): Promise<Book[]> {
  const response = await request.get(`${apiURL}/api/books`)
  if (!response.ok()) {
    throw new Error(`list books: ${response.status()}`)
  }
  return (await response.json()) as Book[]
}

// The fake endpoint stands in for RunPod. It also reads and clears the bucket,
// so a test needs no S3 client of its own.
export async function failDewarp(
  request: APIRequestContext,
  mode: 'job' | 'page' | 'off' = 'job',
): Promise<void> {
  const response = await request.post(`${fakeRunpodURL}/control/fail?mode=${mode}`)
  if (!response.ok()) {
    throw new Error(`arm failure: ${response.status()}`)
  }
}

export async function objectKeys(
  request: APIRequestContext,
  prefix = 'dewarp/',
): Promise<string[]> {
  const response = await request.get(
    `${fakeRunpodURL}/control/objects?prefix=${encodeURIComponent(prefix)}`,
  )
  if (!response.ok()) {
    throw new Error(`list objects: ${response.status()}`)
  }
  return ((await response.json()) as { keys: string[] }).keys
}

export async function resetDewarp(request: APIRequestContext): Promise<void> {
  await request.post(`${fakeRunpodURL}/control/reset`)
  await request.delete(`${fakeRunpodURL}/control/objects?prefix=dewarp/`)
  await request.delete(`${fakeRunpodURL}/control/objects?prefix=ocr/`)
}

// rescanPages reads the named pages of a book again with the engines of the
// profile. The answer is the book it set running.
export async function rescanPages(
  request: APIRequestContext,
  bookId: number,
  pageIndexes: number[],
  profile = 'vl',
): Promise<Book> {
  const response = await request.post(`${apiURL}/api/books/${bookId}/rescan`, {
    data: { pageIndexes, profile },
  })
  if (!response.ok()) {
    throw new Error(`rescan failed: ${response.status()} ${await response.text()}`)
  }
  return (await response.json()) as Book
}

// rescanRefusal answers the status and the reason of a refused rescan. A null
// profile is left out of the body, so a test can send a request naming none.
export async function rescanRefusal(
  request: APIRequestContext,
  bookId: number,
  pageIndexes: number[],
  profile: string | null = 'vl',
): Promise<{ status: number; error: string }> {
  const data: Record<string, unknown> = { pageIndexes }
  if (profile !== null) {
    data.profile = profile
  }

  const response = await request.post(`${apiURL}/api/books/${bookId}/rescan`, { data })
  const body = (await response.json()) as { error?: string }
  return { status: response.status(), error: body.error ?? '' }
}
