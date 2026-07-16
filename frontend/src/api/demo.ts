import type { RequestOptions } from './http'
import type {
  Book,
  Bookmark,
  Chapter,
  CreateDictionaryEntry,
  DailyStatistic,
  DictionaryEntry,
  Highlight,
  Note,
  ReaderPreferences,
  ReadingProgress,
  ReadingSession,
  StatisticsOverview,
  TocItem,
  WordTranslation
} from '../types/api'

const now = new Date('2026-07-15T14:25:00.000Z')
const isoDaysAgo = (days: number) => new Date(now.getTime() - days * 86_400_000).toISOString()
const uuid = (suffix: string) => `019f670d-13bd-7bc3-94fb-${suffix.padStart(12, '0')}`

const chapterOne = uuid('101')
const chapterTwo = uuid('102')
const chapterThree = uuid('103')

let books: Book[] = [
  {
    id: uuid('1'),
    title: 'The Quiet Observatory',
    author: 'Mara Ellison',
    description:
      'A public-domain-inspired short story about patient observation, distance, and the maps we make for ourselves.',
    format: 'epub',
    language: 'en',
    processing_status: 'ready',
    cover_url: null,
    progress_percent: 38,
    current_chapter_id: chapterTwo,
    estimated_minutes_remaining: 74,
    is_favorite: true,
    tags: ['English', 'Fiction'],
    added_at: isoDaysAgo(21),
    last_read_at: isoDaysAgo(1),
    updated_at: isoDaysAgo(1)
  },
  {
    id: uuid('2'),
    title: 'Записки о внимании',
    author: 'Алексей Мирный',
    description: 'Небольшой сборник синтетических эссе для демонстрации BookFlow.',
    format: 'fb2',
    language: 'ru',
    processing_status: 'ready',
    cover_url: null,
    progress_percent: 67,
    current_chapter_id: uuid('201'),
    estimated_minutes_remaining: 31,
    is_favorite: false,
    tags: ['Эссе'],
    added_at: isoDaysAgo(12),
    last_read_at: isoDaysAgo(3),
    updated_at: isoDaysAgo(3)
  },
  {
    id: uuid('3'),
    title: 'A Field Guide to Small Habits',
    author: 'N. Rowan',
    format: 'txt',
    language: 'en',
    processing_status: 'processing',
    cover_url: null,
    progress_percent: 0,
    estimated_minutes_remaining: null,
    is_favorite: false,
    tags: [],
    added_at: isoDaysAgo(0),
    last_read_at: null,
    updated_at: isoDaysAgo(0)
  }
]

const tocByBook: Record<string, TocItem[]> = {
  [uuid('1')]: [
    { id: uuid('1001'), chapter_id: chapterOne, title: 'I. The lamp', level: 1, order: 1 },
    { id: uuid('1002'), chapter_id: chapterTwo, title: 'II. A patient orbit', level: 1, order: 2 },
    { id: uuid('1003'), chapter_id: chapterThree, title: 'III. Coordinates', level: 1, order: 3 }
  ],
  [uuid('2')]: [
    { id: uuid('2001'), chapter_id: uuid('201'), title: 'Тишина', level: 1, order: 1 },
    { id: uuid('2002'), chapter_id: uuid('202'), title: 'Наблюдение', level: 1, order: 2 }
  ]
}

