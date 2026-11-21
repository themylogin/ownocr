<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NCard, NCheckbox, NText, useThemeVars } from 'naive-ui'
import type { Book } from '../api/client'
import { canRetry, groupOf } from '../bookState'
import BookMark from './BookMark.vue'

const props = defineProps<{ book: Book; selectable: boolean; selected: boolean }>()

const emit = defineEmits<{
  (event: 'select', id: number, on: boolean): void
  (event: 'retry', book: Book): void
}>()

const themeVars = useThemeVars()

const group = computed(() => groupOf(props.book))
const open = computed(() => props.book.state === 'complete')

const mark = computed(() => {
  if (group.value === 'done') {
    return 'check' as const
  }
  if (group.value !== 'working') {
    return null
  }
  return props.book.derivedState === 'failed' ? ('warning' as const) : ('spinner' as const)
})

const checkbox = computed(() => props.selectable && group.value === 'ready')

function onCheck(on: boolean) {
  emit('select', props.book.id, on)
}
</script>

<template>
  <li
    class="card"
    :class="group"
    :data-testid="`book-card-${book.id}`"
    :data-group="group"
  >
    <component
      :is="open ? 'RouterLink' : 'div'"
      class="face"
      v-bind="open ? { to: { name: 'book', params: { id: book.id } } } : {}"
    >
      <n-card :hoverable="open" size="small">
        <template #cover>
          <img
            v-if="open && book.coverUrl"
            :src="book.coverUrl"
            :alt="book.name"
            loading="lazy"
          />
          <span v-else-if="open" class="nocover"></span>
          <span v-else class="status">
            <BookMark :kind="group === 'failed' ? 'warning' : 'spinner'" :size="44" />
            <n-text
              class="phase"
              depth="3"
              :type="group === 'failed' ? 'error' : 'default'"
              :data-testid="`book-phase-${book.id}`"
            >
              {{ group === 'failed' ? book.error : book.progress }}
            </n-text>
          </span>
        </template>
        {{ book.name }}
      </n-card>
    </component>

    <n-checkbox
      v-if="checkbox"
      class="corner"
      :checked="selected"
      :data-testid="`book-select-${book.id}`"
      :aria-label="`Select ${book.name}`"
      @update:checked="onCheck"
    />
    <span
      v-else-if="mark"
      class="corner"
      :title="mark === 'check' ? 'Done' : mark === 'warning' ? 'The copy failed' : 'Working'"
      :data-testid="`book-mark-${book.id}`"
    >
      <BookMark :kind="mark" />
    </span>

    <n-button
      v-if="canRetry(book)"
      class="retry"
      size="tiny"
      round
      :data-testid="`retry-${book.id}`"
      @click="emit('retry', book)"
    >
      Retry
    </n-button>
  </li>
</template>

<style scoped>
.card {
  position: relative;
}

/* A book that took its action steps back. The one whose copy is finished steps
   back further, so the two never read alike. */
.card.working .face {
  opacity: 0.75;
}

.card.done .face {
  opacity: 0.55;
}

.face {
  display: block;
  text-decoration: none;
  color: inherit;
}

.corner {
  position: absolute;
  top: 0.45rem;
  right: 0.45rem;
}

/* Only the mark collapses to its icon. A checkbox keeps its own box, or it
   has no height to click. */
span.corner {
  line-height: 0;
}

.retry {
  position: absolute;
  bottom: 0.45rem;
  right: 0.45rem;
}

img,
.nocover,
.status {
  display: block;
  width: 100%;
  height: 14rem;
  object-fit: contain;
}

.status {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.6rem;
  padding: 0 0.75rem;
  background: v-bind('themeVars.actionColor');
}

.phase {
  font-size: 0.8rem;
  text-align: center;
  overflow-wrap: anywhere;
}
</style>
