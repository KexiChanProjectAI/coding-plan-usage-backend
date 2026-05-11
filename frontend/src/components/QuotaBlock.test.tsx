import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { QuotaBlock } from './QuotaBlock'
import type { NormalizedQuota } from '../domain/types'
import { UNLIMITED_LABEL } from '../domain/format'

describe('QuotaBlock', () => {
  it('uses the unlimited label consistently for unavailable quotas', () => {
    const quota: NormalizedQuota = {
      tier: '5H',
      used: 0,
      total: 0,
      percentage: null,
      severity: 'normal',
      resetAt: null,
      isUnavailable: true,
    }

    render(<QuotaBlock quota={quota} />)

    expect(screen.getAllByText(UNLIMITED_LABEL)).toHaveLength(3)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-label', '5H quota remaining: 无限制/INF')
  })
})
