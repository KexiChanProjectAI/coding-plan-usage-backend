export function EmptyState() {
  return (
    <div className="empty-state" data-testid="empty-state">
      <div className="empty-state__icon" aria-hidden="true">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M12 2L2 7l10 5 10-5-10-5z" />
          <path d="M2 17l10 5 10-5" />
          <path d="M2 12l10 5 10-5" />
        </svg>
      </div>
      <p className="empty-state__text">No providers available</p>
      <p className="empty-state__subtext">Waiting for provider data...</p>
    </div>
  )
}
