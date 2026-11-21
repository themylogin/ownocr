import { ref } from 'vue'
import { api, type Action, type Stage } from '../api/client'

const stages = ref<Stage[]>([])

// The stage of the book the user is looking at. A book view has no stage in its
// route, so it reports its own stage here for the tab bar to highlight.
const bookStage = ref<string | null>(null)

let pending: Promise<void> | null = null

export function ensureStages(): Promise<void> {
  if (!pending) {
    pending = api.listStages().then((loaded) => {
      stages.value = loaded
    })
  }
  return pending
}

// The actions a stage offers are the ones that lead to the tabs on its right.
// The server decides which those are, so a new stage needs no change here.
export function actionsFor(stage: string): Action[] {
  return stages.value.find((entry) => entry.id === stage)?.actions ?? []
}

export function titleOf(stage: string): string {
  return stages.value.find((entry) => entry.id === stage)?.title ?? stage
}

export function useStages() {
  return { stages, bookStage, ensureStages, actionsFor, titleOf }
}
