import {
  Bookmark,
  BookOpen,
  ChevronLeft,
  ChevronRight,
  Copy,
  Highlighter,
  Languages,
  ListTree,
  Maximize2,
  NotebookPen,
  Search,
  Settings2,
  Share2,
  X
} from 'lucide-react'
import DOMPurify from 'dompurify'
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type MouseEvent as ReactMouseEvent,
  type WheelEvent as ReactWheelEvent
} from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { apiConfig } from '../api/config'
import { ApiError, apiKeepalive } from '../api/http'
import { booksApi, sessionsApi } from '../api/bookflow'
import {
  queryKeys,
  useAddBookmark,
  useAddHighlight,
  useBook,
  useBookProgress,
  useBookToc,
  useChapter,
  useCreateDictionaryEntry,
  useCreateNote,
  useReaderPreferences,
  useUpdateReaderPreferences,
  useReadingSession,
  useTranslation as useTranslationRequest
} from '../api/hooks'
import { enqueueProgress, readProgressQueue } from '../api/offlineQueue'
import {
  BottomSheet,
  Button,
  Dialog,
  IconButton,
  Input,
  LoadingState,
  ErrorState,
  useToast
} from '../shared/ui'
import { useOfflineStore } from '../stores/offlineStore'
import { useReaderStore } from '../stores/readerStore'
import type { FinishSessionInput, ProgressUpdate, ReadingProgress } from '../types/api'
import { ReaderSettings, readerFontCss } from './ReaderSettings'
import { stripLeadingDuplicateHeading } from './html'
import { resolveSourceLanguage } from './language'
import {
  calculatePagedNavigationTarget,
  calculatePagedScrollStep,
  calculatePagedSnapTarget,
  calculateResumeTarget
} from './pagination'
import { SelectionToolbar, TranslationPopover, type TranslationValue } from './TranslationPopover'
import styles from './reader.module.css'
import { formatSelectedText } from './selectionText'

interface TextSelection {
  text: string
  context: string
  locator: string
  toolbar: { x: number; y: number }
  popover: { x: number; y: number }
}

const idleThresholdMs = 60_000

function getClientId(): string {
  const key = 'bookflow:reader-client-id'
  const existing = localStorage.getItem(key)
  if (existing) return existing
  const created = crypto.randomUUID()
  localStorage.setItem(key, created)
  return created
}

function textFromHtml(html: string): string {
  const element = document.createElement('div')
  element.innerHTML = html
  return element.textContent ?? ''
}

