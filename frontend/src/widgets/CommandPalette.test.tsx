import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { CommandPalette } from './CommandPalette'
import { useUIStore } from '../stores/uiStore'

function LocationProbe() {
  return <output aria-label="location">{useLocation().pathname}</output>
}

describe('CommandPalette keyboard navigation', () => {
  beforeEach(() => {
    useUIStore.setState({ commandOpen: false, appTheme: 'system' })
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve(
          new Response(
            JSON.stringify({ items: [], next_cursor: null, has_more: false, total_count: 0 }),
            {
              status: 200,
              headers: { 'Content-Type': 'application/json' }
            }
          )
        )
      )
    )
  })

  afterEach(() => vi.unstubAllGlobals())

  it('opens with Ctrl+K and selects the next command with arrows and Enter', async () => {
    const user = userEvent.setup()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/library']}>
          <CommandPalette />
          <LocationProbe />
        </MemoryRouter>
      </QueryClientProvider>
    )

    await user.keyboard('{Control>}k{/Control}')
    const search = screen.getByRole('textbox', { name: /найдите|find/i })
    expect(search).toHaveFocus()
    await user.keyboard('{ArrowDown}{Enter}')
    expect(screen.getByLabelText('location')).toHaveTextContent('/dictionary')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes with Escape and restores focus', async () => {
    const user = userEvent.setup()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <button type="button">Before</button>
          <CommandPalette />
        </MemoryRouter>
      </QueryClientProvider>
    )
    const before = screen.getByRole('button', { name: 'Before' })
    before.focus()
    await user.keyboard('{Meta>}k{/Meta}')
    await user.keyboard('{Escape}')
    expect(before).toHaveFocus()
  })
})
