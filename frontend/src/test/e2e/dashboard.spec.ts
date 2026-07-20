import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const mockProviders = [
  {
    platform: 'kimi',
    account_alias: 'primary',
    provider_id: 'kimi-primary',
    quotas: {
      '5H': { used: 20, total: 100, reset_at: '2026-01-01T05:00:00Z' },
      '1W': { used: 200, total: 1000, reset_at: '2026-01-07T00:00:00Z' },
      '1M': { used: 600, total: 5000, reset_at: '2026-02-01T00:00:00Z' },
    },
    last_sync: '2026-01-01T00:00:00Z',
    version: 7,
    status: 'healthy',
  },
  {
    platform: 'kimi',
    account_alias: 'secondary',
    provider_id: 'kimi-secondary',
    quotas: {
      '5H': { used: 40, total: 100, reset_at: '2026-01-01T05:00:00Z' },
      '1W': { used: 500, total: 1000, reset_at: '2026-01-07T00:00:00Z' },
      '1M': { used: 1000, total: 5000, reset_at: '2026-02-01T00:00:00Z' },
    },
    last_sync: '2026-01-01T00:00:00Z',
    version: 8,
    status: 'warning',
  },
]

test.describe('QuotaHub Dashboard', () => {
  test('happy path: renders title, cards, chips, and quotas', async ({ page }) => {
    await page.route('**/api/v1/usage', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockProviders),
      })
    })

    await page.goto('/')

    await expect(page.getByRole('heading', { name: 'Coding Plans' })).toBeVisible()
    await expect(page.getByText('kimi', { exact: false })).toBeVisible()
    await expect(page.getByText('kimi', { exact: false })).toBeVisible()
    await expect(page.getByText('Healthy')).toBeVisible()
    await expect(page.getByText('Warning')).toBeVisible()
    await expect(page.getByText('5H')).toHaveCount(2)
    await expect(page.getByText('1W')).toHaveCount(2)
    await expect(page.getByText('1M')).toHaveCount(2)
  })

  test('error state: shows error banner on API failure', async ({ page }) => {
    await page.route('**/api/v1/usage', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'text/plain',
        body: 'Internal Server Error',
      })
    })

    await page.goto('/')

    await expect(page.getByRole('alert')).toBeVisible()
  })

  test('empty state: shows empty UI when provider list is empty', async ({ page }) => {
    await page.route('**/api/v1/usage', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      })
    })

    await page.goto('/')

    await expect(page.getByTestId('empty-state')).toBeVisible()
  })

  test('accessibility: no serious/critical violations on happy path', async ({ page }) => {
    await page.route('**/api/v1/usage', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockProviders),
      })
    })

    await page.goto('/')

    await page.waitForTimeout(100)

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
      .analyze()

    const severe = results.violations.filter(
      (violation) => violation.impact === 'serious' || violation.impact === 'critical',
    )

    expect(severe).toEqual([])
  })
})

// Real API execution helper (non-CI):
// VITE_QUOTAHUB_API_URL=http://100.64.1.38:8070 npx playwright test --config playwright.config.ts
