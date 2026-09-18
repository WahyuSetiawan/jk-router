import { describe, it, expect, vi, beforeEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import Logs from '../logs.vue'
import { setLocale } from '~/composables/useI18n'

const MOCK_TAIL = { tail: [
  { ts: '2026-09-18T10:00:00Z', combo: 'gpt-4o', provider: 'openai', status: 'ok', latency_ms: 120 },
  { ts: '2026-09-18T09:59:00Z', combo: 'claude-3', provider: 'anthropic', status: 'err', latency_ms: 50 },
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

describe('Logs — smoke', () => {
  it('merender halaman dengan judul route', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_TAIL)])
    setFetch(fetchFn)
    const w = shallowMount(Logs)
    await vi.waitFor(
      () => {
        const l = (w.vm as any).logs
        expect(Array.isArray(l) && l.length > 0).toBe(true)
      },
      { timeout: 2000 }
    )
    expect(w.text()).toContain('/dashboard/logs')
  })

  it('menampilkan data log dari API', async () => {
    const fetchFn = makeFetcher([okJson(MOCK_TAIL)])
    setFetch(fetchFn)
    const w = shallowMount(Logs)
    await vi.waitFor(
      () => {
        const l = (w.vm as any).logs
        expect(l).toHaveLength(2)
      },
      { timeout: 2000 }
    )
    expect(w.text()).toContain('openai')
    expect(w.text()).toContain('anthropic')
  })

  it('menampilkan pesan kosong saat log kosong', async () => {
    const fetchFn = makeFetcher([okJson({ tail: [] })])
    setFetch(fetchFn)
    const w = shallowMount(Logs)
    await vi.waitFor(
      () => expect((w.vm as any).logs).toEqual([]),
      { timeout: 2000 }
    )
  })
})
