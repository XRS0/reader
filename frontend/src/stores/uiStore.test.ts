import { beforeEach, describe, expect, it } from 'vitest'
import { defaultAppThemeColors, useUIStore } from './uiStore'

describe('UI preferences store', () => {
  beforeEach(() => {
    localStorage.clear()
    useUIStore.setState({
      sidebarCollapsed: false,
      libraryView: 'grid',
      appTheme: 'system',
      appThemeColors: defaultAppThemeColors
    })
  })

  it('persists a custom whole-app palette', () => {
    useUIStore.getState().setAppTheme('custom')
    useUIStore.getState().setAppThemeColors({
      background: '#171a19',
      foreground: '#f2f3ef',
      accent: '#79aa98'
    })

    const persisted = JSON.parse(localStorage.getItem('bookflow:ui:v1') ?? '{}') as {
      state?: {
        appTheme?: string
        appThemeColors?: typeof defaultAppThemeColors
      }
    }

    expect(persisted.state?.appTheme).toBe('custom')
    expect(persisted.state?.appThemeColors).toEqual({
      background: '#171a19',
      foreground: '#f2f3ef',
      accent: '#79aa98'
    })
  })
})
