import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useFetchState } from './fetch-state'

describe('useFetchState', () => {
  it('transitions idle -> loading -> success', async () => {
    const { result } = renderHook(() => useFetchState<string>())

    expect(result.current.state.status).toBe('idle')

    let resolveFetcher: (value: string) => void = () => {}
    const fetcher = vi.fn(
      () =>
        new Promise<string>((resolve) => {
          resolveFetcher = resolve
        }),
    )

    await act(async () => {
      void result.current.run(fetcher)
    })

    await waitFor(() => {
      expect(result.current.state.status).toBe('loading')
    })

    await act(async () => {
      resolveFetcher('ok')
    })

    await waitFor(() => {
      expect(result.current.state).toEqual({ status: 'success', data: 'ok', error: null })
    })
  })

  it('transitions idle -> loading -> error', async () => {
    const { result } = renderHook(() => useFetchState<string>())
    const boom = new Error('boom')

    await act(async () => {
      await result.current.run(async () => {
        throw boom
      })
    })

    await waitFor(() => {
      expect(result.current.state.status).toBe('error')
      expect(result.current.state.error?.message).toBe('boom')
      expect(result.current.state.data).toBeNull()
    })
  })

  it('abort/cancel cancels in-flight request and avoids success transition', async () => {
    const { result } = renderHook(() => useFetchState<string>())

    const fetcher = vi.fn((signal: AbortSignal) => {
      return new Promise<string>((resolve, reject) => {
        signal.addEventListener('abort', () => {
          reject(new DOMException('Aborted', 'AbortError'))
        })
        setTimeout(() => resolve('late-success'), 40)
      })
    })

    let pending: Promise<unknown>
    await act(async () => {
      pending = result.current.run(fetcher)
    })

    await waitFor(() => {
      expect(result.current.state.status).toBe('loading')
    })

    act(() => {
      result.current.cancel()
    })

    await pending!

    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(result.current.state.status).toBe('loading')
    expect(result.current.state.data).toBeNull()
  })

  it('reset returns to idle and clears data/error', async () => {
    const { result } = renderHook(() => useFetchState<string>())

    await act(async () => {
      await result.current.run(async () => 'value')
    })

    await waitFor(() => {
      expect(result.current.state.status).toBe('success')
    })

    act(() => {
      result.current.reset()
    })

    expect(result.current.state).toEqual({ status: 'idle', data: null, error: null })
  })
})
