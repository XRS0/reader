import { describe, expect, it } from 'vitest'
import { resolveSourceLanguage } from './language'

describe('resolveSourceLanguage', () => {
  it('falls back from an empty translator language to book metadata or script detection', () => {
    expect(resolveSourceLanguage('', 'de', 'Hallo')).toBe('de')
    expect(resolveSourceLanguage('', 'und', 'Hello')).toBe('en')
    expect(resolveSourceLanguage(undefined, '', 'Привет')).toBe('ru')
  })
})
