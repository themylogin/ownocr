<script setup lang="ts">
import { computed, h, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { NDropdown, useThemeVars } from 'naive-ui'
import { api, type OcrReading, type Page, type PageOcr } from '../api/client'
import PageCompare from './PageCompare.vue'
import PageCutDialog from './PageCutDialog.vue'
import PageDeleteDialog from './PageDeleteDialog.vue'

const props = defineProps<{
  bookId: number
  bookName: string
  rollback: boolean
  cut: boolean
  rescan: boolean

  // The readings a page can answer, by page index. A page of a book that has not
  // been read answers none, and its menu holds no reading group.
  readingsOf?: (index: number) => OcrReading[] | undefined

  // Whether a page is already marked for a rescan. The item says what choosing it
  // does, so it has to know which way the mark stands.
  markedPage?: (index: number) => boolean

  // Why the reading of a page disagrees with its line reading, in words. A page
  // that agrees answers nothing and its menu explains nothing.
  disagreementOf?: (index: number) => string[] | undefined
}>()

const emit = defineEmits<{
  (event: 'deleted', index: number): void
  (event: 'replaced', page: Page): void
  (event: 'rescan', index: number): void
  (event: 'unrescan', index: number): void
  (event: 'reading', index: number, reading: PageOcr): void
}>()

const themeVars = useThemeVars()

// `only` names the one item a menu holds. The read half of the OCR reader opens
// such a menu: every item that reaches the image belongs on the page half.
const menu = ref<{ x: number; y: number; page: Page; only?: string } | null>(null)
const deleting = ref<number | null>(null)
const comparing = ref<number | null>(null)
const cutting = ref<number | null>(null)

// A page offers the rollback while its book allows one, it still names the page
// it came from, and it does not already show that page.
function offersRollback(page: Page | undefined) {
  return Boolean(props.rollback && page && page.sourcePageId !== null && !page.rolledBack)
}

// A page offers the re-cut while its book allows one, it records the cut it was
// made by, and the page it was cut out of is still there.
function offersCut(page: Page | undefined) {
  return Boolean(props.cut && page && page.cutSide !== null && page.sourcePageId !== null)
}

// Every page of a book that offers the rescan offers it. A reading is a choice,
// not a rank, so a page either engine has read can be read again by either.
function offersRescan(page: Page | undefined) {
  return Boolean(props.rescan && page)
}

// One item, which says what choosing it does: it marks a page that is not marked
// and takes the mark off one that is. It names no engine, because the dialog that
// starts the rescan asks for one.
function rescanItemFor(page: Page | undefined) {
  if (page && props.markedPage?.(page.index)) {
    return { label: 'Do not rescan', key: 'unrescan' }
  }
  return { label: 'Rescan', key: 'rescan' }
}

const readingPrefix = 'reading:'

function readingsFor(page: Page | undefined): OcrReading[] {
  if (!page || !props.readingsOf) {
    return []
  }
  return props.readingsOf(page.index) ?? []
}

// The reading that answers carries a mark in the colour of success, on the side
// the reading order starts at. Every other item holds the room for that mark, so
// the labels of the group line up under each other.
function readingMark(answers: boolean) {
  return () =>
    h(
      'span',
      {
        style: answers ? `color: ${themeVars.value.successColor}` : 'visibility: hidden',
        'aria-hidden': answers ? undefined : 'true',
      },
      '✓',
    )
}

// The readings of a page stand at the top of its menu, best first, each named by
// the profile that made it and the engine behind it. A page that holds one
// reading still lists it: choosing it changes nothing but says what is drawn.
function readingItems(page: Page | undefined) {
  const readings = readingsFor(page)
  if (!readings.length) {
    return []
  }

  // The explanation of a doubt follows the reading it judges, which is the one
  // that answers. It stands with that reading and not at the foot of the menu,
  // where it would read as a note on the rescan below it.
  const doubt = disagreementItems(page)

  return [
    ...readings.flatMap((reading) => [
      {
        label: `${reading.profile} (${reading.engine})`,
        key: readingPrefix + reading.model,
        icon: readingMark(reading.answers),
      },
      ...(reading.answers ? doubt : []),
    ]),
    { type: 'divider', key: 'reading-divider' },
  ]
}

// choose makes the page answer another of its readings. The answer is that page
// read from it, so the reader draws it again with nothing asked for twice.
async function choose(page: Page, model: string) {
  try {
    const reading = await api.setPageReading(props.bookId, page.index, model)
    emit('reading', page.index, reading)
  } catch {
    // The page keeps the reading it answered. The only refusal names a reading
    // the page does not hold, which this menu never offers.
  }
}

// The foot of the menu says why the reading of this page is doubted. It is read
// and not chosen, so it carries no key a selection could name.
function disagreementItems(page: Page | undefined) {
  const reasons = page ? (props.disagreementOf?.(page.index) ?? []) : []
  if (!reasons.length) {
    return []
  }

  return [
    {
      type: 'render',
      key: 'disagreement',
      // The styles are inline: this node is rendered inside the menu, which stands
      // in a portal of its own, and the scoped styles of this file do not reach it.
      render: () =>
        h(
          'div',
          {
            'data-testid': 'page-context-disagreement',
            // The left padding of an item of the menu plus the column its mark
            // stands in, so the explanation lines up with the labels above and
            // below it rather than with the edge of the menu. The e2e test holds
            // that alignment, so a change to the menu cannot break it in silence.
            style: {
              maxWidth: '22rem',
              padding: '0.25rem 14px 0.25rem 36px',
              fontSize: '0.8125rem',
              lineHeight: '1.5',
              // The colour of the border it explains, so the words and the mark on
              // the page read as one thing.
              color: themeVars.value.warningColor,
            },
          },
          reasons.map((reason) => h('div', reason)),
        ),
    },
  ]
}

// The menu of the read half is about the reading of that page: which reading it
// answers, why that reading is doubted, and the rescan that would read it again.
// Nothing here reaches the image, which is what the page half is for.
const readingOptions = computed(() => {
  const page = menu.value?.page
  return [...readingItems(page), rescanItemFor(page)]
})

// Every page reaches its own image, so a page always holds the first two items.
// A page that offers a re-cut or a rollback lists that under them. No page
// offers both: an action makes either one page from one page or two.
//
// It holds nothing about the reading: no reading to choose, no rescan and no
// doubt. Those are the read half's, because that half is where the reading is.
const menuOptions = computed(() => {
  if (menu.value?.only === 'rescan') {
    return readingOptions.value
  }

  const image = [
    { label: 'Open in new tab', key: 'open' },
    { label: 'Download', key: 'download' },
    { type: 'divider', key: 'image-divider' },
  ]

  const offered = offersCut(menu.value?.page)
    ? { label: 'Edit', key: 'cut' }
    : offersRollback(menu.value?.page)
      ? { label: 'Show original', key: 'original' }
      : null

  if (!offered) {
    return [...image, { label: 'Delete', key: 'delete' }]
  }
  return [
    ...image,
    offered,
    { type: 'divider', key: 'divider' },
    { label: 'Delete', key: 'delete' },
  ]
})

function nodeProps(option: { key?: string | number }) {
  if (typeof option.key === 'string' && option.key.startsWith(readingPrefix)) {
    return { 'data-testid': `page-context-${option.key}` }
  }
  if (option.key === 'delete') {
    return { 'data-testid': 'page-context-delete', style: `color: ${themeVars.value.errorColor}` }
  }
  return { 'data-testid': `page-context-${option.key}` }
}

// The menu closes for a tick first, so a right click on another page moves it
// instead of leaving it where it was.
async function open(event: MouseEvent, page: Page) {
  menu.value = null
  await nextTick()
  menu.value = { x: event.clientX, y: event.clientY, page }
}

// openRescan is the menu of the read half. A page that offers no rescan opens
// nothing: an empty menu says less than no menu.
async function openRescan(event: MouseEvent, page: Page) {
  if (!offersRescan(page)) {
    return
  }
  menu.value = null
  await nextTick()
  menu.value = { x: event.clientX, y: event.clientY, page, only: 'rescan' }
}

function close() {
  menu.value = null
}

// The stored name of an original is a storage key, so a download carries a name
// built here instead. The extension comes off the URL: an original keeps the
// extension of the file it was uploaded as.
function fileNameOf(page: Page) {
  const extension = page.originalUrl.slice(page.originalUrl.lastIndexOf('.') + 1)
  return `${props.bookName} - ${String(page.index).padStart(4, '0')}.${extension}`
}

// Firefox starts a download only for an anchor that is in the document.
function download(page: Page) {
  const link = document.createElement('a')
  link.href = page.originalUrl
  link.download = fileNameOf(page)
  document.body.appendChild(link)
  link.click()
  link.remove()
}

// The open runs before anything that waits, so it keeps the click that asked
// for it and no popup blocker stops it.
function onSelect(key: string) {
  const page = menu.value?.page
  menu.value = null
  if (!page) {
    return
  }
  if (key === 'open') {
    window.open(page.originalUrl, '_blank', 'noopener')
    return
  }
  if (key === 'download') {
    download(page)
    return
  }
  if (key === 'delete') {
    deleting.value = page.index
    return
  }
  if (key === 'cut') {
    cutting.value = page.index
    return
  }
  if (key === 'rescan') {
    emit('rescan', page.index)
    return
  }
  if (key === 'unrescan') {
    emit('unrescan', page.index)
    return
  }
  if (key.startsWith(readingPrefix)) {
    choose(page, key.slice(readingPrefix.length))
    return
  }
  comparing.value = page.index
}

function onDeleted() {
  if (deleting.value !== null) {
    emit('deleted', deleting.value)
  }
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    close()
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

defineExpose({ open, openRescan })
</script>

<template>
  <n-dropdown
    v-if="menu"
    trigger="manual"
    placement="bottom-start"
    :show="true"
    :x="menu.x"
    :y="menu.y"
    :options="menuOptions"
    :menu-props="() => ({ 'data-testid': 'page-context-menu' })"
    :node-props="nodeProps"
    @select="onSelect"
    @clickoutside="close"
  />

  <PageDeleteDialog
    v-if="deleting !== null"
    :book-id="bookId"
    :index="deleting"
    @close="deleting = null"
    @deleted="onDeleted"
  />

  <PageCompare
    v-if="comparing !== null"
    :book-id="bookId"
    :index="comparing"
    @close="comparing = null"
    @replaced="emit('replaced', $event)"
  />

  <PageCutDialog
    v-if="cutting !== null"
    :book-id="bookId"
    :index="cutting"
    @close="cutting = null"
    @replaced="emit('replaced', $event)"
  />
</template>
