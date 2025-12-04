import { fileURLToPath, URL } from 'node:url'
import { execSync } from 'node:child_process'

import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// Try to read current git commit hash so we can show it in the UI
const getGitHash = () => {
  try {
    const output = execSync('git rev-parse --short HEAD', { stdio: ['ignore', 'pipe', 'ignore'] })
    return output.toString().trim()
  } catch {
    return ''
  }
}

const GIT_HASH = process.env.VITE_APP_GIT_HASH || getGitHash()

// https://vitejs.dev/config/
export default defineConfig({
  base: './',
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      vue: 'vue/dist/vue.esm-bundler.js',
    },
  },
  build: {
    assetsInlineLimit: 100 * 1024, // 100KB
    chunkSizeWarningLimit: 4096, // 4MB
    // __ROLLUP_MANUAL_CHUNKS__
  },
  define: {
    // expose git hash as a "virtual" env variable
    'import.meta.env.VITE_APP_GIT_HASH': JSON.stringify(GIT_HASH),
  },
})
