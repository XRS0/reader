import { contrastRatio } from '../reader/color'
import type { AppTheme, AppThemeColors } from '../stores/uiStore'

const customProperties = [
  '--app-custom-background',
  '--app-custom-foreground',
  '--app-custom-accent',
  '--app-custom-on-accent'
] as const

export function applyAppTheme(element: HTMLElement, theme: AppTheme, colors: AppThemeColors): void {
  element.dataset.appTheme = theme

  if (theme !== 'custom') {
    delete element.dataset.appColorScheme
    customProperties.forEach((property) => element.style.removeProperty(property))
    return
  }

  const darkBackground =
    contrastRatio(colors.background, '#ffffff') > contrastRatio(colors.background, '#000000')
  const onAccent =
    contrastRatio(colors.accent, '#ffffff') >= contrastRatio(colors.accent, '#111111')
      ? '#ffffff'
      : '#111111'

  element.dataset.appColorScheme = darkBackground ? 'dark' : 'light'
  element.style.setProperty('--app-custom-background', colors.background)
  element.style.setProperty('--app-custom-foreground', colors.foreground)
  element.style.setProperty('--app-custom-accent', colors.accent)
  element.style.setProperty('--app-custom-on-accent', onAccent)
}
