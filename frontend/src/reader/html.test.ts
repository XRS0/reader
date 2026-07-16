import { describe, expect, it } from 'vitest'
import { stripLeadingDuplicateHeading } from './html'

describe('stripLeadingDuplicateHeading', () => {
  it('removes an exact leading EPUB heading through transparent wrappers', () => {
    const html = '<span id="chapter"><h1><strong>ВВЕДЕНИЕ</strong></h1><p>Первый абзац.</p></span>'

    expect(stripLeadingDuplicateHeading(html, '  Введение ')).toBe(
      '<span id="chapter"><p>Первый абзац.</p></span>'
    )
  })

  it('preserves a different heading and a matching heading after authored text', () => {
    expect(stripLeadingDuplicateHeading('<h2>Подраздел</h2><p>Текст.</p>', 'Глава')).toBe(
      '<h2>Подраздел</h2><p>Текст.</p>'
    )
    expect(stripLeadingDuplicateHeading('<p>Эпиграф.</p><h1>Глава</h1>', 'Глава')).toBe(
      '<p>Эпиграф.</p><h1>Глава</h1>'
    )
  })

  it('removes empty wrapper chains left by a title-only spine document', () => {
    expect(
      stripLeadingDuplicateHeading('<span><span><h3>Раздел</h3></span></span>', 'Раздел')
    ).toBe('')
  })
})
