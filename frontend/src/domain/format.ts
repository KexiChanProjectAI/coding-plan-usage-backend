export const UNLIMITED_LABEL = '无限制/INF'

export function formatRelativeLastSync(lastSync: Date | null, now = new Date()): string {
  if (!lastSync) {
    return 'never'
  }

  const diffMs = now.getTime() - lastSync.getTime()
  const safeDiffMs = Math.max(0, diffMs)
  const seconds = Math.floor(safeDiffMs / 1000)

  if (seconds < 30) {
    return 'just now'
  }
  if (seconds < 60) {
    return `${seconds}s ago`
  }

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) {
    return `${minutes}m ago`
  }

  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours}h ago`
  }

  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export function formatResetTime(resetAt: Date | null, locale?: string): string {
  if (!resetAt) {
    return UNLIMITED_LABEL
  }

  if (
    resetAt.getUTCFullYear() === 1 &&
    resetAt.getUTCMonth() === 0 &&
    resetAt.getUTCDate() === 1 &&
    resetAt.getUTCHours() === 0 &&
    resetAt.getUTCMinutes() === 0 &&
    resetAt.getUTCSeconds() === 0 &&
    resetAt.getUTCMilliseconds() === 0
  ) {
    return UNLIMITED_LABEL
  }

  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(resetAt)
}
