<script setup lang="ts">
import { computed, ref, type InputHTMLAttributes } from 'vue'
import {
  NAlert,
  NButton,
  NInput,
  NProgress,
  NUpload,
  NUploadDragger,
  useThemeVars,
  type UploadFileInfo,
} from 'naive-ui'
import { api } from '../api/client'
import AppModal from './AppModal.vue'

const emit = defineEmits<{ close: []; uploaded: [] }>()

const themeVars = useThemeVars()

const nameInputProps = { id: 'book-name', 'data-testid': 'upload-name' } as InputHTMLAttributes
const filesInputProps = { id: 'book-files', 'data-testid': 'upload-files' } as InputHTMLAttributes

const name = ref('')
const fileList = ref<UploadFileInfo[]>([])
const error = ref('')
const sending = ref(false)
const sent = ref(0)
const total = ref(0)

const files = computed(() =>
  fileList.value.map((entry) => entry.file).filter((file): file is File => file !== null && file !== undefined),
)

const canSubmit = computed(() => name.value.trim() !== '' && files.value.length > 0 && !sending.value)
const percent = computed(() => (total.value === 0 ? 0 : Math.round((sent.value / total.value) * 100)))

function nameFromPdf(file: File): string {
  return file.name.replace(/\.pdf$/i, '')
}

function onChange(data: { fileList: UploadFileInfo[] }) {
  fileList.value = data.fileList
  error.value = ''

  const single = files.value.length === 1 ? files.value[0] : null
  if (single && /\.pdf$/i.test(single.name) && name.value.trim() === '') {
    name.value = nameFromPdf(single)
  }
}

async function submit() {
  if (!canSubmit.value) {
    return
  }

  sending.value = true
  error.value = ''
  try {
    await api.uploadBook(name.value.trim(), files.value, (loaded, size) => {
      sent.value = loaded
      total.value = size
    })
    emit('uploaded')
    emit('close')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <AppModal title="Upload a book" :busy="sending" @close="emit('close')">
    <form class="dialog" data-testid="upload-dialog" @submit.prevent="submit">
      <n-input
        v-model:value="name"
        placeholder="Book name"
        :input-props="nameInputProps"
      />

      <n-upload
        multiple
        directory-dnd
        :default-upload="false"
        :show-file-list="false"
        :file-list="fileList"
        accept="application/pdf,image/jpeg,image/png"
        :input-props="filesInputProps"
        @change="onChange"
      >
        <n-upload-dragger>Drop a PDF or page images here, or click to choose</n-upload-dragger>
      </n-upload>

      <ol v-if="files.length" class="files" data-testid="upload-file-list">
        <li v-for="file in files" :key="file.name + file.size">{{ file.name }}</li>
      </ol>

      <n-progress
        v-if="sending"
        type="line"
        data-testid="upload-progress"
        :percentage="percent"
        :height="6"
      />
      <n-alert v-if="error" type="error" data-testid="upload-error" :bordered="false">{{ error }}</n-alert>

      <div class="actions">
        <n-button tertiary :disabled="sending" @click="emit('close')">Cancel</n-button>
        <n-button
          type="primary"
          attr-type="submit"
          data-testid="upload-submit"
          :disabled="!canSubmit"
          :loading="sending"
        >
          Upload
        </n-button>
      </div>
    </form>
  </AppModal>
</template>

<style scoped>
.dialog {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.files {
  margin: 0;
  padding-left: 1.25rem;
  max-height: 9rem;
  overflow-y: auto;
  font-size: 0.85rem;
  color: v-bind('themeVars.textColor3');
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.25rem;
}
</style>
