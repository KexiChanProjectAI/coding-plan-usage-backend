import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useRealtimeRefresh } from './use-realtime-refresh'

describe('useRealtimeRefresh', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('falls back to polling when EventSource and WebSocket are unavailable', async () => {
    const onRefresh = vi.fn()
    const onStateChange = vi.fn()

    vi.stubGlobal('EventSource', undefined)
    vi.stubGlobal('WebSocket', undefined)

    renderHook(() => useRealtimeRefresh(onRefresh, onStateChange))

    await waitFor(() => {
      expect(onStateChange).toHaveBeenCalledWith(
        expect.objectContaining({ transport: 'polling', isConnected: false }),
      )
    })

    act(() => {
      vi.advanceTimersByTime(5_000)
    })
    expect(onRefresh).toHaveBeenCalledTimes(1)

    act(() => {
      vi.advanceTimersByTime(5_000)
    })
    expect(onRefresh).toHaveBeenCalledTimes(2)

    vi.unstubAllGlobals()
  })

  it('uses SSE when EventSource connects and calls refresh on message', async () => {
    const onRefresh = vi.fn()
    const onStateChange = vi.fn()

    const mockEs = {
      close: vi.fn(),
      onopen: null as ((ev: Event) => void) | null,
      onmessage: null as ((ev: MessageEvent) => void) | null,
      onerror: null as ((ev: Event) => void) | null,
    }

    class FakeEventSource {
      onopen: ((ev: Event) => void) | null = null
      onmessage: ((ev: MessageEvent) => void) | null = null
      onerror: ((ev: Event) => void) | null = null
      constructor() {
        setTimeout(() => {
          mockEs.onopen = this.onopen
          mockEs.onmessage = this.onmessage
          mockEs.onerror = this.onerror
          this.onopen?.(new Event('open'))
        }, 0)
      }
      close() {
        mockEs.close()
      }
    }

    vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource)
    vi.stubGlobal('WebSocket', undefined)

    renderHook(() => useRealtimeRefresh(onRefresh, onStateChange))

    await waitFor(() => {
      expect(onStateChange).toHaveBeenCalledWith(
        expect.objectContaining({ transport: 'sse', isConnected: true }),
      )
    })

    mockEs.onmessage?.(new MessageEvent('message', { data: '{}' }))
    expect(onRefresh).toHaveBeenCalledTimes(1)

    vi.unstubAllGlobals()
  })

  it('falls back from SSE to WS on error', async () => {
    const onRefresh = vi.fn()
    const onStateChange = vi.fn()

    const mockEs = {
      close: vi.fn(),
      onopen: null as ((ev: Event) => void) | null,
      onmessage: null as ((ev: MessageEvent) => void) | null,
      onerror: null as ((ev: Event) => void) | null,
    }

    class FakeEventSource {
      onopen: ((ev: Event) => void) | null = null
      onmessage: ((ev: MessageEvent) => void) | null = null
      onerror: ((ev: Event) => void) | null = null
      constructor() {
        setTimeout(() => {
          mockEs.onopen = this.onopen
          mockEs.onmessage = this.onmessage
          mockEs.onerror = this.onerror
          this.onerror?.(new Event('error'))
        }, 0)
      }
      close() {
        mockEs.close()
      }
    }

    const mockWs = {
      close: vi.fn(),
      send: vi.fn(),
      onopen: null as ((ev: Event) => void) | null,
      onmessage: null as ((ev: MessageEvent) => void) | null,
      onclose: null as ((ev: CloseEvent) => void) | null,
      onerror: null as ((ev: Event) => void) | null,
    }

    class FakeWebSocket {
      onopen: ((ev: Event) => void) | null = null
      onmessage: ((ev: MessageEvent) => void) | null = null
      onclose: ((ev: CloseEvent) => void) | null = null
      onerror: ((ev: Event) => void) | null = null
      constructor() {
        setTimeout(() => {
          mockWs.onopen = this.onopen
          mockWs.onmessage = this.onmessage
          mockWs.onclose = this.onclose
          mockWs.onerror = this.onerror
          this.onopen?.(new Event('open'))
        }, 0)
      }
      close() {
        mockWs.close()
      }
    }

    vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource)
    vi.stubGlobal('WebSocket', FakeWebSocket as unknown as typeof WebSocket)

    renderHook(() => useRealtimeRefresh(onRefresh, onStateChange))

    await waitFor(() => {
      expect(onStateChange).toHaveBeenCalledWith(
        expect.objectContaining({ transport: 'ws', isConnected: true }),
      )
    })

    vi.unstubAllGlobals()
  })

  it('falls back from WS to polling on close', async () => {
    const onRefresh = vi.fn()
    const onStateChange = vi.fn()

    vi.stubGlobal('EventSource', undefined)

    const mockWs = {
      close: vi.fn(),
      send: vi.fn(),
      onopen: null as ((ev: Event) => void) | null,
      onmessage: null as ((ev: MessageEvent) => void) | null,
      onclose: null as ((ev: CloseEvent) => void) | null,
      onerror: null as ((ev: Event) => void) | null,
    }

    class FakeWebSocket {
      onopen: ((ev: Event) => void) | null = null
      onmessage: ((ev: MessageEvent) => void) | null = null
      onclose: ((ev: CloseEvent) => void) | null = null
      onerror: ((ev: Event) => void) | null = null
      constructor() {
        setTimeout(() => {
          mockWs.onopen = this.onopen
          mockWs.onmessage = this.onmessage
          mockWs.onclose = this.onclose
          mockWs.onerror = this.onerror
          this.onclose?.(new CloseEvent('close'))
        }, 0)
      }
      close() {
        mockWs.close()
      }
    }

    vi.stubGlobal('WebSocket', FakeWebSocket as unknown as typeof WebSocket)

    renderHook(() => useRealtimeRefresh(onRefresh, onStateChange))

    await waitFor(() => {
      expect(onStateChange).toHaveBeenCalledWith(
        expect.objectContaining({ transport: 'polling', isConnected: false }),
      )
    })

    vi.unstubAllGlobals()
  })
})
