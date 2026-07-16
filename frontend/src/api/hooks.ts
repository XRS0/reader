import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import {
  authApi,
  booksApi,
  dictionaryApi,
  notesApi,
  sessionsApi,
  statisticsApi,
  translationsApi
} from './bookflow'
import { ApiError } from './http'
import { clearPrivateOfflineData, ensureOfflineOwner } from './offlineQueue'
import type {
  AuthResponse,
  Book,
  BooksQuery,
  CreateDictionaryEntry,
  DictionaryQuery,
  FinishSessionInput,
  HeartbeatInput,
  Highlight,
  Note,
  ProgressUpdate,
  ReaderPreferences,
  StartSessionInput,
  TextTranslation,
  TranslationRequest,
  WordTranslation
} from '../types/api'

export const queryKeys = {
  auth: ['auth', 'me'] as const,
  books: (query?: BooksQuery) => ['books', query ?? {}] as const,
  book: (id: string) => ['book', id] as const,
  toc: (id: string) => ['book', id, 'toc'] as const,
  chapter: (bookId: string, chapterId: string) => ['book', bookId, 'chapter', chapterId] as const,
  progress: (id: string) => ['book', id, 'progress'] as const,
  preferences: (bookId?: string) => ['reader-preferences', bookId ?? 'global'] as const,
  bookmarks: (bookId: string) => ['book', bookId, 'bookmarks'] as const,
  highlights: (bookId: string) => ['book', bookId, 'highlights'] as const,
  dictionary: (query?: DictionaryQuery) => ['dictionary', query ?? {}] as const,
  notes: (query?: { search?: string; book_id?: string }) => ['notes', query ?? {}] as const,
  overview: (timezone: string, from: string, to: string) =>
    ['statistics', 'overview', timezone, from, to] as const,
  daily: (timezone: string, from: string, to: string) =>
    ['statistics', 'daily', timezone, from, to] as const,
  bookStats: (timezone: string, from: string, to: string) =>
    ['statistics', 'books', timezone, from, to] as const,
  sessions: (from: string, to: string) => ['reading-sessions', from, to] as const
}

export function useCurrentUser(): UseQueryResult<AuthResponse> {
  return useQuery({
    queryKey: queryKeys.auth,
    queryFn: authApi.me,
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 401) && failureCount < 2,
    staleTime: 60_000
  })
}

export function useLogin() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: authApi.login,
    onSuccess: async (data) => {
      await ensureOfflineOwner(data.user.id)
      client.setQueryData(queryKeys.auth, data)
    }
  })
}

export function useRegister() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: authApi.register,
    onSuccess: async (data) => {
      await ensureOfflineOwner(data.user.id)
      client.setQueryData(queryKeys.auth, data)
    }
  })
}

export function useLogout() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      try {
        await authApi.logout()
      } finally {
        await clearPrivateOfflineData()
      }
    },
    onSuccess: () => {
      client.clear()
    }
  })
}

export function useBooks(query: BooksQuery = {}) {
  return useQuery({
    queryKey: queryKeys.books(query),
    queryFn: () => booksApi.list(query),
    refetchInterval: (result) =>
      result.state.data?.items.some((book) =>
        ['uploaded', 'queued', 'processing'].includes(book.processing_status)
      )
        ? 3_000
        : false
  })
}

export function useBook(bookId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.book(bookId ?? ''),
    queryFn: () => booksApi.get(bookId!),
    enabled: Boolean(bookId),
    refetchInterval: (result) =>
      result.state.data &&
      ['uploaded', 'queued', 'processing'].includes(result.state.data.processing_status)
        ? 3_000
        : false
  })
}

export function useBookToc(bookId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.toc(bookId ?? ''),
    queryFn: () => booksApi.toc(bookId!),
    enabled: Boolean(bookId)
  })
}

export function useChapter(bookId: string | undefined, chapterId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.chapter(bookId ?? '', chapterId ?? ''),
    queryFn: () => booksApi.chapter(bookId!, chapterId!),
    enabled: Boolean(bookId && chapterId),
    staleTime: 5 * 60_000
  })
}

export function useBookProgress(bookId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.progress(bookId ?? ''),
    queryFn: () => booksApi.progress(bookId!),
    enabled: Boolean(bookId),
    retry: 1
  })
}

export function useReaderPreferences(bookId?: string) {
  return useQuery({
    queryKey: queryKeys.preferences(bookId),
    queryFn: () => booksApi.preferences(bookId),
    staleTime: 5 * 60_000,
    retry: 1
  })
}

export function useUpdateReaderPreferences(bookId?: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (preferences: ReaderPreferences) => booksApi.updatePreferences(preferences, bookId),
    onSuccess: (data) => client.setQueryData(queryKeys.preferences(bookId), data)
  })
}

export function useUploadBook() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: booksApi.upload,
    onSuccess: () => client.invalidateQueries({ queryKey: ['books'] })
  })
}

export function useUpdateBook(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (
      input: Partial<
        Pick<Book, 'title' | 'author' | 'description' | 'language' | 'is_favorite' | 'tags'>
      >
    ) => booksApi.update(bookId, input),
    onSuccess: (book) => {
      client.setQueryData(queryKeys.book(bookId), book)
      void client.invalidateQueries({ queryKey: ['books'] })
    }
  })
}

export function useUploadBookCover(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => booksApi.updateCover(bookId, file),
    onSuccess: (book) => {
      client.setQueryData(queryKeys.book(bookId), book)
      void client.invalidateQueries({ queryKey: ['books'] })
    }
  })
}

export function useRemoveBookCover(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: () => booksApi.removeCover(bookId),
    onSuccess: (book) => {
      client.setQueryData(queryKeys.book(bookId), book)
      void client.invalidateQueries({ queryKey: ['books'] })
    }
  })
}

