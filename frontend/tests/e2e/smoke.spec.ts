import { expect, test } from '@playwright/test'

test('opens the demo library and command palette', async ({ page }) => {
  await page.goto('/library')
  await expect(page.getByRole('heading', { name: /Библиотека|Library/ })).toBeVisible()
  await expect(page.getByText(/Демо-режим|Demo mode/)).toBeVisible()
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K')
  await expect(page.getByRole('dialog', { name: /Быстрый поиск|Quick search/ })).toBeVisible()
})

test('mobile shell exposes bottom navigation', async ({ page }, testInfo) => {
  test.skip(!testInfo.project.name.includes('mobile'), 'mobile project only')
  await page.goto('/library')
  await expect(page.getByRole('navigation').last()).toBeVisible()
  await page
    .getByRole('link', { name: /Словарь|Dictionary/ })
    .last()
    .click()
  await expect(page.getByRole('heading', { name: /Словарь|Dictionary/ })).toBeVisible()
  await expect(page.getByRole('group', { name: /Словарь|Dictionary/ })).toHaveCount(0)
  const overflowsViewport = await page.evaluate(
    () => document.documentElement.scrollWidth > window.innerWidth
  )
  expect(overflowsViewport).toBe(false)
})

test('opens the reader without an update loop', async ({ page }) => {
  const runtimeErrors: string[] = []
  page.on('pageerror', (error) => runtimeErrors.push(error.message))
  await page.goto('/read/019f670d-13bd-7bc3-94fb-000000000001')
  await expect(page.getByRole('heading', { name: 'II. A patient orbit' })).toBeVisible()
  await expect(page.getByText(/Observation, his teacher once said/)).toBeVisible()
  await page.locator('article').click()
  await page.waitForTimeout(4_400)
  await expect(page.getByRole('banner')).toBeVisible()
  await expect(page.getByRole('contentinfo')).toBeVisible()
  expect(runtimeErrors).toEqual([])
})

test('paged reader hides its scrollbar and settles on complete pages', async ({ page }) => {
  await page.goto('/read/019f670d-13bd-7bc3-94fb-000000000001')
  await page.getByRole('button', { name: /Оформление|Appearance/ }).click()
  const appearance = page.getByRole('dialog', { name: /Оформление|Appearance/ })
  await appearance.getByRole('button', { name: /Страницы|Paged/ }).click()
  await appearance.getByRole('button', { name: /Закрыть|Close/ }).click()

  const content = page.locator('article')
  const geometry = await content.evaluate((element) => {
    const style = getComputedStyle(element)
    const step = element.clientWidth + Number.parseFloat(style.columnGap)
    return {
      step,
      maximum: element.scrollWidth - element.clientWidth,
      scrollbarWidth: style.scrollbarWidth,
      webkitScrollbarDisplay: getComputedStyle(element, '::-webkit-scrollbar').display
    }
  })
  expect(geometry.maximum).toBeGreaterThan(0)
  expect(geometry.scrollbarWidth).toBe('none')
  expect(geometry.webkitScrollbarDisplay).toBe('none')

  await content.evaluate((element, step) => {
    element.scrollLeft = step * 0.62
  }, geometry.step)
  const distanceFromPageBoundary = async () => {
    const position = await content.evaluate((element) => element.scrollLeft)
    return Math.min(
      Math.abs(position - geometry.maximum),
      Math.abs(position / geometry.step - Math.round(position / geometry.step)) * geometry.step
    )
  }
  await expect.poll(distanceFromPageBoundary, { timeout: 3_000 }).toBeLessThan(1)
  const snapped = await content.evaluate((element) => element.scrollLeft)

  const navigateForward = snapped < geometry.maximum - 1
  await page.keyboard.press(navigateForward ? 'ArrowRight' : 'ArrowLeft')
  await expect
    .poll(async () => Math.abs((await content.evaluate((element) => element.scrollLeft)) - snapped))
    .toBeGreaterThan(1)
  await expect.poll(distanceFromPageBoundary, { timeout: 3_000 }).toBeLessThan(1)
  const next = await content.evaluate((element) => element.scrollLeft)
  if (navigateForward) expect(next).toBeGreaterThan(snapped)
  else expect(next).toBeLessThan(snapped)
})

