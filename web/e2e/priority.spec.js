import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// Fase 6B no navegador: o atleta marca um padrão a priorizar (ponto fraco) no questionário e gera o
// bloco. Valida o fluxo (o seletor existe, marca e o bloco é gerado).
test('prioriza um padrão e gera o bloco', async ({ page }) => {
  await gotoQuestionnaire(page)

  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()

  // Marca a prioridade "Puxar" (padrão pull).
  await page.getByRole('checkbox', { name: 'Puxar' }).check()
  await expect(page.getByRole('checkbox', { name: 'Puxar' })).toBeChecked()

  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()
})
