import { LogOut, MonitorSmartphone, ShieldCheck } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useCurrentUser, useLogout } from '../api/hooks'
import { Button } from '../shared/ui'
import { formatDate } from '../shared/format'
import styles from './pages.module.css'

export function ProfilePage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const user = useCurrentUser().data?.user
  const logout = useLogout()
  const signOut = async () => {
    await logout.mutateAsync()
    void navigate('/login', { replace: true })
  }
  const initials =
    user?.display_name
      .split(/\s+/)
      .map((value) => value[0])
      .join('')
      .slice(0, 2)
      .toUpperCase() ?? '?'
  return (
    <div className={`${styles.page} ${styles.pageNarrow}`}>
      <header className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>{t('profile.title')}</h1>
      </header>
      <section className={styles.profileCard}>
        <div className={styles.profileAvatar}>{initials}</div>
        <div>
          <h2 className={styles.profileName}>{user?.display_name}</h2>
          <p className={styles.profileEmail}>{user?.email}</p>
          <p className={styles.flatMeta}>
            {t('profile.memberSince', { date: formatDate(user?.created_at, i18n.language) })}
          </p>
        </div>
      </section>
      <section className={styles.section}>
        <div className={styles.flatRow}>
          <div>
            <p className={styles.flatTitle}>
              <MonitorSmartphone
                size={16}
                style={{ display: 'inline', verticalAlign: 'middle', marginRight: 8 }}
              />
              {t('profile.devices')}
            </p>
            <p className={styles.flatMeta}>{navigator.platform || 'Web'}</p>
          </div>
          <span>{t('common.online')}</span>
        </div>
        <div className={styles.flatRow}>
          <div>
            <p className={styles.flatTitle}>
              <ShieldCheck
                size={16}
                style={{ display: 'inline', verticalAlign: 'middle', marginRight: 8 }}
              />
              {t('profile.security')}
            </p>
            <p className={styles.flatMeta}>HttpOnly · SameSite · CSRF</p>
          </div>
        </div>
      </section>
      <section className={styles.section}>
        <Button
          startIcon={LogOut}
          variant="danger"
          loading={logout.isPending}
          onClick={() => void signOut()}
        >
          {t('auth.logout')}
        </Button>
      </section>
    </div>
  )
}
