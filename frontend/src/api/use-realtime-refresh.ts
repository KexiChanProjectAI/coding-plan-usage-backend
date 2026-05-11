import { useEffect, useRef } from 'react'

export type RefreshTransport = 'sse' | 'ws' | 'polling' | null

export interface RealtimeRefreshState {
  transport: RefreshTransport
  isConnected: boolean
}

const API_BASE_URL = import.meta.env.VITE_QUOTAHUB_API_URL ?? ''
const POLLING_INTERVAL_MS = 5_000

function getSseUrl(baseUrl = API_BASE_URL): string {
  return `${baseUrl}/api/v1/stream`
}

function getWsUrl(baseUrl = API_BASE_URL): string {
  const url = new URL('/ws', baseUrl || window.location.origin)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

export function useRealtimeRefresh(
  onRefresh: () => void,
  onStateChange?: (state: RealtimeRefreshState) => void,
): RealtimeRefreshState {
  const stateRef = useRef<RealtimeRefreshState>({ transport: null, isConnected: false })
  const onRefreshRef = useRef(onRefresh)
  const onStateChangeRef = useRef(onStateChange)

  onRefreshRef.current = onRefresh
  onStateChangeRef.current = onStateChange

  useEffect(() => {
    let cancelled = false
    let eventSource: EventSource | null = null
    let socket: WebSocket | null = null
    let polling: ReturnType<typeof window.setInterval> | null = null

    const setState = (next: RealtimeRefreshState) => {
      if (cancelled) {
        return
      }
      stateRef.current = next
      onStateChangeRef.current?.(next)
    }

    const stopPolling = () => {
      if (polling !== null) {
        window.clearInterval(polling)
        polling = null
      }
    }

    const startPolling = () => {
      if (cancelled || polling !== null) {
        return
      }
      setState({ transport: 'polling', isConnected: false })
      polling = window.setInterval(() => onRefreshRef.current(), POLLING_INTERVAL_MS)
    }

    const startWs = () => {
      if (cancelled || typeof window.WebSocket === 'undefined') {
        startPolling()
        return
      }

      try {
        socket = new window.WebSocket(getWsUrl())
      } catch {
        startPolling()
        return
      }

      socket.onopen = () => setState({ transport: 'ws', isConnected: true })
      socket.onmessage = () => onRefreshRef.current()
      socket.onerror = () => socket?.close()
      socket.onclose = () => {
        if (!cancelled) {
          setState({ transport: 'ws', isConnected: false })
          startPolling()
        }
      }
    }

    const startSse = () => {
      if (cancelled || typeof window.EventSource === 'undefined') {
        startWs()
        return
      }

      try {
        eventSource = new window.EventSource(getSseUrl())
      } catch {
        startWs()
        return
      }

      eventSource.onopen = () => setState({ transport: 'sse', isConnected: true })
      eventSource.onmessage = () => onRefreshRef.current()
      eventSource.onerror = () => {
        eventSource?.close()
        eventSource = null
        setState({ transport: 'sse', isConnected: false })
        startWs()
      }
    }

    startSse()

    return () => {
      cancelled = true
      eventSource?.close()
      socket?.close()
      stopPolling()
    }
  }, [])

  return stateRef.current
}