const chapters: Record<string, Chapter> = {
  [chapterOne]: {
    id: chapterOne,
    book_id: uuid('1'),
    title: 'I. The lamp',
    order: 1,
    previous_chapter_id: null,
    next_chapter_id: chapterTwo,
    word_count: 842,
    html: `<p>At dusk, Elian climbed the narrow stairs to the observatory. The brass key was cold, and the old door resisted him as if the room had learned to prefer silence.</p><p>He lit the reading lamp beside the charts. Beyond the glass, the first stars arrived without ceremony. They seemed patient enough to wait for anyone who knew how to look.</p><h2>A map without names</h2><p>The newest chart had no labels. Its maker had drawn only distances: a sparse geometry of ink, careful and unfinished. Elian placed it beneath the lens and began.</p>`
  },
  [chapterTwo]: {
    id: chapterTwo,
    book_id: uuid('1'),
    title: 'II. A patient orbit',
    order: 2,
    previous_chapter_id: chapterOne,
    next_chapter_id: chapterThree,
    word_count: 1160,
    html: `<p>The object returned every eighty-one minutes. It was too regular for weather and too faint for the catalog. Elian wrote the times in the margin and resisted the temptation to give it a name.</p><p>Observation, his teacher once said, was an agreement to let the world remain <em>unfamiliar</em> for a little longer. Certainty could be useful, but patience revealed the shape beneath it.</p><blockquote>Attend first to what changes. Then attend to what stays.</blockquote><p>On the fourth night, a thin cloud crossed the eastern window. The object disappeared, but its absence kept the same rhythm. Elian smiled at the empty glass. A pattern could continue even when it could not be seen.</p><h2>The interval</h2><p>He walked around the room whenever his eyes tired. He touched the spines of books without opening them, listened to the clock, and returned only when the stars felt distinct again.</p><p>Near dawn, he understood that the orbit was not around the world shown on the chart. The page itself had been turning beneath a fixed light. The mystery was smaller than he had hoped and more beautiful than he expected.</p>`
  },
  [chapterThree]: {
    id: chapterThree,
    book_id: uuid('1'),
    title: 'III. Coordinates',
    order: 3,
    previous_chapter_id: chapterTwo,
    next_chapter_id: null,
    word_count: 930,
    html: `<p>Elian added one line to the chart: not a name, but a coordinate. It connected the measured sky to the moving desk, the patient star to the person who had noticed it.</p><p>By breakfast, the observatory looked ordinary again. Dust rested on the rail. The lamp had gone cool. Yet his small correction changed every observation that would follow.</p>`
  },
  [uuid('201')]: {
    id: uuid('201'),
    book_id: uuid('2'),
    title: 'Тишина',
    order: 1,
    previous_chapter_id: null,
    next_chapter_id: uuid('202'),
    word_count: 630,
    html: '<p>Тишина — не отсутствие звука, а место, где звук становится заметным. Мы редко оставляем для неё достаточно пространства.</p><p>Внимание начинается не с усилия, а с решения не спешить.</p>'
  },
  [uuid('202')]: {
    id: uuid('202'),
    book_id: uuid('2'),
    title: 'Наблюдение',
    order: 2,
    previous_chapter_id: uuid('201'),
    next_chapter_id: null,
    word_count: 710,
    html: '<p>Наблюдать — значит допустить, что первое объяснение может быть не единственным.</p>'
  }
}

let preferences: ReaderPreferences = {
  theme: 'warm',
  background_color: '#f8f1df',
  text_color: '#302d27',
  accent_color: '#3f6658',
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
  controls_brightness: 0.9
}

let progress: ReadingProgress = {
  book_id: uuid('1'),
  chapter_id: chapterTwo,
  locator_type: 'chapter_offset',
  locator: 'chapter:2:offset:312',
  character_offset: 312,
  text_anchor: 'Observation, his teacher once said',
  progress_percent: 38,
  scroll_percent: 27,
  revision: 7,
  client_id: 'demo-web-client',
  updated_at: isoDaysAgo(1),
  server_timestamp: isoDaysAgo(1)
}

