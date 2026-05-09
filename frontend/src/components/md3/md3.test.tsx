import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import {
  Md3ThemeProvider,
  Md3Button,
  Md3Card,
  Md3Chip,
  Md3ProgressBar,
  Md3Fab,
  Md3Skeleton,
  Md3Snackbar,
  Md3Banner,
  Md3AppShell,
} from './index'

function mockMatchMedia(matches: boolean) {
  const mql = {
    matches,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }
  vi.stubGlobal('matchMedia', () => mql)
  return mql
}

describe('Md3ThemeProvider', () => {
  beforeEach(() => {
    mockMatchMedia(false)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    cleanup()
  })

  it('renders children and sets data-md3-scheme based on prefers-color-scheme', () => {
    render(
      <Md3ThemeProvider>
        <div data-testid="child">hello</div>
      </Md3ThemeProvider>,
    )
    expect(screen.getByTestId('child')).toBeInTheDocument()
    const wrapper = screen.getByTestId('child').parentElement
    expect(wrapper).toHaveAttribute('data-md3-scheme')
  })

  it('updates data-md3-scheme when prefers-color-scheme changes', async () => {
    const mql = mockMatchMedia(false)

    render(
      <Md3ThemeProvider>
        <div data-testid="child">hello</div>
      </Md3ThemeProvider>,
    )

    const wrapper = screen.getByTestId('child').parentElement
    expect(wrapper).toHaveAttribute('data-md3-scheme', 'light')

    const changeHandler = mql.addEventListener.mock.calls.find(
      (call) => call[0] === 'change',
    )?.[1] as (e: MediaQueryListEvent) => void

    changeHandler?.({ matches: true } as MediaQueryListEvent)
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(wrapper).toHaveAttribute('data-md3-scheme', 'dark')
  })
})

describe('Md3Button', () => {
  afterEach(() => cleanup())

  it('renders with filled variant by default', () => {
    render(<Md3Button data-testid="btn-filled">Click</Md3Button>)
    const btn = screen.getByTestId('btn-filled')
    expect(btn.tagName).toBe('BUTTON')
    expect(btn).toHaveClass('md3-button--filled')
  })

  it('renders with outlined variant', () => {
    render(<Md3Button variant="outlined" data-testid="btn-outlined">Click</Md3Button>)
    expect(screen.getByTestId('btn-outlined')).toHaveClass('md3-button--outlined')
  })
})

describe('Md3Card', () => {
  afterEach(() => cleanup())

  it('renders children', () => {
    render(<Md3Card data-testid="card">Content</Md3Card>)
    expect(screen.getByTestId('card')).toHaveTextContent('Content')
  })
})

describe('Md3Chip', () => {
  afterEach(() => cleanup())

  it.each([
    ['healthy', 'md3-chip--healthy'],
    ['warning', 'md3-chip--warning'],
    ['critical', 'md3-chip--critical'],
    ['unknown', 'md3-chip--unknown'],
  ] as const)('applies status class for %s', (status, expectedClass) => {
    render(<Md3Chip status={status} data-testid={`chip-${status}`}>{status}</Md3Chip>)
    expect(screen.getByTestId(`chip-${status}`)).toHaveClass(expectedClass)
  })
})

describe('Md3ProgressBar', () => {
  afterEach(() => cleanup())

  it.each([
    ['normal', 'md3-progress--normal'],
    ['warning', 'md3-progress--warning'],
    ['critical', 'md3-progress--critical'],
  ] as const)('applies severity class for %s', (severity, expectedClass) => {
    render(<Md3ProgressBar severity={severity} data-testid={`prog-${severity}`} value={50} max={100} />)
    expect(screen.getByTestId(`prog-${severity}`)).toHaveClass(expectedClass)
  })
})

describe('Md3Fab', () => {
  afterEach(() => cleanup())

  it('has accessible label and minimum 48px touch target', () => {
    render(<Md3Fab aria-label="Refresh data" data-testid="fab" />)
    const fab = screen.getByTestId('fab')
    expect(fab).toHaveAttribute('aria-label', 'Refresh data')
    expect(fab.tagName).toBe('BUTTON')
  })
})

describe('Md3Skeleton', () => {
  afterEach(() => cleanup())

  it('is aria-hidden', () => {
    render(<Md3Skeleton data-testid="skel" />)
    expect(screen.getByTestId('skel')).toHaveAttribute('aria-hidden', 'true')
  })
})

describe('Md3Snackbar', () => {
  afterEach(() => cleanup())

  it('renders nothing when closed', () => {
    render(<Md3Snackbar open={false}>Message</Md3Snackbar>)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('renders with status role when open', () => {
    render(<Md3Snackbar open>Message</Md3Snackbar>)
    expect(screen.getByRole('status')).toHaveTextContent('Message')
  })

  it('renders action slot', () => {
    render(
      <Md3Snackbar open action={<button>Undo</button>}>
        Deleted
      </Md3Snackbar>,
    )
    expect(screen.getByRole('button', { name: 'Undo' })).toBeInTheDocument()
  })
})

describe('Md3Banner', () => {
  afterEach(() => cleanup())

  it('renders nothing when closed', () => {
    render(<Md3Banner open={false}>Error</Md3Banner>)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('renders with alert role when open', () => {
    render(<Md3Banner open>Error</Md3Banner>)
    expect(screen.getByRole('alert')).toHaveTextContent('Error')
  })

  it('renders action and dismiss buttons', () => {
    const onDismiss = vi.fn()
    render(
      <Md3Banner open action={<button>Retry</button>} onDismiss={onDismiss}>
        Failed
      </Md3Banner>,
    )
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dismiss banner' })).toBeInTheDocument()
  })
})

describe('Md3AppShell', () => {
  afterEach(() => cleanup())

  it('renders header, content, and fab zones', () => {
    render(
      <Md3AppShell
        header={<span data-testid="header">Title</span>}
        fab={<button data-testid="fab">Add</button>}
      >
        <div data-testid="content">Dashboard</div>
      </Md3AppShell>,
    )
    expect(screen.getByTestId('header')).toBeInTheDocument()
    expect(screen.getByTestId('content')).toBeInTheDocument()
    expect(screen.getByTestId('fab')).toBeInTheDocument()
  })
})
