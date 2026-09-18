import { defineConfig } from 'vitest/config'
import { resolve } from 'path'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'

export default defineConfig({
  plugins: [vue(), vueJsx()],
  resolve: {
    alias: {
      '~': resolve(import.meta.dirname),
      '@': resolve(import.meta.dirname),
    },
  },
  css: { postcss: { plugins: [] } },
  test: {
    environment: 'jsdom',
    setupFiles: [],
  },
})
