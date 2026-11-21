<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NAlert, NButton, useThemeVars } from 'naive-ui'
import { api, type BookDetail, type PageOcr, type Page, type RescanProfile } from '../api/client'
import { useStages } from '../composables/useStages'
import { useActions } from '../composables/useActions'
import { useLayout } from '../composables/useLayout'
import { usePageOcr } from '../composables/usePageOcr'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import BookActions from '../components/BookActions.vue'
import OcrPage from '../components/OcrPage.vue'
import PageMenu from '../components/PageMenu.vue'
import PageMinimap from '../components/PageMinimap.vue'
import RescanDialog from '../components/RescanDialog.vue'

const route = useRoute()
const router = useRouter()
const themeVars = useThemeVars()
const { bookStage, ensureStages, titleOf } = useStages()
const { error: actionError } = useActions()
const { wide } = useLayout()

const book = ref<BookDetail | null>(null)
const loadError = ref('')

const bookId = computed(() => Number(route.params.id))
const startIndex = computed(() => Number(route.params.index ?? 1))
const alert = computed(() => loadError.value || actionError.value)

// A book that has been read shows its text beside every page.
const reading = computed(() => book.value?.stage === 'ocr')

const { ask, linesOf, replace, disagreeing } = usePageOcr(() => bookId.value)

// How many pages of this book hold a reading that parts company with their line
// reading. A count of zero is not news, so the bar says nothing about it.
const disagreement = computed(() => {
  if (!reading.value || !disagreeing.value) {
    return ''
  }
  return disagreeing.value === 1 ? '1 page disagrees' : `${disagreeing.value} pages disagree`
})

function disagrees(index: number) {
  return linesOf(index)?.disagrees ?? false
}

function readingsOf(index: number) {
  return linesOf(index)?.readings
}

function disagreementOf(index: number) {
  return linesOf(index)?.disagreement
}

// A page that holds an Ultra reading has been read by the model of last resort, so
// a reader can choose that reading without paying for it again.
function holdsUltra(index: number) {
  return (linesOf(index)?.readings ?? []).some(
    (reading) => reading.model === 'claude_merged',
  )
}

// The borders of the read half of a page, innermost first: the choice of the user,
// then the doubt of the system, then the reading it holds in reserve. Each says a
// different thing, so a page with more than one reason shows a ring for each and
// none of them hides another.
//
// They are computed rather than written out as one CSS rule per combination: three
// reasons make seven of those, and a fourth would make fifteen.
const ringWidth = 3

function ringsOf(index: number) {
  const colours = [
    marks.value.has(index) ? themeVars.value.successColor : '',
    disagrees(index) ? themeVars.value.warningColor : '',
    holdsUltra(index) ? themeVars.value.infoColor : '',
  ].filter(Boolean)

  return colours
    .map((colour, ring) => `inset 0 0 0 ${(ring + 1) * ringWidth}px ${colour}`)
    .join(', ')
}

const pageIndexes = computed(() => (book.value?.pages ?? []).map((page) => page.index))

function openPage(index: number) {
  document.querySelector(`[data-page="${index}"]`)?.scrollIntoView({ block: 'start' })
}

function onReading(index: number, chosen: PageOcr) {
  replace(index, chosen)
}

// The pages a rescan will read. A mark belongs to this reader and is never
// stored. Starting the rescan leaves the reader for the stage list, which drops
// every mark with it.
const marks = ref(new Set<number>())

const confirming = ref(false)
const starting = ref(false)
const rescanError = ref('')

const ready = computed(() => book.value?.state === 'complete')

function onRescan(index: number) {
  marks.value = new Set(marks.value).add(index)
}

function onUnrescan(index: number) {
  const left = new Set(marks.value)
  left.delete(index)
  marks.value = left
}

function markedPage(index: number) {
  return marks.value.has(index)
}

async function startRescan(profile: RescanProfile) {
  if (!book.value || starting.value) {
    return
  }

  const indexes = [...marks.value].sort((a, b) => a - b)
  starting.value = true
  rescanError.value = ''

  try {
    await api.rescanPages(book.value.id, indexes, profile)
    confirming.value = false
    leave()
  } catch (cause) {
    rescanError.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    starting.value = false
  }
}

// The row the spreads stand in, measured rather than derived from the window.
//
// A width computed from the window has to guess the padding of the content column
// and whether a scrollbar is there. On the first render it is not: the pages have
// not been laid out, so nothing scrolls yet, and a width read then is too wide by
// the scrollbar about to appear. The halves then overflow the row by that much,
// and paint containment clips whatever leaves it — which showed as a page missing
// its right border until the window was resized.
// It opens on a guess from the window, which is what the row was computed from
// before, and the observer below corrects it before the first paint.
const rowWidth = ref(Math.max(0, document.documentElement.clientWidth - 32))
const pages = ref<HTMLElement | null>(null)

