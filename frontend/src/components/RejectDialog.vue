<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NAlert, NButton, NSpin, useThemeVars } from 'naive-ui'
import { api, type Book } from '../api/client'
import { titleOf } from '../composables/useStages'
import AppModal from './AppModal.vue'

const props = defineProps<{ book: Book }>()
const emit = defineEmits<{ close: []; rejected: [] }>()

const themeVars = useThemeVars()

const doomed = ref<Book[]>([])
const loading = ref(true)
const sending = ref(false)
const error = ref('')

const count = computed(() => doomed.value.length)

// The source book the chain hangs from survives, and takes its action again.
const freed = computed(() => props.book.sourceBookId !== null)

onMounted(async () => {
  try {
    doomed.value = (await api.rejection(props.book.id)).books
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
    await api.rejectBook(props.book.id)
    emit('rejected')
    emit('close')
  } catch (cause) {
    error.value = message(cause)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <AppModal title="Reject this book?" :busy="sending" @close="emit('close')">
    <div class="dialog" data-testid="reject-dialog">
      <n-spin v-if="loading" size="small" />

      <template v-else-if="!error || count">
        <p data-testid="reject-count">
          This destroys {{ count }} {{ count === 1 ? 'book' : 'books' }} and every page they hold.
          It cannot be undone.
        </p>

        <ol class="books" data-testid="reject-list">
          <li v-for="entry in doomed" :key="entry.id">
            {{ entry.name }}
            <span class="stage">{{ titleOf(entry.stage) }}</span>
          </li>
        </ol>

        <p v-if="freed" class="freed" data-testid="reject-freed">
          {{ book.name }} returns to the previous stage, ready for an action again.
        </p>
      </template>

      <n-alert v-if="error" type="error" data-testid="reject-error" :bordered="false">
        {{ error }}
      </n-alert>

      <div class="actions">
        <n-button tertiary :disabled="sending" @click="emit('close')">Cancel</n-button>
        <n-button
          type="error"
          data-testid="reject-confirm"
          :disabled="loading || sending || !count"
          :loading="sending"
          @click="submit"
        >
          Reject
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

.freed {
  color: v-bind('themeVars.textColor3');
  font-size: 0.85rem;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.25rem;
}
</style>