export function useDeleteBook(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: () => booksApi.remove(bookId),
    onSuccess: () => client.invalidateQueries({ queryKey: ['books'] })
  })
}

export function useBookBookmarks(bookId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.bookmarks(bookId ?? ''),
    queryFn: () => booksApi.bookmarks(bookId!),
    enabled: Boolean(bookId)
  })
}

export function useAddBookmark(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: Parameters<typeof booksApi.addBookmark>[1]) =>
      booksApi.addBookmark(bookId, input),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.bookmarks(bookId) })
  })
}

export function useBookHighlights(bookId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.highlights(bookId ?? ''),
    queryFn: () => booksApi.highlights(bookId!),
    enabled: Boolean(bookId)
  })
}

export function useAddHighlight(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (
      input: Omit<Highlight, 'id' | 'book_id' | 'book_title' | 'created_at' | 'updated_at'>
    ) => booksApi.addHighlight(bookId, input),
    onSuccess: () => client.invalidateQueries({ queryKey: queryKeys.highlights(bookId) })
  })
}

export function useDictionary(query: DictionaryQuery = {}) {
  return useQuery({
    queryKey: queryKeys.dictionary(query),
    queryFn: () => dictionaryApi.list(query)
  })
}

export function useCreateDictionaryEntry() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateDictionaryEntry) => dictionaryApi.create(input),
    onSuccess: () => client.invalidateQueries({ queryKey: ['dictionary'] })
  })
}

export function useUpdateDictionaryEntry(entryId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: Parameters<typeof dictionaryApi.update>[1]) =>
      dictionaryApi.update(entryId, input),
    onSuccess: () => client.invalidateQueries({ queryKey: ['dictionary'] })
  })
}

export function useNotes(query: { search?: string; book_id?: string } = {}) {
  return useQuery({ queryKey: queryKeys.notes(query), queryFn: () => notesApi.list(query) })
}

export function useCreateNote() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (
      input: Pick<Note, 'title' | 'blocks'> & Partial<Pick<Note, 'book_id' | 'highlight_id'>>
    ) => notesApi.create(input),
    onSuccess: () => client.invalidateQueries({ queryKey: ['notes'] })
  })
}

export function useTranslation() {
  return useMutation<WordTranslation | TextTranslation, Error, TranslationRequest>({
    mutationFn: async (input) =>
      input.text.trim().split(/\s+/).length === 1
        ? await translationsApi.word(input)
        : await translationsApi.text(input)
  })
}

export function useStatistics(
  timezone: string,
  days: number,
  range?: { from: string; to: string }
) {
  const [anchor] = useState(() => new Date())
  const to = anchor
  const from = new Date(to.getTime() - (days - 1) * 86_400_000)
  const params = range
    ? { ...range, timezone }
    : { from: from.toISOString(), to: to.toISOString(), timezone }
  const overview = useQuery({
    queryKey: queryKeys.overview(timezone, params.from, params.to),
    queryFn: () => statisticsApi.overview(params)
  })
  const daily = useQuery({
    queryKey: queryKeys.daily(timezone, params.from, params.to),
    queryFn: () => statisticsApi.daily(params)
  })
  const books = useQuery({
    queryKey: queryKeys.bookStats(timezone, params.from, params.to),
    queryFn: () => statisticsApi.books(params)
  })
  const sessions = useQuery({
    queryKey: queryKeys.sessions(params.from, params.to),
    queryFn: () => sessionsApi.list({ from: params.from, to: params.to })
  })
  return { overview, daily, books, sessions }
}

export function useReadingSession() {
  const [sessionId, setSessionId] = useState<string>()
  const start = useMutation({
    mutationFn: (input: StartSessionInput) => sessionsApi.start(input),
    onSuccess: (session) => setSessionId(session.id)
  })
  const heartbeat = useMutation({
    mutationFn: ({ id, input }: { id: string; input: HeartbeatInput }) =>
      sessionsApi.heartbeat(id, input)
  })
  const finish = useMutation({
    mutationFn: ({ id, input }: { id: string; input: FinishSessionInput }) =>
      sessionsApi.finish(id, input),
    onSettled: () => setSessionId(undefined)
  })
  return { sessionId, start, heartbeat, finish }
}

export function useSaveProgress(bookId: string) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: ProgressUpdate) => booksApi.updateProgress(bookId, input),
    onSuccess: (data) => client.setQueryData(queryKeys.progress(bookId), data)
  })
}

export function useDebouncedValue<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(timer)
  }, [delay, value])
  return debounced
}

export function useMutateBook() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({
      bookId,
      input
    }: {
      bookId: string
      input: Partial<
        Pick<Book, 'title' | 'author' | 'description' | 'language' | 'is_favorite' | 'tags'>
      >
    }) => booksApi.update(bookId, input),
    onSuccess: (book) => {
      client.setQueryData(queryKeys.book(book.id), book)
      void client.invalidateQueries({ queryKey: ['books'] })
    }
  })
}

export function useRemoveBook() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (bookId: string) => booksApi.remove(bookId),
    onSuccess: () => client.invalidateQueries({ queryKey: ['books'] })
  })
}

export function useReprocessBook() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (bookId: string) => booksApi.reprocess(bookId),
    onSuccess: () => client.invalidateQueries({ queryKey: ['books'] })
  })
}

export function useUpdateNote(noteId: string | undefined) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: Partial<Pick<Note, 'title' | 'blocks'>>) => notesApi.update(noteId!, input),
    onSuccess: () => client.invalidateQueries({ queryKey: ['notes'] })
  })
}
