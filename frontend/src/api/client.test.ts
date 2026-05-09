import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchUsage, getUsageUrl } from './client'

describe('api client URL construction', () => {
  it('uses default relative path when no base URL is provided', () => {
    expect(getUsageUrl('')).toBe('/api/v1/usage')
  })

  it('uses base URL when provided', () => {
    expect(getUsageUrl('https://example.test')).toBe('https://example.test/api/v1/usage')
  })
})

describe('fetchUsage', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns typed array for successful response', async () => {
    const payload = [
      {
        platform: 'codex',
        account_alias: 'codex',
        quotas: {
          '5H': { used: 50, total: 100, reset_at: '2025-01-01T05:00:00Z' },
        },
        last_sync: '2025-01-01T12:00:00Z',
        version: 42,
        status: 'healthy',
      },
    ]

    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: vi.fn().mockResolvedValue(payload),
    } as unknown as Response)

    await expect(fetchUsage()).resolves.toEqual(payload)
  })

  it('throws on non-ok response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: vi.fn(),
    } as unknown as Response)

    await expect(fetchUsage()).rejects.toThrow('Failed to fetch usage: 503 Service Unavailable')
  })

  it('throws on invalid JSON payload type', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: vi.fn().mockResolvedValue({ not: 'array' }),
    } as unknown as Response)

    await expect(fetchUsage()).rejects.toThrow('Usage API returned invalid payload: expected array')
  })

  it('throws on JSON parse failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: vi.fn().mockRejectedValue(new Error('bad json')),
    } as unknown as Response)

    await expect(fetchUsage()).rejects.toThrow('Usage API returned invalid JSON')
  })

  it('passes abort signal and surfaces abort error', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(
      async (_input: RequestInfo | URL, init?: RequestInit) => {
        if (init?.signal?.aborted) {
          throw new DOMException('Aborted', 'AbortError')
        }
        return {
          ok: true,
          status: 200,
          statusText: 'OK',
          json: vi.fn().mockResolvedValue([]),
        } as unknown as Response
      },
    )

    const controller = new AbortController()
    controller.abort()
    await expect(fetchUsage(controller.signal)).rejects.toThrow(/Aborted|AbortError/)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/usage', { signal: controller.signal })
  })
})