// The gap between the two halves, which is the one number the row does not hold.
const gap = 16

// Both halves take the scale of the page they show, and the pair fits the row.
// The 1 keeps the rule the reader follows: never enlarge a page beyond its stored
// size.
function scaleOf(page: Page) {
  const half = (rowWidth.value - gap) / 2
  return Math.min(1, half / page.width)
}

function sizeOf(page: Page) {
  const scale = scaleOf(page)
  return {
    width: `${Math.floor(page.width * scale)}px`,
    height: `${Math.floor(page.height * scale)}px`,
  }
}

// The size a spread takes: both halves and the gap between them. A page off
// screen skips its rendering and holds this size instead, so the reader keeps
// its full height and no page moves.
function spreadSizeOf(page: Page) {
  const scale = scaleOf(page)
  const half = Math.floor(page.width * scale)
  return {
    containIntrinsicSize: `auto ${half * 2 + gap}px ${Math.floor(page.height * scale)}px`,
  }
}

// The row is watched and not read once: it changes with the window, with the
// scrollbar the pages bring, and with anything else that takes room from it.
let watcher: ResizeObserver | null = null

function watchRow() {
  if (!pages.value) {
    return
  }
  rowWidth.value = pages.value.clientWidth
  watcher = new ResizeObserver((entries) => {
    for (const entry of entries) {
      rowWidth.value = entry.contentRect.width
    }
  })
  watcher.observe(pages.value)
}

const crumbs = computed(() => {
  if (!book.value) {
    return []
  }
  return [
    { label: titleOf(book.value.stage), to: { name: 'stage', params: { stage: book.value.stage } } },
    { label: book.value.name, to: { name: 'book', params: { id: book.value.id } } },
    { label: 'View' },
  ]
})

// The book this view shows no longer exists, so there is nothing to return to
// but the stage it was in.
function leave() {
  if (book.value) {
    router.push({ name: 'stage', params: { stage: book.value.stage } })
  }
}

const menu = ref<InstanceType<typeof PageMenu> | null>(null)

function onDeleted(index: number) {
  if (book.value) {
    book.value.pages = book.value.pages.filter((page) => page.index !== index)
  }
}

function onReplaced(replaced: Page) {
  if (book.value) {
    book.value.pages = book.value.pages.map((page) =>
      page.index === replaced.index ? replaced : page,
    )
  }
}

onMounted(async () => {
  ensureStages()
  actionError.value = ''

  try {
    book.value = await api.getBook(bookId.value)
    bookStage.value = book.value.stage
  } catch (cause) {
    loadError.value = cause instanceof Error ? cause.message : String(cause)
    return
  }

  wide.value = reading.value

  await nextTick()
  watchRow()
  document.querySelector(`[data-page="${startIndex.value}"]`)?.scrollIntoView()

  if (reading.value) {
    ask()
  }
})

onUnmounted(() => {
  watcher?.disconnect()
  wide.value = false
})
</script>

