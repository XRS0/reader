import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import {
  BarChart3,
  BookOpenText,
  FilePlus2,
  Languages,
  Library,
  MoonStar,
  NotebookPen,
  Search,
  Settings
} from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useBooks, useDebouncedValue, useDictionary } from '../api/hooks'
import { useUIStore, type AppTheme } from '../stores/uiStore'
import styles from './shell.module.css'

type CommandGroup = 'navigation' | 'books' | 'actions'
interface CommandItem {
  id: string
  label: string
  meta?: string
  keywords: string
  group: CommandGroup
  icon: typeof Library
  action: () => void
}

const themeSequence: AppTheme[] = ['system', 'light', 'warm', 'dark']

export function CommandPalette() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const open = useUIStore((state) => state.commandOpen)
  const setOpen = useUIStore((state) => state.setCommandOpen)
  const appTheme = useUIStore((state) => state.appTheme)
  const setAppTheme = useUIStore((state) => state.setAppTheme)
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const previousFocus = useRef<HTMLElement | null>(null)
  const debouncedQuery = useDebouncedValue(query, 150)
  const booksData = useBooks({ search: debouncedQuery, sort: 'last_read', limit: 6 }).data?.items
  const dictionaryData = useDictionary({ search: debouncedQuery, limit: 4 }).data?.items
  const books = useMemo(() => booksData ?? [], [booksData])
  const dictionary = useMemo(() => dictionaryData ?? [], [dictionaryData])

  useEffect(() => {
    const onShortcut = (event: globalThis.KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setOpen(!useUIStore.getState().commandOpen)
      }
    }
    document.addEventListener('keydown', onShortcut)
    return () => document.removeEventListener('keydown', onShortcut)
  }, [setOpen])

  useEffect(() => {
    if (!open) return undefined
    previousFocus.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null
    setQuery('')
    setActive(0)
    window.setTimeout(() => inputRef.current?.focus(), 0)
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = ''
      previousFocus.current?.focus()
    }
  }, [open])

  const closeAnd = useCallback(
    (action: () => void) => {
      setOpen(false)
      action()
    },
    [setOpen]
  )

  const items = useMemo<CommandItem[]>(() => {
    const navigation: CommandItem[] = [
      {
        id: 'nav-library',
        label: t('nav.library'),
        keywords: 'library books библиотека книги',
        group: 'navigation',
        icon: Library,
        action: () => closeAnd(() => navigate('/library'))
      },
      {
        id: 'nav-dictionary',
        label: t('nav.dictionary'),
        keywords: 'dictionary words словарь слова',
        group: 'navigation',
        icon: Languages,
        action: () => closeAnd(() => navigate('/dictionary'))
      },
      {
        id: 'nav-stats',
        label: t('nav.statistics'),
        keywords: 'statistics activity статистика',
        group: 'navigation',
        icon: BarChart3,
        action: () => closeAnd(() => navigate('/statistics'))
      },
      {
        id: 'nav-notes',
        label: t('nav.notes'),
        keywords: 'notes заметки',
        group: 'navigation',
        icon: NotebookPen,
        action: () => closeAnd(() => navigate('/notes'))
      },
      {
        id: 'nav-settings',
        label: t('nav.settings'),
        keywords: 'settings настройки',
        group: 'navigation',
        icon: Settings,
        action: () => closeAnd(() => navigate('/settings'))
      }
    ]
    const bookItems: CommandItem[] = books.map((book) => ({
      id: `book-${book.id}`,
      label: book.title,
      meta: book.author,
      keywords: `${book.title} ${book.author}`,
      group: 'books',
      icon: BookOpenText,
      action: () =>
        closeAnd(() =>
          navigate(book.processing_status === 'ready' ? `/read/${book.id}` : `/books/${book.id}`)
        )
    }))
    const wordItems: CommandItem[] = dictionary.map((entry) => ({
      id: `word-${entry.id}`,
      label: entry.original_word,
      meta: entry.translation,
      keywords: `${entry.original_word} ${entry.translation}`,
      group: 'books',
      icon: Languages,
      action: () => closeAnd(() => navigate(`/dictionary?entry=${entry.id}`))
    }))
    const nextTheme =
      themeSequence[(themeSequence.indexOf(appTheme) + 1) % themeSequence.length] ?? 'system'
    const actions: CommandItem[] = [
      {
        id: 'action-continue',
        label: t('command.continueReading'),
        keywords: 'continue read продолжить читать',
        group: 'actions',
        icon: BookOpenText,
        action: () => {
          const book = books.find((value) => value.processing_status === 'ready')
          closeAnd(() => navigate(book ? `/read/${book.id}` : '/library'))
        }
      },
      {
        id: 'action-note',
        label: t('command.createNote'),
        keywords: 'create new note новая заметка',
        group: 'actions',
        icon: FilePlus2,
        action: () => closeAnd(() => navigate('/notes?new=1'))
      },
      {
        id: 'action-theme',
        label: t('command.switchTheme'),
        meta: nextTheme,
        keywords: 'theme dark warm тема тёмная',
        group: 'actions',
        icon: MoonStar,
        action: () => {
          setAppTheme(nextTheme)
          setOpen(false)
        }
      }
    ]
    const all = [...navigation, ...bookItems, ...wordItems, ...actions]
    const normalized = query.trim().toLowerCase()
    if (!normalized) return all
    return all.filter((item) =>
      `${item.label} ${item.meta ?? ''} ${item.keywords}`.toLowerCase().includes(normalized)
    )
  }, [appTheme, books, closeAnd, dictionary, navigate, query, setAppTheme, setOpen, t])

  useEffect(() => setActive(0), [query, items.length])

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
    } else if (event.key === 'ArrowDown') {
      event.preventDefault()
      setActive((value) => (items.length ? (value + 1) % items.length : 0))
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      setActive((value) => (items.length ? (value - 1 + items.length) % items.length : 0))
    } else if (event.key === 'Enter' && items[active]) {
      event.preventDefault()
      items[active].action()
    } else if (event.key === 'Tab') {
      event.preventDefault()
    }
  }

  if (!open) return null
  const groups: CommandGroup[] = ['navigation', 'books', 'actions']
  const portal = document.getElementById('portal-root') ?? document.body
  return createPortal(
    <div
      className={`${styles.commandOverlay} ${styles.overlayFallback}`}
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && setOpen(false)}
      style={{
        position: 'fixed',
        inset: 0,
        display: 'grid',
        placeItems: 'start center',
        background: 'var(--color-overlay)'
      }}
    >
      <div
        className={styles.commandDialog}
        role="dialog"
        aria-modal="true"
        aria-label={t('command.title')}
      >
        <div className={styles.commandSearch}>
          <Search size={19} aria-hidden="true" />
          <input
            ref={inputRef}
            className={styles.commandInput}
            value={query}
            placeholder={t('command.placeholder')}
            aria-label={t('command.placeholder')}
            aria-controls="command-results"
            aria-activedescendant={items[active] ? `command-${items[active].id}` : undefined}
            autoComplete="off"
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={onKeyDown}
          />
        </div>
        <div id="command-results" className={styles.commandResults} role="listbox">
          {items.length ? (
            groups.map((group) => {
              const grouped = items.filter((item) => item.group === group)
              if (!grouped.length) return null
              return (
                <section
                  key={group}
                  className={styles.commandGroup}
                  aria-labelledby={`command-group-${group}`}
                >
                  <h2 id={`command-group-${group}`} className={styles.commandGroupTitle}>
                    {t(`command.${group}`)}
                  </h2>
                  {grouped.map((item) => {
                    const itemIndex = items.indexOf(item)
                    const Icon = item.icon
                    return (
                      <button
                        key={item.id}
                        id={`command-${item.id}`}
                        type="button"
                        role="option"
                        aria-selected={itemIndex === active}
                        data-active={itemIndex === active}
                        className={styles.commandItem}
                        onMouseEnter={() => setActive(itemIndex)}
                        onClick={item.action}
                      >
                        <span className={styles.commandItemIcon}>
                          <Icon size={15} aria-hidden="true" />
                        </span>
                        <span>{item.label}</span>
                        {item.meta ? (
                          <span className={styles.commandItemMeta}>{item.meta}</span>
                        ) : null}
                      </button>
                    )
                  })}
                </section>
              )
            })
          ) : (
            <div className={styles.commandEmpty}>{t('command.noResults')}</div>
          )}
        </div>
        <div className={styles.commandFooter}>{t('command.hint')}</div>
      </div>
    </div>,
    portal
  )
}
