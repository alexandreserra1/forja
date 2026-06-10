import { test, expect } from '@playwright/test'

// auth.spec.js testa o fluxo de AuthForm (register → bloco → logout → login).
// O AuthForm só aparece quando o servidor roda com AUTH_SECRET definida (prod mode).
// Em dev mode (AUTH_SECRET vazia) o teste é pulado automaticamente.
// Para rodar: AUTH_SECRET=cfit-e2e-test npx playwright test e2e/auth.spec.js

test.beforeEach(async ({ page }) => {
  const res = await page.request.get('http://localhost:8080/api/auth/status')
  const { required } = await res.json()
  if (!required) test.skip()
})

test('register → gerar bloco → logout → login', async ({ page }) => {
  await page.goto('/')

  // AuthForm inicia no modo login; troca para cadastro.
  await expect(page.getByRole('heading', { name: 'Entrar' })).toBeVisible()
  await page.getByRole('button', { name: 'Cadastre-se' }).click()
  await expect(page.getByRole('heading', { name: 'Criar conta' })).toBeVisible()

  // Preenche cadastro.
  await page.getByLabel('Nome').fill('Atleta Auth E2E')
  await page.getByLabel('E-mail').fill('auth-e2e@cfit.io')
  await page.getByLabel('Senha').fill('senha-e2e-123')
  await page.getByRole('button', { name: 'Criar conta' }).click()

  // App carrega → questionário (atleta novo, sem bloco).
  await expect(page.locator('input[name="q-1"]').first()).toBeVisible({ timeout: 10000 })

  // Gera o bloco.
  await page.locator('input[name="q-1"][value="gt_3y"]').check()
  await page.locator('input[name="q-2"][value="4"]').check()
  await page.locator('input[name="q-3"][value="strength"]').check()
  await page.locator('input[name="q-4"][value="8"]').check()
  await page.getByRole('button', { name: 'Gerar treino' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible()

  // Logout → volta para o AuthForm (modo login).
  await page.getByRole('button', { name: 'Sair' }).click()
  await expect(page.getByRole('heading', { name: 'Entrar' })).toBeVisible()

  // Login com as credenciais cadastradas → bloco já existe.
  await page.getByLabel('E-mail').fill('auth-e2e@cfit.io')
  await page.getByLabel('Senha').fill('senha-e2e-123')
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page.getByRole('heading', { name: 'Bloco de 8 semanas' })).toBeVisible({ timeout: 10000 })
})

test('senha errada mostra erro', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Entrar' })).toBeVisible()

  await page.getByLabel('E-mail').fill('nao-existe@cfit.io')
  await page.getByLabel('Senha').fill('qualquer')
  await page.getByRole('button', { name: 'Entrar' }).click()

  await expect(page.getByText('e-mail ou senha inválidos')).toBeVisible()
})

test('email duplicado mostra erro', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Cadastre-se' }).click()

  // Primeiro cadastro.
  await page.getByLabel('Nome').fill('Primeiro')
  await page.getByLabel('E-mail').fill('dup-e2e@cfit.io')
  await page.getByLabel('Senha').fill('senha123')
  await page.getByRole('button', { name: 'Criar conta' }).click()
  await expect(page.locator('input[name="q-1"]').first()).toBeVisible({ timeout: 10000 })

  // Logout → tenta registrar o mesmo e-mail.
  await page.getByRole('button', { name: 'Sair' }).click()
  await page.getByRole('button', { name: 'Cadastre-se' }).click()
  await page.getByLabel('Nome').fill('Segundo')
  await page.getByLabel('E-mail').fill('dup-e2e@cfit.io')
  await page.getByLabel('Senha').fill('outra')
  await page.getByRole('button', { name: 'Criar conta' }).click()

  await expect(page.getByText('e-mail já cadastrado')).toBeVisible()
})
