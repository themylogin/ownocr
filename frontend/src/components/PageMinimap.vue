<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useThemeVars } from 'naive-ui'

// A strip down the right edge of the reader holding one mark per page worth
// returning to. A book of 124 pages scrolls for a long way, and a page that
// disagrees is a page a reader means to come back to.
//
// Only a page carrying a reason gets a mark. A mark for every page would be a
// second scrollbar and would say nothing.
const props = defineProps<{
  // Every page of the book, in page order, so a mark stands where its page stands.
  indexes: number[]
  disagrees: (index: number) => boolean
  marked: (index: number) => boolean
  holdsUltra: (index: number) => boolean
}>()

const emit = defineEmits<{ open: [index: number] }>()

const themeVars = useThemeVars()

interface Reason {
  index: number
  disagrees: boolean
  marked: boolean
  holdsUltra: boolean
}

interface Mark extends Reason {
  offset: number
}

// Which pages carry a mark, and why. A page carrying no reason gets none.
const reasons = computed<Reason[]>(() =>
  props.indexes
    .map((index) => ({
      index,
      disagrees: props.disagrees(index),
      marked: props.marked(index),
      holdsUltra: props.holdsUltra(index),
    }))
    .filter((page) => page.disagrees || page.marked || page.holdsUltra),
)

// The size of a mark, and half of it, which is how far a ring can reach before it
// meets the ring coming from the other side.
const markSize = 12
const centre = markSize / 2

// The colours of one mark, from its centre outwards, in the order the borders of a
// page wear them: the choice of the user, the doubt of the system, the reading held
// in reserve.
function coloursOf(mark: Reason) {
  return [
    mark.marked ? themeVars.value.successColor : '',
    mark.disagrees ? themeVars.value.warningColor : '',
    mark.holdsUltra ? themeVars.value.infoColor : '',
  ].filter(Boolean)
}

// A mark carries a band per reason, sharing its width between them, so a page with
// three reasons shows three colours in one square and a page with one is that colour
// throughout.
//
// An inset shadow reaches in from every edge, so the nearest band is the shallowest:
// the bands are written outermost first, and the innermost reaches the centre.
function ringsOf(mark: Reason) {
  const colours = coloursOf(mark)
  const band = centre / colours.length

  return colours
    .map((colour, ring) => ({ colour, spread: (colours.length - ring) * band }))
    .sort((one, other) => one.spread - other.spread)
    .map((ring) => `inset 0 0 0 ${ring.spread}px ${ring.colour}`)
    .join(', ')
}

// Where each mark stands, measured from the document and not counted from the page
// index.
//
// The scrollbar of the browser maps the whole scrollable height onto its track, and
// the pages do not begin at the top of that height: the bar, the breadcrumbs and
// the controls stand above them. A mark placed by its share of the page count would
// therefore sit above the thumb that reaches it, by the height of everything the
// reader holds before its first page. So each mark is placed where its page
// actually is.
const marks = ref<Mark[]>([])

function measure() {
  const view = document.documentElement.clientHeight
  const height = document.documentElement.scrollHeight
  if (!view || !height) {
    marks.value = []
    return
  }

  // A mark stands where the thumb of the scrollbar stands when its page is in the
  // window, which is not its share of the document.
  //
  // The thumb is as tall a share of its track as the window is of the document, and
  // it travels the track less its own height: at the top of the document its centre
  // is half a thumb down, not at the very top. A mark placed by the share alone
  // therefore agrees with the thumb in the middle of a book and misses it at both
  // ends, by half a thumb.
  const range = height - view
  const thumb = view * (view / height)
  const travel = view - thumb

  marks.value = reasons.value.flatMap((page) => {
    const element = document.querySelector(`[data-page="${page.index}"]`)
    if (!(element instanceof HTMLElement)) {
      return []
    }

    const middle = element.offsetTop + element.offsetHeight / 2
    if (range <= 0) {
      // The book does not scroll, so the scrollbar has no thumb to agree with.
      return [{ ...page, offset: (middle / height) * 100 }]
    }

    // Where the window stands when that page is centred in it.
    const scroll = Math.min(Math.max(middle - view / 2, 0), range)
    const centre = (scroll / range) * travel + thumb / 2
    return [{ ...page, offset: (centre / view) * 100 }]
  })
}

// The offsets move with anything that changes the height of the document: the
// window, an image that loads, a reading that redraws.
let watcher: ResizeObserver | null = null

onMounted(() => {
  measure()
  watcher = new ResizeObserver(() => measure())
  watcher.observe(document.documentElement)
  window.addEventListener('resize', measure)
})

onUnmounted(() => {
  watcher?.disconnect()
  window.removeEventListener('resize', measure)
})

watch(reasons, async () => {
  await nextTick()
  measure()
})

function titleOf(mark: Mark) {
  const reasons = [
    mark.marked ? 'marked for a rescan' : '',
    mark.disagrees ? 'this reading is doubted' : '',
    mark.holdsUltra ? 'an Ultra reading is available' : '',
  ].filter(Boolean)

  return `Page ${mark.index}: ${reasons.join(', ')}`
}
</script>

<template>
  <div v-if="reasons.length" class="minimap" data-testid="page-minimap">
    <button
      v-for="mark in marks"
      :key="mark.index"
      class="mark"
      :class="{
        marked: mark.marked,
        disagrees: mark.disagrees,
        ultra: mark.holdsUltra,
      }"
      :style="{ top: `${mark.offset}%`, '--rings': ringsOf(mark) }"
      :title="titleOf(mark)"
      :data-page-mark="mark.index"
      :data-testid="`page-mark-${mark.index}`"
      @click="emit('open', mark.index)"
    />
  </div>
</template>

<style scoped>
/* Fixed to the right edge of the window, which puts it inside the viewport and so
   left of the scrollbar of the browser. It takes no room from the pages: the
   reader lays them out against the width it measures, and this stands over it. */
.minimap {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 16px;
  z-index: 2;
  pointer-events: none;
}

/* The bands of a mark are the colours the borders of that page wear, and for the
   same reasons: success for a page a user chose, a warning for a reading the system
   doubts, information for a page holding an Ultra reading. `ringsOf` above builds
   them, so one rule draws any combination. */
.mark {
  position: absolute;
  left: 2px;
  width: 12px;
  height: 12px;
  margin-top: -6px;
  padding: 0;
  border: none;
  border-radius: 2px;
  cursor: pointer;
  pointer-events: auto;
  background: none;
  box-shadow: var(--rings);
}

.mark:hover {
  outline: 2px solid v-bind('themeVars.textColor1');
  outline-offset: 1px;
}
</style>
