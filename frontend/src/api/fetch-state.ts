import { useCallback, useRef, useState } from 'react'
import type { FetchState } from '../domain/types'

export function useFetchState<T>() {
  const [state, setState] = useState<FetchState<T>>({
    status: 'idle',
    data: null,
    error: null,
  })
  const controllerRef = useRef<AbortController | null>(null)

  const run = useCallback(async (fetcher: (signal: AbortSignal) => Promise<T>) => {
    controllerRef.current?.abort()
    const controller = new AbortController()
    controllerRef.current = controller

    setState((prev) => ({ ...prev, status: 'loading', error: null }))

    try {
      const data = await fetcher(controller.signal)
      if (controller.signal.aborted) {
        return null
      }
      setState({ status: 'success', data, error: null })
      return data
    } catch (error) {
      if (controller.signal.aborted) {
        return null
      }
      setState({
        status: 'error',
        data: null,
        error: error instanceof Error ? error : new Error('Unknown fetch error'),
      })
      return null
    }
  }, [])

  const reset = useCallback(() => {
    controllerRef.current?.abort()
    controllerRef.current = null
    setState({ status: 'idle', data: null, error: null })
  }, [])

  const cancel = useCallback(() => {
    controllerRef.current?.abort()
  }, [])

  return { state, run, reset, cancel }
}
