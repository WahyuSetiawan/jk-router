export default defineNuxtConfig({
  modules: ['@nuxtjs/tailwindcss'],
  css: [],
  devtools: { enabled: false },
  compatibilityDate: '2025-01-01',
  app: { head: { title: 'JKRouter Dashboard', meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }] } }
})
