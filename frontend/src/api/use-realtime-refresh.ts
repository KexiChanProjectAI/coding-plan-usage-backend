import { useEffect, useRef, useState } from 'react'

import { normalizeProviders } from '../domain/normalize'
import type { FetchState, NormalizedProvider, UsageResponse } from '../domain/types'
import { fetchUsage } from './client'

export type RefreshTransport = 'sse' | 'ws' | 'polling' | null

export interface RealtimeRefreshState {
  transport: RefreshTransport
  isConnected: boolean
}

export interface DashboardRealtimeState {
  state: FetchState<NormalizedProvider[]>
  refreshState: RealtimeRefreshState
  lastSuccessAt: Date | null
  lastFetchSucceeded: boolean
}

interface StreamBatchMessage {
  type?: string
  data?: UsageResponse[]
}

interface RealtimeDataFetcherOptions {
  fetchUsageImpl?: typeof fetchUsage
  eventSourceFactory?: (url: string) => EventSource
  webSocketFactory?: (url: string) => WebSocket
}

const API_BASE_URL = import.meta.env.VITE_QUOTAHUB_API_URL ?? ''
const POLLING_INTERVAL_MS = 5_000
const SSE_FALLBACK_DELAY_MS = 2_000

const INITIAL_STATE: DashboardRealtimeState = {
  state: {
    status: 'idle',
    data: null,
    error: null,
  },
  refreshState: {
    transport: null,
    isConnected: false,
  },
  lastSuccessAt: null,
  lastFetchSucceeded: true,
}

function getSseUrl(baseUrl = API_BASE_URL): string {
  return `${baseUrl}/api/v1/stream`
}

