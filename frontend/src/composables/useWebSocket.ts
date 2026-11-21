import { getCurrentScope, onScopeDispose, ref } from 'vue'

export interface ServerEvent<T = unknown> {
  type: string
  payload?: T
}

export type ConnectionStatus = 'connecting' | 'open' | 'closed'

const initialRetryDelay = 500
const maxRetryDelay = 10_000

// One socket serves the whole app. Every view that needs live books listens to
// the same stream instead of opening a connection of its own.
const status = ref<ConnectionStatus>('connecting')
const handlers = new Set<(event: ServerEvent) => void>()

let socket: WebSocket | null = null
let retryDelay = initialRetryDelay
let started = false

function socketUrl(path: string): string {
  const url = new URL(path, window.location.href)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

function connect(path: string) {
  status.value = 'connecting'
  socket = new WebSocket(socketUrl(path))

  socket.onopen = () => {
    status.value = 'open'
    retryDelay = initialRetryDelay
  }

  socket.onmessage = (message) => {
    let event: ServerEvent
    try {
      event = JSON.parse(message.data as string) as ServerEvent
    } catch {
      return
    }
    handlers.forEach((handler) => handler(event))
  }

  socket.onclose = () => {
    socket = null
    status.value = 'closed'
    setTimeout(() => connect(path), retryDelay)
    retryDelay = Math.min(retryDelay * 2, maxRetryDelay)
  }

  socket.onerror = () => socket?.close()
}

export function useWebSocket(path = '/api/ws') {
  if (!started) {
    started = true
    connect(path)
  }

  function onEvent(handler: (event: ServerEvent) => void) {
    handlers.add(handler)
    if (getCurrentScope()) {
      onScopeDispose(() => handlers.delete(handler))
    }
  }

  return { status, onEvent }
}
