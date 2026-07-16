export type UUID = string
export type ISODate = string

export interface ApiErrorBody {
  code: string
  message: string
  details?: Record<string, unknown>
  request_id?: string
}

export interface ApiErrorEnvelope {
  error: ApiErrorBody
}

export interface CursorPage<T> {
  items: T[]
  next_cursor?: string | null
  has_more: boolean
  total_count?: number
}

export interface User {
  id: UUID
  email: string
  display_name: string
  locale: 'ru' | 'en'
  timezone: string
  created_at: ISODate
}

export interface AuthResponse {
  user: User
}

export type BookFormat = 'epub' | 'fb2' | 'txt'
export type ProcessingStatus = 'uploaded' | 'queued' | 'processing' | 'ready' | 'failed'

export interface Book {
  id: UUID
  title: string
  author: string
  description?: string
  format: BookFormat
  language: string
  processing_status: ProcessingStatus
  processing_error?: string | null
  cover_url?: string | null
  has_custom_cover?: boolean
  progress_percent: number
  current_chapter_id?: UUID | null
  estimated_minutes_remaining?: number | null
  is_favorite: boolean
  tags: string[]
  added_at: ISODate
  last_read_at?: ISODate | null
  updated_at: ISODate
}

export interface BooksQuery {
  search?: string
  status?: ProcessingStatus | 'all'
  format?: BookFormat | 'all'
  sort?: 'last_read' | 'added' | 'title' | 'progress'
  favorite?: boolean
  cursor?: string
  limit?: number
}

export interface TocItem {
  id: UUID
  chapter_id: UUID
  title: string
  level: number
  order: number
  children?: TocItem[]
}

export interface Chapter {
  id: UUID
  book_id: UUID
  title: string
  order: number
  html: string
  plain_text?: string
  previous_chapter_id?: UUID | null
  next_chapter_id?: UUID | null
  word_count?: number
}

export interface ReadingProgress {
  book_id: UUID
  chapter_id?: UUID | null
  locator_type: 'chapter_offset' | 'epub_cfi'
  locator: string
  character_offset: number
  text_anchor?: string
  progress_percent: number
  scroll_percent: number
  revision: number
  client_id: string
  device_id?: UUID
  server_timestamp?: ISODate
  updated_at: ISODate
}

export interface ProgressUpdate {
  chapter_id: UUID
  locator_type: ReadingProgress['locator_type']
  locator: string
  character_offset: number
  text_anchor?: string
  progress_percent: number
  scroll_percent: number
  revision: number
  client_id: string
  device_id?: UUID
  client_timestamp: ISODate
}

export type ReaderTheme = 'light' | 'warm' | 'sepia' | 'dark' | 'custom'
export type ReadingMode = 'scroll' | 'paged'
export type ReaderFont = 'system' | 'serif' | 'Georgia' | 'Arial' | 'Inter' | 'Source Serif 4'

export interface ReaderPreferences {
  theme: ReaderTheme
  background_color: string
  text_color: string
  accent_color: string
  font_family: ReaderFont
  font_size: number
  font_weight: 400 | 500 | 600
  line_height: number
  letter_spacing: number
  content_width: number
  page_margin: number
  text_align: 'left' | 'justify'
  reading_mode: ReadingMode
  show_progress: boolean
  show_remaining_time: boolean
  controls_brightness: number
}

export type ReadingSessionStatus = 'active' | 'idle' | 'finished' | 'stale' | 'finalized'
export type SessionCloseReason =
  | 'user_closed_reader'
  | 'switched_book'
  | 'app_backgrounded'
  | 'logout'
  | 'idle_timeout'
  | 'connection_lost'
  | 'stale_session_finalized'
  | 'book_finished'
  | 'server_shutdown'
  | 'unknown'

export interface ReadingSession {
  id: UUID
  book_id: UUID
  device_id?: UUID | null
  started_at: ISODate
  last_activity_at: ISODate
  last_heartbeat_at?: ISODate
  ended_at?: ISODate | null
  active_seconds: number
  idle_seconds: number
  start_progress_percent: number
  end_progress_percent?: number
  words_read_estimate: number
  pages_read_estimate: number
  status: ReadingSessionStatus
  close_reason?: SessionCloseReason
}

export interface StartSessionInput {
  book_id: UUID
  chapter_id: UUID
  locator: string
  progress_percent: number
  client_id: string
  client_timestamp: ISODate
}

export interface HeartbeatInput {
  locator: string
  progress_percent: number
  tab_visible: boolean
  window_focused: boolean
  user_active: boolean
  milliseconds_since_interaction: number
  client_timestamp: ISODate
  idempotency_key: string
  sequence_number: number
}

