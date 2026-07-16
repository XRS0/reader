import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { ReaderPreferences } from '../types/api'

export const defaultReaderPreferences: ReaderPreferences = {
  theme: 'light',
  background_color: '#fbfbfa',
  text_color: '#262624',
  accent_color: '#356859',
  font_family: 'Georgia',
  font_size: 20,
  font_weight: 400,
  line_height: 1.7,
  letter_spacing: 0,
  content_width: 720,
  page_margin: 40,
  text_align: 'left',
  reading_mode: 'scroll',
  show_progress: true,
  show_remaining_time: true,
  controls_brightness: 0.92
}

interface ReaderState {
  preferences: ReaderPreferences
  tocOpen: boolean
  settingsOpen: boolean
  updatePreferences: (input: Partial<ReaderPreferences>) => void
  replacePreferences: (input: ReaderPreferences) => void
  resetPreferences: () => void
  setTocOpen: (open: boolean) => void
  setSettingsOpen: (open: boolean) => void
}

export const useReaderStore = create<ReaderState>()(
  persist(
    (set) => ({
      preferences: defaultReaderPreferences,
      tocOpen: false,
      settingsOpen: false,
      updatePreferences: (input) =>
        set((state) => ({ preferences: { ...state.preferences, ...input } })),
      replacePreferences: (preferences) => set({ preferences }),
      resetPreferences: () => set({ preferences: defaultReaderPreferences }),
      setTocOpen: (tocOpen) => set({ tocOpen, settingsOpen: tocOpen ? false : undefined }),
      setSettingsOpen: (settingsOpen) =>
        set({ settingsOpen, tocOpen: settingsOpen ? false : undefined })
    }),
    {
      name: 'bookflow:reader-preferences:v1',
      partialize: (state) => ({ preferences: state.preferences })
    }
  )
)
