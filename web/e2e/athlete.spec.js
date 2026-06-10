import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// Fase 6A no navegador: dois atletas com o MESMO perfil recebem programas DIFERENTES (semente
// determinística por identidade). Cria o atleta 2 pelo seletor e compara a semana 1 dos dois.

async function fillProfileAndGenerate(page) {
  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()
  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()
}

async function week1Exercises(page) {
  await page.getByRole('button', { name: /Semana 1\b/ }).click()
  await expect(page.getByRole('heading', { name: 'Semana 1' })).toBeVisible()
  const names = await page.locator('.min-w-40').allInnerTexts()
  await page.getByRole('button', { name: 'Voltar ao bloco' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()
  return names.join('|')
}

test('atletas diferentes recebem programas diferentes', async ({ page }) => {
  await gotoQuestionnaire(page)

  // Atleta 1 (default): gera e captura a semana 1.
  await fillProfileAndGenerate(page)
  const a1 = await week1Exercises(page)
  expect(a1.length).toBeGreaterThan(0)

  // Cria o atleta 2 pelo seletor; ele começa no questionário (sem bloco).
  await page.getByPlaceholder('novo atleta').fill('Atleta 2')
  await page.getByRole('button', { name: 'Criar' }).click()
  await expect(page.locator('input[name="q-1"]').first()).toBeVisible()

  // Mesmo perfil -> gera e captura a semana 1 do atleta 2.
  await fillProfileAndGenerate(page)
  const a2 = await week1Exercises(page)

  // Os programas diferem (individualização determinística, sem RNG).
  expect(a2).not.toEqual(a1)

  // E voltar ao atleta 1 mostra o bloco DELE (estado escopado e persistido).
  await page.locator('select').selectOption('1')
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()
})
