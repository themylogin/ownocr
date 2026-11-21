<script setup lang="ts">
import { computed, ref } from 'vue'
import { NAlert, NButton, NRadio, NRadioGroup } from 'naive-ui'
import type { OcrProfile } from '../api/client'
import AppModal from './AppModal.vue'

const props = defineProps<{ count: number; busy?: boolean; error?: string }>()
const emit = defineEmits<{ close: []; start: [profile: OcrProfile] }>()

// Normal is the common case, so it opens selected. Extra costs minutes a page
// and is the answer for a book the normal model cannot read.
const profile = ref<OcrProfile>('v6')

const choices: { value: OcrProfile; title: string; engine: string; note: string }[] = [
  {
    value: 'v6',
    title: 'Normal',
    engine: 'PP-OCRv6',
    note: 'Frontier OCR model.',
  },
  {
    value: 'vl',
    title: 'Extra',
    engine: 'PaddleOCR-VL',
    note: 'Visual model. Use for difficult cases like polytonic Greek.',
  },
]

const subject = computed(() =>
  props.count === 1 ? 'this book' : `these ${props.count} books`,
)
</script>

<template>
  <AppModal title="Read the text off the pages" :busy="busy" @close="emit('close')">
    <div class="dialog" data-testid="ocr-dialog">
      <p data-testid="ocr-dialog-count">Choose the model to read {{ subject }} with.</p>

      <n-radio-group v-model:value="profile" :disabled="busy">
        <div class="choices">
          <n-radio
            v-for="choice in choices"
            :key="choice.value"
            :value="choice.value"
            :data-testid="`ocr-profile-${choice.value}`"
          >
            <span class="title">{{ choice.title }}</span>
            <span class="engine">{{ choice.engine }}</span>
            <span class="note">{{ choice.note }}</span>
          </n-radio>
        </div>
      </n-radio-group>

      <n-alert v-if="error" type="error" :show-icon="false" data-testid="ocr-dialog-error">
        {{ error }}
      </n-alert>

      <div class="controls">
        <n-button :disabled="busy" data-testid="ocr-cancel" @click="emit('close')">
          Cancel
        </n-button>
        <n-button
          type="primary"
          :loading="busy"
          :disabled="busy"
          data-testid="ocr-start"
          @click="emit('start', profile)"
        >
          Read
        </n-button>
      </div>
    </div>
  </AppModal>
</template>

<style scoped>
.dialog {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.choices {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.choices :deep(.n-radio__label) {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0 0.5rem;
}

.title {
  font-weight: 600;
}

.engine {
  opacity: 0.7;
  font-size: 0.875rem;
}

.note {
  grid-column: 1 / -1;
  opacity: 0.7;
  font-size: 0.875rem;
}

.controls {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
}
</style>