<template>
  <n-alert v-if="alert" type="error" data-testid="reader-error" :bordered="false">{{ alert }}</n-alert>

  <template v-if="book">
    <!-- The bar follows the reader down the book, so the actions reach every
         page without a scroll back to the top. -->
    <header class="bar">
      <Breadcrumbs :crumbs="crumbs">
        <template #note>
          <li v-if="disagreement" class="disagreement" data-testid="reader-disagreement">
            {{ disagreement }}
          </li>
        </template>
      </Breadcrumbs>

      <BookActions :book="book" @rejected="leave" />

      <n-button
        v-if="marks.size"
        type="success"
        size="small"
        data-testid="rescan-marked"
        :disabled="!ready"
        @click="confirming = true"
      >
        Rescan {{ marks.size }} {{ marks.size === 1 ? 'page' : 'pages' }}
      </n-button>
    </header>

    <div v-if="reading" ref="pages" class="pages reading" data-testid="reader-pages">
      <!-- The read half stacks an image, a veil and drawn text, so a copy saved
           from it would hold the page without one glyph of the reading. The
           spread offers no menu of its own and prevents the menu of the browser;
           the page half brings the page menu back. -->
      <div
        v-for="page in book.pages"
        :key="page.index"
        class="spread"
        :style="spreadSizeOf(page)"
        :data-page="page.index"
        :data-marked="marks.has(page.index) ? 'true' : undefined"
        @contextmenu.prevent
      >
        <img
          class="plain"
          :src="page.displayUrl"
          alt=""
          :style="sizeOf(page)"
          :width="page.width"
          :height="page.height"
          loading="lazy"
          @contextmenu.prevent="menu?.open($event, page)"
        />
        <OcrPage
          :src="page.displayUrl"
          :width="page.width"
          :height="page.height"
          :scale="scaleOf(page)"
          :text="linesOf(page.index)"
          :class="{
            ringed: Boolean(ringsOf(page.index)),
            marked: marks.has(page.index),
            disagrees: disagrees(page.index),
          }"
          :style="{ '--rings': ringsOf(page.index) }"
          @contextmenu.prevent="menu?.openRescan($event, page)"
        />
      </div>
    </div>

    <div v-else class="pages" data-testid="reader-pages">
      <figure
        v-for="page in book.pages"
        :key="page.index"
        :data-page="page.index"
        @contextmenu.prevent="menu?.open($event, page)"
      >
        <img
          :src="page.displayUrl"
          :alt="`Page ${page.index}`"
          :width="page.width"
          :height="page.height"
          loading="lazy"
        />
      </figure>
    </div>

    <PageMinimap
      v-if="reading"
      :indexes="pageIndexes"
      :disagrees="disagrees"
      :marked="markedPage"
      :holds-ultra="holdsUltra"
      @open="openPage"
    />

    <PageMenu
      ref="menu"
      :book-id="book.id"
      :book-name="book.name"
      :rollback="book.pageRollback"
      :cut="book.pageCut"
      :rescan="book.pageRescan"
      :readings-of="readingsOf"
      :marked-page="markedPage"
      :disagreement-of="disagreementOf"
      @deleted="onDeleted"
      @replaced="onReplaced"
      @rescan="onRescan"
      @unrescan="onUnrescan"
      @reading="onReading"
    />

    <RescanDialog
      v-if="confirming"
      :count="marks.size"
      :busy="starting"
      :error="rescanError"
      :book-profile="book.ocrProfile"
      @close="confirming = false"
      @start="startRescan($event)"
    />
  </template>
</template>

<style scoped>
.bar {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
  position: sticky;
  top: 0;
  z-index: 1;
  background: v-bind('themeVars.bodyColor');
  padding: 0.5rem 0;
}

/* The count stands in the trail itself, after the last crumb, so the gap of the
   bar never pushes it away from them and it needs no alignment of its own: it
   shares the line of the crumbs and so their baseline. */
.disagreement {
  display: inline;
  list-style: none;
  margin-left: 0.75rem;
  color: v-bind('themeVars.warningColor');
  font-size: 0.875rem;
  white-space: nowrap;
}

.pages {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

figure {
  margin: 0;
}

/* A page fits the column and is never enlarged past its stored size. The
   intrinsic width and height reserve the right box before the image loads, so
   lazy loading does not make the page jump. */
figure img {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 0 auto;
  background: v-bind('themeVars.cardColor');
}

/* The borders of a page, drawn concentrically so that none of them hides another:
   a marked page waits for a rescan, a disagreeing page holds a reading that parts
   company with its line reading, and a page holding an Ultra reading has one in
   reserve. Every one sits on the read half alone, which is the half that holds the
   text in question.

   `ringsOf` above builds them, so one rule draws any combination of them.

   Each border is drawn inside the box of that half, above the page, and moves no
   page. It cannot be drawn outside it: `content-visibility` below applies paint
   containment, which clips to the padding box, so a border offset outwards shows
   only on the edges that happen to have room — the left one, where the gap between
   the halves leaves some, and the other three never. */
.spread .ringed::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  box-shadow: var(--rings);
}

/* The two halves of one page stand in one row, so they scroll together with no
   listener to drift. Neither half shrinks: a half narrower than the size it was
   given no longer holds the shape its boxes were placed in.

   A page off screen skips its style, its layout and its paint, and stands at the
   size `spreadSizeOf` reserves for it. The find still reaches the text inside a
   page it skips, so Ctrl+F covers the book while the reader lays out the pages
   at the window only. `content-visibility: hidden` would put the text out of
   reach, which is the whole point of drawing it. */
.spread {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  gap: 1rem;
  content-visibility: auto;
}

.spread > * {
  flex: 0 0 auto;
}

.spread .plain {
  display: block;
  object-fit: contain;
  background: v-bind('themeVars.cardColor');
}
</style>