let dictionary: DictionaryEntry[] = [
  {
    id: uuid('301'),
    source_language: 'en',
    target_language: 'ru',
    original_word: 'unfamiliar',
    normalized_word: 'unfamiliar',
    lemma: 'unfamiliar',
    transcription: '/ˌʌnfəˈmɪliə/',
    part_of_speech: 'adjective',
    translation: 'незнакомый',
    alternative_translations: ['непривычный', 'неизвестный'],
    definition: 'Not known or recognized.',
    status: 'learning',
    encounter_count: 3,
    first_seen_at: isoDaysAgo(8),
    last_seen_at: isoDaysAgo(1),
    next_review_at: isoDaysAgo(-1),
    book_id: uuid('1'),
    book_title: 'The Quiet Observatory',
    created_at: isoDaysAgo(8),
    updated_at: isoDaysAgo(1)
  },
  {
    id: uuid('302'),
    source_language: 'en',
    target_language: 'ru',
    original_word: 'beneath',
    normalized_word: 'beneath',
    lemma: 'beneath',
    transcription: '/bɪˈniːθ/',
    part_of_speech: 'preposition',
    translation: 'под, ниже',
    alternative_translations: ['внизу'],
    definition: 'Extending or directly underneath.',
    status: 'known',
    encounter_count: 5,
    first_seen_at: isoDaysAgo(14),
    last_seen_at: isoDaysAgo(4),
    next_review_at: null,
    book_id: uuid('1'),
    book_title: 'The Quiet Observatory',
    created_at: isoDaysAgo(14),
    updated_at: isoDaysAgo(4)
  },
  {
    id: uuid('303'),
    source_language: 'en',
    target_language: 'ru',
    original_word: 'sparse',
    normalized_word: 'sparse',
    lemma: 'sparse',
    transcription: '/spɑːs/',
    part_of_speech: 'adjective',
    translation: 'редкий, разреженный',
    alternative_translations: ['скудный'],
    definition: 'Small in number and spread over an area.',
    status: 'unknown',
    encounter_count: 1,
    first_seen_at: isoDaysAgo(2),
    last_seen_at: isoDaysAgo(2),
    next_review_at: isoDaysAgo(0),
    book_id: uuid('1'),
    book_title: 'The Quiet Observatory',
    created_at: isoDaysAgo(2),
    updated_at: isoDaysAgo(2)
  }
]

let bookmarks: Bookmark[] = [
  {
    id: uuid('401'),
    book_id: uuid('1'),
    chapter_id: chapterTwo,
    locator: 'chapter:2:offset:490',
    progress_percent: 41,
    title: 'The rhythm of absence',
    note: 'Return to this image.',
    created_at: isoDaysAgo(1)
  }
]

let highlights: Highlight[] = [
  {
    id: uuid('501'),
    book_id: uuid('1'),
    book_title: 'The Quiet Observatory',
    chapter_id: chapterTwo,
    locator: 'chapter:2:offset:174',
    text_anchor: 'let the world remain unfamiliar',
    selected_text: 'let the world remain unfamiliar for a little longer',
    context: 'Observation, his teacher once said, was an agreement to…',
    color: 'sand',
    note: 'Patience before categorisation.',
    created_at: isoDaysAgo(2),
    updated_at: isoDaysAgo(2)
  }
]

let notes: Note[] = [
  {
    id: uuid('601'),
    title: 'On patient observation',
    book_id: uuid('1'),
    book_title: 'The Quiet Observatory',
    highlight_id: uuid('501'),
    schema_version: 1,
    blocks: [
      { id: uuid('610'), type: 'paragraph', text: 'A useful reminder for research and reading.' },
      {
        id: uuid('611'),
        type: 'saved_quote',
        text: 'Attend first to what changes. Then attend to what stays.',
        book_id: uuid('1'),
        locator: 'chapter:2:offset:250'
      }
    ],
    created_at: isoDaysAgo(2),
    updated_at: isoDaysAgo(1)
  }
]

const daily: DailyStatistic[] = Array.from({ length: 14 }, (_, index) => ({
  date: isoDaysAgo(13 - index).slice(0, 10),
  active_seconds:
    [0, 940, 1800, 1220, 0, 2440, 3100, 760, 1520, 0, 1980, 2640, 1320, 2240][index] ?? 0,
  idle_seconds: [0, 90, 160, 110, 0, 280, 320, 50, 120, 0, 140, 190, 100, 170][index] ?? 0,
  sessions_count: [0, 1, 2, 1, 0, 2, 2, 1, 1, 0, 2, 2, 1, 2][index] ?? 0,
  words_read_estimate:
    [0, 610, 1120, 740, 0, 1530, 1910, 480, 920, 0, 1220, 1620, 810, 1410][index] ?? 0
}))

const overview: StatisticsOverview = {
  total_reading_seconds: 23_420,
  active_reading_seconds: 21_180,
  idle_seconds: 2240,
  sessions_count: 18,
  books_started: 4,
  books_completed: 1,
  words_read_estimate: 14_290,
  pages_read_estimate: 51,
  current_streak_days: 4,
  longest_streak_days: 9,
  average_session_seconds: 1177,
  median_session_seconds: 1060,
  average_words_per_minute: 178,
  dictionary_words: 31,
  learned_words: 12,
  translations_count: 67
}

