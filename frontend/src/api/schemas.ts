import { z } from 'zod'

const isoDate = z.string()
const uuid = z.string().min(1)
const responseArray = <T extends z.ZodTypeAny>(item: T) =>
  z.preprocess((value) => value ?? [], z.array(item))
const locatorSchema = z
  .unknown()
  .transform((value) => (typeof value === 'string' ? value : JSON.stringify(value ?? {})))

export const userSchema = z.object({
  id: uuid,
  email: z.string().email(),
  display_name: z.string(),
  locale: z.enum(['ru', 'en']),
  timezone: z.string(),
  created_at: isoDate
})

export const authResponseSchema = z.object({ user: userSchema })

export const bookSchema = z.object({
  id: uuid,
  title: z.string(),
  author: z.string().default(''),
  description: z.string().optional(),
  format: z.enum(['epub', 'fb2', 'txt']),
  language: z.string().default('und'),
  processing_status: z.enum(['uploaded', 'queued', 'processing', 'ready', 'failed']),
  processing_error: z.string().nullish(),
  cover_url: z.string().nullish(),
  has_custom_cover: z.boolean().default(false),
  progress_percent: z.number().min(0).max(100).default(0),
  current_chapter_id: uuid.nullish(),
  estimated_minutes_remaining: z.number().nullish(),
  is_favorite: z.boolean().default(false),
  tags: z.array(z.string()).default([]),
  added_at: isoDate,
  last_read_at: isoDate.nullish(),
  updated_at: isoDate
})

export const bookUploadSchema = z
  .union([bookSchema, z.object({ book: bookSchema, duplicate: z.boolean().optional() })])
  .transform((value) => ('book' in value ? value.book : value))

export const booksPageSchema = z
  .object({
    items: responseArray(bookSchema),
    next_cursor: z.string().nullish(),
    next_offset: z.number().optional(),
    has_more: z.boolean().default(false),
    total_count: z.number().optional(),
    total: z.number().optional()
  })
  .transform((value) => ({
    items: value.items,
    next_cursor:
      value.next_cursor ??
      (value.has_more && value.next_offset !== undefined ? String(value.next_offset) : null),
    has_more: value.has_more,
    total_count: value.total_count ?? value.total
  }))

const tocEntrySchema = z
  .object({
    id: uuid,
    chapter_id: uuid.optional(),
    title: z.string(),
    level: z.number().optional(),
    order: z.number().optional(),
    ordinal: z.number().optional(),
    children: z.array(z.unknown()).optional()
  })
  .transform((value) => ({
    id: value.id,
    chapter_id: value.chapter_id ?? value.id,
    title: value.title,
    level: value.level ?? 0,
    order: value.order ?? value.ordinal ?? 0
  }))

export const tocSchema = z
  .union([responseArray(tocEntrySchema), z.object({ items: responseArray(tocEntrySchema) })])
  .transform((value) => (Array.isArray(value) ? value : value.items))

export const chapterSchema = z
  .object({
    id: uuid,
    book_id: uuid,
    title: z.string(),
    order: z.number().optional(),
    ordinal: z.number().optional(),
    html: z.string().optional(),
    content_html: z.string().optional(),
    plain_text: z.string().optional(),
    content_text: z.string().optional(),
    previous_chapter_id: uuid.nullish(),
    next_chapter_id: uuid.nullish(),
    word_count: z.number().optional()
  })
  .transform((value) => ({
    id: value.id,
    book_id: value.book_id,
    title: value.title,
    order: value.order ?? value.ordinal ?? 0,
    html: value.html ?? value.content_html ?? '',
    plain_text: value.plain_text ?? value.content_text,
    previous_chapter_id: value.previous_chapter_id,
    next_chapter_id: value.next_chapter_id,
    word_count: value.word_count
  }))

