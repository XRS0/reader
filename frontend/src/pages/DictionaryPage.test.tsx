import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { DictionaryPage } from './DictionaryPage'
import type { DictionaryEntry } from '../types/api'

const { deleteEntry } = vi.hoisted(() => ({
  deleteEntry: vi.fn(() => Promise.resolve(undefined))
}))

const entry: DictionaryEntry = {
  id: '00000000-0000-4000-8000-000000000001',
  source_language: 'ru',
  target_language: 'ru',
  original_word: 'самобытность',
  normalized_word: 'самобытность',
  translation: '',
  alternative_translations: [],
  definition: 'Неповторимое своеобразие человека, культуры или явления.',
  status: 'learning',
  encounter_count: 2,
  first_seen_at: '2026-07-16T10:00:00.000Z',
  last_seen_at: '2026-07-16T10:00:00.000Z',
  next_review_at: null,
  created_at: '2026-07-16T10:00:00.000Z',
  updated_at: '2026-07-16T10:00:00.000Z'
}

vi.mock('../api/hooks', () => ({
  useDebouncedValue: (value: string) => value,
  useDictionary: () => ({
    data: { items: [entry], total_count: 1 },
    isLoading: false,
    isError: false,
    refetch: vi.fn()
  }),
  useCreateDictionaryEntry: () => ({
    mutateAsync: vi.fn(),
    reset: vi.fn(),
    isPending: false,
    isError: false
  }),
  useUpdateDictionaryEntry: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false
  }),
  useDeleteDictionaryEntry: () => ({
    mutateAsync: deleteEntry,
    reset: vi.fn(),
    isError: false,
    isPending: false
  })
}))

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/dictionary']}>
      <DictionaryPage />
    </MemoryRouter>
  )
}

describe('DictionaryPage', () => {
  it('opens a word card and shows its definition without requiring a translation', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(screen.getByRole('button', { name: /Открыть карточку|Open details/i }))

    const details = screen.getByRole('dialog', { name: 'самобытность' })
    expect(
      within(details).getByText('Неповторимое своеобразие человека, культуры или явления.', {
        selector: 'p'
      })
    ).toBeVisible()
    expect(
      within(details).queryByRole('textbox', { name: /Перевод|Translation/i })
    ).not.toBeInTheDocument()

    await user.click(within(details).getByRole('button', { name: /Изменить|Edit/i }))
    expect(within(details).getByRole('textbox', { name: /Перевод|Translation/i })).toHaveValue('')
  })

  it('requires confirmation and deletes a dictionary card', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(screen.getByRole('button', { name: /Удалить слово|Delete “/i }))
    const confirmation = screen.getByRole('dialog', { name: 'самобытность' })
    await user.click(within(confirmation).getByRole('button', { name: /Удалить|Delete/i }))

    expect(deleteEntry).toHaveBeenCalledWith(entry.id)
  })
})
