import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, shallowMount } from '@vue/test-utils'
import MediaProviders from '../media-providers.vue'
import { setLocale } from '~/composables/useI18n'

const CONNS = { media_connections: [
  { id: 1, provider_id: 'openai', label: 'key1', auth_type: 'api_key', active: true, priority: 0 },
  { id: 2, provider_id: 'elevenlabs', label: 'voice1', auth_type: 'api_key', active: false, priority: 10 },
]}
const PROV = { providers: [
  { id: 'openai', name: 'OpenAI', caps: ['tts', 'stt', 'image'] },
  { id: 'elevenlabs', name: 'ElevenLabs', caps: ['tts'] },
  { id: 'stability', name: 'Stability AI', caps: ['image'] },
]}
const API = '/api/dashboard/media-connections'

function makeFetcher(responses: any[]) {
  const fn = vi.fn()
  responses.forEach(r => fn.mockResolvedValueOnce(r))
  return fn
}

function setFetch(fn: ReturnType<typeof vi.fn>) {
  Object.defineProperty(globalThis, 'fetch', { value: fn, writable: true, configurable: true })
}

function okJson(data: any) {
  return { ok: true, json: () => Promise.resolve(data) }
}

beforeEach(() => {
  setLocale('id')
  vi.restoreAllMocks()
  vi.stubGlobal('confirm', vi.fn(() => true))
})

async function dataReady(w: any) {
  await vi.waitFor(
    () => {
      const conns = (w.vm as any).connections
      expect(Array.isArray(conns) && conns.length > 0).toBe(true)
    },
    { timeout: 2000, interval: 50 }
  )
}

// ─── Smoke ───────────────────────────────────────────────
describe('MediaProviders — smoke', () => {
  it('merender halaman dengan judul dan tombol tambah', async () => {
    setFetch(makeFetcher([okJson(CONNS), okJson(PROV)]))
    const w = shallowMount(MediaProviders)
    await dataReady(w)
    expect(w.text()).toContain('/dashboard/media-providers')
    expect(w.text()).toContain('+ Akun Media')
  })

  it('menampilkan tabel dengan baris koneksi', async () => {
    setFetch(makeFetcher([okJson(CONNS), okJson(PROV)]))
    const w = shallowMount(MediaProviders)
    await dataReady(w)
    expect(w.text()).toContain('openai')
    expect(w.text()).toContain('aktif')
    expect(w.text()).toContain('nonaktif')
  })

  it('menampilkan pesan kosong saat tidak ada koneksi', async () => {
    setFetch(makeFetcher([
      okJson({ media_connections: [] }),
      okJson(PROV),
    ]))
    const w = shallowMount(MediaProviders)
    await vi.waitFor(
      () => expect((w.vm as any).connections).toEqual([]),
      { timeout: 2000 }
    )
    expect(w.text()).toContain('Belum ada akun media')
  })
})

// ─── Search filter ───────────────────────────────────────
describe('MediaProviders — search', () => {
  it('filter tabel berdasarkan label', async () => {
    setFetch(makeFetcher([okJson(CONNS), okJson(PROV)]))
    const w = shallowMount(MediaProviders)
    await dataReady(w)
    w.find('input.search').setValue('key1')
    await vi.waitFor(
      () => {
        const filtered = (w.vm as any).filtered?.()
        expect(filtered).toHaveLength(1)
        expect(filtered![0].label).toBe('key1')
      },
      { timeout: 2000 }
    )
    expect(w.findAll('tr').at(1)?.text()).toContain('key1')
    expect(w.findAll('tr').at(1)?.text()).not.toContain('voice1')
  })

  it('filter tabel berdasarkan provider_id', async () => {
    setFetch(makeFetcher([okJson(CONNS), okJson(PROV)]))
    const w = shallowMount(MediaProviders)
    await dataReady(w)
    w.find('input.search').setValue('elevenlabs')
    await vi.waitFor(
      () => {
        const filtered = (w.vm as any).filtered?.()
        expect(filtered).toHaveLength(1)
        expect(filtered![0].provider_id).toBe('elevenlabs')
      },
      { timeout: 2000 }
    )
    const rowText = w.findAll('tr').at(1)?.text() ?? ''
    expect(rowText).toContain('elevenlabs')
    expect(rowText).not.toContain('openai')
  })
})

