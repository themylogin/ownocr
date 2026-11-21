<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NAlert, NButton, NSpin, useThemeVars } from 'naive-ui'
import { api, type Book } from '../api/client'
import { titleOf } from '../composables/useStages'
import AppModal from './AppModal.vue'

const props = defineProps<{ bookId: number; index: number }>()
const emit = defineEmits<{ close: []; deleted: [] }>()

const themeVars = useThemeVars()

const doomed = ref<Book[]>([])
const allowed = ref(false)
const refusal = ref('')
const loading = ref(true)
const sending = ref(false)
const error = ref('')

const count = computed(() => doomed.value.length)

onMounted(async () => {
  try {
    const answer = await api.pageDeletion(props.bookId, props.index)
    doomed.value = answer.books
    allowed.value = answer.allowed
    refusal.value = answer.reason
  } catch (cause) {
    error.value = message(cause)
  } finally {
    loading.value = false
  }
})

function message(cause: unknown) {
  return cause instanceof Error ? cause.message : String(cause)
}

async function submit() {
  sending.value = true
  error.value = ''
  try {
    await api.deletePage(props.bookId, props.index)
    emit('deleted')
    emit('close')
  } catch (cause) {
    error.value = message(cause)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <AppModal title="Delete this page?" :busy="sending" @close="emit('close')">
    <div class="dialog" data-testid="page-delete-dialog">
      <n-spin v-if="loading" size="small" />

      <template v-else-if="count">
        <p data-testid="page-delete-count">
          This removes the page from {{ count }} {{ count === 1 ? 'book' : 'books' }}.
          It cannot be undone.
        </p>

        <ol class="books" data-testid="page-delete-list">
          <li v-for="entry in doomed" :key="entry.id">
            {{ entry.name }}
            <span class="stage">{{ titleOf(entry.stage) }}</span>
          </li>
        </ol>
      </template>

      <n-alert
        v-if="refusal"
        type="warning"
        data-testid="page-delete-refusal"
        :bordered="false"
      >
        {{ refusal }}
      </n-alert>

      <n-alert v-if="error" type="error" data-testid="page-delete-error" :bordered="false">
        {{ error }}
      </n-alert>

      <div class="actions">
        <n-button tertiary :disabled="sending" @click="emit('close')">Cancel</n-button>
        <n-button
          type="error"
          data-testid="page-delete-confirm"
          :disabled="loading || sending || !allowed || !count"
          :loading="sending"
          @click="submit"
        >
          Delete
        </n-button>
      </div>
    </div>
  </AppModal>
</template>

<style scoped>
.dialog {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

p {
  margin: 0;
}

.books {
  margin: 0;
  padding-left: 1.25rem;
  max-height: 9rem;
  overflow-y: auto;
  font-size: 0.9rem;
}

.stage {
  color: v-bind('themeVars.textColor3');
  font-size: 0.8rem;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.25rem;
}
</style>
