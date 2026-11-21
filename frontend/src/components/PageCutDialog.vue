<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { NAlert, NButton, NSpin, useThemeVars } from 'naive-ui'
import { api, type Page, type PageCut } from '../api/client'
import AppModal from './AppModal.vue'

const props = defineProps<{ bookId: number; index: number }>()
const emit = defineEmits<{ close: []; replaced: [page: Page] }>()

const themeVars = useThemeVars()

const cut = ref<PageCut | null>(null)
const column = ref(0)
const loading = ref(true)
const sending = ref(false)
const error = ref('')

// The room the modal itself takes: the outer margin, the card padding, and the
// two bars above and below the image.
const chrome = { margin: 32, padding: 48, header: 64, footer: 76 }
const minWidth = 360

const viewport = ref({ width: window.innerWidth, height: window.innerHeight })

function onResize() {
  viewport.value = { width: window.innerWidth, height: window.innerHeight }
}

onMounted(() => window.addEventListener('resize', onResize))
onUnmounted(() => {
  window.removeEventListener('resize', onResize)
  stopDrag()
})

onMounted(async () => {
  try {
    cut.value = await api.pageCut(props.bookId, props.index)
    column.value = cut.value.column
  } catch (cause) {
    error.value = message(cause)
  } finally {
    loading.value = false
  }
})

function message(cause: unknown) {
  return cause instanceof Error ? cause.message : String(cause)
}

// The 1 keeps the rule the reader follows: never enlarge a page beyond its
// stored display rendition.
const scale = computed(() => {
  if (!cut.value) {
    return 1
  }

  const paneWidth = viewport.value.width - 2 * chrome.margin - chrome.padding
  const paneHeight = viewport.value.height - 2 * chrome.margin - chrome.header - chrome.footer

  return Math.min(1, paneWidth / cut.value.source.width, paneHeight / cut.value.source.height)
})

const size = computed(() => {
  if (!cut.value) {
    return { width: 0, height: 0 }
  }
  return {
    width: Math.floor(cut.value.source.width * scale.value),
    height: Math.floor(cut.value.source.height * scale.value),
  }
})

const modalWidth = computed(() =>
  cut.value ? Math.max(size.value.width + chrome.padding, minWidth) : minWidth,
)

// Where the bar stands on the rendered image. The column counts in the stored
// image, so it travels through the ratio of the two widths.
const barOffset = computed(() => {
  if (!cut.value) {
    return 0
  }
  return (column.value / cut.value.source.width) * size.value.width
})

// The part of the parent image this page leaves out. The left page leaves out
// everything after the bar; the right page everything before it.
const shade = computed(() => {
  if (!cut.value) {
    return { left: '0px', width: '0px' }
  }
  if (cut.value.side === 'left') {
    return { left: `${barOffset.value}px`, width: `${size.value.width - barOffset.value}px` }
  }
  return { left: '0px', width: `${barOffset.value}px` }
})

// The bar never leaves the image, and never reaches the edge that would leave
// this page with no column of its own.
function clamp(value: number) {
  if (!cut.value) {
    return value
  }
  const width = cut.value.source.width
  const lowest = cut.value.side === 'left' ? 1 : 0
  const highest = cut.value.side === 'left' ? width : width - 1

  return Math.min(highest, Math.max(lowest, value))
}

const image = ref<HTMLElement | null>(null)

function columnAt(clientX: number) {
  if (!cut.value || !image.value || size.value.width === 0) {
    return column.value
  }
  const box = image.value.getBoundingClientRect()
  const share = (clientX - box.left) / size.value.width

  return clamp(Math.round(share * cut.value.source.width))
}

// The pointer keeps the spot of the handle it took hold of, so grabbing the
// handle off centre does not jump the bar under it.
let grab = 0

function onDrag(event: PointerEvent) {
  column.value = columnAt(event.clientX - grab)
}

function stopDrag() {
  window.removeEventListener('pointermove', onDrag)
  window.removeEventListener('pointerup', stopDrag)
}

function startDrag(event: PointerEvent) {
  event.preventDefault()

  const box = image.value?.getBoundingClientRect()
  grab = box ? event.clientX - (box.left + barOffset.value) : 0

  window.addEventListener('pointermove', onDrag)
  window.addEventListener('pointerup', stopDrag)
}

// The modal content is teleported out of this component, so scoped `v-bind`
// styles never reach it. Inline custom properties do.
const colours = computed(() => ({
  '--cut-accent': themeVars.value.primaryColor,
  '--cut-card': themeVars.value.cardColor,
}))

async function save() {
  sending.value = true
  error.value = ''
  try {
    const page = await api.applyPageCut(props.bookId, props.index, column.value)
    emit('replaced', page)
    emit('close')
  } catch (cause) {
    error.value = message(cause)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <AppModal
    title="Edit the cut"
    :width="`${modalWidth}px`"
    :busy="sending"
    @close="emit('close')"
  >
    <div class="cut" data-testid="page-cut">
      <n-spin v-if="loading" size="small" />

      <div
        v-else-if="cut"
        ref="image"
        class="image"
        data-testid="page-cut-image"
        :style="{ width: `${size.width}px`, height: `${size.height}px`, ...colours }"
      >
        <img :src="cut.source.displayUrl" alt="The page this one was cut out of" />
        <div class="shade" data-testid="page-cut-shade" :style="shade" />
        <div
          class="bar"
          data-testid="page-cut-bar"
          :data-column="column"
          :style="{ left: `${barOffset}px` }"
          @pointerdown="startDrag"
        >
          <span class="grab" />
          <span class="handle" data-testid="page-cut-handle" />
        </div>
      </div>

      <n-alert v-if="error" type="error" data-testid="page-cut-error" :bordered="false">
        {{ error }}
      </n-alert>

      <div class="actions">
        <n-button
          type="primary"
          data-testid="page-cut-save"
          :loading="sending"
          :disabled="loading || sending || !cut"
          @click="save"
        >
          Save
        </n-button>
        <n-button tertiary :disabled="sending" data-testid="page-cut-cancel" @click="emit('close')">
          Cancel
        </n-button>
      </div>
    </div>
  </AppModal>
</template>

<style scoped>
.cut {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  align-items: center;
}

.image {
  position: relative;
  touch-action: none;
  user-select: none;
  background: var(--cut-card);
}

.image img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.shade {
  position: absolute;
  top: 0;
  bottom: 0;
  background: rgb(128 128 128 / 50%);
  pointer-events: none;
}

/* The white outline keeps the bar readable over a dark page and over a light
   one, which the accent colour alone cannot do. */
.bar {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  margin-left: -1px;
  background: var(--cut-accent);
  box-shadow: 0 0 0 1px rgb(255 255 255 / 70%);
  cursor: ew-resize;
}

/* The bar is two pixels wide, which no pointer can hit. This strip is the room
   the user has to grab it, above and below the handle. */
.grab {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  width: 1.5rem;
  transform: translateX(-50%);
  cursor: ew-resize;
}

.handle {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 1.5rem;
  height: 3.5rem;
  transform: translate(-50%, -50%);
  border: 2px solid #fff;
  border-radius: 0.75rem;
  background: var(--cut-accent);
  box-shadow: 0 2px 8px rgb(0 0 0 / 45%);
  cursor: ew-resize;
}

/* Three grip lines: the middle one is the element, the other two its shadow. */
.handle::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 2px;
  height: 1.125rem;
  transform: translate(-50%, -50%);
  border-radius: 1px;
  background: #fff;
  box-shadow:
    -5px 0 0 #fff,
    5px 0 0 #fff;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  align-self: flex-end;
}
</style>