// ─── Test connection ─────────────────────────────────────
describe('MediaProviders — test connection', () => {
  it('mengirim request test ke API saat tombol diklik', async () => {
    const fetchFn = makeFetcher([okJson(CONNS), okJson(PROV)])
    setFetch(fetchFn)
    const w = shallowMount(MediaProviders)
    await dataReady(w)

    w.findAll('button').find((b: any) => b.text().includes('+ Akun Media'))?.trigger('click')
    await w.vm.$nextTick()

    const sel = w.find('select')
    await sel.setValue('openai')
    const inp = w.findAll('input')
    await inp[1].setValue('test-label')
    await inp[2].setValue('sk-test-key')
    await w.vm.$nextTick()

    fetchFn.mockResolvedValueOnce(okJson({ ok: true, message: 'API key is valid' }))

    w.findAll('button').find((b: any) => b.text().includes('Uji Koneksi'))?.trigger('click')
    await w.vm.$nextTick()

    const testCalls = fetchFn.mock.calls.filter(
      ([url]: [string]) => String(url).includes('/test')
    )
    expect(testCalls).toHaveLength(1)
    expect(testCalls[0][1]).toMatchObject({ method: 'POST' })
  })

  it('menampilkan pesan error saat test gagal', async () => {
    vi.useFakeTimers()
    const fetchFn = makeFetcher([okJson(CONNS), okJson(PROV)])
    setFetch(fetchFn)
    const w = shallowMount(MediaProviders)
    await dataReady(w)

    w.findAll('button').find((b: any) => b.text().includes('+ Akun Media'))?.trigger('click')
    await w.vm.$nextTick()

    const sel = w.find('select')
    await sel.setValue('openai')
    const inp = w.findAll('input')
    await inp[1].setValue('test-label')
    await inp[2].setValue('sk-bad')
    await w.vm.$nextTick()

    fetchFn.mockResolvedValueOnce(okJson({ ok: false, error: 'invalid key' }))

    w.findAll('button').find((b: any) => b.text().includes('Uji Koneksi'))?.trigger('click')
    // Wait for fetch promise to resolve and testing flag to clear
    await vi.waitFor(
      () => {
        expect((w.vm as any).testing).toBe(false)
        expect((w.vm as any).testResult).toBeTruthy()
      },
      { timeout: 2000 }
    )

    expect(w.text()).toContain('Koneksi gagal')
    vi.useRealTimers()
  })
})

// ─── Create connection ───────────────────────────────────
describe('MediaProviders — create', () => {
  it('POST berhasil → modal tertutup, daftar di-refresh', async () => {
    const fetchFn = makeFetcher([okJson(CONNS), okJson(PROV)])
    setFetch(fetchFn)
    const w = shallowMount(MediaProviders)
    await dataReady(w)

    w.findAll('button').find((b: any) => b.text().includes('+ Akun Media'))?.trigger('click')
    await w.vm.$nextTick()

    const sel = w.find('select')
    await sel.setValue('openai')
    const inp = w.findAll('input')
    await inp[1].setValue('new-key')
    await inp[2].setValue('sk-new')
    await w.vm.$nextTick()

    fetchFn
      .mockResolvedValueOnce(okJson({ id: 99 }))
      .mockResolvedValueOnce(okJson({ media_connections: [{ id: 99, provider_id: 'openai', label: 'new-key', auth_type: 'api_key', active: true, priority: 0 }] }))
      .mockResolvedValueOnce(okJson(PROV))

    w.findAll('button').find((b: any) => b.text().includes('Simpan'))?.trigger('click')
    await vi.waitFor(
      () => expect(w.text()).toContain('Berhasil'),
      { timeout: 3000 }
    )
  })

  it('form kosong → pesan error, tidak POST create', async () => {
    const fetchFn = makeFetcher([okJson(CONNS), okJson(PROV)])
    setFetch(fetchFn)
    const w = shallowMount(MediaProviders)
    await dataReady(w)

    w.findAll('button').find((b: any) => b.text().includes('+ Akun Media'))?.trigger('click')
    await w.vm.$nextTick()

    w.findAll('button').find((b: any) => b.text().includes('Simpan'))?.trigger('click')
    await w.vm.$nextTick()

    const createCalls = fetchFn.mock.calls.filter(
      ([url, init]: [string, any]) =>
        String(url) === API && init?.method === 'POST'
    )
    expect(createCalls).toHaveLength(0)
  })
})

// ─── Toggle ──────────────────────────────────────────────
describe('MediaProviders — toggle', () => {
  it('klik status aktif mengirim PATCH ke API', async () => {
    const fetchFn = makeFetcher([okJson(CONNS), okJson(PROV)])
    setFetch(fetchFn)
    const w = shallowMount(MediaProviders)
    await dataReady(w)

    w.findAll('[class*="st"]').find((el: any) => el.text() === 'aktif')?.trigger('click')
    await w.vm.$nextTick()

    const toggleCalls = fetchFn.mock.calls.filter(
      ([url, init]: [string, any]) =>
        String(url) === `${API}/1/toggle` && init?.method === 'PATCH'
    )
    expect(toggleCalls).toHaveLength(1)
  })
})

// ─── Delete ──────────────────────────────────────────────
describe('MediaProviders — delete', () => {
  it('klik tombol hapus mengirim DELETE ke API', async () => {
    const fetchFn = makeFetcher([okJson(CONNS), okJson(PROV)])
    setFetch(fetchFn)
    const w = shallowMount(MediaProviders)
    await dataReady(w)

    w.findAll('button').find((b: any) => b.text() === '🗑')?.trigger('click')
    await w.vm.$nextTick()

    const delCalls = fetchFn.mock.calls.filter(
      ([url, init]: [string, any]) =>
        String(url) === `${API}/1` && init?.method === 'DELETE'
    )
    expect(delCalls).toHaveLength(1)
  })
})
