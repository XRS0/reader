/**
 * A CSS multi-column page advances by one column width plus the rendered
 * column gap. The gap can differ from the stored reader margin at responsive
 * breakpoints, so derive it from computed CSS rather than duplicating the CSS
 * calculation in JavaScript.
 */
export function calculatePagedScrollStep(clientWidth: number, computedColumnGap: string): number {
  const parsedGap = Number.parseFloat(computedColumnGap)
  const columnGap = Number.isFinite(parsedGap) ? parsedGap : 0
  return Math.max(0, clientWidth + columnGap)
}

function clampScrollTarget(target: number, maximum: number): number {
  return Math.min(Math.max(0, target), Math.max(0, maximum))
}

/** Converts a persisted chapter percentage back into a physical scroll position. */
export function calculateResumeTarget(scrollPercent: number, maximum: number): number {
  const percent = Math.min(100, Math.max(0, Number.isFinite(scrollPercent) ? scrollPercent : 0))
  return clampScrollTarget((percent / 100) * maximum, maximum)
}

/** Returns the closest complete page for a drag/touch scroll that ended between columns. */
export function calculatePagedSnapTarget(
  scrollLeft: number,
  pageStep: number,
  maximum: number
): number {
  if (pageStep <= 0) return 0
  return clampScrollTarget(Math.round(scrollLeft / pageStep) * pageStep, maximum)
}

/** Moves from the closest page boundary by exactly one page in either direction. */
export function calculatePagedNavigationTarget(
  scrollLeft: number,
  pageStep: number,
  maximum: number,
  direction: -1 | 1
): number {
  if (pageStep <= 0) return 0
  const currentPage = Math.round(scrollLeft / pageStep)
  return clampScrollTarget((currentPage + direction) * pageStep, maximum)
}