test('desktop sidebar has one active destination and a usable collapsed rail', async ({
  page
}, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'desktop project only')
  await page.goto('/library?filter=continue')

  const sidebar = page.getByRole('complementary', { name: 'BookFlow' })
  const navigation = sidebar.getByRole('navigation', { name: 'BookFlow' })
  await expect(navigation.locator('a[aria-current="page"]')).toHaveCount(1)
  await expect(navigation.locator('a[aria-current="page"]')).toHaveAccessibleName(
    /Библиотека|Library/
  )
  await expect(
    navigation.getByRole('link', { name: /Продолжить чтение|Continue reading/ })
  ).toHaveCount(0)
  await expect(navigation.getByRole('link', { name: /^Недавние$|^Recent$/ })).toHaveCount(0)

  const sort = page.getByRole('combobox', { name: /Сортировка|Sort/ })
  await sort.selectOption('title')
  await expect(sort).toHaveValue('title')
  await expect(page).toHaveURL(/sort=title/)
  await sort.selectOption('progress')
  await expect(sort).toHaveValue('progress')
  await expect(page).toHaveURL(/sort=progress/)

  await sidebar.getByRole('button', { name: /Свернуть|Collapse/ }).click()
  await expect(sidebar.getByRole('button', { name: /Развернуть|Expand/ })).toBeVisible()
  await expect(sidebar.getByRole('button', { name: /Поиск|Search/ })).toBeVisible()
  await expect
    .poll(async () => (await sidebar.boundingBox())?.width ?? Number.POSITIVE_INFINITY)
    .toBeLessThanOrEqual(72)

  await sidebar.getByRole('button', { name: /Развернуть|Expand/ }).press('Enter')
  await expect(sidebar.getByRole('button', { name: /Свернуть|Collapse/ })).toBeVisible()
})

test('statistics overview shows a complete week and navigates between weeks', async ({ page }) => {
  await page.goto('/statistics')
  await expect(page.getByRole('heading', { name: /Статистика|Statistics/ })).toBeVisible()

  const navigator = page.getByRole('group', { name: /Переключение недель|Week navigation/ })
  await expect(navigator).toBeVisible()
  await expect(navigator.getByText(/Текущая неделя|Current week/)).toBeVisible()
  const currentRange = await navigator.locator('span').textContent()

  const chart = page.getByRole('img', { name: 'Reading activity' })
  await expect(chart.locator(':scope > div')).toHaveCount(7)

  await navigator.getByRole('button', { name: /Предыдущая неделя|Previous week/ }).click()
  await expect(navigator.getByText(/Текущая неделя|Current week/)).toHaveCount(0)
  await expect.poll(() => navigator.locator('span').textContent()).not.toBe(currentRange)
  await expect(navigator.getByRole('button', { name: /Следующая неделя|Next week/ })).toBeEnabled()

  await navigator.getByRole('button', { name: /Следующая неделя|Next week/ }).click()
  await expect(navigator.getByText(/Текущая неделя|Current week/)).toBeVisible()
  await expect(navigator.getByRole('button', { name: /Следующая неделя|Next week/ })).toBeDisabled()
})

test('dictionary accepts a Russian word with a definition and no translation', async ({ page }) => {
  const word = 'Эпистемологическим'
  await page.goto('/dictionary')
  await page.getByRole('button', { name: /Добавить слово|Add word/ }).click()

  const dialog = page.getByRole('dialog', { name: /Новое слово|New word/ })
  await dialog.getByLabel(/^Слово$|^Word$/).fill(word)
  await dialog
    .getByLabel(/Значение \/ описание|Meaning \/ definition/)
    .fill('Неповторимое своеобразие человека или явления.')
  await dialog.getByRole('button', { name: /Добавить слово|Add word/ }).click()

  await expect(dialog).toBeHidden()
  const wordTitle = page.getByText(word, { exact: true })
  await expect(wordTitle).toBeVisible()
  await expect
    .poll(() =>
      wordTitle.evaluate((element) => {
        const range = document.createRange()
        range.selectNodeContents(element)
        const widthsByLine = new Map<number, number>()
        for (const rect of range.getClientRects()) {
          const lineTop = Math.round(rect.top)
          widthsByLine.set(lineTop, (widthsByLine.get(lineTop) ?? 0) + rect.width)
        }
        const lineWidths = Array.from(widthsByLine.values()).filter((width) => width > 0)
        if (lineWidths.length > 2) return false
        if (lineWidths.length < 2) return true
        return Math.min(...lineWidths) / Math.max(...lineWidths) >= 0.45
      })
    )
    .toBe(true)
  await expect(page.getByText('Неповторимое своеобразие человека или явления.')).toBeVisible()

  const wordCard = page.getByRole('button', {
    name: /^(?:Открыть карточку слова «Эпистемологическим»|Open details for “Эпистемологическим”)$/
  })
  await expect
    .poll(() => wordCard.evaluate((element) => getComputedStyle(element).backgroundColor))
    .toBe('rgba(0, 0, 0, 0)')

  await wordCard.click()
  const details = page.getByRole('dialog', { name: word })
  await expect(
    details.getByRole('paragraph').filter({
      hasText: 'Неповторимое своеобразие человека или явления.'
    })
  ).toBeVisible()

  await details
    .getByRole('button', { name: /Закрыть|Close/ })
    .filter({ hasText: /Закрыть|Close/ })
    .click()
  await expect(details).toBeHidden()
  await expect
    .poll(() => wordCard.evaluate((element) => getComputedStyle(element.parentElement!).transform))
    .toBe('none')

  await wordCard.click()
  await details.getByRole('button', { name: /Удалить|Delete/ }).click()

  const confirmation = page.getByRole('dialog').filter({
    has: page.getByText(/Действие нельзя отменить|This cannot be undone/)
  })
  await confirmation.getByRole('button', { name: /Удалить|Delete/ }).click()
  await expect(page.getByText(word, { exact: true })).toHaveCount(0)
})

