import { describe, expect, it } from 'vitest'
import {
  calculateQuotaPercentage,
  deriveProviderKey,
  isInitializingPlaceholder,
  mapProviderStatus,
  normalizeProviders,
  quotaSeverity,
} from './normalize'
import type { UsageResponse } from './types'

describe('mapProviderStatus', () => {
  it('maps healthy/active to Healthy', () => {
    expect(mapProviderStatus('healthy')).toBe('Healthy')
    expect(mapProviderStatus('active')).toBe('Healthy')
  })

  it('maps warning/limited/degraded to Warning', () => {
    expect(mapProviderStatus('warning')).toBe('Warning')
    expect(mapProviderStatus('limited')).toBe('Warning')
    expect(mapProviderStatus('degraded')).toBe('Warning')
  })

  it('maps critical/error/down to Critical', () => {
    expect(mapProviderStatus('critical')).toBe('Critical')
    expect(mapProviderStatus('error')).toBe('Critical')
    expect(mapProviderStatus('down')).toBe('Critical')
  })

  it('maps unknown, initializing, and missing to Unknown', () => {
    expect(mapProviderStatus('initializing')).toBe('Unknown')
    expect(mapProviderStatus('unexpected')).toBe('Unknown')
    expect(mapProviderStatus(undefined)).toBe('Unknown')
  })
})

describe('quota calculations', () => {
  it('calculates 50/100 as 50 normal', () => {
    const pct = calculateQuotaPercentage(50, 100)
    expect(pct).toBe(50)
    expect(quotaSeverity(pct)).toBe('normal')
  })

  it('calculates 70/100 warning boundary', () => {
    const pct = calculateQuotaPercentage(70, 100)
    expect(pct).toBe(70)
    expect(quotaSeverity(pct)).toBe('warning')
  })

  it('calculates 90/100 critical boundary', () => {
    const pct = calculateQuotaPercentage(90, 100)
    expect(pct).toBe(90)
    expect(quotaSeverity(pct)).toBe('critical')
  })

  it('clamps over-usage to 100', () => {
    expect(calculateQuotaPercentage(150, 100)).toBe(100)
  })

  it('clamps negative used to 0', () => {
    expect(calculateQuotaPercentage(-10, 100)).toBe(0)
  })

  it('marks unavailable when total <= 0', () => {
    expect(calculateQuotaPercentage(50, 0)).toBeNull()
    expect(calculateQuotaPercentage(50, -10)).toBeNull()
  })
})

describe('key and placeholder behavior', () => {
  it('uses provider_id as primary key', () => {
    const provider: UsageResponse = {
      platform: 'codex',
      account_alias: 'main',
      quotas: {},
      last_sync: '2025-01-01T00:00:00Z',
      version: 1,
      status: 'healthy',
      provider_id: 'provider-123',
    }
    expect(deriveProviderKey(provider)).toBe('provider-123')
  })

  it('falls back to platform:account_alias and default alias', () => {
    const withAlias: UsageResponse = {
      platform: 'codex',
      account_alias: 'alt',
      quotas: {},
      last_sync: '2025-01-01T00:00:00Z',
      version: 1,
      status: 'healthy',
    }
    const withoutAlias: UsageResponse = {
      platform: 'codex',
      account_alias: '',
      quotas: {},
      last_sync: '2025-01-01T00:00:00Z',
      version: 1,
      status: 'healthy',
    }
    expect(deriveProviderKey(withAlias)).toBe('codex:alt')
    expect(deriveProviderKey(withoutAlias)).toBe('codex:default')
  })

  it('filters initializing placeholder from normalized providers', () => {
    const input: UsageResponse[] = [
      {
        platform: '',
        account_alias: '',
        last_sync: '',
        version: 0,
        status: 'initializing',
      },
      {
        platform: 'codex',
        account_alias: 'codex',
        quotas: {
          '5H': { used: 10, total: 100, reset_at: '2025-01-01T05:00:00Z' },
        },
        last_sync: '2025-01-01T12:00:00Z',
        version: 5,
        status: 'degraded',
      },
    ]

    expect(isInitializingPlaceholder(input[0])).toBe(true)
    const normalized = normalizeProviders(input)
    expect(normalized).toHaveLength(1)
    expect(normalized[0].platform).toBe('codex')
    expect(normalized[0].status).toBe('Warning')
    expect(normalized[0].quotas).toHaveLength(3)
    expect(normalized[0].quotas.map((q) => q.tier)).toEqual(['5H', '1W', '1M'])
  })

  it('backfills missing quota tiers as unavailable', () => {
    const normalized = normalizeProviders([
      {
        platform: 'kimi',
        account_alias: 'work',
        quotas: {
          '1W': { used: 80, total: 100, reset_at: '2025-01-06T00:00:00Z' },
        },
        last_sync: '2025-01-01T12:00:00Z',
        version: 3,
        status: 'healthy',
      },
    ])

    const quotas = normalized[0].quotas
    expect(quotas[0].tier).toBe('5H')
    expect(quotas[0].isUnavailable).toBe(true)
    expect(quotas[1].tier).toBe('1W')
    expect(quotas[1].isUnavailable).toBe(false)
    expect(quotas[2].tier).toBe('1M')
    expect(quotas[2].isUnavailable).toBe(true)
  })
})
