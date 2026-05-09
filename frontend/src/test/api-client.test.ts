import { describe, expect, it } from 'vitest'
import { getUsageUrl } from '../api/client'

describe('api client URL construction', () => {
  it('uses default relative path when no base URL is provided', () => {
    expect(getUsageUrl('')).toBe('/api/v1/usage')
  })

  it('uses VITE_QUOTAHUB_API_URL-style base prefix when provided', () => {
    expect(getUsageUrl('https://example.test')).toBe('https://example.test/api/v1/usage')
  })
})