export const progressSchema = z.object({
  book_id: uuid,
  chapter_id: uuid.nullish(),
  locator_type: z
    .enum(['chapter_offset', 'epub_cfi'])
    .or(z.literal(''))
    .optional()
    .transform((value) => value || 'chapter_offset'),
  locator: locatorSchema,
  character_offset: z.number().default(0),
  text_anchor: z.string().optional(),
  progress_percent: z.number().default(0),
  scroll_percent: z.number().default(0),
  revision: z.number().default(0),
  client_id: z.string().default(''),
  device_id: uuid.optional(),
  server_timestamp: isoDate.optional(),
  updated_at: isoDate
})

const fontSchema = z
  .enum([
    'system',
    'serif',
    'Georgia',
    'Arial',
    'Inter',
    'Source Serif 4',
    'georgia',
    'arial',
    'inter',
    'source-serif',
    'system-ui'
  ])
  .transform((value) => {
    if (value === 'georgia') return 'Georgia' as const
    if (value === 'arial') return 'Arial' as const
    if (value === 'inter') return 'Inter' as const
    if (value === 'source-serif') return 'Source Serif 4' as const
    if (value === 'system-ui') return 'system' as const
    return value
  })

export const preferencesSchema = z.object({
  theme: z.enum(['light', 'warm', 'sepia', 'dark', 'custom']),
  background_color: z.string(),
  text_color: z.string(),
  accent_color: z.string(),
  font_family: fontSchema,
  font_size: z.number(),
  font_weight: z.number().transform((value) => (value >= 550 ? 600 : value >= 450 ? 500 : 400)),
  line_height: z.number(),
  letter_spacing: z.number().transform((value) => Math.max(-0.02, Math.min(0.08, value))),
  content_width: z.number(),
  page_margin: z.number(),
  text_align: z.enum(['left', 'justify']),
  reading_mode: z.enum(['scroll', 'paged']),
  show_progress: z.boolean(),
  show_remaining_time: z.boolean(),
  controls_brightness: z.number()
})

const closeReasonSchema = z.preprocess(
  (value) => (value === '' || value === null ? undefined : value),
  z
    .enum([
      'user_closed_reader',
      'switched_book',
      'app_backgrounded',
      'logout',
      'idle_timeout',
      'connection_lost',
      'stale_session_finalized',
      'book_finished',
      'server_shutdown',
      'unknown'
    ])
    .optional()
)

export const sessionSchema = z.object({
  id: uuid,
  book_id: uuid,
  device_id: uuid.nullish(),
  started_at: isoDate,
  last_activity_at: isoDate,
  last_heartbeat_at: isoDate.optional(),
  ended_at: isoDate.nullish(),
  active_seconds: z.number().default(0),
  idle_seconds: z.number().default(0),
  start_progress_percent: z.number().default(0),
  end_progress_percent: z.number().optional(),
  words_read_estimate: z.number().default(0),
  pages_read_estimate: z.number().default(0),
  status: z.enum(['active', 'idle', 'finished', 'stale', 'finalized']),
  close_reason: closeReasonSchema
})

export const wordTranslationSchema = z
  .object({
    original_text: z.string(),
    normalized_form: z.string(),
    lemma: z.string().optional(),
    translation: z.string(),
    transcription: z.string().optional(),
    part_of_speech: z.string().optional(),
    definition: z.string().optional(),
    alternatives: responseArray(z.string()).optional(),
    alternative_translations: responseArray(z.string()).optional(),
    example: z.string().optional(),
    source_language: z.string(),
    target_language: z.string(),
    confidence: z.number().optional(),
    cached: z.boolean().optional()
  })
  .transform((value) => ({
    ...value,
    alternatives: value.alternatives ?? value.alternative_translations ?? []
  }))

export const textTranslationSchema = z
  .object({
    original_text: z.string().optional(),
    original: z.string().optional(),
    translation: z.string(),
    detected_language: z.string(),
    explanation: z.string().optional(),
    cached: z.boolean().optional()
  })
  .transform((value) => ({
    original_text: value.original_text ?? value.original ?? '',
    translation: value.translation,
    detected_language: value.detected_language,
    explanation: value.explanation,
    cached: value.cached
  }))

