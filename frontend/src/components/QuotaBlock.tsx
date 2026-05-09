import type { NormalizedQuota } from '../domain/types'
import { Md3ProgressBar } from './md3'
import { formatResetTime } from '../domain/format'

interface QuotaBlockProps {
  quota: NormalizedQuota
}

export function QuotaBlock({ quota }: QuotaBlockProps) {
  const percentageText = quota.percentage !== null ? `${Math.round(quota.percentage)}%` : 'N/A'
  const remainingPercentage = quota.percentage !== null ? 100 - quota.percentage : 0
  const remainingPercentageText = quota.percentage !== null ? `${Math.round(remainingPercentage)}%` : 'N/A'
  const usedTotalText = `${quota.used} / ${quota.total}`
  const severityLabel = quota.severity === 'critical' ? 'Critical' : quota.severity === 'warning' ? 'Warning' : ''
  const resetText = quota.isUnavailable ? 'Unavailable' : formatResetTime(quota.resetAt)

  return (
    <div className="quota-block" data-testid={`quota-${quota.tier}`}>
      <div className="quota-block__header">
        <span className="quota-block__tier">{quota.tier}</span>
        {severityLabel && (
          <span className={`quota-block__severity quota-block__severity--${quota.severity}`}>
            {severityLabel}
          </span>
        )}
      </div>
      <Md3ProgressBar
        severity={quota.severity}
        value={remainingPercentage}
        max={100}
        className="quota-block__progress"
        aria-label={`${quota.tier} quota remaining: ${remainingPercentageText}`}
      />
      <div className="quota-block__numbers">
        <span className="quota-block__used-total">{usedTotalText}</span>
        <span className="quota-block__percentage">{percentageText}</span>
      </div>
      <div className="quota-block__reset">{resetText}</div>
    </div>
  )
}
