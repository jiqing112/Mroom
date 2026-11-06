import { writeFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// emptyOutDir wipes the whole dist tree; go:embed needs the directory to exist
// even on a fresh checkout, so put the committed placeholder back after builds.
const keepGitkeep = {
  closeBundle() {
    writeFileSync(new URL('./dist/.gitkeep', import.meta.url), '')
  }
}

export default defineConfig({
  plugins: [svelte(), tailwindcss(), keepGitkeep],
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
