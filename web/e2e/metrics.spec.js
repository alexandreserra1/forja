import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// Fase 6D: o atleta preenche dados opcionais (idade + esporte) no questionário e gera o bloco.
// Valida que o formulário existe, aceita os dados e o bloco é gerado com sucesso.
test('preenche métricas opcionais e gera o bloco', async ({ page }) => {
  await gotoQuestionnaire(page)

  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()

  // Preenche idade.
  await page.locator('#m-age').fill('35')
  await expect(page.locator('#m-age')).toHaveValue('35')

  // Seleciona esporte.
  await page.locator('#m-sport').selectOption('weightlifting')
  await expect(page.locator('#m-sport')).toHaveValue('weightlifting')

  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByTestId('dashboard')).toBeVisible()
})