function page<T>(items: T[]) {
  return { items, next_cursor: null, has_more: false, total_count: items.length }
}

function bodyOf<T>(options: RequestOptions): T {
  return options.body as T
}

function findBook(id: string): Book {
  return books.find((book) => book.id === id) ?? books[0]!
}

function demoTranslation(text: string): WordTranslation {
  const key = text.trim().toLowerCase()
  const known: Record<
    string,
    Pick<
      WordTranslation,
      'translation' | 'transcription' | 'part_of_speech' | 'definition' | 'alternatives'
    >
  > = {
    unfamiliar: {
      translation: 'незнакомый',
      transcription: '/ˌʌnfəˈmɪliə/',
      part_of_speech: 'adjective',
      definition: 'Not known or recognized.',
      alternatives: ['непривычный', 'неизвестный']
    },
    patience: {
      translation: 'терпение',
      transcription: '/ˈpeɪʃəns/',
      part_of_speech: 'noun',
      definition: 'The capacity to accept delay without becoming annoyed.',
      alternatives: ['выдержка']
    },
    beneath: {
      translation: 'под, ниже',
      transcription: '/bɪˈniːθ/',
      part_of_speech: 'preposition',
      definition: 'Extending directly underneath.',
      alternatives: ['внизу']
    }
  }
  const match = known[key]
  return {
    original_text: text,
    normalized_form: key,
    lemma: key,
    translation: match?.translation ?? `перевод: ${text}`,
    transcription: match?.transcription,
    part_of_speech: match?.part_of_speech,
    definition: match?.definition,
    alternatives: match?.alternatives ?? [],
    source_language: 'en',
    target_language: 'ru',
    confidence: 0.92,
    cached: Boolean(match)
  }
}