export const dictionaryEntrySchema = z.object({
  id: uuid,
  source_language: z.string(),
  target_language: z.string(),
  original_word: z.string(),
  normalized_word: z.string(),
  lemma: z.string().optional(),
  transcription: z.string().optional(),
  part_of_speech: z.string().optional(),
  translation: z.string(),
  alternative_translations: responseArray(z.string()),
  definition: z.string().optional(),
  note: z.string().optional(),
  status: z.enum(['unknown', 'learning', 'known', 'mastered', 'ignored']),
  encounter_count: z.number().default(1),
  first_seen_at: isoDate,
  last_seen_at: isoDate,
  next_review_at: isoDate.nullish(),
  book_id: uuid.optional(),
  book_title: z.string().optional(),
  created_at: isoDate,
  updated_at: isoDate
})

export const dictionaryPageSchema = z
  .object({
    items: responseArray(dictionaryEntrySchema),
    next_cursor: z.string().nullish(),
    next_offset: z.number().optional(),
    has_more: z.boolean().default(false),
    total_count: z.number().optional(),
    total: z.number().optional()
  })
  .transform((value) => ({
    items: value.items,
    next_cursor:
      value.next_cursor ??
      (value.has_more && value.next_offset !== undefined ? String(value.next_offset) : null),
    has_more: value.has_more,
    total_count: value.total_count ?? value.total
  }))

export const bookmarkSchema = z.object({
  id: uuid,
  book_id: uuid,
  chapter_id: uuid.nullish(),
  locator: locatorSchema,
  progress_percent: z.number(),
  title: z.string(),
  note: z.string().optional(),
  created_at: isoDate
})

const highlightColorSchema = z
  .enum(['sand', 'sage', 'blue', 'rose', 'yellow', 'green', 'pink', 'purple', 'gray'])
  .transform((value) => {
    if (value === 'yellow' || value === 'gray') return 'sand' as const
    if (value === 'green') return 'sage' as const
    if (value === 'pink' || value === 'purple') return 'rose' as const
    return value
  })

export const highlightSchema = z.object({
  id: uuid,
  book_id: uuid,
  book_title: z.string().optional(),
  chapter_id: uuid.nullish(),
  locator: locatorSchema,
  text_anchor: z.string(),
  selected_text: z.string(),
  context: z.string().optional(),
  color: highlightColorSchema,
  note: z.string().optional(),
  created_at: isoDate,
  updated_at: isoDate
})

export const noteBlockSchema = z.object({
  id: uuid,
  type: z.enum([
    'paragraph',
    'heading1',
    'heading2',
    'heading3',
    'bulleted_list',
    'numbered_list',
    'task',
    'quote',
    'callout',
    'divider',
    'link',
    'book_link',
    'saved_quote'
  ]),
  text: z.string().optional(),
  checked: z.boolean().optional(),
  url: z.string().optional(),
  book_id: uuid.optional(),
  locator: z.string().optional()
})

export const noteSchema = z.object({
  id: uuid,
  title: z.string(),
  book_id: uuid.nullish(),
  book_title: z.string().optional(),
  highlight_id: uuid.nullish(),
  schema_version: z.literal(1),
  blocks: z.array(noteBlockSchema),
  created_at: isoDate,
  updated_at: isoDate
})

