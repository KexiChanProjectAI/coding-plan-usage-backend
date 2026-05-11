import {
  Md3ThemeProvider,
  Md3AppShell,
  Md3Fab,
  Md3Banner,
} from './components/md3'
import { ProviderCard } from './components/ProviderCard'
import { EmptyState } from './components/EmptyState'
import { LoadingSkeleton } from './components/LoadingSkeleton'
import { formatLastSyncTimestamp } from './domain/format'
import { useRealtimeData } from './api/use-realtime-refresh'
import './App.css'

function Dashboard() {
  const { state, refreshState, lastSuccessAt, lastFetchSucceeded, refresh } = useRealtimeData()

  const refreshStatus = (() => {
    if (refreshState.transport === 'polling') {
      return 'POLLING (retrying)'
    }

    const label = refreshState.transport ? refreshState.transport.toUpperCase() : 'OFF'
    return refreshState.isConnected ? label : `${label} (reconnecting)`
  })()

  const isLoadingFirst = state.status === 'idle' || state.status === 'loading'
  const hasError = state.status === 'error'
  const providers = state.data ?? []
  const lastSyncLabel = lastFetchSucceeded ? 'Updated' : 'Last successful update'

  const runRefresh = () => {
    void refresh()
  }

  const header = (
    <>
      <h1 className="md3-app-header__title">Coding Plans</h1>
      {lastSuccessAt && (
        <span className="header-last-sync" data-testid="header-last-sync">
          {lastSyncLabel} {formatLastSyncTimestamp(lastSuccessAt)}
          {refreshStatus && (
            <span className="header-refresh-status" data-testid="header-refresh-status">
              {' '}
              · {refreshStatus}
            </span>
          )}
        </span>
      )}
    </>
  )

  const fab = (
    <Md3Fab
      aria-label="Refresh quota data"
      onClick={runRefresh}
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
              onClick={runRefresh}
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
