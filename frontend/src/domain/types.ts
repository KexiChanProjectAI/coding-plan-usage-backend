export type BackendTier = '5H' | '1W' | '1M'

export interface QuotaTier {
  used: number
  total: number
  reset_at: string
}

export interface UsageResponse {
  platform: string
  account_alias: string
  quotas?: Partial<Record<BackendTier, QuotaTier>>
  last_sync: string
  version: number
  status?: string
  error_message?: string
  provider_id?: string
}

export type ProviderStatus = 'Healthy' | 'Warning' | 'Critical' | 'Unknown'
export type QuotaSeverity = 'normal' | 'warning' | 'critical'

export interface NormalizedQuota {
  tier: BackendTier
  used: number
  total: number
  percentage: number | null
  severity: QuotaSeverity
  resetAt: Date | null
  isUnavailable: boolean
}

export interface NormalizedProvider {
  key: string
  platform: string
  accountAlias: string
  status: ProviderStatus
  errorMessage?: string
  lastSync: Date | null
  version: number
  quotas: [NormalizedQuota, NormalizedQuota, NormalizedQuota]
}

export type FetchStatus = 'idle' | 'loading' | 'success' | 'error'

export interface FetchState<T> {
  status: FetchStatus
  data: T | null
  error: Error | null
}
