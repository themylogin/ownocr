<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  NButton,
  NConfigProvider,
  NDropdown,
  NGlobalStyle,
  NTag,
  darkTheme,
  useOsTheme,
  useThemeVars,
} from 'naive-ui'
import { api, type OcrProfile } from './api/client'
import { isReady } from './bookState'
import { useWebSocket } from './composables/useWebSocket'
import { useStages } from './composables/useStages'
import { useLayout } from './composables/useLayout'
import { useSelection } from './composables/useSelection'
import { useActions } from './composables/useActions'
import OcrDialog from './components/OcrDialog.vue'
import UploadDialog from './components/UploadDialog.vue'

const route = useRoute()
const themeVars = useThemeVars()

const health = ref('unknown')
const showUpload = ref(false)

const { status: connection } = useWebSocket('/api/ws')

const { stages, bookStage, ensureStages, actionsFor } = useStages()
const { wide } = useLayout()
const { selected, count, selectAll, clear } = useSelection()

// The books a pending OCR action names. The dialog is open while it holds any.
const reading = ref<number[]>([])
const readingBusy = ref(false)
const { error: actionError, run } = useActions()

const currentStage = computed(() =>
  route.name === 'stage' ? String(route.params.stage ?? '') : (bookStage.value ?? ''),
)

const actions = computed(() => (route.name === 'stage' ? actionsFor(currentStage.value) : []))
const actionOptions = computed(() =>
  actions.value.map((action) => ({ label: action.title, key: action.id })),
)

const osTheme = useOsTheme()
const theme = computed(() => (osTheme.value === 'dark' ? darkTheme : null))

function tagType(state: string) {
  if (state === 'ok' || state === 'open') {
    return 'success'
  }
  if (state === 'degraded' || state === 'closed') {
    return 'error'
  }
  return 'default'
}

// Select all covers the books of the current tab that can still take an
// action. The list is the same one the checkboxes come from.
async function onSelectAll() {
  const books = await api.listByStage(currentStage.value)
  selectAll(books.filter(isReady).map((book) => book.id))
}

function bulkNodeProps(option: { key?: string | number }) {
  return { 'data-testid': `bulk-${option.key}` }
}

// The OCR action takes a model, so it asks before it starts. The selection
// survives until it does, so cancelling leaves the books selected.
function onBulkAction(action: string) {
  const ids = [...selected.value]
  if (action === 'ocr') {
    reading.value = ids
    return
  }
  clear()
  run(action, ids)
}

async function startBulkOcr(profile: OcrProfile) {
  readingBusy.value = true
  await run('ocr', reading.value, { ocrProfile: profile })
  readingBusy.value = false
  reading.value = []
  clear()
}

onMounted(async () => {
  ensureStages()
  try {
    health.value = (await api.health()).status
  } catch {
    health.value = 'degraded'
  }
})
</script>

<template>
  <n-config-provider :theme="theme">
    <n-global-style />
    <main :class="{ wide }" data-testid="content">
      <header>
        <h1>ownocr</h1>
        <div class="badges">
          <n-tag size="small" round :type="tagType(health)">api: {{ health }}</n-tag>
          <n-tag size="small" round :type="tagType(connection)">ws: {{ connection }}</n-tag>
        </div>
      </header>

      <nav class="tabs" data-testid="stage-tabs">
        <RouterLink
          v-for="stage in stages"
          :key="stage.id"
          class="tab"
          :data-testid="`stage-tab-${stage.id}`"
          :class="{ selected: stage.id === currentStage }"
          :to="{ name: 'stage', params: { stage: stage.id } }"
        >
          {{ stage.title }}
        </RouterLink>

        <n-button
          class="upload"
          type="primary"
          size="small"
          data-testid="upload-button"
          @click="showUpload = true"
        >
          Upload
        </n-button>

        <n-button
          v-if="actions.length"
          size="small"
          data-testid="select-all"
          @click="onSelectAll"
        >
          Select all
        </n-button>

        <template v-if="actions.length && count">
          <n-button size="small" data-testid="deselect-all" @click="clear">
            Deselect all
          </n-button>

          <n-dropdown
            trigger="click"
            :options="actionOptions"
            :menu-props="() => ({ 'data-testid': 'bulk-menu' })"
            :node-props="bulkNodeProps"
            @select="onBulkAction"
          >
            <n-button size="small" type="primary" ghost data-testid="bulk-actions">
              {{ count }} selected
            </n-button>
          </n-dropdown>
        </template>
      </nav>

      <RouterView />
    </main>

    <OcrDialog
      v-if="reading.length"
      :count="reading.length"
      :busy="readingBusy"
      :error="actionError"
      @close="reading = []"
      @start="startBulkOcr"
    />

    <UploadDialog v-if="showUpload" @close="showUpload = false" />
  </n-config-provider>
</template>

<style scoped>
main {
  max-width: 72rem;
  margin: 0 auto;
  padding: 1.5rem 1rem 4rem;
}

/* The OCR reader shows two pages side by side, which the column cannot hold. */
main.wide {
  max-width: none;
}

header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  margin-bottom: 1.5rem;
}

h1 {
  margin: 0;
  font-size: 1.5rem;
}

.badges {
  display: flex;
  gap: 0.5rem;
}

.tabs {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin-bottom: 1.25rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid v-bind('themeVars.dividerColor');
}

.tab {
  padding: 0.4rem 0.9rem;
  border-radius: v-bind('themeVars.borderRadius');
  color: v-bind('themeVars.textColor3');
  text-decoration: none;
  font-size: 0.95rem;
}

.tab:hover {
  color: v-bind('themeVars.textColor1');
}

.tab.selected {
  color: v-bind('themeVars.primaryColor');
  background: v-bind('themeVars.hoverColor');
}

/* The upload control ends the tab row. The selection controls follow it. */
.upload {
  margin-left: auto;
}
</style>
