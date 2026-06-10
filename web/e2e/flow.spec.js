import { test, expect } from '@playwright/test'
import { gotoQuestionnaire } from './helpers'

// Jornada completa no navegador: questionário -> gera bloco -> abre semana -> vê a variação por
// fase -> marca tudo feito e confirma que a UI reflete o realizado (releitura assíncrona). É o mesmo
// caminho do usuário; valida back -> front -> db de verdade, com cliques e renderização React.
test('questionário -> bloco -> marcar feito -> variação por fase', async ({ page }) => {
  await gotoQuestionnaire(page)

  // --- Questionário: avançado / 4 dias / força / 8 semanas (equipamento em branco = assume tudo) ---
  // Os inputs são rádios com value = valor interno da opção (mesmos valores dos testes de integração).
  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()
  await page.getByRole('button', { name: 'Gerar treino' }).click()

  // --- Visão do bloco: 8 semanas, a última é deload ---
  await expect(page.getByTestId('dashboard')).toBeVisible()
  await expect(page.getByRole('button', { name: /^Semana \d/ })).toHaveCount(8)
  await expect(page.getByRole('button', { name: /Semana 8.*Deload/ })).toBeVisible()

  // --- Semana 1 (Acumulação): ênfase em técnica, 4 dias de treino ---
  await page.getByRole('button', { name: /Semana 1\b/ }).click()
  await expect(page.getByRole('heading', { name: 'Semana 1' })).toBeVisible()
  await expect(page.getByText('Acumulação')).toBeVisible()
  await expect(page.getByText('ênfase em técnica e base')).toBeVisible()
  await expect(page.getByRole('heading', { name: /^Dia \d/ })).toHaveCount(4)

  // --- Marcar feito: registra TODA a semana, uma prescrição por vez. Cada clique dispara o reload
  //     assíncrono da WeekView; o botão "Marcar feito" some e vira o selo "✓ Feito". ---
  // Conta só linhas de prescrição (li) — WodCards têm o mesmo botão mas ficam em div.
  const rows = page.locator('li', {
    has: page.getByRole('button', { name: 'Marcar feito' }),
  })
  const total = await rows.count()
  expect(total).toBeGreaterThan(0)
  for (let remaining = total; remaining > 0; remaining--) {
    const row = rows.first()
    await row.locator('input[type="number"]').fill('8')
    await row.getByRole('button', { name: 'Marcar feito' }).click()
    // Espera a releitura: a contagem de linhas (li) pendentes cai em 1 (prova ida-e-volta com o banco).
    await expect(rows).toHaveCount(remaining - 1)
  }
  // Toda a semana registrada: vários selos "Feito (RPE 8)" e nenhum botão pendente.
  await expect(page.getByText(/✓ Feito/).first()).toBeVisible()
  await expect(page.getByText('RPE 8').first()).toBeVisible()

  // --- Variação por fase visível na UI: a semana 4 (Intensificação) puxa ênfase em FORÇA ---
  await page.getByRole('button', { name: 'Voltar ao bloco' }).click()
  await page.getByRole('button', { name: /Semana 4\b/ }).click()
  await expect(page.getByRole('heading', { name: 'Semana 4' })).toBeVisible()
  await expect(page.getByText('Intensificação')).toBeVisible()
  await expect(page.getByText('ênfase em força')).toBeVisible()
})
