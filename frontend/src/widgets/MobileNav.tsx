import { BarChart3, BookOpenText, Languages, Library, UserRound } from 'lucide-react'
import { NavLink } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { useBooks } from '../api/hooks'
import styles from './shell.module.css'

export function MobileNav() {
  const { t } = useTranslation()
  const recent = useBooks({ sort: 'last_read', limit: 1 }).data?.items[0]
  const items = [
    { to: '/library', label: t('nav.library'), icon: Library },
    {
      to: recent?.processing_status === 'ready' ? `/read/${recent.id}` : '/library',
      label: t('nav.continue'),
      icon: BookOpenText
    },
    { to: '/dictionary', label: t('nav.dictionary'), icon: Languages },
    { to: '/statistics', label: t('nav.statistics'), icon: BarChart3 },
    { to: '/profile', label: t('nav.profile'), icon: UserRound }
  ]
  return (
    <nav className={styles.mobileNav} aria-label={t('common.appName')}>
      {items.map((item) => {
        const Icon = item.icon
        return (
          <NavLink
            key={item.label}
            to={item.to}
            className={({ isActive }) =>
              clsx(styles.mobileNavLink, isActive && styles.mobileNavActive)
            }
          >
            <Icon size={20} aria-hidden="true" />
            <span>{item.label}</span>
          </NavLink>
        )
      })}
    </nav>
  )
}
