<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NAlert } from 'naive-ui'
import { api, type Book, type OcrProfile } from '../api/client'
import { isReady } from '../bookState'
import { useWebSocket } from '../composables/useWebSocket'
import { useStages } from '../composables/useStages'
import { useSelection } from '../composables/useSelection'
import { useActions } from '../composables/useActions'
import BookList from '../components/BookList.vue'
import OcrDialog from '../components/OcrDialog.vue'

const route = useRoute()
const router = useRouter()

const { stages, ensureStages, actionsFor } = useStages()
const { useIn, keepOnly } = useSelection()
const { error, started, run } = useActions()

const books = ref<Book[]>([])
const loadError = ref('')
const ready = ref(false)

// The books a pending OCR action names. The dialog is open while it holds any.
const reading = ref<number[]>([])
const readingBusy = ref(false)

const currentStage = computed(() => String(route.params.stage ?? ''))
const actions = computed(() => actionsFor(currentStage.value))
const alert = computed(() => loadError.value || error.value)

const { onEvent } = useWebSocket()

// A book belongs to the tab of its own stage, in every state. Applying a book
// either updates a card here, adds one, or takes one away.
function apply(book: Book) {
  const rest = books.value.filter((entry) => entry.id !== book.id)
  books.value = book.stage === currentStage.value ? [...rest, book] : rest
  keepOnly(books.value.filter(isReady).map((entry) => entry.id))
}

function drop(id: number) {
  books.value = books.value.filter((entry) => entry.id !== id)
  keepOnly(books.value.filter(isReady).map((entry) => entry.id))
}

onEvent((event) => {
  if (event.type === 'book.created' || event.type === 'book.updated') {
    apply(event.payload as Book)
    return
  }
  if (event.type === 'book.deleted') {
    drop((event.payload as { id: number }).id)
  }
})

// The copies an action just created, whoever started it. The socket carries
// them too, but this way the list moves even while it is reconnecting.
watch(started, (list) => list.forEach(apply))

async function loadStages() {
  await ensureStages()

  if (stages.value.length === 0) {
    return
  }
  if (!stages.value.some((stage) => stage.id === currentStage.value)) {
    await router.replace({ name: 'stage', params: { stage: stages.value[0].id } })
  }
}

async function loadBooks() {
  if (!currentStage.value) {
    return
  }
  books.value = await api.listByStage(currentStage.value)
}

async function refresh() {
  try {
    await loadStages()
    await loadBooks()
    loadError.value = ''
  } catch (cause) {
    loadError.value = message(cause)
  } finally {
    ready.value = true
  }
}

function message(cause: unknown) {
  return cause instanceof Error ? cause.message : String(cause)
}

// The OCR action takes a model, so it asks before it starts.
function onRun(action: string, bookIds: number[]) {
  if (action === 'ocr') {
    reading.value = bookIds
    return
  }
  run(action, bookIds)
}

async function startOcr(profile: OcrProfile) {
  readingBusy.value = true
  await run('ocr', reading.value, { ocrProfile: profile })
  readingBusy.value = false
  reading.value = []
}

// The retry runs the action of the failed copy on the book it came from. The
// server reuses the same copy, so no third book appears.
//
// It carries the profile the copy records so the request passes the argument
// check. The server reads the profile off the copy either way, so a retry
// cannot change the model a copy was started with.
function retry(book: Book) {
  if (book.sourceBookId === null || book.action === null) {
    return
  }
  run(book.action, [book.sourceBookId], { ocrProfile: book.ocrProfile ?? undefined })
}

watch(
  currentStage,
  (stage) => {
    useIn(stage)
    error.value = ''
    loadBooks().catch((cause) => (loadError.value = message(cause)))
  },
  { immediate: true },
)

onMounted(refresh)
</script>

<template>
  <n-alert v-if="alert" type="error" data-testid="stage-error" :bordered="false">
    {{ alert }}
  </n-alert>

  <BookList
    :books="books"
    :actions="actions"
    :ready="ready"
    @run="onRun"
    @retry="retry"
    @rejected="loadBooks"
  />

  <OcrDialog
    v-if="reading.length"
    :count="reading.length"
    :busy="readingBusy"
    :error="error"
    @close="reading = []"
    @start="startOcr"
  />
</template>
