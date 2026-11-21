<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { NDropdown, NEmpty, useThemeVars } from 'naive-ui'
import type { Action, Book } from '../api/client'
import { compareBooks, isReady } from '../bookState'
import { useSelection } from '../composables/useSelection'
import BookCard from './BookCard.vue'
import RejectDialog from './RejectDialog.vue'

const props = defineProps<{ books: Book[]; actions: Action[]; ready: boolean }>()

const emit = defineEmits<{
  (event: 'run', action: string, bookIds: number[]): void
  (event: 'retry', book: Book): void
  (event: 'rejected'): void
}>()

const themeVars = useThemeVars()
const { has, toggle } = useSelection()

const sorted = computed(() => props.books.slice().sort(compareBooks))

const menu = ref<{ x: number; y: number; book: Book } | null>(null)
const rejecting = ref<Book | null>(null)

// Every book can be rejected, so a card always has at least one menu item. A
// book that is ready for an action lists those first.
const menuOptions = computed(() => {
  const book = menu.value?.book
  const actions =
    book && isReady(book)
      ? props.actions.map((action) => ({ label: action.title, key: action.id }))
      : []

  if (actions.length === 0) {
    return [{ label: 'Reject', key: 'reject' }]
  }
  return [...actions, { type: 'divider', key: 'divider' }, { label: 'Reject', key: 'reject' }]
})

function nodeProps(option: { key?: string | number }) {
  if (option.key === 'reject') {
    return { 'data-testid': 'context-reject', style: `color: ${themeVars.value.errorColor}` }
  }
  return { 'data-testid': `context-${option.key}` }
}

// The menu closes for a tick first, so a right click on another card moves it
// instead of leaving it where it was.
async function openMenu(event: MouseEvent, book: Book) {
  menu.value = null
  await nextTick()
  menu.value = { x: event.clientX, y: event.clientY, book }
}

function closeMenu() {
  menu.value = null
}

function onSelect(key: string) {
  const book = menu.value?.book
  menu.value = null
  if (!book) {
    return
  }
  if (key === 'reject') {
    rejecting.value = book
    return
  }
  emit('run', key, [book.id])
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    closeMenu()
  }
}

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <section class="books" data-testid="stage-books">
    <n-empty
      v-if="ready && !sorted.length"
      data-testid="stage-empty"
      description="No books here yet. Upload one to start."
    />

    <ul v-else class="grid">
      <BookCard
        v-for="book in sorted"
        :key="book.id"
        :book="book"
        :selectable="actions.length > 0"
        :selected="has(book.id)"
        @contextmenu.prevent="openMenu($event, book)"
        @select="toggle"
        @retry="emit('retry', $event)"
      />
    </ul>
  </section>

  <n-dropdown
    v-if="menu"
    trigger="manual"
    placement="bottom-start"
    :show="true"
    :x="menu.x"
    :y="menu.y"
    :options="menuOptions"
    :menu-props="() => ({ 'data-testid': 'context-menu' })"
    :node-props="nodeProps"
    @select="onSelect"
    @clickoutside="closeMenu"
  />

  <RejectDialog
    v-if="rejecting"
    :book="rejecting"
    @close="rejecting = null"
    @rejected="emit('rejected')"
  />
</template>

<style scoped>
.grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
  gap: 1rem;
}
</style>
