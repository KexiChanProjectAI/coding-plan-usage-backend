import { act, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'

import type { UsageResponse } from '../domain/types'
import { RealtimeDataFetcher } from './use-realtime-refresh'

const payload: UsageResponse[] = [
  {
    platform: 'codex',
    account_alias: 'default',
    quotas: {},
    last_sync: '2025-01-01T00:00:00Z',
    version: 1,
    status: 'healthy',
  },
]

function batchMessage(data: UsageResponse[] = payload) {
  return JSON.stringify({ type: 'batch', data })
}

describe('RealtimeDataFetcher', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('starts with polling state and performs bootstrap fetch', async () => {
    const fetchUsageImpl = vi.fn().mockResolvedValue(payload)
    const fetcher = new RealtimeDataFetcher({ fetchUsageImpl })

    fetcher.start()

    await waitFor(() => {
      expect(fetcher.getState().state.status).toBe('success')
    })

    expect(fetcher.getState().refreshState).toEqual({ transport: 'polling', isConnected: false })
    expect(fetchUsageImpl).toHaveBeenCalledTimes(1)
    fetcher.stop()
  })

  it('keeps provider data stable but refreshes timestamp when incoming usage data is unchanged', async () => {
    const ws = {
      close: vi.fn(),
      onopen: null as (() => void) | null,
      onmessage: null as ((event: { data: string }) => void) | null,
      onerror: null as (() => void) | null,
      onclose: null as (() => void) | null,
    }

    const fetcher = new RealtimeDataFetcher({
      fetchUsageImpl: vi.fn().mockResolvedValue(payload),
      webSocketFactory: () => ws as unknown as WebSocket,
    })
    const listener = vi.fn()
    fetcher.subscribe(listener)

    fetcher.start()

    await waitFor(() => {
      expect(fetcher.getState().state.status).toBe('success')
    })

    const previousData = fetcher.getState().state.data
    const previousLastSuccessAt = fetcher.getState().lastSuccessAt

    listener.mockClear()
    ws.onopen?.()
    listener.mockClear()

    act(() => {
      vi.advanceTimersByTime(1_000)
    })

    ws.onmessage?.({ data: batchMessage(payload) })

    expect(listener).toHaveBeenCalledTimes(1)
    expect(fetcher.getState().state.data).toBe(previousData)
    expect(fetcher.getState().lastSuccessAt?.getTime()).toBeGreaterThan(
      previousLastSuccessAt?.getTime() ?? 0,
    )

    fetcher.stop()
  })

  it('prefers WebSocket when it connects before SSE starts', async () => {
    const ws = {
      close: vi.fn(),
      onopen: null as (() => void) | null,
      onmessage: null as ((event: { data: string }) => void) | null,
      onerror: null as (() => void) | null,
      onclose: null as (() => void) | null,
    }

    const fetcher = new RealtimeDataFetcher({
      fetchUsageImpl: vi.fn().mockResolvedValue(payload),
      webSocketFactory: () => {
        setTimeout(() => ws.onopen?.(), 0)
        return ws as unknown as WebSocket
      },
      eventSourceFactory: () => {
        throw new Error('SSE should not start when WS already won')
      },
    })

    fetcher.start()

    await waitFor(() => {
      expect(fetcher.getState().refreshState).toEqual({ transport: 'ws', isConnected: true })
    })

    act(() => {
      vi.advanceTimersByTime(2_100)
    })

    ws.onmessage?.({ data: batchMessage() })

    await waitFor(() => {
      expect(fetcher.getState().state.data?.[0]?.platform).toBe('codex')
    })

    fetcher.stop()
  })

  it('starts SSE after two seconds when WS has not connected', async () => {
    vi.stubGlobal(
      'EventSource',
      class {
        static readonly CONNECTING = 0
        static readonly OPEN = 1
        static readonly CLOSED = 2
      } as unknown as typeof EventSource,
    )

    const es = {
      close: vi.fn(),
      onopen: null as (() => void) | null,
      onmessage: null as ((event: { data: string }) => void) | null,
      onerror: null as (() => void) | null,
    }

    const fetcher = new RealtimeDataFetcher({
      fetchUsageImpl: vi.fn().mockResolvedValue(payload),
      webSocketFactory: () => ({ close: vi.fn() }) as unknown as WebSocket,
      eventSourceFactory: () => {
        setTimeout(() => es.onopen?.(), 0)
        return es as unknown as EventSource
      },
    })

    fetcher.start()

    act(() => {
      vi.advanceTimersByTime(2_100)
      vi.advanceTimersByTime(1)
    })

    await waitFor(() => {
      expect(fetcher.getState().refreshState).toEqual({ transport: 'sse', isConnected: true })
    })

    es.onmessage?.({ data: batchMessage() })
    await waitFor(() => {
      expect(fetcher.getState().state.data?.[0]?.platform).toBe('codex')
    })

    fetcher.stop()
  })

  it('restarts the full acquisition flow after the realtime winner disconnects', async () => {
    const fetchUsageImpl = vi.fn().mockResolvedValue(payload)
    let socketCount = 0
    const ws = {
      close: vi.fn(),
      onopen: null as (() => void) | null,
      onmessage: null as ((event: { data: string }) => void) | null,
      onerror: null as (() => void) | null,
      onclose: null as (() => void) | null,
    }

    const fetcher = new RealtimeDataFetcher({
      fetchUsageImpl,
      webSocketFactory: () => {
        socketCount += 1
        if (socketCount === 1) {
          setTimeout(() => ws.onopen?.(), 0)
        }
        return ws as unknown as WebSocket
      },
      eventSourceFactory: () => ({ close: vi.fn() }) as unknown as EventSource,
    })

    fetcher.start()

    await waitFor(() => {
      expect(fetcher.getState().refreshState.transport).toBe('ws')
    })

    act(() => {
      ws.onclose?.()
      vi.advanceTimersByTime(1)
    })

    await waitFor(() => {
      expect(fetchUsageImpl).toHaveBeenCalledTimes(2)
    })

    expect(socketCount).toBe(2)
    expect(fetcher.getState().refreshState).toEqual({ transport: 'polling', isConnected: false })

    fetcher.stop()
  })
})
