import { describe, expect, it } from 'vitest'
import { formatSelectedText } from './selectionText'

describe('formatSelectedText', () => {
  it('preserves paragraphs and removes layout whitespace', () => {
    expect(formatSelectedText('  First   line\r\n\r\n\r\n Second\tline  ')).toBe(
      'First line\n\nSecond line'
    )
  })

  it('limits stored selections', () => {
    expect(formatSelectedText('abcdef', 4)).toBe('abcd')
  })
})