export interface FinishSessionInput {
  locator: string
  progress_percent: number
  reason: SessionCloseReason
  client_timestamp: ISODate
  idempotency_key: string
}

export interface WordTranslation {
  original_text: string
  normalized_form: string
  lemma?: string
  translation: string
  transcription?: string
  part_of_speech?: string
  definition?: string
  alternatives: string[]
  example?: string
  source_language: string
  target_language: string
  confidence?: number
  cached?: boolean
}

export interface TextTranslation {
  original_text: string
  translation: string
  detected_language: string
  explanation?: string
  cached?: boolean
}

export interface TranslationRequest {
  text: string
  source_language?: string
  target_language: string
  surrounding_context?: string
  book_id?: UUID
  chapter_id?: UUID
  locator?: string
}

export type DictionaryStatus = 'unknown' | 'learning' | 'known' | 'mastered' | 'ignored'

export interface DictionaryEntry {
  id: UUID
  source_language: string
  target_language: string
  original_word: string
  normalized_word: string
  lemma?: string
  transcription?: string
  part_of_speech?: string
  translation: string
  alternative_translations: string[]
  definition?: string
  note?: string
  status: DictionaryStatus
  encounter_count: number
  first_seen_at: ISODate
  last_seen_at: ISODate
  next_review_at?: ISODate | null
  book_id?: UUID
  book_title?: string
  created_at: ISODate
  updated_at: ISODate
}

export interface DictionaryQuery {
  search?: string
  status?: DictionaryStatus | 'all'
  source_language?: string
  book_id?: UUID
  sort?: 'last_seen' | 'first_seen' | 'word' | 'encounters' | 'review'
  cursor?: string
  limit?: number
}

export interface CreateDictionaryEntry {
  source_language: string
  target_language?: string
  original_word: string
  normalized_word?: string
  lemma?: string
  transcription?: string
  part_of_speech?: string
  translation?: string
  alternative_translations?: string[]
  definition?: string
  note?: string
  status?: DictionaryStatus
  occurrence?: {
    book_id: UUID
    chapter_id: UUID
    locator: string
    sentence: string
    context_before?: string
    context_after?: string
    encountered_at: ISODate
  }
}

export interface WordOccurrence {
  id: UUID
  dictionary_entry_id: UUID
  book_id: UUID
  book_title?: string
  chapter_id: UUID
  chapter_title?: string
  locator: string
  sentence: string
  context_before?: string
  context_after?: string
  encountered_at: ISODate
}

export interface Bookmark {
  id: UUID
  book_id: UUID
  chapter_id?: UUID | null
  locator: string
  progress_percent: number
  title: string
  note?: string
  created_at: ISODate
}

export type HighlightColor = 'sand' | 'sage' | 'blue' | 'rose'

export interface Highlight {
  id: UUID
  book_id: UUID
  book_title?: string
  chapter_id?: UUID | null
  locator: string
  text_anchor: string
  selected_text: string
  context?: string
  color: HighlightColor
  note?: string
  created_at: ISODate
  updated_at: ISODate
}

export type NoteBlockType =
  | 'paragraph'
  | 'heading1'
  | 'heading2'
  | 'heading3'
  | 'bulleted_list'
  | 'numbered_list'
  | 'task'
  | 'quote'
  | 'callout'
  | 'divider'
  | 'link'
  | 'book_link'
  | 'saved_quote'

export interface NoteBlock {
  id: UUID
  type: NoteBlockType
  text?: string
  checked?: boolean
  url?: string
  book_id?: UUID
  locator?: string
}

export interface Note {
  id: UUID
  title: string
  book_id?: UUID | null
  book_title?: string
  highlight_id?: UUID | null
  schema_version: 1
  blocks: NoteBlock[]
  created_at: ISODate
  updated_at: ISODate
}

export interface StatisticsOverview {
  total_reading_seconds: number
  active_reading_seconds: number
  idle_seconds: number
  sessions_count: number
  books_started: number
  books_completed: number
  words_read_estimate: number
  pages_read_estimate: number
  current_streak_days: number
  longest_streak_days: number
  average_session_seconds: number
  median_session_seconds: number
  average_words_per_minute: number
  dictionary_words: number
  learned_words: number
  translations_count: number
}

export interface DailyStatistic {
  date: string
  active_seconds: number
  idle_seconds: number
  sessions_count: number
  words_read_estimate: number
}

export interface BookStatistic {
  book_id: UUID
  book_title: string
  active_seconds: number
  sessions_count: number
  progress_percent: number
  average_words_per_minute: number
  last_read_at?: ISODate
}
