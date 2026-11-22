export type BookState = 'pending' | 'processing' | 'complete' | 'failed'

export interface Action {
  id: string
  title: string
}

// The models the OCR action reads with. Normal runs PP-OCRv6 alone. Extra adds
// PaddleOCR-VL, which reads pages that v6 cannot, such as polytonic Greek.
export type OcrProfile = 'v6' | 'vl'

// The profiles a rescan reads with. Ultra is one of them and the OCR action of a
// whole book offers it not, so it is no OcrProfile.
export type RescanProfile = OcrProfile | 'glm'

// The arguments an action takes. Only OCR takes any.
export interface ActionArguments {
  ocrProfile?: OcrProfile
}

export interface Stage {
  id: string
  title: string
  books: number
  actions: Action[]
}

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

export interface ActionFailure {
  bookId: number
  error: string
}

export interface ActionResult {
  started: Book[]
  failed: ActionFailure[]
}

export interface Rejection {
  books: Book[]
}

export type CutSide = 'left' | 'right'

export interface Page {
  index: number
  width: number
  height: number
  originalUrl: string
  displayUrl: string
  thumbnailUrl: string
  sourcePageId: number | null
  rolledBack: boolean
  cutSide: CutSide | null
}

export interface GridCell {
  width: number
  height: number
}

export interface BookDetail extends Book {
  gridCell: GridCell
  pageRollback: boolean
  pageCut: boolean
  pageRescan: boolean
  pages: Page[]
}

export interface PageDeletion {
  books: Book[]
  allowed: boolean
  reason: string
}

export interface ComparedPage {
  stage: string
  stageTitle: string
  displayUrl: string
  width: number
  height: number
}

export interface PageRollback {
  source: ComparedPage
  current: ComparedPage
}

export interface CutSource {
  displayUrl: string
  width: number
  height: number
}

// column counts in the columns of the stored image of the source page, which is
// what width measures. The left page holds every column before it; the right
// page holds it and every column after.
export interface PageCut {
  source: CutSource
  side: CutSide
  column: number
}

// A box is left, top, right and bottom, in the pixels of the stored page image.
// Those are the width and the height the page reports, so one scale places
// every box on the rendered image.
export type OcrBox = [number, number, number, number]

export interface OcrLine {
  box: OcrBox
  text: string
  // A line of a merged reading whose text the merge took from the line engine,
  // because the reading engine and the line engine did not read it alike. Absent
  // for every line of an engine result.
  fromLineReading?: boolean
}

// The strip of a split page that holds the ink of the page beside it. The side
// names the edge it lies against and the column how far it reaches from that
// edge, in the pixels the boxes count in. A page that carries none reports null.
export interface OcrTrim {
  side: 'left' | 'right'
  column: number
}

// The reading the server draws from: `merged` where a page holds one, and the
// engine that reports boxes where it does not. The server has already dropped
// the lines that lie inside the trim, so a renderer covers the strip and counts
// nothing.
export interface PageOcr {
  model: string
  width: number
  height: number
  trim: OcrTrim | null
  lines: OcrLine[]
  readings: OcrReading[]
  // Only the whole book answer reports these. A page asked for on its own always
  // answers false and explains nothing.
  disagrees: boolean
  // Why it disagrees, in words and with the numbers that decided it.
  disagreement?: string[]
}

// One reading a page can answer. The profile is what a reader sees — "Normal",
// "Extra", "Ultra" — and the model is what a request names. A reading a renderer
// cannot draw is never reported.
export interface OcrReading {
  model: string
  profile: string
  engine: string
  answers: boolean
}

export interface BookOcrPage extends PageOcr {
  index: number
}

export interface BookOcr {
  pages: BookOcrPage[]
  // How many pages of the book hold a reading that disagrees with their line
  // reading. Zero for a book whose pages hold one reading each.
  disagreeing: number
}

export interface Health {
  status: string
  service: string
}

function messageOf(response: Response, body: string): string {
  let message = `${response.status} ${response.statusText}`
  try {
    const parsed = JSON.parse(body) as { error?: string }
    if (parsed.error) {
      message = parsed.error
    }
  } catch {
    if (body) {
      message = body
    }
  }
  return message
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init)
  const body = await response.text()

  if (!response.ok) {
    throw new Error(messageOf(response, body))
  }

  return JSON.parse(body) as T
}