test('book details allow adding, replacing and removing a custom cover', async ({ page }) => {
  await page.goto('/books/019f670d-13bd-7bc3-94fb-000000000001')
  const fileInput = page.getByLabel(/Добавить обложку|Add cover/)
  await fileInput.setInputFiles({
    name: 'cover.png',
    mimeType: 'image/png',
    buffer: Buffer.concat([Buffer.from('\x89PNG\r\n\x1a\n', 'binary'), Buffer.alloc(520)])
  })
  await expect(page.getByRole('button', { name: /Заменить обложку|Replace cover/ })).toBeVisible()
  await expect(
    page.getByRole('button', { name: /Удалить свою обложку|Remove custom cover/ })
  ).toBeVisible()

  await page.getByRole('button', { name: /Удалить свою обложку|Remove custom cover/ }).click()
  await expect(page.getByRole('button', { name: /Добавить обложку|Add cover/ })).toBeVisible()
})

test('library allows managing a book avatar from the book card', async ({ page }) => {
  await page.goto('/library')
  const card = page.locator('article').filter({ hasText: 'The Quiet Observatory' }).first()
  await card.hover()
  await card.getByRole('button', { name: /Больше действий|More actions/ }).click()
  await page.getByRole('menuitem', { name: /Аватарка книги|Book avatar/ }).click()

  const dialog = page.getByRole('dialog', { name: /Аватарка книги|Book avatar/ })
  await dialog.locator('input[type="file"]').setInputFiles({
    name: 'avatar.webp',
    mimeType: 'image/webp',
    buffer: Buffer.alloc(640)
  })
  await dialog.getByRole('button', { name: /Добавить обложку|Add cover/ }).click()
  await expect(
    dialog.getByRole('button', {
      name: /Удалить свою обложку|Remove custom cover/
    })
  ).toBeVisible()
})

test('highlights preserve paragraphs and support edit, note conversion and deletion', async ({
  page
}) => {
  await page.goto('/highlights')
  const card = page.locator('article').filter({ hasText: 'let the world remain unfamiliar' })
  await card.getByRole('button', { name: /Изменить|Edit/ }).click()

  const editDialog = page.getByRole('dialog', { name: /Изменить выделение|Edit highlight/ })
  const textareas = editDialog.locator('textarea')
  await textareas.nth(0).fill('Первый абзац.\n\nВторой абзац.')
  await textareas.nth(1).fill('Комментарий к фрагменту')
  await editDialog.locator('select').selectOption('rose')
  await editDialog.getByRole('button', { name: /Сохранить|Save/ }).click()
  await expect(editDialog).toBeHidden()

  const editedCard = page.locator('article').filter({ hasText: 'Первый абзац.' })
  const quote = editedCard.locator('blockquote')
  await expect(quote).toContainText('Второй абзац.')
  await expect
    .poll(() => quote.evaluate((element) => getComputedStyle(element).whiteSpace))
    .toBe('pre-wrap')
  await expect(editedCard).toContainText('Комментарий к фрагменту')

  await editedCard.getByRole('button', { name: /В заметку|Add to note/ }).click()
  await expect(page).toHaveURL(/\/notes\?note=/)
  await expect(page.locator('textarea').first()).toHaveValue('Первый абзац.\n\nВторой абзац.')

  await page.goBack()
  await expect(page).toHaveURL(/\/highlights$/)
  const persistedCard = page.locator('article').filter({ hasText: 'Первый абзац.' })
  await persistedCard.getByRole('button', { name: /Удалить|Delete/ }).click()
  const confirmation = page.getByRole('dialog', { name: /Удалить выделение|Delete highlight/ })
  await confirmation.getByRole('button', { name: /Удалить|Delete/ }).click()
  await expect(page.getByText('Первый абзац.', { exact: false })).toHaveCount(0)
})
