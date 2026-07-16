import { z } from 'zod'
import { apiRequest, apiRequestVoid, queryString } from './http'
import type { DeviceIdentity } from './device'
import {
  authResponseSchema,
  bookmarkSchema,
  bookmarksPageSchema,
  bookSchema,
  bookStatisticsResponseSchema,
  bookUploadSchema,
  booksPageSchema,
  chapterSchema,
  dailyStatisticsResponseSchema,
  dictionaryEntrySchema,
  dictionaryPageSchema,
  highlightSchema,
  highlightsPageSchema,
  noteSchema,
  notesPageSchema,
  overviewSchema,
  preferencesSchema,
  progressSchema,
  sessionSchema,
  sessionsPageSchema,
  textTranslationSchema,
  tocSchema,
  wordTranslationSchema
} from './schemas'
import type {
  AuthResponse,
  Book,
  Bookmark,
  BooksQuery,
  BookStatistic,
  Chapter,
  CreateDictionaryEntry,
  CursorPage,
  DailyStatistic,
  DictionaryEntry,
  DictionaryQuery,
  FinishSessionInput,
  HeartbeatInput,
  Highlight,
  Note,
  ProgressUpdate,
  ReaderPreferences,
  ReadingProgress,
  ReadingSession,
  StartSessionInput,
  StatisticsOverview,
  TextTranslation,
  TocItem,
  TranslationRequest,
  User,
  WordTranslation
} from '../types/api'

const id = (value: string) => encodeURIComponent(value)

export const authApi = {
  me: (): Promise<AuthResponse> => apiRequest('/auth/me', authResponseSchema),
  login: (input: { email: string; password: string } & DeviceIdentity): Promise<AuthResponse> =>
    apiRequest('/auth/login', authResponseSchema, {
      method: 'POST',
      body: input,
      retryAuth: false
    }),
  register: (
    input: {
      email: string
      password: string
      display_name: string
      locale: 'ru' | 'en'
      timezone: string
    } & DeviceIdentity
  ): Promise<AuthResponse> =>
    apiRequest('/auth/register', authResponseSchema, {
      method: 'POST',
      body: input,
      retryAuth: false
    }),
  logout: (): Promise<void> => apiRequestVoid('/auth/logout', { method: 'POST', retryAuth: false }),
  logoutAll: (): Promise<void> =>
    apiRequestVoid('/auth/logout-all', { method: 'POST', retryAuth: false }),
  refresh: (): Promise<AuthResponse> =>
    apiRequest('/auth/refresh', authResponseSchema, { method: 'POST', retryAuth: false })
}

export const booksApi = {
  list: (query: BooksQuery = {}): Promise<CursorPage<Book>> =>
    apiRequest(
      `/books${queryString({
        search: query.search,
        status: query.status,
        format: query.format,
        sort: query.sort,
        favorite: query.favorite,
        cursor: query.cursor,
        limit: query.limit
      })}`,
      booksPageSchema
    ),
  get: (bookId: string): Promise<Book> => apiRequest(`/books/${id(bookId)}`, bookSchema),
  upload: (file: File): Promise<Book> => {
    const form = new FormData()
    form.set('file', file)
    return apiRequest('/books', bookUploadSchema, { method: 'POST', body: form })
  },
  update: (
    bookId: string,
    input: Partial<
      Pick<Book, 'title' | 'author' | 'description' | 'language' | 'is_favorite' | 'tags'>
    >
  ): Promise<Book> =>
    apiRequest(`/books/${id(bookId)}`, bookSchema, { method: 'PATCH', body: input }),
  updateCover: (bookId: string, file: File): Promise<Book> => {
    const form = new FormData()
    form.set('file', file)
    return apiRequest(`/books/${id(bookId)}/cover`, bookSchema, { method: 'PUT', body: form })
  },
  removeCover: (bookId: string): Promise<Book> =>
    apiRequest(`/books/${id(bookId)}/cover`, bookSchema, { method: 'DELETE' }),
  remove: (bookId: string): Promise<void> =>
    apiRequestVoid(`/books/${id(bookId)}`, { method: 'DELETE' }),
  reprocess: (bookId: string): Promise<void> =>
    apiRequestVoid(`/books/${id(bookId)}/reprocess`, { method: 'POST' }),
  toc: (bookId: string): Promise<TocItem[]> =>
    apiRequest(`/books/${id(bookId)}/toc`, tocSchema as z.ZodType<TocItem[]>),
  chapter: (bookId: string, chapterId: string): Promise<Chapter> =>
    apiRequest(`/books/${id(bookId)}/chapters/${id(chapterId)}`, chapterSchema),
  progress: (bookId: string): Promise<ReadingProgress> =>
    apiRequest(`/books/${id(bookId)}/progress`, progressSchema),
  updateProgress: (bookId: string, input: ProgressUpdate): Promise<ReadingProgress> =>
    apiRequest(`/books/${id(bookId)}/progress`, progressSchema, { method: 'PUT', body: input }),
  preferences: (bookId?: string): Promise<ReaderPreferences> =>
    apiRequest(
      bookId ? `/books/${id(bookId)}/reader-preferences` : '/reader/preferences',
      preferencesSchema
    ),
  updatePreferences: (input: ReaderPreferences, bookId?: string): Promise<ReaderPreferences> =>
    apiRequest(
      bookId ? `/books/${id(bookId)}/reader-preferences` : '/reader/preferences',
      preferencesSchema,
      {
        method: 'PUT',
        body: preferencesPayload(input)
      }
    ),
  download: (bookId: string): Promise<string> =>
    apiRequest(`/books/${id(bookId)}/download`, z.object({ url: z.string().url() })).then(
      (response) => response.url
    ),
  bookmarks: (bookId: string): Promise<CursorPage<Bookmark>> =>
    apiRequest(`/books/${id(bookId)}/bookmarks`, bookmarksPageSchema),
  addBookmark: (
    bookId: string,
    input: Omit<Bookmark, 'id' | 'book_id' | 'created_at'>
  ): Promise<Bookmark> =>
    apiRequest(`/books/${id(bookId)}/bookmarks`, bookmarkSchema, { method: 'POST', body: input }),
  highlights: (bookId: string): Promise<CursorPage<Highlight>> =>
    apiRequest(`/books/${id(bookId)}/highlights`, highlightsPageSchema),
  addHighlight: (
    bookId: string,
    input: Omit<Highlight, 'id' | 'book_id' | 'book_title' | 'created_at' | 'updated_at'>
  ): Promise<Highlight> =>
    apiRequest(`/books/${id(bookId)}/highlights`, highlightSchema, { method: 'POST', body: input })
}

