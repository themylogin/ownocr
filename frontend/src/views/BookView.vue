<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NAlert, useThemeVars } from 'naive-ui'
import { api, type BookDetail, type Page } from '../api/client'
import { useStages } from '../composables/useStages'
import { useActions } from '../composables/useActions'
import { usePageOcr } from '../composables/usePageOcr'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import BookActions from '../components/BookActions.vue'
import PageMenu from '../components/PageMenu.vue'

const route = useRoute()
const router = useRouter()
const themeVars = useThemeVars()
const { bookStage, ensureStages, titleOf } = useStages()
const { error: actionError } = useActions()

const book = ref<BookDetail | null>(null)
const loadError = ref('')

const bookId = computed(() => Number(route.params.id))
const alert = computed(() => loadError.value || actionError.value)

// A book that has been read holds a reading per page, which the grid marks where
// it disagrees with the line reading of that page.
const { ask, linesOf, disagreeing } = usePageOcr(() => bookId.value)

function disagrees(index: number) {
  return linesOf(index)?.disagrees ?? false
}

// How many pages of this book hold a reading that parts company with its line
// reading. A count of zero is not news, so the bar says nothing about it.
const disagreement = computed(() => {
  if (!disagreeing.value) {
    return ''
  }
  return disagreeing.value === 1 ? '1 page disagrees' : `${disagreeing.value} pages disagree`
})

const crumbs = computed(() => {
  if (!book.value) {
    return []
  }
  return [
    { label: titleOf(book.value.stage), to: { name: 'stage', params: { stage: book.value.stage } } },
    { label: book.value.name },
  ]
})

const cellStyle = computed(() => {
  const cell = book.value?.gridCell
  if (!cell) {
    return {}
  }
  return { '--cell-aspect': `${cell.width} / ${cell.height}` }
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

  if (book.value.stage === 'ocr') {
    ask()
  }
})
</script>

<template>
  <n-alert v-if="alert" type="error" data-testid="book-error" :bordered="false">{{ alert }}</n-alert>

  <template v-if="book">
    <header class="bar">
      <Breadcrumbs :crumbs="crumbs">
        <template #note>
          <li v-if="disagreement" class="disagreement" data-testid="book-disagreement">
            {{ disagreement }}
          </li>
        </template>
      </Breadcrumbs>

      <BookActions :book="book" @rejected="leave" />
    </header>

    <ul class="grid" data-testid="page-grid" :style="cellStyle">
      <li
        v-for="page in book.pages"
        :key="page.index"
        :data-testid="`page-cell-${page.index}`"
        @contextmenu.prevent="menu?.open($event, page)"
      >
        <RouterLink :to="{ name: 'reader', params: { id: book.id, index: page.index } }">
          <img
            :src="page.thumbnailUrl"
            :alt="`Page ${page.index}`"
            :class="{ disagrees: disagrees(page.index) }"
            loading="lazy"
          />
        </RouterLink>
      </li>
    </ul>

    <PageMenu
      ref="menu"
      :book-id="book.id"
      :book-name="book.name"
      :rollback="book.pageRollback"
      :cut="book.pageCut"
      :rescan="false"
      @deleted="onDeleted"
      @replaced="onReplaced"
    />
  </template>
</template>

<style scoped>
.bar {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1rem;
}

/* The count stands in the trail itself, as it does in the reader, so it shares the
   line of the crumbs and their baseline. */
.disagreement {
  display: inline;
  list-style: none;
  margin-left: 0.75rem;
  color: v-bind('themeVars.warningColor');
  font-size: 0.875rem;
  white-space: nowrap;
}

/* Five cells per row, whatever the window is. The cell keeps the shape the
   server computed from the pages of this book. */
.grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 0.75rem;
}

.grid li {
  aspect-ratio: var(--cell-aspect, 1);
}

.grid a,
.grid img {
  display: block;
  width: 100%;
  height: 100%;
}

/* contain, never cover: a page must not be cropped and must not be enlarged. */
.grid img {
  object-fit: contain;
  background: v-bind('themeVars.cardColor');
  border: 1px solid v-bind('themeVars.dividerColor');
  border-radius: v-bind('themeVars.borderRadiusSmall');
}

.grid a:hover img {
  border-color: v-bind('themeVars.primaryColor');
}

/* A page whose reading parts company with its line reading is worth opening. The
   border is drawn inside the thumbnail, so it moves no cell and changes no size,
   and it stands whatever the cell is showing. */
.grid img.disagrees {
  outline: 3px solid v-bind('themeVars.warningColor');
  outline-offset: -3px;
  border-color: v-bind('themeVars.warningColor');
}

.grid a:hover img.disagrees {
  border-color: v-bind('themeVars.warningColor');
}
</style>
