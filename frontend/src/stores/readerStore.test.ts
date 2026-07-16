import { defaultReaderPreferences, useReaderStore } from './readerStore'

describe('reader preferences store', () => {
  beforeEach(() => {
    useReaderStore.persist.clearStorage()
    useReaderStore.setState({ preferences: defaultReaderPreferences })
  })

  it('persists the warm theme between sessions', () => {
    useReaderStore.getState().updatePreferences({ theme: 'warm', font_size: 22 })

    const persisted = JSON.parse(
      localStorage.getItem('bookflow:reader-preferences:v1') ?? '{}'
    ) as {
      state?: { preferences?: { theme?: string; font_size?: number } }
    }
    expect(persisted.state?.preferences?.theme).toBe('warm')
    expect(persisted.state?.preferences?.font_size).toBe(22)
  })

  it('updates typography without replacing unrelated preferences', () => {
    useReaderStore.getState().updatePreferences({ line_height: 1.9, content_width: 840 })
    const preferences = useReaderStore.getState().preferences
    expect(preferences.line_height).toBe(1.9)
    expect(preferences.content_width).toBe(840)
    expect(preferences.font_family).toBe(defaultReaderPreferences.font_family)
  })
})
