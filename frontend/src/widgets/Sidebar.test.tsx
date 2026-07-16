import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { Sidebar } from './Sidebar'
import { useUIStore } from '../stores/uiStore'

vi.mock('../api/hooks', () => ({
  useCurrentUser: () => ({
    data: {
      user: { display_name: 'Reader', email: 'reader@example.com' }
    }
  })
}))

function renderSidebar(entry: string) {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Sidebar />
    </MemoryRouter>
  )
}

describe('Sidebar', () => {
  beforeEach(() => useUIStore.setState({ sidebarCollapsed: false }))

  it.each([
    ['/library', /Library|Библиотека/],
    ['/library?filter=continue', /Library|Библиотека/],
    ['/library?sort=last_read', /Library|Библиотека/],
    ['/library?favorite=true', /Favorites|Избранное/]
  ])('marks only the matching library destination active for %s', (entry, activeName) => {
    renderSidebar(entry)
    const navigation = screen.getByRole('navigation', { name: 'BookFlow' })
    const current = within(navigation)
      .getAllByRole('link')
      .filter((link) => link.hasAttribute('aria-current'))
    expect(current).toHaveLength(1)
    expect(current[0]).toHaveAccessibleName(activeName)
  })

  it('does not render redundant continue or recent destinations', () => {
    renderSidebar('/library')
    const navigation = screen.getByRole('navigation', { name: 'BookFlow' })

    expect(
      within(navigation).queryByRole('link', { name: /Continue reading|Продолжить чтение/ })
    ).not.toBeInTheDocument()
    expect(
      within(navigation).queryByRole('link', { name: /^Recent$|^Недавние$/ })
    ).not.toBeInTheDocument()
  })

  it('keeps the rail toggle at the top and supports keyboard activation both ways', async () => {
    const user = userEvent.setup()
    renderSidebar('/library')
    fireEvent.click(
      screen.getByRole('button', { name: /Collapse sidebar|Свернуть боковую панель/ })
    )

    const expand = screen.getByRole('button', { name: /Expand sidebar|Развернуть боковую панель/ })
    const search = screen.getByRole('button', { name: /Search|Поиск/ })
    expect(expand.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    expand.focus()
    await user.keyboard('{Enter}')
    expect(
      screen.getByRole('button', { name: /Collapse sidebar|Свернуть боковую панель/ })
    ).toBeVisible()
  })
})
