import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:4533',
      '/rest': 'http://127.0.0.1:4533'
    }
  },
  build: {
    outDir: 'dist',
    target: 'es2020',
    cssCodeSplit: false
  }
})
