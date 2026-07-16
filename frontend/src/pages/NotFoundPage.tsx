import { BookOpen } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Button } from '../shared/ui'
import styles from './pages.module.css'

export function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <main className={styles.notFound}>
      <BookOpen size={34} aria-hidden="true" />
      <span className={styles.notFoundCode}>404</span>
      <h1 className={styles.pageTitle}>{t('notFound.title')}</h1>
      <p className={styles.pageSubtitle}>{t('notFound.body')}</p>
      <Link to="/library">
        <Button>{t('notFound.home')}</Button>
      </Link>
    </main>
  )
}
