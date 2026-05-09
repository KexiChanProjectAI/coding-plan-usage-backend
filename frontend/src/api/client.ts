import type { UsageResponse } from '../domain/types'

const API_BASE_URL = import.meta.env.VITE_QUOTAHUB_API_URL ?? ''
const DEFAULT_USAGE_PATH = '/api/v1/usage'

export function getUsageUrl(baseUrl = API_BASE_URL): string {
  return `${baseUrl}${DEFAULT_USAGE_PATH}`
}

export async function fetchUsage(signal?: AbortSignal): Promise<UsageResponse[]> {
  let response: Response
  try {
    response = await fetch(getUsageUrl(), { signal })
  } catch (error) {
    if (signal?.aborted) {
      throw new Error('AbortError')
    }
    if (error instanceof Error) {
      throw error
    }
    throw new Error('Failed to fetch usage: network error')
  }

  if (!response.ok) {
    throw new Error(`Failed to fetch usage: ${response.status} ${response.statusText}`.trim())
  }

  try {
    const payload = (await response.json()) as unknown
    if (!Array.isArray(payload)) {
      throw new Error('Usage API returned invalid payload: expected array')
    }
    return payload as UsageResponse[]
  } catch (error) {
    if (error instanceof Error && error.message.includes('expected array')) {
      throw error
    }
    throw new Error('Usage API returned invalid JSON')
  }
}
