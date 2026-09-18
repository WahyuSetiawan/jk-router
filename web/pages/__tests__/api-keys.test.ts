import { describe, it, expect, vi, beforeEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import ApiKeys from '../api-keys.vue'
import { setLocale } from '~/composables/useI18n'

const MOCK_KEYS = { keys: [
  { id: 1, label: 'prod-key', key: 'sk-abc123', created_at: '2026-09-01' },
]}

function okJson(data: any) {
  return { ok: true, json: () => Promise.resolve(data) }
}

function makeFetcher(responses: any[]) {
  const fn = vi.fn()
  responses.forEach(r => fn.mockResolvedValueOnce(r))
  return fn
}

function setFetch(fn: ReturnType<typeof vi.fn>) {
  Object.defineProperty(globalThis, 'fetch', { value: fn, writable: true, configurable: true })
}

beforeEach(() => {
  setLocale('id')
  vi.restoreAllMocks()
})

describe('ApiKeys — smoke', () => {
  it('merender halaman dengan judul route', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_KEYS)])
    setFetch(fetchFn)
    const w = shallowMount(ApiKeys)
    await vi.waitFor(
      () => {
        const k = (w.vm as any).keys
        expect(Array.isArray(k) && k.length > 0).toBe(true)
      },
      { timeout: 2000 }
    )
    expect(w.text()).toContain('/dashboard/keys')
  })

  it('menampilkan daftar key dari API', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_KEYS)])
    setFetch(fetchFn)
    const w = shallowMount(ApiKeys)
    await vi.waitFor(
      () => expect((w.vm as any).keys).toHaveLength(1),
      { timeout: 2000 }
    )
    expect(w.text()).toContain('prod-key')
  })

  it('menampilkan pesan kosong saat tidak ada key', async () => {
    const fetchFn = makeFetcher([okJson({ keys: [] })])
    setFetch(fetchFn)
    const w = shallowMount(ApiKeys)
    await vi.waitFor(
      () => expect((w.vm as any).keys).toEqual([]),
      { timeout: 2000 }
    )
    expect(w.text()).toContain('No API keys')
  })
})
