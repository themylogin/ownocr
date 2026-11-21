import { ref } from 'vue'
import { api, type ActionArguments, type Book } from '../api/client'

// The top bar runs a bulk action, the list runs one from a context menu. Both
// report through here, so the stage view shows one error and applies one
// answer whichever started it.
const error = ref('')
const started = ref<Book[]>([])

async function run(action: string, bookIds: number[], args: ActionArguments = {}) {
  if (bookIds.length === 0) {
    return
  }

  try {
    const result = await api.runAction(action, bookIds, args)
    error.value = result.failed.map((failure) => failure.error).join('; ')
    started.value = result.started
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : String(cause)
    started.value = []
  }
}

export function useActions() {
  return { error, started, run }
}
