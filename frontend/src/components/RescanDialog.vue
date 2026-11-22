<script setup lang="ts">
import { ref } from 'vue'
import { NAlert, NButton, NRadio, NRadioGroup } from 'naive-ui'
import type { OcrProfile, RescanProfile } from '../api/client'
import AppModal from './AppModal.vue'

const props = defineProps<{
  count: number
  busy?: boolean
  error?: string
  // The profile the book was read with. It decides which model opens selected.
  bookProfile?: OcrProfile | null
}>()

const emit = defineEmits<{ close: []; start: [profile: RescanProfile] }>()

// The model that opens selected is the one after the model that read the book,
// because a user rescans a page the model it was read with read badly. A book read
// with PP-OCRv6 opens on Extra; a book already read with PaddleOCR-VL opens on
// Ultra, which is the only model left to try.
const profile = ref<RescanProfile>(props.bookProfile === 'vl' ? 'glm' : 'vl')

const choices: { value: RescanProfile; title: string; engine: string; note: string }[] = [
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
  {
    value: 'glm',
    title: 'Ultra',
    engine: 'GLM-5.3 Flash',
    note: 'Frontier visual model. Use when even PaddleOCR-VL fails.',
  },
]
</script>

<template>
  <AppModal title="Read the pages again" :busy="busy" @close="emit('close')">
    <div class="dialog" data-testid="rescan-dialog">
      <p data-testid="rescan-dialog-count">
        The pages will be processed again. Use this if some text is recognized
        incorrectly or missing.
      </p>

      <n-radio-group v-model:value="profile" :disabled="busy">
        <div class="choices">
          <n-radio
            v-for="choice in choices"
            :key="choice.value"
            :value="choice.value"
            :data-testid="`rescan-profile-${choice.value}`"
          >
            <span class="title">{{ choice.title }}</span>
            <span class="engine">{{ choice.engine }}</span>
            <span class="note">{{ choice.note }}</span>
          </n-radio>
        </div>
      </n-radio-group>

      <n-alert v-if="error" type="error" :show-icon="false" data-testid="rescan-dialog-error">
        {{ error }}
      </n-alert>

      <div class="controls">
        <n-button :disabled="busy" data-testid="rescan-cancel" @click="emit('close')">
          Cancel
        </n-button>
        <n-button
          type="warning"
          :loading="busy"
          :disabled="busy"
          data-testid="rescan-start"
          @click="emit('start', profile)"
        >
          Rescan
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
