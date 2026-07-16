import { describe, expect, it } from 'vitest'
import { addCalendarDays, formatWeekday, getWeekRange } from './week'

describe('weekly statistics ranges', () => {
  it('returns Monday through Sunday for the current UTC week', () => {
    const range = getWeekRange('UTC', 0, new Date('2026-07-16T12:00:00Z'))
    expect(range.days).toEqual([
      '2026-07-13',
      '2026-07-14',
      '2026-07-15',
      '2026-07-16',
      '2026-07-17',
      '2026-07-18',
      '2026-07-19'
    ])
    expect(range.from).toBe('2026-07-13T00:00:00.000Z')
    expect(range.to).toBe('2026-07-20T00:00:00.000Z')
  })

  it('uses the configured timezone and can navigate to previous weeks', () => {
    const current = getWeekRange('Asia/Yekaterinburg', 0, new Date('2026-07-12T20:30:00Z'))
    const previous = getWeekRange('Asia/Yekaterinburg', -1, new Date('2026-07-12T20:30:00Z'))
    expect(current.days[0]).toBe('2026-07-13')
    expect(current.from).toBe('2026-07-12T19:00:00.000Z')
    expect(previous.days[0]).toBe('2026-07-06')
    expect(previous.to).toBe(current.from)
  })

  it('keeps calendar arithmetic and weekday labels deterministic', () => {
    expect(addCalendarDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(formatWeekday('2026-07-13', 'en')).toMatch(/Mon/i)
  })
})
