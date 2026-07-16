import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCurrentUser, useStatistics } from '../api/hooks'
import {
  DataTable,
  EmptyState,
  ErrorState,
  IconButton,
  Skeleton,
  Tabs,
  type Column
} from '../shared/ui'
import { formatDate, formatDateTime, formatDuration, formatNumber } from '../shared/format'
import { formatWeekRange, formatWeekday, getWeekRange } from '../shared/week'
import type { BookStatistic, DailyStatistic, ReadingSession } from '../types/api'
import styles from './pages.module.css'

export function StatisticsPage() {
  const { t, i18n } = useTranslation()
  const [weekOffset, setWeekOffset] = useState(0)
  const timezone =
    useCurrentUser().data?.user.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
  const week = useMemo(() => getWeekRange(timezone, weekOffset), [timezone, weekOffset])
  const queries = useStatistics(timezone, 7, { from: week.from, to: week.to })
  const overview = queries.overview.data
  const daily = useMemo<DailyStatistic[]>(() => {
    const byDate = new Map((queries.daily.data ?? []).map((item) => [item.date, item]))
    return week.days.map(
      (date) =>
        byDate.get(date) ?? {
          date,
          active_seconds: 0,
          idle_seconds: 0,
          sessions_count: 0,
          words_read_estimate: 0
        }
    )
  }, [queries.daily.data, week.days])
  const hasWeekActivity = daily.some((item) => item.active_seconds > 0)

  const bookColumns = useMemo<Column<BookStatistic>[]>(
    () => [
      {
        key: 'book',
        header: t('library.tableBook'),
        render: (book) => <strong>{book.book_title}</strong>
      },
      {
        key: 'time',
        header: t('statistics.activeTime'),
        render: (book) => formatDuration(book.active_seconds, i18n.language)
      },
      { key: 'sessions', header: t('statistics.sessions'), render: (book) => book.sessions_count },
      {
        key: 'progress',
        header: t('library.tableProgress'),
        render: (book) => `${Math.round(book.progress_percent)}%`
      },
      {
        key: 'speed',
        header: t('statistics.averageSpeed'),
        render: (book) => t('statistics.wpm', { count: Math.round(book.average_words_per_minute) })
      },
      {
        key: 'last',
        header: t('library.tableLastRead'),
        render: (book) => formatDate(book.last_read_at, i18n.language)
      }
    ],
    [i18n.language, t]
  )
  const sessionColumns = useMemo<Column<ReadingSession>[]>(
    () => [
      {
        key: 'start',
        header: t('statistics.startedAt'),
        render: (session) => formatDateTime(session.started_at, i18n.language)
      },
      {
        key: 'end',
        header: t('statistics.endedAt'),
        render: (session) => formatDateTime(session.ended_at, i18n.language)
      },
      {
        key: 'active',
        header: t('statistics.active'),
        render: (session) => formatDuration(session.active_seconds, i18n.language)
      },
      {
        key: 'idle',
        header: t('statistics.idle'),
        render: (session) => formatDuration(session.idle_seconds, i18n.language)
      },
      {
        key: 'progress',
        header: t('library.tableProgress'),
        render: (session) =>
          `${Math.round(session.start_progress_percent)}% → ${Math.round(session.end_progress_percent ?? session.start_progress_percent)}%`
      },
      {
        key: 'status',
        header: t('statistics.sessionStatusLabel'),
        render: (session) => t(`statistics.sessionStatus.${session.status}`)
      }
    ],
    [i18n.language, t]
  )

  if (queries.overview.isLoading) {
    return (
      <div className={styles.page}>
        <div className={styles.statsGrid}>
          {Array.from({ length: 8 }, (_, index) => (
            <Skeleton key={index} height={86} />
          ))}
        </div>
      </div>
    )
  }
  if (queries.overview.isError) {
    return (
      <ErrorState
        title={t('common.errorTitle')}
        body={t('common.errorMessage')}
        retryLabel={t('common.retry')}
        onRetry={() => void queries.overview.refetch()}
      />
    )
  }

  const overviewContent = (
    <>
      <div className={styles.statsGrid}>
        <Metric
          label={t('statistics.activeTime')}
          value={formatDuration(overview?.active_reading_seconds ?? 0, i18n.language)}
          foot={`${t('statistics.totalTime')}: ${formatDuration(overview?.total_reading_seconds ?? 0, i18n.language)}`}
        />
        <Metric
          label={t('statistics.sessions')}
          value={formatNumber(overview?.sessions_count ?? 0, i18n.language)}
          foot={`${t('statistics.averageSession')}: ${formatDuration(overview?.average_session_seconds ?? 0, i18n.language)}`}
        />
        <Metric
          label={t('statistics.pages')}
          value={formatNumber(overview?.pages_read_estimate ?? 0, i18n.language)}
          foot={
            formatNumber(overview?.words_read_estimate ?? 0, i18n.language) +
            ` ${t('statistics.words').toLowerCase()}`
          }
        />
        <Metric
          label={t('statistics.streak')}
          value={t('statistics.days', { count: overview?.current_streak_days ?? 0 })}
          foot={`${t('statistics.averageSpeed')}: ${t('statistics.wpm', { count: overview?.average_words_per_minute ?? 0 })}`}
        />
      </div>
      <section className={styles.section} aria-labelledby="activity-chart">
        <div className={styles.sectionHeader}>
          <h2 id="activity-chart" className={styles.sectionTitle}>
            {t('statistics.readingActivity')}
          </h2>
          <div className={styles.legend}>
            <span className={styles.legendItem}>
              <span className={styles.legendDot} />
              {t('statistics.active')}
            </span>
          </div>
        </div>
        <ActivityChart
          values={daily.map((item) => ({
            date: item.date,
            label: formatWeekday(item.date, i18n.language),
            value: item.active_seconds
          }))}
        />
        {!hasWeekActivity ? (
          <p className={styles.emptyChartNote}>{t('statistics.noActivity')}</p>
        ) : null}
      </section>
    </>
  )

  const vocabularyContent = (
    <div className={styles.statsGrid}>
      <Metric
        label={t('dictionary.title')}
        value={formatNumber(overview?.dictionary_words ?? 0, i18n.language)}
      />
      <Metric
        label={t('dictionary.mastered')}
        value={formatNumber(overview?.learned_words ?? 0, i18n.language)}
      />
      <Metric
        label={t('reader.translation')}
        value={formatNumber(overview?.translations_count ?? 0, i18n.language)}
      />
      <Metric
        label={t('dictionary.learning')}
        value={formatNumber(
          Math.max(0, (overview?.dictionary_words ?? 0) - (overview?.learned_words ?? 0)),
          i18n.language
        )}
      />
    </div>
  )

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>{t('statistics.title')}</h1>
          <p className={styles.pageSubtitle}>{t('statistics.subtitle')}</p>
        </div>
        <div
          className={styles.weekNavigator}
          role="group"
          aria-label={t('statistics.weekNavigation')}
        >
          <IconButton
            icon={ChevronLeft}
            label={t('statistics.previousWeek')}
            onClick={() => setWeekOffset((value) => value - 1)}
          />
          <div className={styles.weekRange} aria-live="polite">
            <strong>
              {weekOffset === 0 ? t('statistics.currentWeek') : t('statistics.selectedWeek')}
            </strong>
            <span>{formatWeekRange(week.days, i18n.language)}</span>
          </div>
          <IconButton
            icon={ChevronRight}
            label={t('statistics.nextWeek')}
            disabled={weekOffset >= 0}
            onClick={() => setWeekOffset((value) => Math.min(0, value + 1))}
          />
        </div>
      </header>
      <Tabs
        items={[
          { id: 'overview', label: t('book.overview'), content: overviewContent },
          {
            id: 'daily',
            label: t('statistics.readingActivity'),
            content: <ActivityTable values={daily} />
          },
          {
            id: 'books',
            label: t('statistics.byBooks'),
            content: queries.books.data?.length ? (
              <DataTable
                columns={bookColumns}
                items={queries.books.data}
                rowKey={(book) => book.book_id}
                label={t('statistics.byBooks')}
              />
            ) : (
              <EmptyState title={t('statistics.noActivity')} body={t('statistics.subtitle')} />
            )
          },
          {
            id: 'sessions',
            label: t('statistics.sessionHistory'),
            content: queries.sessions.data?.items.length ? (
              <DataTable
                columns={sessionColumns}
                items={queries.sessions.data.items}
                rowKey={(session) => session.id}
                label={t('statistics.sessionHistory')}
              />
            ) : (
              <EmptyState title={t('statistics.noActivity')} body={t('statistics.subtitle')} />
            )
          },
          { id: 'vocabulary', label: t('statistics.vocabulary'), content: vocabularyContent }
        ]}
      />
    </div>
  )
}

