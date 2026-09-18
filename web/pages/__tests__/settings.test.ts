import { describe, it, expect, vi, beforeEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import Settings from '../settings.vue'
import { setLocale } from '~/composables/useI18n'

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

describe('Settings — smoke', () => {
  it('merender halaman dengan judul route', async () => {
    const fetchFn = makeFetcher([
      okJson({ settings: { port: 20128, bind: '127.0.0.1', dataDir: '~/.jkrouter' } }),
      okJson([]),
    ])
    setFetch(fetchFn)
    const w = shallowMount(Settings)
    await vi.waitFor(
      () => expect((w.vm as any).settings.port).toBeTruthy(),
      { timeout: 2000 }
    )
    expect(w.text()).toContain('/dashboard/settings')
  })

  it('memuat setting dari API dan merge ke state', async () => {
    const fetchFn = makeFetcher([
      okJson({ settings: { port: 19999, bind: '0.0.0.0' } }),
      okJson([]),
    ])
    setFetch(fetchFn)
    const w = shallowMount(Settings)
    await vi.waitFor(
      () => expect((w.vm as any).settings.port).toBe(19999),
      { timeout: 2000 }
    )
    expect((w.vm as any).settings.bind).toBe('0.0.0.0')
  })
})
