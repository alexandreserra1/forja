import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// Fase 5B no navegador: o WOD montado pelo compositor aparece na semana, ao lado da força, com a
// linguagem honesta ("enfatiza o sistema X — não treina exclusivamente") e seus movimentos.
test('WOD montado aparece na semana com ênfase de sistema', async ({ page }) => {
  await gotoQuestionnaire(page)

  // Avançado / 4 dias / força / 8 semanas (o seed tem dose de condicionamento p/ strength).
  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()
  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()

  // Procura a primeira semana com WOD (a geração é determinística; haverá).
  let found = false
  for (let w = 1; w <= 8 && !found; w++) {
    await page.getByRole('button', { name: new RegExp(`Semana ${w}\\b`) }).click()
    await expect(page.getByRole('heading', { name: `Semana ${w}` })).toBeVisible()

    if ((await page.getByText('WOD', { exact: true }).count()) > 0) {
      found = true
      const card = page.locator('div').filter({ hasText: 'Enfatiza o sistema' }).first()
      await expect(card).toContainText(/Enfatiza o sistema/)
      await expect(card).toContainText(/não treina/) // promessa modesta
    }

    await page.getByRole('button', { name: 'Voltar ao bloco' }).click()
    await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()
  }
  expect(found).toBeTruthy()
})
