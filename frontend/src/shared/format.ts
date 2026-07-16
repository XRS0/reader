export function formatDate(value: string | null | undefined, locale: string): string {
  if (!value) return '—'
  return new Intl.DateTimeFormat(locale, {
    day: 'numeric',
    month: 'short',
    year: 'numeric'
  }).format(new Date(value))
}

export function formatDateTime(value: string | null | undefined, locale: string): string {
  if (!value) return '—'
  return new Intl.DateTimeFormat(locale, {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

export function formatDuration(seconds: number, locale: string): string {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.round((seconds % 3600) / 60)
  if (hours) return locale.startsWith('ru') ? `${hours} ч ${minutes} мин` : `${hours}h ${minutes}m`
  return locale.startsWith('ru') ? `${minutes} мин` : `${minutes}m`
}

export function formatNumber(value: number, locale: string): string {
  return new Intl.NumberFormat(locale).format(Math.round(value))
}
