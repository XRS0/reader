import {
  bookStatisticsResponseSchema,
  booksPageSchema,
  bookUploadSchema,
  chapterSchema,
  dailyStatisticsResponseSchema,
  dictionaryPageSchema,
  overviewSchema,
  preferencesSchema,
  sessionsPageSchema,
  tocSchema,
  wordTranslationSchema
} from './schemas'

const id = '019f670d-13bd-7bc3-94fb-e042a746c2be'

describe('backend HTTP contract adapters', () => {
  it('normalizes the accepted upload envelope and chapter payload', () => {
    const book = bookUploadSchema.parse({
      book: {
        id,
        title: 'Book',
        author: 'Author',
        format: 'epub',
        language: 'en',
        processing_status: 'queued',
        progress_percent: 0,
        is_favorite: false,
        tags: [],
        added_at: '2026-07-15T00:00:00Z',
        updated_at: '2026-07-15T00:00:00Z'
      },
      duplicate: false
    })
    expect(book.title).toBe('Book')

    const chapter = chapterSchema.parse({
      id,
      book_id: id,
      title: 'One',
      ordinal: 1,
      content_html: '<p>Text</p>',
      content_text: 'Text',
      word_count: 1
    })
    expect(chapter).toMatchObject({ order: 1, html: '<p>Text</p>', plain_text: 'Text' })
  })

  it('normalizes TOC wrappers, backend font identifiers, and translations', () => {
    const toc = tocSchema.parse({ items: [{ id, title: 'One', ordinal: 1 }] })
    expect(toc[0]).toMatchObject({ chapter_id: id, order: 1, level: 0 })

    const preferences = preferencesSchema.parse({
      theme: 'warm',
      background_color: '#f8f1df',
      text_color: '#302d27',
      accent_color: '#456957',
      font_family: 'source-serif',
      font_size: 20,
      font_weight: 400,
      line_height: 1.7,
      letter_spacing: 0.2,
      content_width: 720,
      page_margin: 32,
      text_align: 'left',
      reading_mode: 'scroll',
      show_progress: true,
      show_remaining_time: true,
      controls_brightness: 0.8
    })
    expect(preferences.font_family).toBe('Source Serif 4')
    expect(preferences.letter_spacing).toBe(0.08)

    const translation = wordTranslationSchema.parse({
      original_text: 'book',
      normalized_form: 'book',
      translation: 'книга',
      alternative_translations: ['том'],
      source_language: 'en',
      target_language: 'ru'
    })
    expect(translation.alternatives).toEqual(['том'])

    const translationWithoutAlternatives = wordTranslationSchema.parse({
      original_text: 'hello',
      normalized_form: 'hello',
      translation: 'привет',
      alternatives: null,
      source_language: 'en',
      target_language: 'ru'
    })
    expect(translationWithoutAlternatives.alternatives).toEqual([])
  })

  it('normalizes statistics wrappers and compatibility aliases', () => {
    const overview = overviewSchema.parse({
      active_seconds: 120,
      idle_seconds: 30,
      session_count: 2,
      books_started: 1,
      books_completed: 0,
      words_read: 400,
      pages_read_estimate: 2,
      average_session_seconds: 60,
      dictionary_words: 4,
      dictionary_mastered: 1
    })
    expect(overview).toMatchObject({
      total_reading_seconds: 150,
      sessions_count: 2,
      learned_words: 1
    })

    const daily = dailyStatisticsResponseSchema.parse({
      items: [
        {
          period: '2026-07-15T00:00:00Z',
          active_seconds: 120,
          idle_seconds: 5,
          session_count: 1,
          words_read: 300
        }
      ]
    })
    expect(daily[0]).toMatchObject({ sessions_count: 1, words_read_estimate: 300 })

    const books = bookStatisticsResponseSchema.parse({
      items: [
        { book_id: id, title: 'Book', active_seconds: 120, session_count: 1, progress_percent: 30 }
      ]
    })
    expect(books[0]).toMatchObject({ book_title: 'Book', sessions_count: 1 })
  })

  it('normalizes nullable backend list slices to empty arrays', () => {
    expect(
      booksPageSchema.parse({ items: null, next_cursor: null, has_more: false }).items
    ).toEqual([])
    expect(
      dictionaryPageSchema.parse({ items: null, next_cursor: null, has_more: false }).items
    ).toEqual([])
    expect(
      sessionsPageSchema.parse({ items: null, next_cursor: null, has_more: false }).items
    ).toEqual([])
  })
})
