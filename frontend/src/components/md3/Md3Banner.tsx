import type { HTMLAttributes, ReactNode } from 'react'

export type Md3BannerProps = HTMLAttributes<HTMLDivElement> & {
  open?: boolean
  action?: ReactNode
  onDismiss?: () => void
}

export function Md3Banner({ open = false, children, action, onDismiss, ...props }: Md3BannerProps) {
  if (!open) {
    return null
  }

  return (
    <div
      role="alert"
      className="md3-banner"
      {...props}
    >
      <span className="md3-banner__message">{children}</span>
      <span className="md3-banner__actions">
        {action}
        {onDismiss && (
          <button
            type="button"
            className="md3-banner__dismiss"
            onClick={onDismiss}
            aria-label="Dismiss banner"
          >
            Dismiss
          </button>
        )}
      </span>
    </div>
  )
}
