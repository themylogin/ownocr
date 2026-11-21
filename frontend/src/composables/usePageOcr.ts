import { ref } from 'vue'
import { api, type PageOcr } from '../api/client'

// The lines of every page of one book, by page index. The reader draws the whole
// book so that find-in-page reaches it, and one request carries the lot. A page
// the server holds no text for records nothing, and shows its image alone.
//
// The same answer carries, per page, the readings that page can answer and
// whether the one it answers disagrees with its line reading. The count of
// disagreeing pages belongs to the book, so it comes from the same request.
export function usePageOcr(bookId: () => number) {
  const byIndex = ref(new Map<number, PageOcr>())
  const disagreeing = ref(0)

  async function ask() {
    let answer
    try {
      answer = await api.bookOcr(bookId())
    } catch {
      // The reader draws what it has. A book whose text does not arrive keeps
      // every page as an image, and raises nothing to the reader.
      return
    }

    const pages = new Map<number, PageOcr>()
    for (const page of answer.pages) {
      if (page.lines.length) {
        pages.set(page.index, page)
      }
    }
    byIndex.value = pages
    disagreeing.value = answer.disagreeing
  }

  function linesOf(index: number): PageOcr | undefined {
    return byIndex.value.get(index)
  }

  // replace draws a page from another of its readings. The answer of the choice
  // is that page, so nothing is asked for again.
  //
  // It keeps the doubt the book answer reported, flag and words together. Only the
  // whole book answer judges a reading, so the answer of a choice carries neither,
  // and taking them from it would drop the doubt of a page the moment a reader
  // looked at another of its readings.
  function replace(index: number, reading: PageOcr) {
    const held = byIndex.value.get(index)
    const pages = new Map(byIndex.value)
    pages.set(index, {
      ...reading,
      disagrees: held?.disagrees ?? false,
      disagreement: held?.disagreement,
    })
    byIndex.value = pages
  }

  return { ask, linesOf, replace, disagreeing }
}
