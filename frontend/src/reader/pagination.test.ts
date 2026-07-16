import { describe, expect, it } from 'vitest'
import {
  calculatePagedNavigationTarget,
  calculatePagedScrollStep,
  calculatePagedSnapTarget
} from './pagination'

describe('calculatePagedScrollStep', () => {
  it('uses the actual responsive CSS column gap', () => {
    expect(calculatePagedScrollStep(358, '32px')).toBe(390)
    expect(calculatePagedScrollStep(720, '64px')).toBe(784)
  })

  it('falls back to the column width for a non-numeric gap', () => {
    expect(calculatePagedScrollStep(358, 'normal')).toBe(358)
  })

  it('snaps a free scroll to the closest complete page', () => {
    expect(calculatePagedSnapTarget(422, 390, 1560)).toBe(390)
    expect(calculatePagedSnapTarget(620, 390, 1560)).toBe(780)
    expect(calculatePagedSnapTarget(1600, 390, 1560)).toBe(1560)
  })

  it('navigates exactly one page from the closest boundary and clamps at the ends', () => {
    expect(calculatePagedNavigationTarget(422, 390, 1560, 1)).toBe(780)
    expect(calculatePagedNavigationTarget(422, 390, 1560, -1)).toBe(0)
    expect(calculatePagedNavigationTarget(1560, 390, 1560, 1)).toBe(1560)
  })
})
