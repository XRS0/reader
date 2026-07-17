import { useCallback, useEffect } from 'react'
import { Menu, Search } from 'lucide-react'
import { Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { apiConfig } from '../api/config'
import { flushProgressQueue, readProgressQueue } from '../api/offlineQueue'
import { Drawer, IconButton } from '../shared/ui'
import { useOfflineStore } from '../stores/offlineStore'
import { useUIStore } from '../stores/uiStore'
import { CommandPalette } from './CommandPalette'
import { MobileNav } from './MobileNav'
import { Sidebar } from './Sidebar'
import styles from './shell.module.css'

export function AppShell() {
  const { t } = useTranslation()
  const collapsed = useUIStore((state) => state.sidebarCollapsed)
  const drawerOpen = useUIStore((state) => state.mobileDrawerOpen)
  const setDrawerOpen = useUIStore((state) => state.setMobileDrawerOpen)
  const setCommandOpen = useUIStore((state) => state.setCommandOpen)
  const setOnline = useOfflineStore((state) => state.setOnline)
  const setPending = useOfflineStore((state) => state.setPending)
  const setSyncing = useOfflineStore((state) => state.setSyncing)

  const synchronize = useCallback(async () => {
    const online = navigator.onLine
    setOnline(online)
    if (online) {
      setSyncing(true)
      await flushProgressQueue()
      setSyncing(false)
    }
    const queue = await readProgressQueue()
    setPending(queue.length)
  }, [setOnline, setPending, setSyncing])

  useEffect(() => {
    void synchronize()
    const onOnline = () => void synchronize()
    const onOffline = () => setOnline(false)
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    return () => {
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
    }
  }, [setOnline, synchronize])

  return (
    <div className={clsx(styles.shell, collapsed && styles.shellCollapsed)}>
      <a className="skip-link" href="#main-content">
        {t('nav.library')}
      </a>
      <Sidebar />
      <div className={styles.content}>
        {apiConfig.demo ? (
          <div className={styles.demoBanner} role="status">
            {t('common.demo')}
          </div>
        ) : null}
        <header className={styles.mobileHeader}>
          <IconButton icon={Menu} label={t('nav.menu')} onClick={() => setDrawerOpen(true)} />
          <span className={styles.logo} style={{ flex: 'initial' }}>
            <span className={styles.logoMark}>B</span>
            <span>{t('common.appName')}</span>
          </span>
          <IconButton
            icon={Search}
            label={t('common.search')}
            onClick={() => setCommandOpen(true)}
          />
        </header>
        <main id="main-content">
          <Outlet />
        </main>
      </div>
      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        label={t('nav.menu')}
        closeLabel={t('common.close')}
      >
        <Sidebar drawer onNavigate={() => setDrawerOpen(false)} />
      </Drawer>
      <MobileNav />
      <CommandPalette />
    </div>
  )
}