function getWsUrl(baseUrl = API_BASE_URL): string {
  const url = new URL('/ws', baseUrl || window.location.origin)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

export class RealtimeDataFetcher {
  private readonly listeners = new Set<() => void>()
  private readonly fetchUsageImpl: typeof fetchUsage
  private readonly eventSourceFactory: (url: string) => EventSource
  private readonly webSocketFactory: (url: string) => WebSocket

  private state: DashboardRealtimeState = INITIAL_STATE
  private started = false
  private roundId = 0
  private fetchController: AbortController | null = null
  private pollingTimer: ReturnType<typeof window.setInterval> | null = null
  private sseDelayTimer: ReturnType<typeof window.setTimeout> | null = null
  private restartTimer: ReturnType<typeof window.setTimeout> | null = null
  private socket: WebSocket | null = null
  private eventSource: EventSource | null = null
  private winner: Exclude<RefreshTransport, 'polling' | null> | null = null

  constructor(options: RealtimeDataFetcherOptions = {}) {
    this.fetchUsageImpl = options.fetchUsageImpl ?? fetchUsage
    this.eventSourceFactory = options.eventSourceFactory ?? ((url) => new window.EventSource(url))
    this.webSocketFactory = options.webSocketFactory ?? ((url) => new window.WebSocket(url))
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  getState(): DashboardRealtimeState {
    return this.state
  }

  start() {
    if (this.started) {
      return
    }

    this.started = true
    this.beginAcquisitionRound(true)
  }

  stop() {
    this.started = false
    this.roundId += 1
    this.fetchController?.abort()
    this.fetchController = null
    this.clearPolling()
    this.clearTimers()
    this.closeSocket()
    this.closeEventSource()
  }

  async refresh() {
    await this.performFetch(true)
  }

  private emit() {
    for (const listener of this.listeners) {
      listener()
    }
  }

  private patchState(patch: Partial<DashboardRealtimeState>) {
    this.state = {
      ...this.state,
      ...patch,
    }
    this.emit()
  }

  private clearPolling() {
    if (this.pollingTimer !== null) {
      window.clearInterval(this.pollingTimer)
      this.pollingTimer = null
    }
  }

  private clearTimers() {
    if (this.sseDelayTimer !== null) {
      window.clearTimeout(this.sseDelayTimer)
      this.sseDelayTimer = null
    }
    if (this.restartTimer !== null) {
      window.clearTimeout(this.restartTimer)
      this.restartTimer = null
    }
  }

  private closeSocket() {
    const socket = this.socket
    this.socket = null
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onerror = null
      socket.onclose = null
      socket.close()
    }
  }

  private closeEventSource() {
    const eventSource = this.eventSource
    this.eventSource = null
    if (eventSource) {
      eventSource.onopen = null
      eventSource.onmessage = null
      eventSource.onerror = null
      eventSource.close()
    }
  }

  private beginAcquisitionRound(includeBootstrapFetch: boolean) {
    if (!this.started) {
      return
    }

    this.roundId += 1
    const roundId = this.roundId
    this.winner = null
    this.clearPolling()
    this.clearTimers()
    this.closeSocket()
    this.closeEventSource()
    this.patchState({
      refreshState: { transport: 'polling', isConnected: false },
    })

    if (includeBootstrapFetch) {
      void this.performFetch(true)
    }

    this.pollingTimer = window.setInterval(() => {
      void this.performFetch(false)
    }, POLLING_INTERVAL_MS)

    this.startWebSocket(roundId)
    this.sseDelayTimer = window.setTimeout(() => {
      if (this.roundId !== roundId || this.winner !== null) {
        return
      }
      this.startSse(roundId)
    }, SSE_FALLBACK_DELAY_MS)
  }

  private async performFetch(markLoading: boolean) {
    this.fetchController?.abort()
    const controller = new AbortController()
    this.fetchController = controller

    if (markLoading) {
      this.patchState({
        state: {
          ...this.state.state,
          status: 'loading',
          error: null,
        },
      })
    }

    try {
      const payload = await this.fetchUsageImpl(controller.signal)
      if (controller.signal.aborted || this.fetchController !== controller) {
        return
      }
      this.applyUsage(payload)
    } catch (error) {
      if (controller.signal.aborted || this.fetchController !== controller) {
        return
      }

      this.patchState({
        state: {
          status: 'error',
          data: null,
          error: error instanceof Error ? error : new Error('Unknown fetch error'),
        },
        lastFetchSucceeded: false,
      })
    }
  }

  private applyUsage(payload: UsageResponse[]) {
    this.patchState({
      state: {
        status: 'success',
        data: normalizeProviders(payload),
        error: null,
      },
      lastSuccessAt: new Date(),
      lastFetchSucceeded: true,
    })
  }

  private startWebSocket(roundId: number) {
    if (typeof window.WebSocket === 'undefined') {
      return
    }

    try {
      const socket = this.webSocketFactory(getWsUrl())
      this.socket = socket
      socket.onopen = () => {
        if (this.roundId !== roundId || this.winner !== null) {
          socket.close()
          return
        }

        this.winner = 'ws'
        this.clearPolling()
        if (this.sseDelayTimer !== null) {
          window.clearTimeout(this.sseDelayTimer)
          this.sseDelayTimer = null
        }
        this.closeEventSource()
        this.patchState({
          refreshState: { transport: 'ws', isConnected: true },
        })
      }
      socket.onmessage = (event) => {
        if (this.roundId !== roundId || this.winner !== 'ws') {
          return
        }
        this.handleRealtimePayload(event.data)
      }
      socket.onerror = () => {
        socket.close()
      }
      socket.onclose = () => {
        if (this.roundId !== roundId) {
          return
        }

        if (this.winner === 'ws') {
          this.winner = null
          this.patchState({
            refreshState: { transport: 'ws', isConnected: false },
          })
          this.scheduleRestart()
        }
      }
    } catch {
      this.socket = null
    }
  }

  private startSse(roundId: number) {
    if (typeof window.EventSource === 'undefined') {
      return
    }

    try {
      const eventSource = this.eventSourceFactory(getSseUrl())
      this.eventSource = eventSource
      eventSource.onopen = () => {
        if (this.roundId !== roundId || this.winner !== null) {
          eventSource.close()
          return
        }

        this.winner = 'sse'
        this.clearPolling()
        this.closeSocket()
        this.patchState({
          refreshState: { transport: 'sse', isConnected: true },
        })
      }
      eventSource.onmessage = (event) => {
        if (this.roundId !== roundId || this.winner !== 'sse') {
          return
        }
        this.handleRealtimePayload(event.data)
      }
      eventSource.onerror = () => {
        if (this.roundId !== roundId) {
          return
        }

        eventSource.close()
        if (this.eventSource === eventSource) {
          this.eventSource = null
        }

        if (this.winner === 'sse') {
          this.winner = null
          this.patchState({
            refreshState: { transport: 'sse', isConnected: false },
          })
          this.scheduleRestart()
        }
      }
    } catch {
      this.eventSource = null
    }
  }

  private handleRealtimePayload(raw: string) {
    try {
      const parsed = JSON.parse(raw) as StreamBatchMessage | UsageResponse[]
      if (Array.isArray(parsed)) {
        this.applyUsage(parsed)
        return
      }

      if (parsed.type === 'batch' && Array.isArray(parsed.data)) {
        this.applyUsage(parsed.data)
      }
    } catch {
      // Ignore malformed realtime payloads.
    }
  }

  private scheduleRestart() {
    this.clearPolling()
    this.clearTimers()
    this.closeSocket()
    this.closeEventSource()
    this.restartTimer = window.setTimeout(() => {
      this.beginAcquisitionRound(true)
    }, 0)
  }
}

export function useRealtimeData(): DashboardRealtimeState & { refresh: () => Promise<void> } {
  const fetcherRef = useRef<RealtimeDataFetcher | null>(null)
  const [snapshot, setSnapshot] = useState<DashboardRealtimeState>(INITIAL_STATE)

  if (fetcherRef.current === null) {
    fetcherRef.current = new RealtimeDataFetcher()
  }

  useEffect(() => {
    const fetcher = fetcherRef.current
    if (!fetcher) {
      return
    }

    setSnapshot(fetcher.getState())
    const unsubscribe = fetcher.subscribe(() => {
      setSnapshot(fetcher.getState())
    })

    fetcher.start()

    return () => {
      unsubscribe()
      fetcher.stop()
    }
  }, [])

  return {
    ...snapshot,
    refresh: async () => {
      await fetcherRef.current?.refresh()
    },
  }
}
