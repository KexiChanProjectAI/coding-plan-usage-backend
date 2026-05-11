import { describe, expect, it } from 'vitest'
import { formatLastSyncTimestamp, formatRelativeLastSync, formatResetTime, UNLIMITED_LABEL } from './format'

describe('formatLastSyncTimestamp', () => {
  it('returns never when null', () => {
    expect(formatLastSyncTimestamp(null)).toBe('never')
  })

  it('returns formatted timestamp for valid date', () => {
    const lastSync = new Date('2026-01-01T15:30:00.000Z')
    const expected = new Intl.DateTimeFormat('en-US', {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(lastSync)

    expect(formatLastSyncTimestamp(lastSync, 'en-US')).toBe(expected)
  })
})

describe('formatRelativeLastSync', () => {
  const now = new Date('2026-01-01T00:00:00.000Z')

  it('returns never when null', () => {
    expect(formatRelativeLastSync(null, now)).toBe('never')
  })

  it('returns just now for less than 30 seconds', () => {
    const lastSync = new Date('2025-12-31T23:59:40.000Z')
    expect(formatRelativeLastSync(lastSync, now)).toBe('just now')
  })

  it('returns minutes ago', () => {
    const lastSync = new Date('2025-12-31T23:59:00.000Z')
    expect(formatRelativeLastSync(lastSync, now)).toBe('1m ago')
  })

  it('returns hours ago', () => {
    const lastSync = new Date('2025-12-31T23:00:00.000Z')
    expect(formatRelativeLastSync(lastSync, now)).toBe('1h ago')
  })

  it('returns days ago', () => {
    const lastSync = new Date('2025-12-31T00:00:00.000Z')
    expect(formatRelativeLastSync(lastSync, now)).toBe('1d ago')
  })
})

describe('formatResetTime', () => {
  it('returns unlimited label when null', () => {
    expect(formatResetTime(null)).toBe(UNLIMITED_LABEL)
  })

  it('returns unlimited label for sentinel reset time', () => {
    const resetAt = new Date('0001-01-01T00:00:00.000Z')

    expect(formatResetTime(resetAt)).toBe(UNLIMITED_LABEL)
  })

  it('returns formatted date for valid reset time', () => {
    const resetAt = new Date('2026-01-01T15:30:00.000Z')
    const expected = new Intl.DateTimeFormat('en-US', {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(resetAt)

    expect(formatResetTime(resetAt, 'en-US')).toBe(expected)
  })
})
