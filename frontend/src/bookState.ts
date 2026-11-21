import type { Book } from './api/client'

// The five groups a card can be in, in the order the list shows them.
export type BookGroup = 'failed' | 'running' | 'ready' | 'working' | 'done'

const order: BookGroup[] = ['failed', 'running', 'ready', 'working', 'done']

export function groupOf(book: Book): BookGroup {
  if (book.state === 'failed') {
    return 'failed'
  }
  if (book.state !== 'complete') {
    return 'running'
  }
  if (book.derivedState === null) {
    return 'ready'
  }
  return book.derivedState === 'complete' ? 'done' : 'working'
}

export function rankOf(book: Book): number {
  return order.indexOf(groupOf(book))
}

// The same key the server sorts by. A phase change leaves it untouched, so a
// card only moves when its state or its copy changes.
export function compareBooks(a: Book, b: Book): number {
  const byGroup = rankOf(a) - rankOf(b)
  if (byGroup !== 0) {
    return byGroup
  }
  if (a.createdAt !== b.createdAt) {
    return a.createdAt < b.createdAt ? 1 : -1
  }
  return b.id - a.id
}

export function isReady(book: Book): boolean {
  return groupOf(book) === 'ready'
}

export function canRetry(book: Book): boolean {
  return book.state === 'failed' && book.sourceBookId !== null && book.action !== null
}
