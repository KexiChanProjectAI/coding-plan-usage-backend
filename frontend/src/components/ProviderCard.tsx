import type { NormalizedProvider } from '../domain/types'
import { Md3Card, Md3Chip } from './md3'
import { QuotaBlock } from './QuotaBlock'
import { formatRelativeLastSync } from '../domain/format'

const AVATAR_PALETTE = [
  '#6750a4',
  '#1a73e8',
  '#188038',
  '#c5221f',
  '#b06000',
  '#9334e6',
  '#007b83',
  '#d01884',
]

function getAvatarColor(key: string): string {
  let hash = 0
  for (let i = 0; i < key.length; i++) {
    hash = key.charCodeAt(i) + ((hash << 5) - hash)
  }
  const index = Math.abs(hash) % AVATAR_PALETTE.length
  return AVATAR_PALETTE[index]
}

interface ProviderCardProps {
  provider: NormalizedProvider
}

export function ProviderCard({ provider }: ProviderCardProps) {
  const avatarColor = getAvatarColor(provider.key)
  const statusLower = provider.status.toLowerCase() as 'healthy' | 'warning' | 'critical' | 'unknown'
  const showAlias = provider.accountAlias && provider.accountAlias !== provider.platform

  return (
    <Md3Card className="provider-card" data-testid={`provider-card-${provider.key}`}>
      <div className="provider-card__header">
        <div
          className="provider-card__avatar"
          style={{ backgroundColor: avatarColor }}
          aria-hidden="true"
        >
          {provider.platform.charAt(0).toUpperCase()}
        </div>
        <div className="provider-card__title-group">
          <span className="provider-card__platform">{provider.platform}</span>
          {showAlias && (
            <span className="provider-card__alias">{provider.accountAlias}</span>
          )}
        </div>
        <Md3Chip status={statusLower} className="provider-card__chip">
          {provider.status}
        </Md3Chip>
      </div>

      {provider.errorMessage && (
        <div className="provider-card__error" role="alert">
          {provider.errorMessage}
        </div>
      )}

      <div className="provider-card__quotas">
        {provider.quotas.map((quota) => (
          <QuotaBlock key={quota.tier} quota={quota} />
        ))}
      </div>

      <div className="provider-card__footer">
        <span className="provider-card__last-sync">
          Last sync: {formatRelativeLastSync(provider.lastSync)}
        </span>
        <span className="provider-card__version">
          v{provider.version}
        </span>
      </div>
    </Md3Card>
  )
}
