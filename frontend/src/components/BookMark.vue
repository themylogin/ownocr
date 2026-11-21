<script setup lang="ts">
import { useThemeVars } from 'naive-ui'

export type MarkKind = 'spinner' | 'warning' | 'check'

const props = withDefaults(defineProps<{ kind: MarkKind; size?: number }>(), { size: 18 })

const themeVars = useThemeVars()

const color = () => {
  if (props.kind === 'check') {
    return themeVars.value.successColor
  }
  if (props.kind === 'warning') {
    return themeVars.value.errorColor
  }
  return themeVars.value.textColor3
}
</script>

<template>
  <!-- The spinner never turns. It marks work in flight on a card that may sit
       there for an hour, and an animation that long only distracts. -->
  <svg
    class="mark"
    :class="kind"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    :stroke="color()"
    stroke-width="2.4"
    stroke-linecap="round"
    stroke-linejoin="round"
    :data-testid="`mark-${kind}`"
    aria-hidden="true"
  >
    <template v-if="kind === 'spinner'">
      <circle cx="12" cy="12" r="9" :stroke="themeVars.dividerColor" />
      <path d="M12 3a9 9 0 0 1 9 9" />
    </template>
    <template v-else-if="kind === 'warning'">
      <path d="M12 3.6 22 20H2z" />
      <path d="M12 10v4" />
      <path d="M12 17.2v.1" />
    </template>
    <template v-else>
      <path d="M4 12.5 9.5 18 20 7" />
    </template>
  </svg>
</template>

<style scoped>
.mark {
  display: block;
}

/* The spinner turns while the work runs, and only the spinner turns. */
.mark.spinner {
  animation: turn 1.1s linear infinite;
}

@keyframes turn {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .mark.spinner {
    animation: none;
  }
}
</style>