// requestOrNull answers null for what the server does not hold. A page read by
// no engine is a state the reader shows, not a failure it reports.
async function requestOrNull<T>(path: string): Promise<T | null> {
  const response = await fetch(path)
  const body = await response.text()

  if (response.status === 404) {
    return null
  }
  if (!response.ok) {
    throw new Error(messageOf(response, body))
  }

  return JSON.parse(body) as T
}

export interface UploadProgress {
  (loaded: number, total: number): void
}

// fetch cannot report upload progress, and a book is large enough that the user
// needs to see it move. XMLHttpRequest is the only API that reports it.
function upload(name: string, files: File[], onProgress?: UploadProgress): Promise<Book> {
  const form = new FormData()
  form.append('name', name)
  for (const file of files) {
    form.append('files', file, file.name)
  }

  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest()
    request.open('POST', '/api/books')

    request.upload.onprogress = (event) => {
      if (onProgress && event.lengthComputable) {
        onProgress(event.loaded, event.total)
      }
    }

    request.onload = () => {
      if (request.status >= 200 && request.status < 300) {
        resolve(JSON.parse(request.responseText) as Book)
        return
      }
      let message = `${request.status} ${request.statusText}`
      try {
        const parsed = JSON.parse(request.responseText) as { error?: string }
        if (parsed.error) {
          message = parsed.error
        }
      } catch {
        if (request.responseText) {
          message = request.responseText
        }
      }
      reject(new Error(message))
    }

    request.onerror = () => reject(new Error('the upload failed'))
    request.send(form)
  })
}

export const api = {
  health: () => request<Health>('/api/health'),

  listStages: () => request<Stage[]>('/api/stages'),

  listByStage: (stage: string) => request<Book[]>(`/api/books?stage=${encodeURIComponent(stage)}`),

  getBook: (id: number) => request<BookDetail>(`/api/books/${id}`),

  // The ids are source books. The answer holds the copy of every book that
  // started, and a reason for every book that did not.
  runAction: (action: string, bookIds: number[], args: ActionArguments = {}) =>
    request<ActionResult>(`/api/books/actions/${encodeURIComponent(action)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bookIds, ...args }),
    }),

  // What a reject would destroy: the book and every copy made from it.
  rejection: (id: number) => request<Rejection>(`/api/books/${id}/rejection`),

  rejectBook: (id: number) =>
    request<{ destroyed: number[] }>(`/api/books/${id}/reject`, { method: 'POST' }),

  // What a page delete would remove: this book and every book derived from it.
  // It also reports whether the chain allows the delete right now.
  pageDeletion: (id: number, index: number) =>
    request<PageDeletion>(`/api/books/${id}/pages/${index}/deletion`),

  deletePage: (id: number, index: number) =>
    request<{ books: number[] }>(`/api/books/${id}/pages/${index}/delete`, { method: 'POST' }),

  // The two pages a rollback compares: the page it would install and the page
  // it would replace.
  pageRollback: (id: number, index: number) =>
    request<PageRollback>(`/api/books/${id}/pages/${index}/rollback`),

  applyPageRollback: (id: number, index: number) =>
    request<Page>(`/api/books/${id}/pages/${index}/rollback`, { method: 'POST' }),

  // The page a re-cut cuts out of, and the cut this page holds now.
  pageCut: (id: number, index: number) =>
    request<PageCut>(`/api/books/${id}/pages/${index}/cut`),

  applyPageCut: (id: number, index: number, column: number) =>
    request<Page>(`/api/books/${id}/pages/${index}/cut`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ column }),
    }),

  // Reads the named pages again with the engines of the profile. The answer is
  // the book it set running.
  rescanPages: (id: number, pageIndexes: number[], profile: RescanProfile) =>
    request<Book>(`/api/books/${id}/rescan`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pageIndexes, profile }),
    }),

  // The lines a renderer draws over the page. It answers null for a page no
  // engine has read yet.
  pageOcr: (id: number, index: number) =>
    requestOrNull<PageOcr>(`/api/books/${id}/pages/${index}/ocr`),

  // The lines of every page of a book. The reader draws the whole book, so it
  // asks once. A page no engine has read yet carries no lines.
  bookOcr: (id: number) => request<BookOcr>(`/api/books/${id}/ocr`),

  // Makes a page answer one of the readings it holds. The answer is that page,
  // read from the reading it now answers, so the reader draws it again.
  setPageReading: (id: number, index: number, model: string) =>
    request<PageOcr>(`/api/books/${id}/pages/${index}/reading`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ model }),
    }),

  uploadBook: upload,
}
