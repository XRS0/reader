export interface WeekRange {
  from: string
  to: string
  days: string[]
}

interface CalendarParts {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  second: number
}

function partsAt(instant: Date, timezone: string): CalendarParts {
  const values = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23'
    })
      .formatToParts(instant)
      .filter((part) => part.type !== 'literal')
      .map((part) => [part.type, Number(part.value)])
  )
  return values as unknown as CalendarParts
}

function calendarDateAt(instant: Date, timezone: string): string {
  const { year, month, day } = partsAt(instant, timezone)
  return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

export function addCalendarDays(date: string, days: number): string {
  const [year, month, day] = date.split('-').map(Number)
  const value = new Date(Date.UTC(year!, month! - 1, day! + days))
  return value.toISOString().slice(0, 10)
}

function zonedMidnight(date: string, timezone: string): Date {
  const [year, month, day] = date.split('-').map(Number)
  const desired = Date.UTC(year!, month! - 1, day)
  let candidate = desired
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const parts = partsAt(new Date(candidate), timezone)
    const represented = Date.UTC(
      parts.year,
      parts.month - 1,
      parts.day,
      parts.hour,
      parts.minute,
      parts.second
    )
    candidate = desired - (represented - candidate)
  }
  return new Date(candidate)
}

export function getWeekRange(timezone: string, weekOffset = 0, now = new Date()): WeekRange {
  const today = calendarDateAt(now, timezone)
  const [year, month, day] = today.split('-').map(Number)
  const weekday = new Date(Date.UTC(year!, month! - 1, day)).getUTCDay()
  const daysSinceMonday = (weekday + 6) % 7
  const monday = addCalendarDays(today, -daysSinceMonday + weekOffset * 7)
  const days = Array.from({ length: 7 }, (_, index) => addCalendarDays(monday, index))
  const nextMonday = addCalendarDays(monday, 7)
  return {
    from: zonedMidnight(monday, timezone).toISOString(),
    to: zonedMidnight(nextMonday, timezone).toISOString(),
    days
  }
}

export function formatWeekRange(days: string[], locale: string): string {
  const formatter = new Intl.DateTimeFormat(locale, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC'
  })
  const first = new Date(`${days[0]}T12:00:00Z`)
  const last = new Date(`${days[days.length - 1]}T12:00:00Z`)
  return `${formatter.format(first)} — ${formatter.format(last)}`
}

export function formatWeekday(date: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, { weekday: 'short', timeZone: 'UTC' }).format(
    new Date(`${date}T12:00:00Z`)
  )
}
