import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TranslationPopover } from './TranslationPopover'
import type { WordTranslation } from '../types/api'

const translation: WordTranslation = {
  original_text: 'unfamiliar',
  normalized_form: 'unfamiliar',
  lemma: 'unfamiliar',
  translation: 'незнакомый',
  transcription: '/ˌʌnfəˈmɪliə/',
  part_of_speech: 'adjective',
  definition: 'Not known or recognized.',
  alternatives: ['непривычный'],
  source_language: 'en',
  target_language: 'ru'
}

describe('TranslationPopover', () => {
  it('renders word details and adds the word to the dictionary', async () => {
    const user = userEvent.setup()
    const onAdd = vi.fn()
    render(
      <TranslationPopover
        selectedText="unfamiliar"
        value={translation}
        loading={false}
        error={false}
        added={false}
        onAdd={onAdd}
        onClose={() => undefined}
      />
    )

    expect(screen.getByText('незнакомый')).toBeInTheDocument()
    expect(screen.getByText('/ˌʌnfəˈmɪliə/ · adjective')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /словарь|dictionary/i }))
    expect(onAdd).toHaveBeenCalledOnce()
  })

  it('announces the loading state', () => {
    render(
      <TranslationPopover
        selectedText="patience"
        loading
        error={false}
        added={false}
        onAdd={() => undefined}
        onClose={() => undefined}
      />
    )
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('shows a dictionary-specific error and locks the button while saving', () => {
    render(
      <TranslationPopover
        selectedText="unfamiliar"
        value={translation}
        loading={false}
        error={false}
        addError
        adding
        added={false}
        onAdd={() => undefined}
        onClose={() => undefined}
      />
    )
    expect(screen.getByText(/Не удалось добавить слово|Could not add the word/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /словарь|dictionary/i })).toBeDisabled()
  })
})
