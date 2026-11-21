<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton } from 'naive-ui'
import type { Book, OcrProfile } from '../api/client'
import { groupOf } from '../bookState'
import { useStages } from '../composables/useStages'
import { useActions } from '../composables/useActions'
import BookMark from './BookMark.vue'
import OcrDialog from './OcrDialog.vue'
import RejectDialog from './RejectDialog.vue'

const props = defineProps<{ book: Book }>()
const emit = defineEmits<{ rejected: [] }>()

const { actionsFor } = useStages()
const { error, started, run } = useActions()

// The copy state changes under this component when an action starts, and the
// parent holds a book it fetched once. This is the live view of it.
const derived = ref(props.book.derivedState)
watch(() => props.book.derivedState, (state) => (derived.value = state))

const running = ref('')
const rejecting = ref(false)

// The OCR action takes a model, so it asks before it starts. Every other action
// starts at once.
const choosing = ref(false)

const group = computed(() => groupOf({ ...props.book, derivedState: derived.value }))
const actions = computed(() => (group.value === 'ready' ? actionsFor(props.book.stage) : []))

const mark = computed(() => {
  if (group.value === 'done') {
    return 'check' as const
  }
  if (group.value !== 'working') {
    return null
  }
  return derived.value === 'failed' ? ('warning' as const) : ('spinner' as const)
})

const title = computed(() => {
  if (mark.value === 'check') {
    return 'Done'
  }
  return mark.value === 'warning' ? 'The copy failed' : 'Working'
})

watch(started, (list) => {
  const copy = list.find((entry) => entry.sourceBookId === props.book.id)
  if (copy) {
    derived.value = copy.state
  }
  running.value = ''
})

async function start(action: string) {
  if (running.value) {
    return
  }
  if (action === 'ocr') {
    choosing.value = true
    return
  }
  running.value = action
  await run(action, [props.book.id])
  running.value = ''
}

async function startOcr(profile: OcrProfile) {
  running.value = 'ocr'
  await run('ocr', [props.book.id], { ocrProfile: profile })
  running.value = ''
  choosing.value = false
}
</script>

<template>
  <span class="controls">
    <n-button
      v-for="action in actions"
      :key="action.id"
      type="primary"
      size="small"
      :data-testid="`action-${action.id}`"
      :loading="running === action.id"
      :disabled="Boolean(running)"
      @click="start(action.id)"
    >
      {{ action.title }}
    </n-button>

    <span v-if="mark" class="mark" :data-testid="`book-mark-${book.id}`" :title="title">
      <BookMark :kind="mark" :size="20" />
    </span>

    <n-button
      type="error"
      size="small"
      ghost
      data-testid="action-reject"
      :disabled="Boolean(running)"
      @click="rejecting = true"
    >
      Reject
    </n-button>
  </span>

  <OcrDialog
    v-if="choosing"
    :count="1"
    :busy="running === 'ocr'"
    :error="error"
    @close="choosing = false"
    @start="startOcr"
  />

  <RejectDialog
    v-if="rejecting"
    :book="book"
    @close="rejecting = false"
    @rejected="emit('rejected')"
  />
</template>

<style scoped>
.controls {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.mark {
  line-height: 0;
}
</style>
