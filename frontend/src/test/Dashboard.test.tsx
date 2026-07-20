import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { act, render, screen, waitFor, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from '../App'
import * as client from '../api/client'
import type { UsageResponse } from '../domain/types'

function mockMatchMedia(matches: boolean) {
  const mql = {
    matches,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }
  vi.stubGlobal('matchMedia', () => mql)
  return mql
}

const mockProviders: UsageResponse[] = [
  {
    platform: 'kimi',
    account_alias: 'kimi',
    quotas: {
      '5H': { used: 50, total: 500, reset_at: '2025-01-01T05:00:00Z' },
      '1W': { used: 2000, total: 10000, reset_at: '2025-01-06T00:00:00Z' },
      '1M': { used: 8000, total: 50000, reset_at: '2025-02-01T00:00:00Z' },
    },
    last_sync: '2025-01-01T12:00:00Z',
    version: 42,
    status: 'healthy',
  },
  {
    platform: 'kimi',
    account_alias: 'work',
    quotas: {
      '5H': { used: 450, total: 500, reset_at: '2025-01-01T05:00:00Z' },
      '1W': { used: 9500, total: 10000, reset_at: '2025-01-06T00:00:00Z' },
      '1M': { used: 49000, total: 50000, reset_at: '2025-02-01T00:00:00Z' },
    },
    last_sync: '2025-01-01T11:55:00Z',
    version: 7,
    status: 'warning',
  },
]

describe('Dashboard', () => {
  beforeEach(() => {
    mockMatchMedia(false)
    vi.useFakeTimers({ shouldAdvanceTime: true })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    vi.useRealTimers()
    cleanup()
  })

  it('shows skeletons during initial loading', () => {
    vi.spyOn(client, 'fetchUsage').mockImplementation(() => new Promise(() => {}))

    render(<App />)

    expect(screen.getByTestId('loading-skeleton')).toBeInTheDocument()
    const hiddenSkeletons = screen.getAllByText('', { selector: '.md3-skeleton' })
    for (const item of hiddenSkeletons) {
      expect(item).toHaveAttribute('aria-hidden', 'true')
    }
    expect(screen.queryByTestId('empty-state')).not.toBeInTheDocument()
  })

  it('renders provider cards with status chips and quota blocks on success', async () => {
    vi.spyOn(client, 'fetchUsage').mockResolvedValue(mockProviders)

    render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-kimi:kimi')).toBeInTheDocument()
    })

    expect(screen.getByTestId('provider-card-kimi:work')).toBeInTheDocument()

    expect(screen.getByText('Healthy')).toBeInTheDocument()
    expect(screen.getByText('Warning')).toBeInTheDocument()

    expect(screen.getAllByTestId('quota-5H')).toHaveLength(2)
    expect(screen.getAllByTestId('quota-1W')).toHaveLength(2)
    expect(screen.getAllByTestId('quota-1M')).toHaveLength(2)
  })

  it('renders providers in deterministic alphabetical order', async () => {
    vi.spyOn(client, 'fetchUsage').mockResolvedValue([mockProviders[1], mockProviders[0]])

    render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-kimi:kimi')).toBeInTheDocument()
    })

    const providerCards = screen.getAllByTestId(/provider-card-/)
    expect(providerCards[0]).toHaveAttribute('data-testid', 'provider-card-kimi:kimi')
    expect(providerCards[1]).toHaveAttribute('data-testid', 'provider-card-kimi:work')
  })

  it('shows error banner with retry button on fetch failure', async () => {
    vi.spyOn(client, 'fetchUsage').mockRejectedValue(new Error('Network error'))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })

    expect(screen.getByTestId('error-banner')).toHaveTextContent('Network error')
    expect(screen.getByTestId('error-banner')).toHaveAttribute('role', 'alert')
    expect(screen.getByTestId('retry-button')).toBeInTheDocument()
  })

  it('refresh controls expose accessible label', async () => {
    vi.spyOn(client, 'fetchUsage').mockResolvedValue(mockProviders)

    render(<App />)

    const fab = await screen.findByRole('button', { name: 'Refresh quota data' })
    expect(fab).toHaveAccessibleName('Refresh quota data')
  })

  it('refresh FAB is disabled while loading', async () => {
    vi.spyOn(client, 'fetchUsage').mockImplementation(() => new Promise(() => {}))

    render(<App />)

    const fab = screen.getByRole('button', { name: 'Refresh quota data' })
    expect(fab).toBeDisabled()
  })

  it('shows empty state when no providers returned', async () => {
    vi.spyOn(client, 'fetchUsage').mockResolvedValue([])

    render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    })

    expect(screen.getByText('No providers available')).toBeInTheDocument()
  })

  it('shows empty state when only initializing placeholder is returned', async () => {
    const initializing: UsageResponse[] = [
      {
        platform: '',
        account_alias: '',
        quotas: {},
        last_sync: '',
        version: 0,
      },
    ]
    vi.spyOn(client, 'fetchUsage').mockResolvedValue(initializing)

    render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    })
  })

  it('falls back to 5-second polling and cleans up on unmount', async () => {
    vi.stubGlobal('EventSource', undefined)
    vi.stubGlobal('WebSocket', undefined)

    const fetchSpy = vi.spyOn(client, 'fetchUsage').mockResolvedValue(mockProviders)

    const { unmount } = render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-kimi:kimi')).toBeInTheDocument()
    })

    expect(screen.getByTestId('header-refresh-status')).toHaveTextContent('POLLING (retrying)')

    expect(fetchSpy.mock.calls.length).toBeGreaterThanOrEqual(1)

    act(() => {
      vi.advanceTimersByTime(5_000)
    })

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledTimes(2)
    })

    act(() => {
      vi.advanceTimersByTime(5_000)
    })

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledTimes(3)
    })

    unmount()
    act(() => {
      vi.advanceTimersByTime(5_000)
    })

    expect(fetchSpy).toHaveBeenCalledTimes(3)

    vi.unstubAllGlobals()
  })

  it('keeps the last successful timestamp and marks it stale after a failed refresh', async () => {
    const fetchSpy = vi.spyOn(client, 'fetchUsage')
      .mockResolvedValueOnce(mockProviders)
      .mockRejectedValueOnce(new Error('Network error'))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-kimi:kimi')).toBeInTheDocument()
    })

    expect(screen.getByTestId('header-last-sync')).toHaveTextContent('Updated')
    expect(screen.getByTestId('header-last-sync')).not.toHaveTextContent('just now')

    await userEvent.click(screen.getByRole('button', { name: 'Refresh quota data' }))

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledTimes(2)
    })

    await waitFor(() => {
      expect(screen.getByTestId('header-last-sync')).toHaveTextContent('Last successful update')
    })
  })

  it('manual refresh triggers fetch and updates timestamp after success', async () => {
    const fetchSpy = vi.spyOn(client, 'fetchUsage').mockResolvedValue(mockProviders)

    render(<App />)

    await waitFor(() => {
      expect(screen.getByTestId('provider-card-kimi:kimi')).toBeInTheDocument()
    })

    expect(screen.getByTestId('header-last-sync')).toBeInTheDocument()
    expect(fetchSpy).toHaveBeenCalledTimes(1)

    await userEvent.click(screen.getByRole('button', { name: 'Refresh quota data' }))

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledTimes(2)
    })

    await waitFor(() => {
      expect(screen.getByTestId('header-last-sync')).toBeInTheDocument()
    })
  })
})
