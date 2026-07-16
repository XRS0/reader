import {
  BookMarked,
  ChevronLeft,
  ChevronRight,
  Highlighter,
  Library,
  NotebookPen,
  Search,
  Settings,
  Sparkles,
  Star,
  Languages
} from 'lucide-react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { Avatar, IconButton, Tooltip } from '../shared/ui'
import { useCurrentUser } from '../api/hooks'
import { useUIStore } from '../stores/uiStore'
import styles from './shell.module.css'

const navItems = [
  { to: '/library', key: 'library', icon: Library },
  { to: '/library?favorite=true', key: 'favorites', icon: Star },
  { to: '/dictionary', key: 'dictionary', icon: Languages },
  { to: '/statistics', key: 'statistics', icon: Sparkles },
  { to: '/notes', key: 'notes', icon: NotebookPen },
  { to: '/highlights', key: 'highlights', icon: Highlighter },
  { to: '/settings', key: 'settings', icon: Settings }
] as const

export function Sidebar({
  drawer = false,
  onNavigate
}: {
  drawer?: boolean
  onNavigate?: () => void
}) {
  const { t } = useTranslation()
  const location = useLocation()
  const collapsed = useUIStore((state) => state.sidebarCollapsed)
  const toggleSidebar = useUIStore((state) => state.toggleSidebar)
  const setCommandOpen = useUIStore((state) => state.setCommandOpen)
  const user = useCurrentUser().data?.user
  const isCollapsed = collapsed && !drawer
  const activeLibraryItem = getActiveLibraryItem(location.pathname, location.search)

  const navLink = (item: (typeof navItems)[number]) => {
    const Icon = item.icon
    const label = t(`nav.${item.key}`)
    const active = item.to.startsWith('/library')
      ? activeLibraryItem === item.key
      : location.pathname === item.to
    const content = (
      <Link
        to={item.to}
        onClick={onNavigate}
        className={clsx(styles.navLink, active && styles.navLinkActive)}
        aria-current={active ? 'page' : undefined}
        aria-label={isCollapsed ? label : undefined}
      >
        <Icon size={17} aria-hidden="true" />
        <span className={styles.navLabel}>{label}</span>
      </Link>
    )
    return isCollapsed ? (
      <Tooltip key={item.key} content={label}>
        {content}
      </Tooltip>
    ) : (
      <span key={item.key} style={{ display: 'contents' }}>
        {content}
      </span>
    )
  }

  return (
    <aside
      className={clsx(
        styles.sidebar,
        isCollapsed && styles.collapsed,
        drawer && styles.drawerSidebar
      )}
      aria-label={t('common.appName')}
    >
      <div className={styles.sidebarHeader}>
        <Link
          to="/library"
          className={styles.logo}
          onClick={onNavigate}
          aria-label={t('common.appName')}
        >
          <span className={styles.logoMark}>
            <BookMarked size={17} aria-hidden="true" />
          </span>
          <span className={styles.logoText}>{t('common.appName')}</span>
        </Link>
        {!isCollapsed && !drawer ? (
          <IconButton
            size="small"
            icon={ChevronLeft}
            label={t('nav.collapse')}
            onClick={toggleSidebar}
          />
        ) : null}
      </div>

      {isCollapsed ? (
        <Tooltip content={t('nav.expand')}>
          <IconButton
            className={styles.railToggle}
            size="small"
            icon={ChevronRight}
            label={t('nav.expand')}
            onClick={toggleSidebar}
          />
        </Tooltip>
      ) : null}

      {isCollapsed ? (
        <Tooltip content={t('common.search')}>
          <button
            type="button"
            className={styles.searchButton}
            aria-label={t('common.search')}
            onClick={() => setCommandOpen(true)}
          >
            <Search size={17} aria-hidden="true" />
            <span>{t('common.search')}</span>
          </button>
        </Tooltip>
      ) : (
        <button type="button" className={styles.searchButton} onClick={() => setCommandOpen(true)}>
          <Search size={17} aria-hidden="true" />
          <span>{t('common.search')}</span>
          <kbd className={styles.shortcut}>⌘ K</kbd>
        </button>
      )}

      <div className={styles.sidebarScroll}>
        <nav className={styles.navSection} aria-label={t('common.appName')}>
          {navItems.map(navLink)}
        </nav>
      </div>

      <div className={styles.sidebarFooter}>
        {isCollapsed ? (
          <Tooltip content={user?.display_name ?? t('nav.profile')}>
            <NavLink className={styles.profileLink} to="/profile">
              <Avatar name={user?.display_name ?? '?'} />
            </NavLink>
          </Tooltip>
        ) : (
          <NavLink className={styles.profileLink} to="/profile" onClick={onNavigate}>
            <Avatar name={user?.display_name ?? '?'} />
            <span className={styles.profileText}>
              <span className={styles.profileName}>{user?.display_name ?? t('nav.profile')}</span>
              <span className={styles.profileEmail}>{user?.email ?? ''}</span>
            </span>
          </NavLink>
        )}
      </div>
    </aside>
  )
}

export function getActiveLibraryItem(pathname: string, search: string): string | undefined {
  if (pathname !== '/library') return undefined
  const params = new URLSearchParams(search)
  if (params.get('favorite') === 'true') return 'favorites'
  return 'library'
}
