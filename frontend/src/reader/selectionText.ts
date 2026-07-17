/** Keeps paragraph boundaries from a DOM selection while removing layout-only whitespace. */
export function formatSelectedText(value: string, maxLength = 20_000): string {
  return decodeHtmlEntities(value)
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((line) => line.replace(/[\t\f\v ]+/g, ' ').trim())
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
    .slice(0, maxLength)
}

const namedEntities: Record<string, string> = {
  amp: '&',
  apos: "'",
  gt: '>',
  lt: '<',
  nbsp: '\u00a0',
  quot: '"'
}

/** Decodes numeric and common named entities, including malformed numeric entities without `;`. */
export function decodeHtmlEntities(value: string): string {
  let decoded = value
  for (let pass = 0; pass < 3; pass += 1) {
    const next = decoded.replace(
      /&(?:#(x[0-9a-f]+|\d+)|(amp|apos|gt|lt|nbsp|quot));?/gi,
      (entity, numeric: string | undefined, named: string | undefined) => {
        if (numeric) {
          const codePoint =
            numeric[0]?.toLowerCase() === 'x'
              ? Number.parseInt(numeric.slice(1), 16)
              : Number.parseInt(numeric, 10)
          if (Number.isFinite(codePoint) && codePoint > 0 && codePoint <= 0x10ffff) {
            return String.fromCodePoint(codePoint)
          }
          return entity
        }
        return namedEntities[named?.toLowerCase() ?? ''] ?? entity
      }
    )
    if (next === decoded) break
    decoded = next
  }
  return decoded
}
