import { type HTMLAttributes, type ButtonHTMLAttributes, type ProgressHTMLAttributes, type PropsWithChildren } from 'react'
import './theme.css'

export type ColorScheme = 'light' | 'dark'

export function Md3ThemeProvider({ children }: PropsWithChildren) {
  return <div data-md3-scheme="dark">{children}</div>
}

export type Md3ButtonProps = HTMLAttributes<HTMLButtonElement> & {
  variant?: 'filled' | 'outlined'
}

export function Md3Button({ variant = 'filled', className = '', ...props }: Md3ButtonProps) {
  const variantClass = variant === 'outlined' ? 'md3-button--outlined' : 'md3-button--filled'
  return <button {...props} className={`md3-button ${variantClass} ${className}`.trim()} />
}

export type Md3CardProps = HTMLAttributes<HTMLDivElement>

export function Md3Card({ children, className = '', ...props }: Md3CardProps) {
  return (
    <div {...props} className={`md3-card ${className}`.trim()}>
      {children}
    </div>
  )
}

export type Status = 'healthy' | 'warning' | 'critical' | 'unknown'

export type Md3ChipProps = HTMLAttributes<HTMLSpanElement> & {
  status?: Status
}

export function Md3Chip({ status, className = '', ...props }: Md3ChipProps) {
  const statusClass = status ? `md3-chip--${status}` : ''
  return <span {...props} className={`md3-chip ${statusClass} ${className}`.trim()} />
}

export type Severity = 'normal' | 'warning' | 'critical'

export type Md3ProgressBarProps = ProgressHTMLAttributes<HTMLProgressElement> & {
  severity?: Severity
}

export function Md3ProgressBar({ severity = 'normal', className = '', ...props }: Md3ProgressBarProps) {
  const severityClass = `md3-progress--${severity}`
  return <progress {...props} className={`md3-progress ${severityClass} ${className}`.trim()} />
}

export type Md3FabProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  'aria-label': string
}

export function Md3Fab({ className = '', ...props }: Md3FabProps) {
  return <button {...props} className={`md3-fab ${className}`.trim()} />
}

export function Md3Skeleton(props: HTMLAttributes<HTMLDivElement>) {
  return <div aria-hidden="true" {...props} className={`md3-skeleton ${props.className ?? ''}`.trim()} />
}
