/** Keeps paragraph boundaries from a DOM selection while removing layout-only whitespace. */
export function formatSelectedText(value: string, maxLength = 20_000): string {
  return value
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((line) => line.replace(/[\t\f\v ]+/g, ' ').trim())
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
    .slice(0, maxLength)
}
