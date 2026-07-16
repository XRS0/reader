const transparentLeadingContainers = new Set(['SPAN', 'DIV', 'SECTION', 'ARTICLE'])

function firstSignificantChild(parent: ParentNode): ChildNode | null {
  for (const child of parent.childNodes) {
    if (child.nodeType === Node.COMMENT_NODE) continue
    if (child.nodeType === Node.TEXT_NODE && child.textContent?.trim() === '') continue
    return child
  }
  return null
}

function normalizedHeading(value: string): string {
  return value.normalize('NFKC').replace(/\s+/gu, ' ').trim().toLocaleLowerCase()
}

/**
 * The reader renders its own accessible chapter heading. EPUB documents often
 * repeat the same heading as their first semantic element, sometimes below a
 * few transparent span wrappers. Remove only that exact duplicate; preserve a
 * different heading or any heading preceded by authored content.
 */
export function stripLeadingDuplicateHeading(html: string, chapterTitle: string): string {
  if (!html || !chapterTitle) return html

  const root = document.createElement('div')
  root.innerHTML = html
  let candidate = firstSignificantChild(root)

  while (candidate?.nodeType === Node.ELEMENT_NODE) {
    const element = candidate as Element
    if (!transparentLeadingContainers.has(element.tagName)) break
    candidate = firstSignificantChild(element)
  }

  if (candidate?.nodeType !== Node.ELEMENT_NODE) return html
  const heading = candidate as Element
  if (!/^H[1-6]$/u.test(heading.tagName)) return html
  if (normalizedHeading(heading.textContent ?? '') !== normalizedHeading(chapterTitle)) return html

  let parent = heading.parentElement
  heading.remove()
  while (parent && parent !== root && firstSignificantChild(parent) === null) {
    const next = parent.parentElement
    parent.remove()
    parent = next
  }
  return root.innerHTML
}
