export default defineNuxtConfig({
  css: ['~/assets/main.css'],
  devtools: { enabled: false },
  compatibilityDate: '2025-01-01',
  app: { head: { title: 'JKRouter Dashboard', meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }] } },
  // Development: proxy /api/* → Go backend (default :20127)
  nitro: {
    compatibility: { minify: false },
  },
  vite: {
    server: {
      proxy: process.env.VITE_API_URL
        ? {} // no proxy when running against remote
        : { '/api': { target: 'http://localhost:20127', changeOrigin: true } }
    }
  }
})
