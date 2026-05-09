import type { PropsWithChildren, ReactNode } from 'react'
import '../../styles/layout.css'

export type Md3AppShellProps = PropsWithChildren & {
  header?: ReactNode
  fab?: ReactNode
}

export function Md3AppShell({ children, header, fab }: Md3AppShellProps) {
  return (
    <div className="md3-app-shell">
      {header && <header className="md3-app-header">{header}</header>}
      <main className="md3-app-content">
        <div className="md3-container">{children}</div>
      </main>
      {fab && <div className="md3-fab-zone">{fab}</div>}
    </div>
  )
}
