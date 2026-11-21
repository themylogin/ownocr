<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { NAlert, NButton, NSpin, useThemeVars } from 'naive-ui'
import { api, type ComparedPage, type Page, type PageRollback } from '../api/client'
import AppModal from './AppModal.vue'

const props = defineProps<{ bookId: number; index: number }>()
const emit = defineEmits<{ close: []; replaced: [page: Page] }>()

const themeVars = useThemeVars()

const pages = ref<PageRollback | null>(null)
const loading = ref(true)
const confirming = ref(false)
const sending = ref(false)
const error = ref('')

// The room the modal itself takes: the outer margin, the gap between the two
// panes, the card padding, and the two bars above and below them.
const chrome = { margin: 32, gap: 16, padding: 48, header: 64, footer: 76 }
const minWidth = 360

const viewport = ref({ width: window.innerWidth, height: window.innerHeight })

function onResize() {
  viewport.value = { width: window.innerWidth, height: window.innerHeight }
}

onMounted(() => window.addEventListener('resize', onResize))
onUnmounted(() => window.removeEventListener('resize', onResize))

onMounted(async () => {
  try {
    pages.value = await api.pageRollback(props.bookId, props.index)
  } catch (cause) {
    error.value = message(cause)
  } finally {
    loading.value = false
  }
})

function message(cause: unknown) {
  return cause instanceof Error ? cause.message : String(cause)
}

// Both panes take the same scale, so the two pages stay comparable. The 1 keeps
// the rule the reader follows: never enlarge a page beyond its stored
// rendition.
const scale = computed(() => {
  if (!pages.value) {
    return 1
  }

  const paneWidth = (viewport.value.width - 2 * chrome.margin - chrome.padding - chrome.gap) / 2
  const paneHeight = viewport.value.height - 2 * chrome.margin - chrome.header - chrome.footer

  return [pages.value.source, pages.value.current].reduce(
    (smallest, page) => Math.min(smallest, paneWidth / page.width, paneHeight / page.height),
    1,
  )
})

function sizeOf(page: ComparedPage) {
  return {
    width: `${Math.floor(page.width * scale.value)}px`,
    height: `${Math.floor(page.height * scale.value)}px`,
  }
}

// The modal shrinks to the room the scaled pages need, so no dead space is
// left beside them.
const modalWidth = computed(() => {
  if (!pages.value) {
    return minWidth
  }

  const used =
    Math.floor(pages.value.source.width * scale.value) +
    Math.floor(pages.value.current.width * scale.value) +
    chrome.gap +
    chrome.padding

  return Math.max(used, minWidth)
})

async function submit() {
  sending.value = true
  error.value = ''
  try {
    const page = await api.applyPageRollback(props.bookId, props.index)
    emit('replaced', page)
    emit('close')
  } catch (cause) {
    error.value = message(cause)
    confirming.value = false
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <!-- Busy while the confirmation is up, so the mask and Escape reach that
       modal alone. -->
  <AppModal
    title="Compare this page"
    :width="`${modalWidth}px`"
    :busy="sending || confirming"
    @close="emit('close')"
  >
    <div class="compare" data-testid="page-compare">
      <n-spin v-if="loading" size="small" />

      <div v-else-if="pages" class="panes">
        <figure data-testid="compare-source">
          <figcaption>{{ pages.source.stageTitle }}</figcaption>
          <img :src="pages.source.displayUrl" :alt="pages.source.stageTitle" :style="sizeOf(pages.source)" />
        </figure>
        <figure data-testid="compare-current">
          <figcaption>{{ pages.current.stageTitle }}</figcaption>
          <img :src="pages.current.displayUrl" :alt="pages.current.stageTitle" :style="sizeOf(pages.current)" />
        </figure>
      </div>

      <n-alert v-if="error" type="error" data-testid="page-compare-error" :bordered="false">
        {{ error }}
      </n-alert>

      <div class="actions">
        <n-button
          type="error"
          data-testid="use-original"
          :disabled="loading || sending || !pages"
          @click="confirming = true"
        >
          Use original
        </n-button>
        <n-button tertiary :disabled="sending" data-testid="compare-close" @click="emit('close')">
          Close
        </n-button>
      </div>
    </div>
  </AppModal>

  <AppModal
    v-if="confirming"
    title="Use the original page?"
    :busy="sending"
    @close="confirming = false"
  >
    <div class="confirm" data-testid="use-original-dialog">
      <p>
        This replaces the page with the one it was made from. It cannot be undone: getting
        this page back means running the action for the whole book again.
      </p>

      <div class="actions">
        <n-button tertiary :disabled="sending" @click="confirming = false">Cancel</n-button>
        <n-button
          type="error"
          data-testid="use-original-confirm"
          :loading="sending"
          :disabled="sending"
          @click="submit"
        >
          Use original
        </n-button>
      </div>
    </div>
  </AppModal>
</template>

<style scoped>
.compare,
.confirm {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

p {
  margin: 0;
}

.panes {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  gap: 1rem;
}

figure {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

figcaption {
  font-size: 0.8rem;
  color: v-bind('themeVars.textColor3');
}

img {
  display: block;
  object-fit: contain;
  background: v-bind('themeVars.cardColor');
  border: 1px solid v-bind('themeVars.dividerColor');
  border-radius: v-bind('themeVars.borderRadiusSmall');
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
