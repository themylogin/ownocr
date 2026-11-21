<script setup lang="ts">
import { NBreadcrumb, NBreadcrumbItem } from 'naive-ui'
import { useRouter, type RouteLocationRaw } from 'vue-router'

export interface Crumb {
  label: string
  to?: RouteLocationRaw
}

defineProps<{ crumbs: Crumb[] }>()

const router = useRouter()

function open(crumb: Crumb) {
  if (crumb.to) {
    router.push(crumb.to)
  }
}
</script>

<template>
  <n-breadcrumb class="trail" data-testid="breadcrumbs">
    <n-breadcrumb-item
      v-for="(crumb, index) in crumbs"
      :key="index"
      :clickable="Boolean(crumb.to)"
      :data-testid="`crumb-${index}`"
      @click="open(crumb)"
    >
      {{ crumb.label }}
    </n-breadcrumb-item>

    <!-- A note the caller hangs off the last crumb. It stands in the same list as
         the crumbs, so it shares their line and sits on their baseline with no
         alignment of its own. -->
    <slot name="note" />
  </n-breadcrumb>
</template>

<style scoped>
.trail {
  min-width: 0;
}
</style>
