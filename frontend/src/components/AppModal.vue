<script setup lang="ts">
import { NModal } from 'naive-ui'

withDefaults(defineProps<{ title: string; width?: string; busy?: boolean }>(), {
  width: 'min(30rem, 100%)',
  busy: false,
})

const emit = defineEmits<{ close: [] }>()

// Every dialog closes through this one path. Naive-ui reports a mask click for
// any click outside a modal's own body, which includes every click inside a
// modal above it, and reports esc even when a modal above it takes the key.
// Its own update:show applies both guards, so nothing else may close a modal.
function onUpdateShow(show: boolean) {
  if (!show) {
    emit('close')
  }
}
</script>

<template>
  <n-modal
    :show="true"
    preset="card"
    :title="title"
    :style="{ width, maxWidth: '100%' }"
    :mask-closable="!busy"
    :close-on-esc="!busy"
    @close="emit('close')"
    @update:show="onUpdateShow"
  >
    <slot />
  </n-modal>
</template>
