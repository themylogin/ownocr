import { computed, ref } from 'vue'

// The top bar and the list are different components that share one selection.
// It lives here, like the stage list, instead of being passed between them.
const selected = ref<Set<number>>(new Set())

// A selection belongs to the tab it was made in. Leaving the tab drops it.
let owner = ''

const count = computed(() => selected.value.size)

function has(id: number): boolean {
  return selected.value.has(id)
}

function toggle(id: number, on: boolean) {
  const next = new Set(selected.value)
  if (on) {
    next.add(id)
  } else {
    next.delete(id)
  }
  selected.value = next
}

function selectAll(ids: number[]) {
  selected.value = new Set(ids)
}

function clear() {
  selected.value = new Set()
}

function useIn(stage: string) {
  if (owner !== stage) {
    owner = stage
    clear()
  }
}

// A book that leaves the ready group can no longer take an action, so it must
// not stay selected.
function keepOnly(ids: number[]) {
  const allowed = new Set(ids)
  const next = new Set([...selected.value].filter((id) => allowed.has(id)))
  if (next.size !== selected.value.size) {
    selected.value = next
  }
}

export function useSelection() {
  return { selected, count, has, toggle, selectAll, clear, useIn, keepOnly }
}
