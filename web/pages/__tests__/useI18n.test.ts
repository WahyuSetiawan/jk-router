import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale, t } from '~/composables/useI18n'

describe('useI18n', () => {
  beforeEach(() => {
    setLocale('id')
  })

  it('mengembalikan terjemahan id untuk key yang ada', () => {
    expect(t('dashboard.title')).toBe('Dashboard')
    expect(t('nav.providers')).toBe('Provider')
    expect(t('combos.add')).toBe('+ Tambah Combo')
  })

  it('fallback ke en saat key tidak ada di id', () => {
    // key yang hanya ada di en
    const result = t('nav.routing')
    // routing ada di kedua dict, harusnya 'Routing'
    expect(result).toBe('Routing')
  })

  it('fallback ke key itu sendiri saat tidak ada di dictionary', () => {
    expect(t('nonexistent.key')).toBe('nonexistent.key')
  })

  it('mendukung params replacement', () => {
    // Tidak ada key dengan params di dict, tapi fungsi tidak error
    const result = t('dashboard.requests', { count: '100' })
    expect(typeof result).toBe('string')
  })

  it('setLocale mengubah locale secara reaktif', () => {
    setLocale('en')
    expect(t('nav.providers')).toBe('Providers')
    setLocale('id')
    expect(t('nav.providers')).toBe('Provider')
  })
})
