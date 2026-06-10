import { test, expect } from '@playwright/test'

// Testes do Timer (Fase C). O timer é frontend-only: zero backend necessário.
// Os testes abrem a app, clicam no botão ⏱ do header, e exercitam os modos.

test.describe('Timer — standalone (header)', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
    // Aguarda a app carregar (auth/status retorna).
    await page.waitForLoadState('networkidle')
    // Se AuthForm aparecer (prod mode com AUTH_SECRET), pula os testes.
    const authForm = page.getByRole('heading', { name: /entrar|cadastrar/i })
    if ((await authForm.count()) > 0) {
      test.skip()
    }
  })

  test('abre TimerControls pelo botão do header', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    // TimerControls deve estar visível com os 4 botões de modo.
    await expect(page.getByRole('button', { name: 'AMRAP' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'For Time' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'EMOM' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Tabata' })).toBeVisible()
  })

  test('fecha TimerControls com o botão ✕', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    await expect(page.getByRole('button', { name: 'AMRAP' })).toBeVisible()
    await page.getByRole('button', { name: 'Fechar timer' }).click()
    await expect(page.getByRole('button', { name: 'AMRAP' })).not.toBeVisible()
  })

  test('AMRAP: inicia timer e mostra display de contagem regressiva', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    await page.getByRole('button', { name: 'AMRAP' }).click()

    // Ajusta para 1 minuto (menor possível para não esperar muito).
    const minInput = page.locator('input[type="number"]').first()
    await minInput.fill('1')

    await page.getByRole('button', { name: 'Iniciar' }).click()

    // TimerView deve aparecer com o display.
    const display = page.getByTestId('timer-display')
    await expect(display).toBeVisible()

    // Clica em Iniciar dentro do TimerView para iniciar o timer.
    await page.getByTestId('timer-startstop').click()

    // Aguarda 1s e verifica que o tempo está sendo exibido (formato MM:SS).
    await page.waitForTimeout(1000)
    const text = await display.textContent()
    expect(text).toMatch(/^\d{2}:\d{2}$/)
  })

  test('For Time: inicia e mostra contagem crescente', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    await page.getByRole('button', { name: 'For Time' }).click()
    await page.getByRole('button', { name: 'Iniciar' }).click()

    const display = page.getByTestId('timer-display')
    await expect(display).toBeVisible()
    // Começa em 00:00 antes de iniciar.
    await expect(display).toHaveText('00:00')

    await page.getByTestId('timer-startstop').click()
    await page.waitForTimeout(1500)
    const text = await display.textContent()
    // Após 1.5s deve estar em 00:01 ou 00:02.
    expect(text).toMatch(/^00:0[12]$/)
  })

  test('EMOM: mostra label de round', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    await page.getByRole('button', { name: 'EMOM' }).click()

    // Verifica que os dois inputs de EMOM aparecem dentro do TimerControls.
    const controls = page.getByTestId('timer-controls')
    const inputs = controls.locator('input[type="number"]')
    await expect(inputs).toHaveCount(2)

    await page.getByRole('button', { name: 'Iniciar' }).click()
    const display = page.getByTestId('timer-display')
    await expect(display).toBeVisible()

    // Label deve indicar round 1 / N.
    await expect(page.getByText(/Round 1 \//)).toBeVisible()
  })

  test('Pausar e Continuar funcionam', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    await page.getByRole('button', { name: 'AMRAP' }).click()
    await page.getByRole('button', { name: 'Iniciar' }).click()
    await page.getByTestId('timer-startstop').click() // Iniciar

    await page.waitForTimeout(600)
    await page.getByTestId('timer-startstop').click() // Pausar
    await expect(page.getByTestId('timer-startstop')).toHaveText('Continuar')

    // O display deve estar parado (mesmo valor após 500ms).
    const t1 = await page.getByTestId('timer-display').textContent()
    await page.waitForTimeout(500)
    const t2 = await page.getByTestId('timer-display').textContent()
    expect(t1).toBe(t2)

    await page.getByTestId('timer-startstop').click() // Continuar
    await expect(page.getByTestId('timer-startstop')).toHaveText('Pausar')
  })

  test('Reset volta para 00:00', async ({ page }) => {
    await page.getByRole('button', { name: 'Abrir timer' }).click()
    await page.getByRole('button', { name: 'For Time' }).click()
    await page.getByRole('button', { name: 'Iniciar' }).click()

    await page.getByTestId('timer-startstop').click()
    await page.waitForTimeout(1000)
    await page.getByRole('button', { name: 'Reset' }).click()

    await expect(page.getByTestId('timer-display')).toHaveText('00:00')
    await expect(page.getByTestId('timer-startstop')).toHaveText('Iniciar')
  })
})