export const overviewSchema = z
  .object({
    total_reading_seconds: z.number().optional(),
    active_reading_seconds: z.number().optional(),
    active_seconds: z.number().optional(),
    idle_seconds: z.number().default(0),
    sessions_count: z.number().optional(),
    session_count: z.number().optional(),
    books_started: z.number().default(0),
    books_completed: z.number().default(0),
    words_read_estimate: z.number().optional(),
    words_read: z.number().optional(),
    pages_read_estimate: z.number().optional(),
    pages_read: z.number().optional(),
    current_streak_days: z.number().optional(),
    longest_streak_days: z.number().optional(),
    average_session_seconds: z.number().default(0),
    median_session_seconds: z.number().default(0),
    average_words_per_minute: z.number().default(0),
    dictionary_words: z.number().default(0),
    learned_words: z.number().optional(),
    dictionary_mastered: z.number().optional(),
    translations_count: z.number().default(0)
  })
  .transform((value) => {
    const active = value.active_reading_seconds ?? value.active_seconds ?? 0
    return {
      total_reading_seconds: value.total_reading_seconds ?? active + value.idle_seconds,
      active_reading_seconds: active,
      idle_seconds: value.idle_seconds,
      sessions_count: value.sessions_count ?? value.session_count ?? 0,
      books_started: value.books_started,
      books_completed: value.books_completed,
      words_read_estimate: value.words_read_estimate ?? value.words_read ?? 0,
      pages_read_estimate: value.pages_read_estimate ?? value.pages_read ?? 0,
      current_streak_days: value.current_streak_days ?? 0,
      longest_streak_days: value.longest_streak_days ?? 0,
      average_session_seconds: value.average_session_seconds,
      median_session_seconds: value.median_session_seconds,
      average_words_per_minute: value.average_words_per_minute,
      dictionary_words: value.dictionary_words,
      learned_words: value.learned_words ?? value.dictionary_mastered ?? 0,
      translations_count: value.translations_count
    }
  })

export const dailyStatisticSchema = z
  .object({
    date: z.string().optional(),
    period: z.string().optional(),
    active_seconds: z.number(),
    idle_seconds: z.number(),
    sessions_count: z.number().optional(),
    session_count: z.number().optional(),
    words_read_estimate: z.number().optional(),
    words_read: z.number().optional()
  })
  .transform((value) => ({
    date: value.date ?? value.period ?? '',
    active_seconds: value.active_seconds,
    idle_seconds: value.idle_seconds,
    sessions_count: value.sessions_count ?? value.session_count ?? 0,
    words_read_estimate: value.words_read_estimate ?? value.words_read ?? 0
  }))

export const dailyStatisticsResponseSchema = z
  .union([
    responseArray(dailyStatisticSchema),
    z.object({ items: responseArray(dailyStatisticSchema) })
  ])
  .transform((value) => (Array.isArray(value) ? value : value.items))

export const bookStatisticSchema = z
  .object({
    book_id: uuid,
    book_title: z.string().optional(),
    title: z.string().optional(),
    active_seconds: z.number(),
    sessions_count: z.number().optional(),
    session_count: z.number().optional(),
    progress_percent: z.number(),
    average_words_per_minute: z.number().default(0),
    last_read_at: isoDate.optional()
  })
  .transform((value) => ({
    book_id: value.book_id,
    book_title: value.book_title ?? value.title ?? '',
    active_seconds: value.active_seconds,
    sessions_count: value.sessions_count ?? value.session_count ?? 0,
    progress_percent: value.progress_percent,
    average_words_per_minute: value.average_words_per_minute,
    last_read_at: value.last_read_at
  }))

export const bookStatisticsResponseSchema = z
  .union([
    responseArray(bookStatisticSchema),
    z.object({ items: responseArray(bookStatisticSchema) })
  ])
  .transform((value) => (Array.isArray(value) ? value : value.items))

function listPageSchema<T extends z.ZodTypeAny>(item: T) {
  return z
    .object({
      items: responseArray(item),
      next_cursor: z.string().nullish(),
      next_offset: z.number().optional(),
      has_more: z.boolean().default(false),
      total_count: z.number().optional()
    })
    .transform((value) => ({
      items: value.items,
      next_cursor:
        value.next_cursor ??
        (value.has_more && value.next_offset !== undefined ? String(value.next_offset) : null),
      has_more: value.has_more,
      total_count: value.total_count
    }))
}

export const sessionsPageSchema = listPageSchema(sessionSchema)
export const bookmarksPageSchema = listPageSchema(bookmarkSchema)
export const highlightsPageSchema = listPageSchema(highlightSchema)
export const notesPageSchema = listPageSchema(noteSchema)
