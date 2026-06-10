import { expect } from '@playwright/test'

const E2E_EMAIL = 'e2e@cfit.io'
const E2E_PASS  = 'e2e-senha123'
const E2E_NAME  = 'Atleta E2E'

// sessionToken: cache do token dentro da mesma execução Playwright (evita múltiplos register/login
// que esgotariam o rate limiter). O cache é válido apenas quando auth é requerida (prod mode).
let sessionToken = null

// ensureAuth só age quando o servidor exige JWT (AUTH_SECRET definida).
// Em dev mode (AUTH_SECRET vazia) a UI já carrega sem token — não é necessário fazer nada.
async function ensureAuth(page) {
  const statusRes = await page.request.get('http://localhost:8080/api/auth/status')
  const { required } = await statusRes.json()
  if (!required) return // dev mode: sem portão de auth na UI

  // Verifica se o browser desta página já tem token.
  const existing = await page.evaluate(() => localStorage.getItem('cfit_token'))
  if (existing) return

  // Reutiliza token da sessão Playwright para não bater no rate limiter.
  if (sessionToken) {
    await page.evaluate((t) => localStorage.setItem('cfit_token', t), sessionToken)
    return
  }

  // Primeira autenticação: tenta registrar; se e-mail já existe, faz login.
  let body
  const regRes = await page.request.post('http://localhost:8080/api/auth/register', {
    data: { name: E2E_NAME, email: E2E_EMAIL, password: E2E_PASS },
  })
  if (regRes.status() === 201) {
    body = await regRes.json()
  } else {
    const loginRes = await page.request.post('http://localhost:8080/api/auth/login', {
      data: { email: E2E_EMAIL, password: E2E_PASS },
    })
    body = await loginRes.json()
  }

  sessionToken = body.token
  await page.evaluate((t) => localStorage.setItem('cfit_token', t), sessionToken)
}

// gotoQuestionnaire garante que a página comece no QUESTIONÁRIO, mesmo que um teste anterior
// tenha deixado um bloco ativo no backend compartilhado (o db efêmero persiste durante toda a
// rodada). Se a visão do bloco aparecer, "Refazer questionário" volta ao formulário.
export async function gotoQuestionnaire(page) {
  await page.goto('/')
  await ensureAuth(page)
  if (sessionToken) {
    // Em prod mode recarrega para que App.jsx leia authed=true do localStorage.
    await page.reload()
  }

  const refazer = page.getByRole('button', { name: 'Refazer questionário' })
  // Espera a UI assentar após o fetch inicial: ou o formulário, ou a visão do bloco.
  await expect(page.locator('input[name="q-1"]').first().or(refazer)).toBeVisible()
  if ((await refazer.count()) > 0) {
    await refazer.click()
  }
  await expect(page.locator('input[name="q-1"]').first()).toBeVisible()
}
