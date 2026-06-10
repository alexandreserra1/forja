import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // Encaminha /api/* para o servidor Go. O front usa caminhos relativos
    // (fetch('/api/questions')) e o Vite faz a ponte — sem dor de CORS no dev.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
