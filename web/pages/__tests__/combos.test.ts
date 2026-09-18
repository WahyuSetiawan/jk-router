import { describe, it, expect, vi, beforeEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import Combos from '../combos.vue'
import { setLocale } from '~/composables/useI18n'

const MOCK_COMBOS = { combos: [
  { id: 1, name: 'gpt-default', model_ids: 'gpt-4o,gpt-3.5-turbo', strategy: 'fallback' },
]}
const MOCK_PROVIDERS = { providers: [{ id: 'openai', name: 'OpenAI', caps: ['chat'] }] }

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

async function dataReady(w: any) {
  await vi.waitFor(
    () => {
      const c = (w.vm as any).combos
      expect(Array.isArray(c) && c.length > 0).toBe(true)
    },
    { timeout: 2000, interval: 50 }
  )
}

describe('Combos — smoke', () => {
  it('merender halaman dengan judul route', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_COMBOS), okJson(MOCK_PROVIDERS)])
    setFetch(fetchFn)
    const w = shallowMount(Combos)
    await dataReady(w)
    expect(w.text()).toContain('/dashboard/combos')
  })

  it('menampilkan daftar combo dari API', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_COMBOS), okJson(MOCK_PROVIDERS)])
    setFetch(fetchFn)
    const w = shallowMount(Combos)
    await dataReady(w)
    expect(w.text()).toContain('gpt-default')
    expect(w.text()).toContain('fallback')
  })

  it('menampilkan pesan kosong saat tidak ada combo', async () => {
    const fetchFn = makeFetcher([okJson({ combos: [] }), okJson(MOCK_PROVIDERS)])
    setFetch(fetchFn)
    const w = shallowMount(Combos)
    await vi.waitFor(
      () => expect((w.vm as any).combos).toEqual([]),
      { timeout: 2000 }
    )
    expect(w.text()).toContain('Belum ada combo')
  })
})
