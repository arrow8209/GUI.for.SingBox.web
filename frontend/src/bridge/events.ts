type EventHandler = (...args: any[]) => void

const handlers = new Map<string, Set<EventHandler>>()
const pendingSubscriptions = new Set<string>()

let socket: WebSocket | null = null
let reconnectTimer = 1000
let manualClose = false
const messageQueue: string[] = []

import { useAuthStore } from '@/stores/auth'

const apiBaseURL = import.meta.env.VITE_API_BASE || '/api'
const apiUrl = new URL(apiBaseURL, window.location.origin)
const BASE_WS_URL = `${apiUrl.protocol === 'https:' ? 'wss:' : 'ws:'}//${apiUrl.host}/ws`

const getWsUrl = () => {
  const token = useAuthStore().token
  if (!token) return ''
  const query = `?token=${encodeURIComponent(token)}`
  return `${BASE_WS_URL}${query}`
}

function connect() {
  const wsUrl = getWsUrl()
  if (!wsUrl) {
    return
  }
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    return
  }

  manualClose = false
  socket = new WebSocket(wsUrl)

  socket.onopen = () => {
    reconnectTimer = 1000
    for (const msg of messageQueue.splice(0, messageQueue.length)) {
      socket?.send(msg)
    }
    for (const event of handlers.keys()) {
      pendingSubscriptions.add(event)
    }
    flushSubscriptions()
  }

  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      const callbacks = handlers.get(data.event)
      callbacks?.forEach((cb) => cb(...(data.payload || [])))
    } catch (err) {
      console.warn('Invalid WS payload', err)
    }
  }

  socket.onclose = () => {
    socket = null
    if (!manualClose) {
      setTimeout(connect, reconnectTimer)
      reconnectTimer = Math.min(reconnectTimer * 2, 10_000)
    }
  }

  socket.onerror = () => {
    socket?.close()
  }
}

function flushSubscriptions() {
  if (!socket || socket.readyState !== WebSocket.OPEN) return
  for (const event of pendingSubscriptions) {
    pendingSubscriptions.delete(event)
    const hasHandlers = handlers.get(event)?.size
    if (hasHandlers) {
      socket.send(JSON.stringify({ action: 'subscribe', event }))
    }
  }
}

function sendMessage(msg: any) {
  const payload = JSON.stringify(msg)
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(payload)
  } else {
    messageQueue.push(payload)
  }
}

export function EventsOn(event: string, handler: EventHandler) {
  connect()
  if (!handlers.has(event)) {
    handlers.set(event, new Set())
    pendingSubscriptions.add(event)
    flushSubscriptions()
  }
  handlers.get(event)!.add(handler)
  return () => EventsOff(event, handler)
}

export function EventsOff(event: string, handler?: EventHandler) {
  const set = handlers.get(event)
  if (!set) return
  if (handler) {
    set.delete(handler)
  } else {
    set.clear()
  }
  if (set.size === 0) {
    handlers.delete(event)
    sendMessage({ action: 'unsubscribe', event })
  }
}

export function EventsEmit(event: string, ...payload: any[]) {
  connect()
  sendMessage({ action: 'emit', event, payload })
}

export function closeEvents() {
  manualClose = true
  socket?.close()
  socket = null
}
