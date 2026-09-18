import { describe, it, expect, vi, beforeEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import Providers from '../providers.vue'
import { setLocale } from '~/composables/useI18n'

const MOCK_PROVIDERS = { providers: [
  { id: 'openai', name: 'OpenAI', caps: ['chat', 'tts'] },
  { id: 'anthropic', name: 'Anthropic', caps: ['chat'] },
]}
const MOCK_POOLS = { proxy_pools: [] }
const MOCK_CONNS = { connections: [] }

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
      const p = (w.vm as any).providers
      expect(Array.isArray(p) && p.length > 0).toBe(true)
    },
    { timeout: 2000, interval: 50 }
  )
}

describe('Providers — smoke', () => {
  it('merender halaman dengan judul route', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_PROVIDERS), okJson(MOCK_POOLS), okJson(MOCK_CONNS)])
    setFetch(fetchFn)
    const w = shallowMount(Providers)
    await dataReady(w)
    expect(w.text()).toContain('/dashboard/providers')
  })

  it('menampilkan daftar provider dari API', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_PROVIDERS), okJson(MOCK_POOLS), okJson(MOCK_CONNS)])
    setFetch(fetchFn)
    const w = shallowMount(Providers)
    await dataReady(w)
    expect(w.text()).toContain('OpenAI')
    expect(w.text()).toContain('Anthropic')
  })

  it('menampilkan pesan kosong saat tidak ada provider', async () => {
    const fetchFn = makeFetcher([okJson({ providers: [] }), okJson(MOCK_POOLS), okJson(MOCK_CONNS)])
    setFetch(fetchFn)
    const w = shallowMount(Providers)
    await vi.waitFor(
      () => expect((w.vm as any).providers).toEqual([]),
      { timeout: 2000 }
    )
    expect(w.text()).toContain('Belum ada provider')
  })
})
