import type {
  BackendTier,
  NormalizedProvider,
  NormalizedQuota,
  ProviderStatus,
  QuotaSeverity,
  QuotaTier,
  UsageResponse,
} from './types'

const CANONICAL_TIERS: BackendTier[] = ['5H', '1W', '1M']

export function mapProviderStatus(status?: string): ProviderStatus {
  switch (status?.toLowerCase()) {
    case 'healthy':
    case 'active':
      return 'Healthy'
    case 'warning':
    case 'limited':
    case 'degraded':
      return 'Warning'
    case 'critical':
    case 'error':
    case 'down':
      return 'Critical'
    case 'initializing':
    default:
      return 'Unknown'
  }
}

export function deriveProviderKey(provider: UsageResponse): string {
  if (provider.provider_id && provider.provider_id.trim().length > 0) {
    return provider.provider_id
  }

  const accountAlias = provider.account_alias?.trim() || 'default'
  return `${provider.platform}:${accountAlias}`
}

export function isInitializingPlaceholder(provider: UsageResponse): boolean {
  return provider.platform === '' && provider.version === 0
}

export function calculateQuotaPercentage(used: number, total: number): number | null {
  if (!Number.isFinite(total) || total <= 0) {
    return null
  }

  const safeUsed = Number.isFinite(used) ? used : 0
  const clampedUsed = Math.max(0, safeUsed)
  const raw = (clampedUsed / total) * 100
  return Math.min(100, Math.max(0, raw))
}

export function quotaSeverity(percentage: number | null): QuotaSeverity {
  if (percentage === null) {
    return 'normal'
  }
  if (percentage >= 90) {
    return 'critical'
  }
  if (percentage >= 70) {
    return 'warning'
  }
  return 'normal'
}

function parseDate(value: string | undefined): Date | null {
  if (!value) {
    return null
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

function normalizeQuota(tier: BackendTier, quota?: QuotaTier): NormalizedQuota {
  if (!quota) {
    return {
      tier,
      used: 0,
      total: 0,
      percentage: null,
      severity: 'normal',
      resetAt: null,
      isUnavailable: true,
    }
  }

  const percentage = calculateQuotaPercentage(quota.used, quota.total)
  const unavailable = percentage === null

  return {
    tier,
    used: quota.used,
    total: quota.total,
    percentage,
    severity: quotaSeverity(percentage),
    resetAt: parseDate(quota.reset_at),
    isUnavailable: unavailable,
  }
}

export function normalizeProviders(providers: UsageResponse[]): NormalizedProvider[] {
  return providers
    .filter((provider) => !isInitializingPlaceholder(provider))
    .map((provider) => {
      const normalizedQuotas = CANONICAL_TIERS.map((tier) =>
        normalizeQuota(tier, provider.quotas?.[tier]),
      ) as [NormalizedQuota, NormalizedQuota, NormalizedQuota]

      return {
        key: deriveProviderKey(provider),
        platform: provider.platform,
        accountAlias: provider.account_alias || '',
        status: mapProviderStatus(provider.status),
        errorMessage: provider.error_message,
        lastSync: parseDate(provider.last_sync),
        version: provider.version,
        quotas: normalizedQuotas,
      }
    })
}
