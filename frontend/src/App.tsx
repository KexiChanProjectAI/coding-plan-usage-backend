import { useEffect, useState, useCallback } from 'react'
import {
  Md3ThemeProvider,
  Md3AppShell,
  Md3Fab,
  Md3Banner,
} from './components/md3'
import { ProviderCard } from './components/ProviderCard'
import { EmptyState } from './components/EmptyState'
import { LoadingSkeleton } from './components/LoadingSkeleton'
import { normalizeProviders } from './domain/normalize'
import { formatRelativeLastSync } from './domain/format'
import { fetchUsage } from './api/client'
import { useFetchState } from './api/fetch-state'
import type { NormalizedProvider } from './domain/types'
import './App.css'

const REFRESH_INTERVAL_MS = 30_000

function Dashboard() {
  const { state, run } = useFetchState<NormalizedProvider[]>()
  const [lastSuccessAt, setLastSuccessAt] = useState<Date | null>(null)
  const [now, setNow] = useState(() => new Date())

  const doFetch = useCallback(async () => {
    const data = await run(async (signal) => {
      const response = await fetchUsage(signal)
      return normalizeProviders(response)
    })
    if (data) {
      setLastSuccessAt(new Date())
    }
  }, [run])

  useEffect(() => {
    doFetch()
  }, [doFetch])

  useEffect(() => {
    const interval = setInterval(() => {
      doFetch()
    }, REFRESH_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [doFetch])

  useEffect(() => {
    const tick = setInterval(() => setNow(new Date()), 60_000)
    return () => clearInterval(tick)
  }, [])

  const isLoadingFirst = state.status === 'idle' || state.status === 'loading'
  const hasError = state.status === 'error'
  const providers = state.data ?? []

  const header = (
    <>
      <h1 className="md3-app-header__title">Coding Plans</h1>
      {lastSuccessAt && (
        <span className="header-last-sync" data-testid="header-last-sync">
          Updated {formatRelativeLastSync(lastSuccessAt, now)}
        </span>
      )}
    </>
  )

  const fab = (
    <Md3Fab
      aria-label="Refresh quota data"
      onClick={doFetch}
      disabled={state.status === 'loading'}
      className={state.status === 'loading' ? 'md3-fab--spinning' : ''}
      data-testid="refresh-fab"
    >
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        aria-hidden="true"
      >
        <path d="M23 4v6h-6" />
        <path d="M1 20v-6h6" />
        <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" />
      </svg>
    </Md3Fab>
  )

  return (
    <Md3AppShell header={header} fab={fab}>
      {hasError && (
        <Md3Banner
          open
          action={
            <button
              type="button"
              className="md3-button md3-button--outlined"
              onClick={doFetch}
              data-testid="retry-button"
            >
              Retry
            </button>
          }
          data-testid="error-banner"
        >
          {state.error?.message ?? 'Failed to fetch quota data'}
        </Md3Banner>
      )}

      <div className="dashboard-content">
        {isLoadingFirst && providers.length === 0 ? (
          <LoadingSkeleton />
        ) : providers.length === 0 ? (
          <EmptyState />
        ) : (
          <div className="provider-list">
            {providers.map((provider) => (
              <ProviderCard key={provider.key} provider={provider} />
            ))}
          </div>
        )}
      </div>
    </Md3AppShell>
  )
}

function App() {
  return (
    <Md3ThemeProvider>
      <Dashboard />
    </Md3ThemeProvider>
  )
}

export default App
