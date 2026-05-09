import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import App from '../App'
import * as client from '../api/client'

function mockMatchMedia(matches: boolean) {
  const mql = {
    matches,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }
  vi.stubGlobal('matchMedia', () => mql)
  return mql
}

describe('App', () => {
  beforeEach(() => {
    mockMatchMedia(false)
    vi.spyOn(client, 'fetchUsage').mockResolvedValue([])
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('renders dashboard title', async () => {
    render(<App />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Coding Plans' })).toBeInTheDocument()
    })
  })
})
