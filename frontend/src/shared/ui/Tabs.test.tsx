import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Tabs } from './index'

describe('Tabs', () => {
  const items = [
    { id: 'overview', label: 'Overview', content: <p>Overview content</p> },
    { id: 'contents', label: 'Contents', content: <p>Contents content</p> },
    { id: 'notes', label: 'Notes', content: <p>Notes content</p> }
  ]

  it('moves both selection and keyboard focus with arrow keys', () => {
    render(<Tabs items={items} />)

    const overview = screen.getByRole('tab', { name: 'Overview' })
    overview.focus()
    fireEvent.keyDown(overview, { key: 'ArrowRight' })

    const contents = screen.getByRole('tab', { name: 'Contents' })
    expect(contents).toHaveAttribute('aria-selected', 'true')
    expect(contents).toHaveAttribute('tabindex', '0')
    expect(contents).toHaveFocus()
    expect(screen.getByRole('tabpanel', { name: 'Contents' })).toHaveTextContent('Contents content')
  })

  it('supports Home, End and wrapping navigation', () => {
    render(<Tabs items={items} defaultTab="notes" />)

    const notes = screen.getByRole('tab', { name: 'Notes' })
    notes.focus()
    fireEvent.keyDown(notes, { key: 'ArrowRight' })
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveFocus()

    fireEvent.keyDown(document.activeElement!, { key: 'End' })
    expect(notes).toHaveFocus()

    fireEvent.keyDown(notes, { key: 'Home' })
    expect(screen.getByRole('tab', { name: 'Overview' })).toHaveFocus()
  })
})
