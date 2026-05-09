import type { HTMLAttributes, ReactNode } from 'react'

export type Md3SnackbarProps = HTMLAttributes<HTMLDivElement> & {
  open?: boolean
  action?: ReactNode
}

export function Md3Snackbar({ open = false, children, action, ...props }: Md3SnackbarProps) {
  if (!open) {
    return null
  }

  return (
    <div
      role="status"
      aria-live="polite"
      className="md3-snackbar"
      {...props}
    >
      <span className="md3-snackbar__message">{children}</span>
      {action && <span className="md3-snackbar__action">{action}</span>}
    </div>
  )
}