export function ReaderPage() {
  const { bookId } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { notify } = useToast()
  const queryClient = useQueryClient()
  const bookQuery = useBook(bookId)
  const tocQuery = useBookToc(bookId)
  const progressQuery = useBookProgress(bookId)
  const requestedChapter = searchParams.get('chapter') ?? undefined
  const chapterId =
    requestedChapter ||
    progressQuery.data?.chapter_id ||
    bookQuery.data?.current_chapter_id ||
    tocQuery.data?.[0]?.chapter_id
  const chapterQuery = useChapter(bookId, chapterId)
  const serverPreferences = useReaderPreferences(bookId)
  const { mutate: persistPreferences } = useUpdateReaderPreferences(bookId)
  const preferences = useReaderStore((state) => state.preferences)
  const replacePreferences = useReaderStore((state) => state.replacePreferences)
  const updatePreferences = useReaderStore((state) => state.updatePreferences)
  const tocOpen = useReaderStore((state) => state.tocOpen)
  const settingsOpen = useReaderStore((state) => state.settingsOpen)
  const setTocOpen = useReaderStore((state) => state.setTocOpen)
  const setSettingsOpen = useReaderStore((state) => state.setSettingsOpen)
  const [selection, setSelection] = useState<TextSelection>()
  const [translationValue, setTranslationValue] = useState<TranslationValue>()
  const [dictionaryAdded, setDictionaryAdded] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [bookSearch, setBookSearch] = useState('')
  const [readingPercent, setReadingPercent] = useState(progressQuery.data?.progress_percent ?? 0)
  const [scrollPercent, setScrollPercent] = useState(progressQuery.data?.scroll_percent ?? 0)
  const [resumeMarkerTop, setResumeMarkerTop] = useState<number>()
  const [positionReady, setPositionReady] = useState(false)
  const [mobile, setMobile] = useState(() => matchMedia('(max-width: 720px)').matches)
  const viewportRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLElement>(null)
  const progressTimer = useRef<number | undefined>(undefined)
  const pagedSnapTimer = useRef<number | undefined>(undefined)
  const wheelLockedUntil = useRef(0)
  const preferencesTimer = useRef<number | undefined>(undefined)
  const lastActivityAt = useRef(Date.now())
  const revision = useRef(progressQuery.data?.revision ?? 0)
  const sequence = useRef(0)
  const sessionStarted = useRef(false)
  const finishSent = useRef(false)
  const positionRef = useRef({
    progress: readingPercent,
    scroll: scrollPercent,
    locator: progressQuery.data?.locator ?? 'chapter:0:offset:0'
  })
  const clientId = useMemo(getClientId, [])
  const translation = useTranslationRequest()
  const createDictionary = useCreateDictionaryEntry()
  const createHighlight = useAddHighlight(bookId ?? '')
  const createNote = useCreateNote()
  const addBookmark = useAddBookmark(bookId ?? '')
  const session = useReadingSession()
  const setPending = useOfflineStore((state) => state.setPending)
  const online = useOfflineStore((state) => state.online)
  const initialPreferenceApplied = useRef(false)
  const progressLoaded = useRef(false)
  const sessionStartProgress = useRef<ReadingProgress | undefined>(undefined)
  const restoringPosition = useRef(false)
  const restoredPositionKey = useRef('')

  const chapter = chapterQuery.data
  const book = bookQuery.data
  const toc = tocQuery.data ?? []
  const currentChapterIndex = Math.max(
    0,
    toc.findIndex((item) => item.chapter_id === chapterId)
  )
  const sanitizedHtml = useMemo(
    () =>
      DOMPurify.sanitize(chapter?.html ?? '', {
        USE_PROFILES: { html: true },
        FORBID_TAGS: ['style', 'script', 'iframe', 'object', 'embed', 'form'],
        FORBID_ATTR: ['style', 'onerror', 'onclick', 'onload']
      }),
    [chapter?.html]
  )
  const readerHtml = useMemo(
    () => stripLeadingDuplicateHeading(sanitizedHtml, chapter?.title ?? ''),
    [chapter?.title, sanitizedHtml]
  )
  const chapterText = useMemo(
    () => (chapter?.plain_text ? chapter.plain_text : textFromHtml(sanitizedHtml)),
    [chapter?.plain_text, sanitizedHtml]
  )
  const searchResults = useMemo(() => {
    const query = bookSearch.trim().toLowerCase()
    if (query.length < 2) return []
    const lower = chapterText.toLowerCase()
    const values: string[] = []
    let offset = 0
    while (values.length < 8) {
      const index = lower.indexOf(query, offset)
      if (index < 0) break
      values.push(
        chapterText.slice(
          Math.max(0, index - 55),
          Math.min(chapterText.length, index + query.length + 75)
        )
      )
      offset = index + query.length
    }
    return values
  }, [bookSearch, chapterText])

  useEffect(() => {
    const media = matchMedia('(max-width: 720px)')
    const listener = () => setMobile(media.matches)
    media.addEventListener('change', listener)
    return () => media.removeEventListener('change', listener)
  }, [])

  useEffect(() => {
    if (serverPreferences.data && !initialPreferenceApplied.current) {
      initialPreferenceApplied.current = true
      replacePreferences(serverPreferences.data)
    }
  }, [replacePreferences, serverPreferences.data])

  useEffect(() => {
    if (!initialPreferenceApplied.current) return undefined
    window.clearTimeout(preferencesTimer.current)
    preferencesTimer.current = window.setTimeout(() => {
      persistPreferences(preferences)
    }, 700)
    return () => window.clearTimeout(preferencesTimer.current)
  }, [persistPreferences, preferences])

  useEffect(() => {
    if (progressQuery.data) {
      // Freeze the visible "you stopped here" boundary for this reader
      // session. Live autosaves may update the query cache, but the marker
      // must move only after the session ends and the book is opened again.
      sessionStartProgress.current ??= progressQuery.data
      progressLoaded.current = true
      revision.current = progressQuery.data.revision
      setReadingPercent(progressQuery.data.progress_percent)
      setScrollPercent(progressQuery.data.scroll_percent)
      positionRef.current = {
        progress: progressQuery.data.progress_percent,
        scroll: progressQuery.data.scroll_percent,
        locator: progressQuery.data.locator
      }
    }
  }, [progressQuery.data])

  useEffect(() => {
    const events: Array<keyof WindowEventMap> = [
      'pointermove',
      'pointerdown',
      'keydown',
      'touchstart'
    ]
    const noteActivity = () => {
      lastActivityAt.current = Date.now()
    }
    events.forEach((event) => window.addEventListener(event, noteActivity, { passive: true }))
    return () => {
      events.forEach((event) => window.removeEventListener(event, noteActivity))
    }
  }, [])

  const persistProgress = useCallback(
    async (nextProgress: number, nextScroll: number, locator: string) => {
      if (!bookId || !chapterId) return
      const input: ProgressUpdate = {
        chapter_id: chapterId,
        locator_type: 'chapter_offset',
        locator,
        character_offset: Math.round((nextScroll / 100) * chapterText.length),
        text_anchor: chapterText.slice(
          Math.max(0, Math.round((nextScroll / 100) * chapterText.length) - 24),
          Math.round((nextScroll / 100) * chapterText.length) + 48
        ),
        progress_percent: nextProgress,
        scroll_percent: nextScroll,
        revision: revision.current,
        client_id: clientId,
        client_timestamp: new Date().toISOString()
      }
      try {
        if (!navigator.onLine) throw new Error('offline')
        const result = await booksApi.updateProgress(bookId, input)
        revision.current = result.revision
        queryClient.setQueryData(queryKeys.progress(bookId), result)
      } catch (error) {
        if (error instanceof ApiError && error.status === 409) {
          const current = error.details?.current
          if (
            current &&
            typeof current === 'object' &&
            'revision' in current &&
            typeof current.revision === 'number'
          ) {
            revision.current = current.revision
          }
          return
        }
        await enqueueProgress(bookId, input)
        const queue = await readProgressQueue()
        setPending(queue.length)
      }
    },
    [bookId, chapterId, chapterText, clientId, queryClient, setPending]
  )

  useEffect(() => {
    if (!chapter || !chapterId || progressQuery.isLoading) return undefined
    const progress = sessionStartProgress.current
    const isSavedChapter = Boolean(progress?.chapter_id && progress.chapter_id === chapterId)
    const layoutKey = [
      bookId,
      chapterId,
      preferences.reading_mode,
      progress?.updated_at,
      preferences.font_family,
      preferences.font_size,
      preferences.line_height,
      preferences.content_width,
      preferences.page_margin
    ].join(':')

    if (!isSavedChapter) {
      restoredPositionKey.current = layoutKey
      setResumeMarkerTop(undefined)
      setPositionReady(true)
      return undefined
    }
    if (restoredPositionKey.current === layoutKey) return undefined

    restoringPosition.current = true
    setPositionReady(false)
    let secondFrame = 0
    const firstFrame = window.requestAnimationFrame(() => {
      secondFrame = window.requestAnimationFrame(() => {
        const viewport = viewportRef.current
        const content = contentRef.current
        if (!viewport || !content || !progress) return

        if (preferences.reading_mode === 'paged') {
          const maximum = Math.max(0, content.scrollWidth - content.clientWidth)
          const step = calculatePagedScrollStep(
            content.clientWidth,
            window.getComputedStyle(content).columnGap
          )
          const target = calculatePagedSnapTarget(
            calculateResumeTarget(progress.scroll_percent, maximum),
            step,
            maximum
          )
          const behavior = content.style.scrollBehavior
          content.style.scrollBehavior = 'auto'
          content.scrollLeft = target
          content.style.scrollBehavior = behavior
          setResumeMarkerTop(undefined)
        } else {
          const maximum = Math.max(0, viewport.scrollHeight - viewport.clientHeight)
          const target = calculateResumeTarget(progress.scroll_percent, maximum)
          const behavior = viewport.style.scrollBehavior
          viewport.style.scrollBehavior = 'auto'
          viewport.scrollTop = target
          viewport.style.scrollBehavior = behavior
          const contentRect = content.getBoundingClientRect()
          const markerProbeY = Math.min(window.innerHeight - 72, 92)
          const markerRange = document.caretRangeFromPoint?.(
            contentRect.left + contentRect.width / 2,
            markerProbeY
          )
          const markerLine = markerRange?.getClientRects()[0]
          const markerTop = markerLine ? markerLine.bottom - contentRect.top + 7 : target + 96
          setResumeMarkerTop(
            progress.scroll_percent > 0
              ? Math.max(68, Math.min(content.scrollHeight - 28, markerTop))
              : undefined
          )
        }

        restoredPositionKey.current = layoutKey
        setPositionReady(true)
        window.requestAnimationFrame(() => {
          restoringPosition.current = false
        })
      })
    })
    return () => {
      window.cancelAnimationFrame(firstFrame)
      window.cancelAnimationFrame(secondFrame)
      restoringPosition.current = false
    }
  }, [
    bookId,
    chapter,
    chapterId,
    preferences.content_width,
    preferences.font_family,
    preferences.font_size,
    preferences.line_height,
    preferences.page_margin,
    preferences.reading_mode,
    progressQuery.data,
    progressQuery.isLoading
  ])

  const scheduleProgress = useCallback(
    (nextScroll: number) => {
      const totalChapters = Math.max(1, toc.length)
      const nextProgress = Math.min(
        100,
        ((currentChapterIndex + nextScroll / 100) / totalChapters) * 100
      )
      const locator = `chapter:${currentChapterIndex + 1}:percent:${nextScroll.toFixed(2)}`
      setScrollPercent(nextScroll)
      setReadingPercent(nextProgress)
      positionRef.current = { progress: nextProgress, scroll: nextScroll, locator }
      window.clearTimeout(progressTimer.current)
      progressTimer.current = window.setTimeout(
        () => void persistProgress(nextProgress, nextScroll, locator),
        900
      )
    },
    [currentChapterIndex, persistProgress, toc.length]
  )

  const pagedMetrics = useCallback(() => {
    const content = contentRef.current
    if (!content) return undefined
    const step = calculatePagedScrollStep(
      content.clientWidth,
      window.getComputedStyle(content).columnGap
    )
    return {
      content,
      step,
      maximum: Math.max(0, content.scrollWidth - content.clientWidth)
    }
  }, [])

  const navigatePaged = useCallback(
    (direction: -1 | 1) => {
      const metrics = pagedMetrics()
      if (!metrics) return
      window.clearTimeout(pagedSnapTimer.current)
      metrics.content.scrollTo({
        left: calculatePagedNavigationTarget(
          metrics.content.scrollLeft,
          metrics.step,
          metrics.maximum,
          direction
        ),
        behavior: 'smooth'
      })
    },
    [pagedMetrics]
  )

  const schedulePagedSnap = useCallback(() => {
    window.clearTimeout(pagedSnapTimer.current)
    pagedSnapTimer.current = window.setTimeout(() => {
      const metrics = pagedMetrics()
      if (!metrics) return
      const target = calculatePagedSnapTarget(
        metrics.content.scrollLeft,
        metrics.step,
        metrics.maximum
      )
      if (Math.abs(target - metrics.content.scrollLeft) < 1) return
      metrics.content.scrollTo({ left: target, behavior: 'smooth' })
    }, 120)
  }, [pagedMetrics])

  const onReaderScroll = useCallback(() => {
    if (restoringPosition.current) return
    lastActivityAt.current = Date.now()
    const element = preferences.reading_mode === 'paged' ? contentRef.current : viewportRef.current
    if (!element) return
    const current = preferences.reading_mode === 'paged' ? element.scrollLeft : element.scrollTop
    const maximum =
      preferences.reading_mode === 'paged'
        ? element.scrollWidth - element.clientWidth
        : element.scrollHeight - element.clientHeight
    scheduleProgress(maximum > 0 ? Math.max(0, Math.min(100, (current / maximum) * 100)) : 100)
    if (preferences.reading_mode === 'paged') schedulePagedSnap()
  }, [preferences.reading_mode, schedulePagedSnap, scheduleProgress])

  useEffect(() => {
    return () => {
      window.clearTimeout(progressTimer.current)
      window.clearTimeout(pagedSnapTimer.current)
      if (!progressLoaded.current) return
      const position = positionRef.current
      void persistProgress(position.progress, position.scroll, position.locator)
    }
  }, [persistProgress])

  const onPagedWheel = useCallback(
    (event: ReactWheelEvent<HTMLElement>) => {
      const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY
      if (Math.abs(delta) < 2) return
      event.preventDefault()
      const now = performance.now()
      if (now < wheelLockedUntil.current) return
      wheelLockedUntil.current = now + 360
      navigatePaged(delta > 0 ? 1 : -1)
    },
    [navigatePaged]
  )

  useEffect(() => {
    if (!bookId || !chapterId || sessionStarted.current || progressQuery.isLoading) return
    sessionStarted.current = true
    session.start.mutate({
      book_id: bookId,
      chapter_id: chapterId,
      locator: positionRef.current.locator,
      progress_percent: positionRef.current.progress,
      client_id: clientId,
      client_timestamp: new Date().toISOString()
    })
  }, [bookId, chapterId, clientId, progressQuery.isLoading, session.start])

  useEffect(() => {
    if (!session.sessionId) return undefined
    const sendHeartbeat = () => {
      const sinceInteraction = Date.now() - lastActivityAt.current
      sequence.current += 1
      void sessionsApi.heartbeat(session.sessionId!, {
        locator: positionRef.current.locator,
        progress_percent: positionRef.current.progress,
        tab_visible: document.visibilityState === 'visible',
        window_focused: document.hasFocus(),
        user_active: sinceInteraction < idleThresholdMs,
        milliseconds_since_interaction: sinceInteraction,
        client_timestamp: new Date().toISOString(),
        idempotency_key: crypto.randomUUID(),
        sequence_number: sequence.current
      })
    }
    const timer = window.setInterval(sendHeartbeat, apiConfig.heartbeatMs)
    return () => window.clearInterval(timer)
  }, [session.sessionId])

  const beaconFinish = useCallback(() => {
    if (finishSent.current) return
    finishSent.current = true
    window.clearTimeout(progressTimer.current)
    const position = positionRef.current
    if (bookId && chapterId) {
      apiKeepalive(
        `/books/${encodeURIComponent(bookId)}/progress`,
        {
          chapter_id: chapterId,
          locator_type: 'chapter_offset',
          locator: position.locator,
          character_offset: Math.round((position.scroll / 100) * chapterText.length),
          text_anchor: chapterText.slice(
            Math.max(0, Math.round((position.scroll / 100) * chapterText.length) - 24),
            Math.round((position.scroll / 100) * chapterText.length) + 48
          ),
          progress_percent: position.progress,
          scroll_percent: position.scroll,
          revision: revision.current,
          client_id: clientId,
          client_timestamp: new Date().toISOString()
        } satisfies ProgressUpdate,
        'PUT'
      )
    }
    if (!session.sessionId) return
    const input: FinishSessionInput = {
      locator: positionRef.current.locator,
      progress_percent: positionRef.current.progress,
      reason: 'app_backgrounded',
      client_timestamp: new Date().toISOString(),
      idempotency_key: crypto.randomUUID()
    }
    apiKeepalive(`/reading-sessions/${encodeURIComponent(session.sessionId)}/finish`, input)
  }, [bookId, chapterId, chapterText, clientId, session.sessionId])

  useEffect(() => {
    window.addEventListener('pagehide', beaconFinish)
    return () => window.removeEventListener('pagehide', beaconFinish)
  }, [beaconFinish])

  const finishAndExit = async () => {
    window.clearTimeout(progressTimer.current)
    const position = positionRef.current
    await persistProgress(position.progress, position.scroll, position.locator)
    if (session.sessionId && !finishSent.current) {
      finishSent.current = true
      await session.finish
        .mutateAsync({
          id: session.sessionId,
          input: {
            locator: positionRef.current.locator,
            progress_percent: positionRef.current.progress,
            reason: 'user_closed_reader',
            client_timestamp: new Date().toISOString(),
            idempotency_key: crypto.randomUUID()
          }
        })
        .catch(() => undefined)
    }
    void navigate(`/books/${bookId ?? ''}`)
  }

  const goToChapter = (nextChapterId: string) => {
    const position = positionRef.current
    void persistProgress(position.progress, position.scroll, position.locator)
    setSearchParams({ chapter: nextChapterId })
    setTocOpen(false)
    setSelection(undefined)
    window.setTimeout(() => {
      viewportRef.current?.scrollTo({ top: 0 })
      contentRef.current?.scrollTo({ left: 0 })
    }, 0)
  }

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setSelection(undefined)
        setTocOpen(false)
        setSettingsOpen(false)
      }
      if (
        preferences.reading_mode === 'paged' &&
        !['INPUT', 'TEXTAREA', 'SELECT'].includes((event.target as HTMLElement).tagName)
      ) {
        if (event.key === 'ArrowRight' || event.key === 'PageDown' || event.key === ' ') {
          event.preventDefault()
          navigatePaged(1)
        }
        if (event.key === 'ArrowLeft' || event.key === 'PageUp') {
          event.preventDefault()
          navigatePaged(-1)
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigatePaged, preferences.reading_mode, setSettingsOpen, setTocOpen])

  const handleSelection = (event: ReactMouseEvent) => {
    if (!contentRef.current?.contains(event.target as Node)) return
    const browserSelection = window.getSelection()
    const text = formatSelectedText(browserSelection?.toString() ?? '', 20_000)
    if (!browserSelection || browserSelection.rangeCount === 0 || text.length < 1) {
      setSelection(undefined)
      return
    }
    const range = browserSelection.getRangeAt(0)
    const rect = range.getBoundingClientRect()
    const plain = chapterText
    const textIndex = Math.max(0, plain.toLowerCase().indexOf(text.toLowerCase().slice(0, 40)))
    setTranslationValue(undefined)
    setDictionaryAdded(false)
    translation.reset()
    setSelection({
      text,
      context: plain.slice(
        Math.max(0, textIndex - 120),
        Math.min(plain.length, textIndex + text.length + 120)
      ),
      locator: `chapter:${currentChapterIndex + 1}:offset:${textIndex}`,
      toolbar: {
        x: Math.max(160, Math.min(window.innerWidth - 160, rect.left + rect.width / 2)),
        y: Math.max(48, rect.top)
      },
      popover: {
        x: Math.max(12, Math.min(window.innerWidth - 352, rect.left)),
        y: Math.min(window.innerHeight - 300, rect.bottom + 12)
      }
    })
  }

  const translateSelection = async (): Promise<TranslationValue | undefined> => {
    if (!selection) return undefined
    const value = await translation.mutateAsync({
      text: selection.text,
      target_language: 'ru',
      surrounding_context: selection.context,
      book_id: bookId,
      chapter_id: chapterId,
      locator: selection.locator
    })
    setTranslationValue(value)
    return value
  }

  const addSelectionToDictionary = async () => {
    if (!selection || !bookId || !chapterId) return
    const value = translationValue ?? (await translateSelection())
    if (!value || !('normalized_form' in value)) return
    await createDictionary.mutateAsync({
      source_language: resolveSourceLanguage(value.source_language, book?.language, selection.text),
      target_language: value.target_language || 'ru',
      original_word: value.original_text,
      normalized_word: value.normalized_form,
      lemma: value.lemma,
      transcription: value.transcription,
      part_of_speech: value.part_of_speech,
      translation: value.translation,
      alternative_translations: value.alternatives,
      definition: value.definition,
      status: 'unknown',
      occurrence: {
        book_id: bookId,
        chapter_id: chapterId,
        locator: selection.locator,
        sentence: selection.context,
        encountered_at: new Date().toISOString()
      }
    })
    setDictionaryAdded(true)
  }

  const highlightSelection = async () => {
    if (!selection || !chapterId) return
    await createHighlight.mutateAsync({
      chapter_id: chapterId,
      locator: selection.locator,
      text_anchor: selection.text.slice(0, 100),
      selected_text: selection.text,
      context: selection.context,
      color: 'sand'
    })
    notify(t('reader.highlight'), 'success')
    setSelection(undefined)
  }

  const noteSelection = async () => {
    if (!selection) return
    await createNote.mutateAsync({
      title: selection.text.slice(0, 54),
      book_id: bookId,
      blocks: [
        {
          id: crypto.randomUUID(),
          type: 'saved_quote',
          text: selection.text,
          book_id: bookId,
          locator: selection.locator
        },
        { id: crypto.randomUUID(), type: 'paragraph', text: '' }
      ]
    })
    notify(t('notes.newNote'), 'success')
    setSelection(undefined)
  }

  const copySelection = async () => {
    if (!selection) return
    await navigator.clipboard.writeText(selection.text)
    notify(t('reader.copy'), 'success')
    setSelection(undefined)
  }

  const shareSelection = async () => {
    if (!selection) return
    if (navigator.share) await navigator.share({ text: selection.text, title: book?.title })
    else await navigator.clipboard.writeText(selection.text)
    setSelection(undefined)
  }

  const addCurrentBookmark = async () => {
    if (!bookId || !chapterId) return
    await addBookmark.mutateAsync({
      chapter_id: chapterId,
      locator: positionRef.current.locator,
      progress_percent: positionRef.current.progress,
      title: chapter?.title ?? book?.title ?? t('reader.bookmark'),
      note: ''
    })
    notify(t('reader.bookmark'), 'success')
  }

  const toggleFullscreen = async () => {
    if (document.fullscreenElement) await document.exitFullscreen()
    else await document.documentElement.requestFullscreen()
  }

  const readerStyle = {
    '--reader-width': `${preferences.content_width}px`,
    '--reader-margin': `${preferences.page_margin}px`,
    '--reader-font': readerFontCss(preferences.font_family),
    '--reader-font-size': `${preferences.font_size}px`,
    '--reader-font-weight': preferences.font_weight,
    '--reader-line-height': preferences.line_height,
    '--reader-letter-spacing': `${preferences.letter_spacing}em`,
    '--reader-align': preferences.text_align,
    '--controls-opacity': preferences.controls_brightness,
    '--custom-reader-bg': preferences.background_color,
    '--custom-reader-text': preferences.text_color,
    '--custom-reader-accent': preferences.accent_color
  } as CSSProperties

  if (
    bookQuery.isLoading ||
    chapterQuery.isLoading ||
    tocQuery.isLoading ||
    progressQuery.isLoading
  )
    return <LoadingState label={t('common.loading')} />
  if (bookQuery.isError || chapterQuery.isError || !book || !chapter) {
    return (
      <ErrorState
        title={t('common.errorTitle')}
        body={t('common.errorMessage')}
        retryLabel={t('common.retry')}
        onRetry={() => {
          void bookQuery.refetch()
          void chapterQuery.refetch()
        }}
      />
    )
  }

  const previousChapter = chapter.previous_chapter_id ?? toc[currentChapterIndex - 1]?.chapter_id
  const nextChapter = chapter.next_chapter_id ?? toc[currentChapterIndex + 1]?.chapter_id

  const actionProps = {
    onTranslate: () => void translateSelection(),
    onDictionary: () => void addSelectionToDictionary(),
    onHighlight: () => void highlightSelection(),
    onNote: () => void noteSelection(),
    onCopy: () => void copySelection(),
    onShare: () => void shareSelection()
  }

  return (
    <div className={styles.reader} data-theme={preferences.theme} style={readerStyle}>
      <header className={styles.toolbar}>
        <div className={styles.toolbarLeft}>
          <IconButton
            className={styles.readerIcon}
            icon={X}
            label={t('reader.exit')}
            onClick={() => void finishAndExit()}
          />
          <IconButton
            className={styles.readerIcon}
            icon={ListTree}
            label={t('reader.contents')}
            onClick={() => setTocOpen(!tocOpen)}
          />
        </div>
        <div className={styles.toolbarTitle}>
          <span className={styles.toolbarBook}>{book.title}</span>
          <span className={styles.toolbarChapter}>{chapter.title}</span>
        </div>
        <div className={styles.toolbarRight}>
          <IconButton
            className={clsx(styles.readerIcon, styles.desktopOnly)}
            icon={Search}
            label={t('reader.search')}
            onClick={() => setSearchOpen(true)}
          />
          <IconButton
            className={styles.readerIcon}
            icon={Bookmark}
            label={t('reader.bookmark')}
            loading={addBookmark.isPending}
            onClick={() => void addCurrentBookmark()}
          />
          <IconButton
            className={clsx(styles.readerIcon, styles.desktopOnly)}
            icon={Maximize2}
            label={t('reader.fullscreen')}
            onClick={() => void toggleFullscreen()}
          />
          <IconButton
            className={styles.readerIcon}
            icon={Settings2}
            label={t('reader.appearance')}
            onClick={() => setSettingsOpen(!settingsOpen)}
          />
        </div>
      </header>

      <div
        ref={viewportRef}
        className={clsx(
          styles.viewport,
          preferences.reading_mode === 'paged' && styles.viewportPaged
        )}
        onScroll={preferences.reading_mode === 'scroll' ? onReaderScroll : undefined}
      >
        <article
          ref={contentRef}
          className={clsx(
            styles.content,
            preferences.reading_mode === 'paged' && styles.contentPaged,
            !positionReady && styles.contentRestoring
          )}
          onMouseUp={handleSelection}
          onWheel={preferences.reading_mode === 'paged' ? onPagedWheel : undefined}
          onScroll={preferences.reading_mode === 'paged' ? onReaderScroll : undefined}
        >
          {preferences.reading_mode === 'scroll' && resumeMarkerTop !== undefined ? (
            <div
              className={styles.resumeMarker}
              style={{ top: resumeMarkerTop }}
              role="note"
              aria-label={t('reader.resumeMarker')}
            >
              <span>{t('reader.resumeMarker')}</span>
            </div>
          ) : null}
          <header className={styles.chapterHeading}>
            <span className={styles.chapterEyebrow}>
              {t('reader.chapterOf', { current: currentChapterIndex + 1, total: toc.length })}
            </span>
            <h1 className={styles.chapterTitle}>{chapter.title}</h1>
          </header>
          <div dangerouslySetInnerHTML={{ __html: readerHtml }} />
          <nav className={styles.chapterNavigation} aria-label={t('reader.contents')}>
            <button
              type="button"
              className={styles.chapterButton}
              disabled={!previousChapter}
              onClick={() => previousChapter && goToChapter(previousChapter)}
            >
              <ChevronLeft size={17} aria-hidden="true" />
              {t('reader.chapterPrevious')}
            </button>
            <button
              type="button"
              className={styles.chapterButton}
              disabled={!nextChapter}
              onClick={() => nextChapter && goToChapter(nextChapter)}
            >
              {t('reader.chapterNext')}
              <ChevronRight size={17} aria-hidden="true" />
            </button>
          </nav>
        </article>
      </div>

      {preferences.show_progress || preferences.show_remaining_time ? (
        <footer className={styles.bottomStatus}>
          {preferences.show_progress ? (
            <div className={styles.progressLine}>
              <div className={styles.progressLineValue} style={{ width: `${readingPercent}%` }} />
            </div>
          ) : null}
          <div className={styles.statusInner}>
            <span>
              {t('reader.chapterOf', { current: currentChapterIndex + 1, total: toc.length })}
            </span>
            {preferences.show_progress ? (
              <span>{t('reader.readingProgress', { value: Math.round(readingPercent) })}</span>
            ) : null}
            {preferences.show_remaining_time ? (
              <span>{t('reader.remaining', { count: book.estimated_minutes_remaining ?? 0 })}</span>
            ) : null}
          </div>
        </footer>
      ) : null}

      {!online ? (
        <div className={styles.offlineNotice} role="status">
          {t('reader.offlineChapter')}
        </div>
      ) : null}

      {tocOpen || settingsOpen ? (
        <button
          className={styles.panelOverlay}
          type="button"
          aria-label={t('common.close')}
          onClick={() => {
            setTocOpen(false)
            setSettingsOpen(false)
          }}
        />
      ) : null}
      {tocOpen ? (
        <aside className={clsx(styles.panel, styles.panelLeft)} aria-label={t('reader.contents')}>
          <div className={styles.panelHeader}>
            <h2 className={styles.panelTitle}>{t('reader.contents')}</h2>
            <IconButton
              className={styles.readerIcon}
              size="small"
              icon={X}
              label={t('common.close')}
              onClick={() => setTocOpen(false)}
            />
          </div>
          <nav className={styles.toc}>
            {toc.map((item, index) => (
              <button
                key={item.id}
                type="button"
                className={clsx(
                  styles.tocItem,
                  item.chapter_id === chapterId && styles.tocItemActive
                )}
                style={{ paddingLeft: `${8 + item.level * 12}px` }}
                onClick={() => goToChapter(item.chapter_id)}
              >
                <span className={styles.tocNumber}>{index + 1}</span>
                <span>{item.title}</span>
              </button>
            ))}
          </nav>
        </aside>
      ) : null}
      {settingsOpen ? (
        <ReaderSettings
          preferences={preferences}
          onChange={updatePreferences}
          onClose={() => setSettingsOpen(false)}
        />
      ) : null}

      {selection && !mobile ? (
        <SelectionToolbar position={selection.toolbar} {...actionProps} />
      ) : null}
      {selection && translationValue && !mobile ? (
        <TranslationPopover
          selectedText={selection.text}
          value={translationValue}
          loading={translation.isPending}
          error={translation.isError}
          addError={createDictionary.isError}
          adding={createDictionary.isPending}
          position={selection.popover}
          added={dictionaryAdded}
          onAdd={() => void addSelectionToDictionary()}
          onClose={() => setTranslationValue(undefined)}
        />
      ) : selection && translation.isPending && !mobile ? (
        <TranslationPopover
          selectedText={selection.text}
          loading
          error={false}
          position={selection.popover}
          added={false}
          onAdd={() => undefined}
          onClose={() => translation.reset()}
        />
      ) : null}

      <BottomSheet
        open={Boolean(selection && mobile)}
        onClose={() => setSelection(undefined)}
        label={t('reader.selectHint')}
      >
        {selection ? (
          <div className={styles.mobileSelection}>
            <div className={styles.mobileActions}>
              <MobileAction
                icon={Languages}
                label={t('reader.translate')}
                onClick={actionProps.onTranslate}
              />
              <MobileAction
                icon={BookOpen}
                label={t('reader.addDictionary')}
                onClick={actionProps.onDictionary}
              />
              <MobileAction
                icon={Highlighter}
                label={t('reader.highlight')}
                onClick={actionProps.onHighlight}
              />
              <MobileAction
                icon={NotebookPen}
                label={t('reader.addNote')}
                onClick={actionProps.onNote}
              />
              <MobileAction icon={Copy} label={t('reader.copy')} onClick={actionProps.onCopy} />
              <MobileAction icon={Share2} label={t('reader.share')} onClick={actionProps.onShare} />
            </div>
            {translationValue || translation.isPending || translation.isError ? (
              <TranslationPopover
                mobile
                selectedText={selection.text}
                value={translationValue}
                loading={translation.isPending}
                error={translation.isError}
                addError={createDictionary.isError}
                adding={createDictionary.isPending}
                added={dictionaryAdded}
                onAdd={() => void addSelectionToDictionary()}
                onClose={() => translation.reset()}
              />
            ) : null}
          </div>
        ) : null}
      </BottomSheet>

      <Dialog
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        title={t('reader.search')}
        closeLabel={t('common.close')}
      >
        <Input
          autoFocus
          value={bookSearch}
          placeholder={t('reader.search')}
          aria-label={t('reader.search')}
          onChange={(event) => setBookSearch(event.target.value)}
        />
        <div className={styles.searchResults}>
          {searchResults.map((result, index) => (
            <div key={`${result}-${index}`} className={styles.searchResult}>
              {result}
            </div>
          ))}
          {bookSearch.length > 1 && !searchResults.length ? <p>{t('common.noResults')}</p> : null}
        </div>
      </Dialog>
    </div>
  )
}

function MobileAction({
  icon: Icon,
  label,
  onClick
}: {
  icon: typeof Languages
  label: string
  onClick: () => void
}) {
  return (
    <Button variant="ghost" startIcon={Icon} onClick={onClick}>
      {label}
    </Button>
  )
}
