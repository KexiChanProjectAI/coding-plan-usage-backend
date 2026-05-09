import { Md3Skeleton } from './md3'

export function LoadingSkeleton() {
  return (
    <div className="loading-skeleton" data-testid="loading-skeleton">
      <Md3Card className="skeleton-card">
        <div className="skeleton-card__header">
          <Md3Skeleton className="skeleton-card__avatar" />
          <Md3Skeleton className="skeleton-card__title" />
          <Md3Skeleton className="skeleton-card__chip" />
        </div>
        <div className="skeleton-card__quotas">
          <Md3Skeleton className="skeleton-card__quota" />
          <Md3Skeleton className="skeleton-card__quota" />
          <Md3Skeleton className="skeleton-card__quota" />
        </div>
        <div className="skeleton-card__footer">
          <Md3Skeleton className="skeleton-card__footer-item" />
          <Md3Skeleton className="skeleton-card__footer-item" />
        </div>
      </Md3Card>
    </div>
  )
}

function Md3Card({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return <div className={`md3-card ${className}`.trim()}>{children}</div>
}