export const sessionsApi = {
  start: (input: StartSessionInput): Promise<ReadingSession> =>
    apiRequest('/reading-sessions', sessionSchema, { method: 'POST', body: input }),
  heartbeat: (sessionId: string, input: HeartbeatInput): Promise<ReadingSession> =>
    apiRequest(`/reading-sessions/${id(sessionId)}/heartbeat`, sessionSchema, {
      method: 'POST',
      body: input
    }),
  finish: (sessionId: string, input: FinishSessionInput): Promise<void> =>
    apiRequestVoid(`/reading-sessions/${id(sessionId)}/finish`, { method: 'POST', body: input }),
  list: (
    query: { cursor?: string; book_id?: string; from?: string; to?: string } = {}
  ): Promise<CursorPage<ReadingSession>> =>
    apiRequest(
      `/reading-sessions${queryString({ cursor: query.cursor, book_id: query.book_id, from: query.from, to: query.to })}`,
      sessionsPageSchema
    )
}

export const translationsApi = {
  word: (input: TranslationRequest): Promise<WordTranslation> =>
    apiRequest('/translations/word', wordTranslationSchema, {
      method: 'POST',
      body: { ...input, context: input.surrounding_context }
    }),
  text: (input: TranslationRequest): Promise<TextTranslation> =>
    apiRequest('/translations/text', textTranslationSchema, {
      method: 'POST',
      body: { ...input, context: input.surrounding_context }
    })
}

export const dictionaryApi = {
  list: (query: DictionaryQuery = {}): Promise<CursorPage<DictionaryEntry>> =>
    apiRequest(
      `/dictionary${queryString({
        search: query.search,
        status: query.status,
        source_language: query.source_language,
        language: query.source_language,
        book_id: query.book_id,
        sort: query.sort,
        cursor: query.cursor,
        limit: query.limit
      })}`,
      dictionaryPageSchema
    ),
  get: (entryId: string): Promise<DictionaryEntry> =>
    apiRequest(`/dictionary/${id(entryId)}`, dictionaryEntrySchema),
  create: (input: CreateDictionaryEntry): Promise<DictionaryEntry> =>
    apiRequest('/dictionary', dictionaryEntrySchema, { method: 'POST', body: input }),
  update: (entryId: string, input: Partial<DictionaryEntry>): Promise<DictionaryEntry> =>
    apiRequest(`/dictionary/${id(entryId)}`, dictionaryEntrySchema, {
      method: 'PATCH',
      body: input
    }),
  remove: (entryId: string): Promise<void> =>
    apiRequestVoid(`/dictionary/${id(entryId)}`, { method: 'DELETE' }),
  restore: (entryId: string): Promise<DictionaryEntry> =>
    apiRequest(`/dictionary/${id(entryId)}/restore`, dictionaryEntrySchema, { method: 'POST' }),
  export: () =>
    apiRequest(
      '/dictionary/export',
      z.object({ schema_version: z.number(), exported_entries: z.array(dictionaryEntrySchema) }),
      { method: 'POST' }
    )
}

export const notesApi = {
  list: (
    query: { search?: string; book_id?: string; cursor?: string } = {}
  ): Promise<CursorPage<Note>> =>
    apiRequest(
      `/notes${queryString({ search: query.search, book_id: query.book_id, cursor: query.cursor })}`,
      notesPageSchema
    ),
  get: (noteId: string): Promise<Note> => apiRequest(`/notes/${id(noteId)}`, noteSchema),
  create: (
    input: Pick<Note, 'title' | 'blocks'> & Partial<Pick<Note, 'book_id' | 'highlight_id'>>
  ): Promise<Note> =>
    apiRequest('/notes', noteSchema, { method: 'POST', body: { ...input, schema_version: 1 } }),
  update: (noteId: string, input: Partial<Pick<Note, 'title' | 'blocks'>>): Promise<Note> =>
    apiRequest(`/notes/${id(noteId)}`, noteSchema, { method: 'PATCH', body: input }),
  remove: (noteId: string): Promise<void> =>
    apiRequestVoid(`/notes/${id(noteId)}`, { method: 'DELETE' })
}

export const statisticsApi = {
  overview: (params: {
    from?: string
    to?: string
    timezone: string
  }): Promise<StatisticsOverview> =>
    apiRequest(`/statistics/overview${queryString(params)}`, overviewSchema),
  daily: (params: { from?: string; to?: string; timezone: string }): Promise<DailyStatistic[]> =>
    apiRequest(`/statistics/daily${queryString(params)}`, dailyStatisticsResponseSchema),
  books: (params: { from?: string; to?: string; timezone: string }): Promise<BookStatistic[]> =>
    apiRequest(`/statistics/books${queryString(params)}`, bookStatisticsResponseSchema)
}

export type CurrentUser = User

function preferencesPayload(input: ReaderPreferences): Record<string, unknown> {
  return { ...input }
}
