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
    return 'Unavailable'
  }

  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(resetAt)
}
