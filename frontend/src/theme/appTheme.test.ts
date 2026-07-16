import { afterEach, describe, expect, it } from 'vitest'
import { applyAppTheme } from './appTheme'

const colors = {
  background: '#171a19',
  foreground: '#f2f3ef',
  accent: '#79aa98'
}

describe('applyAppTheme', () => {
  afterEach(() => {
    document.documentElement.removeAttribute('data-app-theme')
    document.documentElement.removeAttribute('data-app-color-scheme')
    document.documentElement.removeAttribute('style')
  })

  it('applies a persisted custom palette to the whole document', () => {
    applyAppTheme(document.documentElement, 'custom', colors)

    expect(document.documentElement.dataset.appTheme).toBe('custom')
    expect(document.documentElement.dataset.appColorScheme).toBe('dark')
    expect(document.documentElement.style.getPropertyValue('--app-custom-background')).toBe(
      '#171a19'
    )
    expect(document.documentElement.style.getPropertyValue('--app-custom-foreground')).toBe(
      '#f2f3ef'
    )
    expect(document.documentElement.style.getPropertyValue('--app-custom-accent')).toBe('#79aa98')
  })

  it('clears custom properties when a preset is selected', () => {
    applyAppTheme(document.documentElement, 'custom', colors)
    applyAppTheme(document.documentElement, 'light', colors)

    expect(document.documentElement.dataset.appTheme).toBe('light')
    expect(document.documentElement.dataset.appColorScheme).toBeUndefined()
    expect(document.documentElement.style.getPropertyValue('--app-custom-background')).toBe('')
  })
})
