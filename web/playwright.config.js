import { defineConfig, devices } from '@playwright/test'

// E2E de NAVEGADOR de verdade: o Playwright sobe o stack inteiro (Go + Vite) e dirige o Chromium
// pela mesma UI que o usuário usa. Prova back -> front -> db de ponta a ponta, complementando os
// testes de integração HTTP em Go (que validam a borda da API sem renderização).
export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  // Backend único e STATEFUL (um db efêmero compartilhado): cada teste gera/arquiva o bloco ativo.
  // Um worker serializa os testes e evita que a geração de um corrompa o estado do outro.
  workers: 1,
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  // Dois servidores: backend Go (db efêmero) e o dev server do Vite (que faz proxy de /api -> :8080).
  webServer: [
    {
      command: 'bash e2e/start-backend.sh',
      url: 'http://localhost:8080/api/questions',
      reuseExistingServer: false,
      timeout: 60_000,
      stdout: 'pipe',
      stderr: 'pipe',
    },
    {
      command: 'npm run dev -- --port 5173 --strictPort',
      url: 'http://localhost:5173',
      reuseExistingServer: false,
      timeout: 60_000,
    },
  ],
})