export async function demoRequest(path: string, options: RequestOptions): Promise<unknown> {
  await new Promise((resolve) => setTimeout(resolve, 120))
  const method = (options.method || 'GET').toUpperCase()
  const url = new URL(path, 'https://demo.bookflow.local')
  const pathname = url.pathname

  if (pathname === '/auth/me') {
    return {
      user: {
        id: uuid('900'),
        email: 'reader@bookflow.demo',
        display_name: 'Анна',
        locale: 'ru',
        timezone: 'Asia/Yekaterinburg',
        created_at: isoDaysAgo(90)
      }
    }
  }
  if (pathname === '/auth/login' || pathname === '/auth/register')
    return demoRequest('/auth/me', {})
  if (pathname.startsWith('/auth/')) return undefined

  if (pathname === '/books' && method === 'GET') {
    const search = (url.searchParams.get('search') || '').toLowerCase()
    const status = url.searchParams.get('status')
    const format = url.searchParams.get('format')
    const filtered = books.filter(
      (book) =>
        (!search || `${book.title} ${book.author}`.toLowerCase().includes(search)) &&
        (!status || status === 'all' || book.processing_status === status) &&
        (!format || format === 'all' || book.format === format)
    )
    return page(filtered)
  }
  if (pathname === '/books' && method === 'POST') {
    const form = options.body instanceof FormData ? options.body : undefined
    const file = form?.get('file')
    const title =
      file instanceof File ? file.name.replace(/\.(epub|fb2|txt)$/i, '') : 'Uploaded book'
    const book: Book = {
      id: uuid(String(books.length + 10)),
      title,
      author: 'Processing metadata…',
      format:
        file instanceof File && file.name.toLowerCase().endsWith('.fb2')
          ? 'fb2'
          : file instanceof File && file.name.toLowerCase().endsWith('.txt')
            ? 'txt'
            : 'epub',
      language: 'und',
      processing_status: 'queued',
      cover_url: null,
      progress_percent: 0,
      estimated_minutes_remaining: null,
      is_favorite: false,
      tags: [],
      added_at: now.toISOString(),
      last_read_at: null,
      updated_at: now.toISOString()
    }
    books = [book, ...books]
    return book
  }

  const chapterMatch = pathname.match(/^\/books\/([^/]+)\/chapters\/([^/]+)$/)
  if (chapterMatch) return chapters[chapterMatch[2]!] ?? chapters[chapterTwo]
  const tocMatch = pathname.match(/^\/books\/([^/]+)\/toc$/)
  if (tocMatch) return tocByBook[tocMatch[1]!] ?? []
  const progressMatch = pathname.match(/^\/books\/([^/]+)\/progress$/)
  if (progressMatch) {
    if (method === 'PUT') {
      const update = bodyOf<Partial<ReadingProgress>>(options)
      progress = {
        ...progress,
        ...update,
        book_id: progressMatch[1]!,
        revision: (update.revision ?? progress.revision) + 1,
        updated_at: now.toISOString(),
        server_timestamp: now.toISOString()
      }
    }
    return { ...progress, book_id: progressMatch[1]! }
  }
  const preferencesMatch = pathname.match(/^\/books\/([^/]+)\/reader-preferences$/)
  if (preferencesMatch) {
    if (method === 'PUT')
      preferences = { ...preferences, ...bodyOf<Partial<ReaderPreferences>>(options) }
    return preferences
  }
  if (pathname === '/reader/preferences') {
    if (method === 'PUT')
      preferences = { ...preferences, ...bodyOf<Partial<ReaderPreferences>>(options) }
    return preferences
  }
  const bookmarkListMatch = pathname.match(/^\/books\/([^/]+)\/bookmarks$/)
  if (bookmarkListMatch) {
    if (method === 'POST') {
      const value = bodyOf<Omit<Bookmark, 'id' | 'created_at' | 'book_id'>>(options)
      const item = {
        ...value,
        id: uuid(String(420 + bookmarks.length)),
        book_id: bookmarkListMatch[1]!,
        created_at: now.toISOString()
      }
      bookmarks = [item, ...bookmarks]
      return item
    }
    return page(bookmarks.filter((item) => item.book_id === bookmarkListMatch[1]))
  }
  const highlightListMatch = pathname.match(/^\/books\/([^/]+)\/highlights$/)
  if (highlightListMatch) {
    if (method === 'POST') {
      const value = bodyOf<Omit<Highlight, 'id' | 'created_at' | 'updated_at' | 'book_id'>>(options)
      const item = {
        ...value,
        id: uuid(String(520 + highlights.length)),
        book_id: highlightListMatch[1]!,
        created_at: now.toISOString(),
        updated_at: now.toISOString()
      }
      highlights = [item, ...highlights]
      return item
    }
    return page(highlights.filter((item) => item.book_id === highlightListMatch[1]))
  }
  const bookIdMatch = pathname.match(/^\/books\/([^/]+)$/)
  if (bookIdMatch) {
    const id = bookIdMatch[1]!
    if (method === 'DELETE') {
      books = books.filter((book) => book.id !== id)
      return undefined
    }
    if (method === 'PATCH') {
      books = books.map((book) =>
        book.id === id ? { ...book, ...bodyOf<Partial<Book>>(options) } : book
      )
    }
    return findBook(id)
  }

  if (pathname === '/reading-sessions' && method === 'POST') {
    const input = bodyOf<{ book_id: string; progress_percent: number }>(options)
    const session: ReadingSession = {
      id: uuid('701'),
      book_id: input.book_id,
      started_at: now.toISOString(),
      last_activity_at: now.toISOString(),
      last_heartbeat_at: now.toISOString(),
      ended_at: null,
      active_seconds: 0,
      idle_seconds: 0,
      start_progress_percent: input.progress_percent,
      words_read_estimate: 0,
      pages_read_estimate: 0,
      status: 'active'
    }
    return session
  }
  if (/^\/reading-sessions\/[^/]+\/heartbeat$/.test(pathname)) {
    return {
      id: uuid('701'),
      book_id: uuid('1'),
      started_at: now.toISOString(),
      last_activity_at: now.toISOString(),
      last_heartbeat_at: now.toISOString(),
      ended_at: null,
      active_seconds: 15,
      idle_seconds: 0,
      start_progress_percent: 38,
      words_read_estimate: 40,
      pages_read_estimate: 0.2,
      status: 'active'
    }
  }
  if (/^\/reading-sessions\/[^/]+\/finish$/.test(pathname)) return undefined
  if (pathname === '/reading-sessions') {
    return page([
      {
        id: uuid('710'),
        book_id: uuid('1'),
        started_at: isoDaysAgo(1),
        last_activity_at: isoDaysAgo(1),
        last_heartbeat_at: isoDaysAgo(1),
        ended_at: isoDaysAgo(1),
        active_seconds: 2240,
        idle_seconds: 170,
        start_progress_percent: 31,
        end_progress_percent: 38,
        words_read_estimate: 1410,
        pages_read_estimate: 5,
        status: 'finished',
        close_reason: 'user_closed_reader'
      }
    ])
  }

  if (pathname === '/translations/word') {
    const request = bodyOf<{ text: string }>(options)
    return demoTranslation(request.text)
  }
  if (pathname === '/translations/text') {
    const request = bodyOf<{ text: string }>(options)
    return {
      original_text: request.text,
      translation: `Перевод: ${request.text}`,
      detected_language: 'en',
      cached: false
    }
  }

  if (pathname === '/dictionary' && method === 'GET') {
    const search = (url.searchParams.get('search') || '').toLowerCase()
    const status = url.searchParams.get('status')
    return page(
      dictionary.filter(
        (entry) =>
          (!search ||
            `${entry.original_word} ${entry.translation}`.toLowerCase().includes(search)) &&
          (!status || status === 'all' || entry.status === status)
      )
    )
  }
  if (pathname === '/dictionary' && method === 'POST') {
    const input = bodyOf<CreateDictionaryEntry>(options)
    const existing = dictionary.find((entry) => entry.normalized_word === input.normalized_word)
    if (existing) {
      existing.encounter_count += 1
      existing.last_seen_at = now.toISOString()
      return existing
    }
    const entry: DictionaryEntry = {
      ...input,
      id: uuid(String(330 + dictionary.length)),
      alternative_translations: input.alternative_translations ?? [],
      status: input.status ?? 'unknown',
      encounter_count: 1,
      first_seen_at: now.toISOString(),
      last_seen_at: now.toISOString(),
      next_review_at: null,
      book_id: input.occurrence?.book_id,
      book_title: input.occurrence ? findBook(input.occurrence.book_id).title : undefined,
      created_at: now.toISOString(),
      updated_at: now.toISOString()
    }
    dictionary = [entry, ...dictionary]
    return entry
  }
  const dictionaryMatch = pathname.match(/^\/dictionary\/([^/]+)$/)
  if (dictionaryMatch) {
    const id = dictionaryMatch[1]!
    const index = dictionary.findIndex((entry) => entry.id === id)
    if (method === 'PATCH' && index >= 0) {
      dictionary[index] = {
        ...dictionary[index]!,
        ...bodyOf<Partial<DictionaryEntry>>(options),
        updated_at: now.toISOString()
      }
    }
    if (method === 'DELETE' && index >= 0) dictionary.splice(index, 1)
    return index >= 0 ? dictionary[index] : dictionary[0]
  }

  if (pathname === '/notes') {
    if (method === 'POST') {
      const input = bodyOf<Pick<Note, 'title' | 'blocks'> & Partial<Note>>(options)
      const item: Note = {
        id: uuid(String(630 + notes.length)),
        title: input.title,
        blocks: input.blocks,
        book_id: input.book_id,
        book_title: input.book_title,
        highlight_id: input.highlight_id,
        schema_version: 1,
        created_at: now.toISOString(),
        updated_at: now.toISOString()
      }
      notes = [item, ...notes]
      return item
    }
    return page(notes)
  }
  if (pathname === '/highlights') return page(highlights)

  if (pathname === '/statistics/overview') return overview
  if (pathname === '/statistics/daily') return daily
  if (pathname === '/statistics/books') {
    return books
      .filter((book) => book.progress_percent > 0)
      .map((book, index) => ({
        book_id: book.id,
        book_title: book.title,
        active_seconds: 12_400 - index * 4200,
        sessions_count: 11 - index * 4,
        progress_percent: book.progress_percent,
        average_words_per_minute: 178 - index * 12,
        last_read_at: book.last_read_at ?? undefined
      }))
  }

  throw new Error(`Demo transport does not implement ${method} ${pathname}`)
}