function Metric({ label, value, foot }: { label: string; value: string; foot?: string }) {
  return (
    <div className={styles.metric}>
      <span className={styles.metricLabel}>{label}</span>
      <strong className={styles.metricValue}>{value}</strong>
      {foot ? <span className={styles.metricFoot}>{foot}</span> : null}
    </div>
  )
}
function ActivityChart({
  values
}: {
  values: Array<{ date: string; label: string; value: number }>
}) {
  const max = Math.max(...values.map((item) => item.value), 1)
  return (
    <div className={styles.chart} role="img" aria-label="Reading activity">
      {values.map((item) => (
        <div
          key={item.date}
          className={styles.chartColumn}
          title={`${item.date}: ${Math.round(item.value / 60)} min`}
        >
          <div className={styles.chartTrack}>
            <div className={styles.chartBar} style={{ height: `${(item.value / max) * 100}%` }} />
          </div>
          <span className={styles.chartLabel}>{item.label}</span>
        </div>
      ))}
    </div>
  )
}
function ActivityTable({
  values
}: {
  values: Array<{
    date: string
    active_seconds: number
    idle_seconds: number
    sessions_count: number
    words_read_estimate: number
  }>
}) {
  const { t, i18n } = useTranslation()
  const columns: Column<(typeof values)[number]>[] = [
    {
      key: 'date',
      header: t('statistics.readingActivity'),
      render: (item) => formatDate(item.date, i18n.language)
    },
    {
      key: 'active',
      header: t('statistics.active'),
      render: (item) => formatDuration(item.active_seconds, i18n.language)
    },
    {
      key: 'idle',
      header: t('statistics.idle'),
      render: (item) => formatDuration(item.idle_seconds, i18n.language)
    },
    { key: 'sessions', header: t('statistics.sessions'), render: (item) => item.sessions_count },
    {
      key: 'words',
      header: t('statistics.words'),
      render: (item) => formatNumber(item.words_read_estimate, i18n.language)
    }
  ]
  return values.length ? (
    <DataTable
      columns={columns}
      items={values}
      rowKey={(item) => item.date}
      label={t('statistics.readingActivity')}
    />
  ) : (
    <EmptyState title={t('statistics.noActivity')} body={t('statistics.subtitle')} />
  )
}
