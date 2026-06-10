import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// Fase 5A no navegador: um CONJUGADO prescrito aparece com o selo "conjugado" e a sequência de
// componentes ("N séries de: A ×r + B ×r + ..."), em vez da linha simples sets×reps.
test('conjugado renderiza a sequência de componentes', async ({ page }) => {
  await gotoQuestionnaire(page)

  // Avançado / 4 dias / força / 8 semanas, sem equipamento (conjugados ficam viáveis).
  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()
  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()

  // Procura a primeira semana que contém um conjugado (a geração é determinística; haverá pelo menos
  // uma). Abre, checa o selo + a sequência, e volta.
  let found = false
  for (let w = 1; w <= 8 && !found; w++) {
    await page.getByRole('button', { name: new RegExp(`Semana ${w}\\b`) }).click()
    await expect(page.getByRole('heading', { name: `Semana ${w}` })).toBeVisible()

    if ((await page.getByText('conjugado').count()) > 0) {
      found = true
      // A linha do conjugado mostra a sequência encadeada com as reps de cada componente.
      const row = page.locator('li', { has: page.getByText('conjugado') }).first()
      await expect(row).toContainText(/séries? de:/)
      await expect(row).toContainText(/×\d/) // reps de cada componente (ex.: "Clean Pull ×1")
    }

    await page.getByRole('button', { name: 'Voltar ao bloco' }).click()
    await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()
  }
  expect(found).toBeTruthy()
})
