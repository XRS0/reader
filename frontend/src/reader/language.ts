export function resolveSourceLanguage(
  reportedLanguage: string | undefined,
  bookLanguage: string | undefined,
  text: string
): string {
  const reported = reportedLanguage?.trim()
  if (reported) return reported
  const book = bookLanguage?.trim()
  if (book && book !== 'und') return book
  return /[а-яё]/i.test(text) ? 'ru' : 'en'
}
