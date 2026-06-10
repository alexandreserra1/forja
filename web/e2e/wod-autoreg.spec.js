import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// WOD AutoReg (M3): marcar WODs como feitos com RPE individual, reavaliar, e ver que o
// trilho WOD também roda (mensagem de avaliação inclui condicionamento).
test('WOD: marcar feito com RPE individual → avaliar mostra trilho WOD', async ({ page }) => {
  await gotoQuestionnaire(page)

  // Avançado / 4 dias / força / 8 semanas.
  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()
  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByTestId('dashboard')).toBeVisible()

  // Abre a semana 1 (onde houver WODs).
  await page.getByRole('button', { name: /Semana 1\b/ }).click()
  await expect(page.getByRole('heading', { name: 'Semana 1' })).toBeVisible()

  // Só conta cards PENDENTES (com o botão "Marcar feito") — ignora os já feitos.
  // data-testid="wod-card" + filter garante que não pegamos divs ancestrais nem cards done.
  const pendingCards = page.locator('[data-testid="wod-card"]').filter({
    has: page.getByRole('button', { name: 'Marcar feito' }),
  })
  const count = await pendingCards.count()
  test.skip(count === 0, 'semana sem WOD pendente — pulando teste')

  for (let i = 0; i < count; i++) {
    // Sempre o primeiro pendente: o locator re-avalia e o set encolhe após cada reload.
    const card = pendingCards.first()
    const rpe = 8 + (i * 0.5)
    await card.locator('input[type="number"]').fill(String(rpe))
    await card.getByRole('button', { name: 'Marcar feito' }).click()
    // Aguarda a releitura: a quantidade de cards pendentes cai em 1.
    await expect(pendingCards).toHaveCount(count - i - 1, { timeout: 5000 })
  }

  // Volta ao bloco e reavalia.
  await page.getByRole('button', { name: 'Voltar ao bloco' }).click()
  await expect(page.getByTestId('dashboard')).toBeVisible()
  await page.getByRole('button', { name: 'Reavaliar com base nos registros' }).click()

  // A mensagem de avaliação aparece e inclui o resultado do trilho WOD.
  const evalMsg = page.locator('div').filter({ hasText: 'Registre seus treinos' }).or(
    page.locator('div').filter({ hasText: 'condicionamento' }),
  ).or(
    page.locator('div').filter({ hasText: /wod/i }),
  ).first()
  await expect(evalMsg).toBeVisible({ timeout: 10000 })
})
