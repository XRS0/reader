import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ContextMenu } from './index'

describe('ContextMenu', () => {
  it('opens from the context-menu key path and exposes accessible actions', async () => {
    const user = userEvent.setup()
    const open = vi.fn()
    render(
      <ContextMenu
        label="Book actions"
        items={[
          { id: 'open', label: 'Open', onSelect: open },
          { id: 'delete', label: 'Delete', danger: true, onSelect: () => undefined }
        ]}
      >
        <button type="button">A book</button>
      </ContextMenu>
    )

    await user.pointer({
      keys: '[MouseRight]',
      target: screen.getByRole('button', { name: 'A book' })
    })
    const menu = screen.getByRole('menu', { name: 'Book actions' })
    expect(menu).toBeInTheDocument()
    const first = screen.getByRole('menuitem', { name: 'Open' })
    first.focus()
    await user.keyboard('{Enter}')
    expect(open).toHaveBeenCalledOnce()
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })
})
