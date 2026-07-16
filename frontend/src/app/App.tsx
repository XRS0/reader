import { lazy, Suspense, useEffect, type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ApiError } from '../api/http'
import { ensureOfflineOwner } from '../api/offlineQueue'
import { useCurrentUser } from '../api/hooks'
import { ErrorState, LoadingState, ToastProvider } from '../shared/ui'
import { useUIStore } from '../stores/uiStore'
import { applyAppTheme } from '../theme/appTheme'
import { AppShell } from '../widgets/AppShell'

const AuthPage = lazy(() =>
  import('../pages/AuthPage').then((module) => ({ default: module.AuthPage }))
)
const BookPage = lazy(() =>
  import('../pages/BookPage').then((module) => ({ default: module.BookPage }))
)
const DictionaryPage = lazy(() =>
  import('../pages/DictionaryPage').then((module) => ({ default: module.DictionaryPage }))
)
const HighlightsPage = lazy(() =>
  import('../pages/HighlightsPage').then((module) => ({ default: module.HighlightsPage }))
)
const LibraryPage = lazy(() =>
  import('../pages/LibraryPage').then((module) => ({ default: module.LibraryPage }))
)
const NotesPage = lazy(() =>
  import('../pages/NotesPage').then((module) => ({ default: module.NotesPage }))
)
const NotFoundPage = lazy(() =>
  import('../pages/NotFoundPage').then((module) => ({ default: module.NotFoundPage }))
)
const ProfilePage = lazy(() =>
  import('../pages/ProfilePage').then((module) => ({ default: module.ProfilePage }))
)
const SettingsPage = lazy(() =>
  import('../pages/SettingsPage').then((module) => ({ default: module.SettingsPage }))
)
const StatisticsPage = lazy(() =>
  import('../pages/StatisticsPage').then((module) => ({ default: module.StatisticsPage }))
)
const ReaderPage = lazy(() =>
  import('../reader/ReaderPage').then((module) => ({ default: module.ReaderPage }))
)

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1, refetchOnWindowFocus: false },
    mutations: { retry: 0 }
  }
})

function RequireAuth({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const location = useLocation()
  const auth = useCurrentUser()
  if (auth.isLoading) return <LoadingState label={t('auth.sessionChecking')} />
  if (auth.error instanceof ApiError && auth.error.status === 401) {
    return (
      <Navigate to="/login" replace state={{ from: `${location.pathname}${location.search}` }} />
    )
  }
  if (auth.isError) {
    return (
      <ErrorState
        title={t('common.errorTitle')}
        body={t('common.errorMessage')}
        retryLabel={t('common.retry')}
        onRetry={() => void auth.refetch()}
      />
    )
  }
  return auth.data ? (
    <OfflineOwnerBoundary userId={auth.data.user.id}>{children}</OfflineOwnerBoundary>
  ) : null
}

function OfflineOwnerBoundary({ userId, children }: { userId: string; children: ReactNode }) {
  useEffect(() => {
    void ensureOfflineOwner(userId)
  }, [userId])
  return children
}

function PublicOnly({ children }: { children: ReactNode }) {
  const auth = useCurrentUser()
  if (auth.isSuccess) return <Navigate to="/library" replace />
  return children
}

function ThemeAndLocale() {
  const theme = useUIStore((state) => state.appTheme)
  const colors = useUIStore((state) => state.appThemeColors)
  const { i18n } = useTranslation()
  useEffect(() => {
    applyAppTheme(document.documentElement, theme, colors)
  }, [colors, theme])
  useEffect(() => {
    document.documentElement.lang = i18n.language.startsWith('ru') ? 'ru' : 'en'
  }, [i18n.language])
  return null
}

function AppRoutes() {
  const { t } = useTranslation()
  return (
    <Suspense fallback={<LoadingState label={t('common.loading')} />}>
      <Routes>
        <Route
          path="/login"
          element={
            <PublicOnly>
              <AuthPage mode="login" />
            </PublicOnly>
          }
        />
        <Route
          path="/register"
          element={
            <PublicOnly>
              <AuthPage mode="register" />
            </PublicOnly>
          }
        />
        <Route
          element={
            <RequireAuth>
              <AppShell />
            </RequireAuth>
          }
        >
          <Route index element={<Navigate to="/library" replace />} />
          <Route path="/library" element={<LibraryPage />} />
          <Route path="/books/:bookId" element={<BookPage />} />
          <Route path="/dictionary" element={<DictionaryPage />} />
          <Route path="/statistics" element={<StatisticsPage />} />
          <Route path="/notes" element={<NotesPage />} />
          <Route path="/highlights" element={<HighlightsPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/profile" element={<ProfilePage />} />
        </Route>
        <Route
          path="/read/:bookId"
          element={
            <RequireAuth>
              <ReaderPage />
            </RequireAuth>
          }
        />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Suspense>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <ThemeAndLocale />
        <AppRoutes />
      </ToastProvider>
    </QueryClientProvider>
  )
}
