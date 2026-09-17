<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, onMounted } from 'vue'

const connections = ref<any[]>([])
const showAdd = ref(false)
const search = ref('')
const form = ref({ provider_id: '', label: '', secret: '', auth_type: 'api_key', priority: 0 })
const msg = ref('')
const msgType = ref<'ok' | 'err'>('ok')

const mediaRegistries = ref<any[]>([])

async function loadRegistries() {
  try {
    const r = await fetch('/api/dashboard/providers').then(x => x.json())
    // Pull media-capable registries from the registry list
    const all = (r as any).providers || []
    // We don't have a dedicated media-registry endpoint yet;
    // derive available provider IDs from the registry.
    mediaRegistries.value = [
      { id: 'openai', name: 'OpenAI', caps: ['tts', 'stt', 'image'] },
      { id: 'elevenlabs', name: 'ElevenLabs', caps: ['tts'] },
      { id: 'stability', name: 'Stability AI', caps: ['image'] },
    ]
  } catch {}
}

async function load() {
  try {
    const r = await fetch('/api/dashboard/media-connections').then(x => x.json())
    connections.value = r.media_connections || []
  } catch (e) {
    setMsg('Gagal memuat: ' + (e as Error).message, 'err')
  }
}

function openAdd(pid?: string) {
  form.value = { provider_id: pid || '', label: '', secret: '', auth_type: 'api_key', priority: 0 }
  showAdd.value = true
}

async function create() {
  if (!form.value.label || !form.value.provider_id) {
    setMsg(t('common.error'), 'err')
    return
  }
  try {
    const r = await fetch('/api/dashboard/media-connections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value),
    })
    if (!r.ok) {
      const e = await r.json().catch(() => ({}))
      throw new Error((e as any).error || 'gagal')
    }
    showAdd.value = false
    await load()
    setMsg('Akun berhasil ditambahkan', 'ok')
  } catch (e) {
    setMsg('Gagal: ' + (e as Error).message, 'err')
  }
}

async function toggle(a: any) {
  await fetch(`/api/dashboard/media-connections/${a.id}/toggle`, { method: 'PATCH' })
  await load()
}

async function del(a: any) {
  if (!confirm(`t('providers.delete') + ' akun "${a.label}"?`)) return
  await fetch(`/api/dashboard/media-connections/${a.id}`, { method: 'DELETE' })
  await load()
}

function setMsg(t: string, type: 'ok' | 'err') {
  msg.value = t
  msgType.value = type
  setTimeout(() => { msg.value = '' }, 3000)
}

const filtered = () => {
  if (!search.value) return connections.value
  const s = search.value.toLowerCase()
  return connections.value.filter(c =>
    c.label.toLowerCase().includes(s) || c.provider_id.toLowerCase().includes(s)
  )
}

function capChip(cap: string) {
  const cls = cap === 'tts' ? 'chip' : cap === 'stt' ? 'chip' : cap === 'image' ? 'chip' : 'chip'
  return `<span class="${cls}">${cap}</span>`
}

onMounted(() => { load(); loadRegistries() })
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">
        <span class="tag p2">P2</span> /dashboard/media-providers
      </h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">TTS · STT · Image · Video accounts</small>
      <div style="display:flex;gap:.5rem;align-items:center;flex-wrap:wrap">
        <input v-model="search" class="search" style="flex:1;max-width:200px" placeholder="{{ t('providers.search') }}" />
        <button class="btn ghost" @click="load()">↻ Refresh</button>
        <button class="btn" @click="openAdd()">+ Akun Media</button>
      </div>
    </div>

    <!-- Toast -->
    <div v-if="msg" :style="{color:msgType==='ok'?'var(--jkr-grn)':'var(--jkr-red)',marginBottom:'1rem'}" class="note">
      {{ msg }}
    </div>

    <!-- Add modal -->
    <div v-if="showAdd" class="modal-overlay" @click.self="showAdd=false">
      <div class="modal">
        <h3>{{ t('media.add') }}</h3>
        <div class="kv" style="margin-bottom:.8rem">
          <dt>Provider</dt>
          <dd>
            <select v-model="form.provider_id" class="select">
              <option value="" disabled>Pilih provider…</option>
              <option v-for="p in mediaRegistries" :key="p.id" :value="p.id">
                {{ p.name }} ({{ p.caps.join(', ') }})
              </option>
            </select>
          </dd>
          <dt>{{ t('providers.label') }}</dt><dd><input v-model="form.label" class="input" placeholder="e.g. work-key" /></dd>
          <dt>Auth Type</dt>
          <dd>
            <select v-model="form.auth_type" class="select">
              <option value="api_key">API Key</option>
            </select>
          </dd>
          <dt v-if="form.auth_type==='api_key'">t('providers.api_key')</dt>
          <dd v-if="form.auth_type==='api_key'"><input v-model="form.secret" class="input" type="password" placeholder="sk-..." /></dd>
          <dt>{{ t('providers.priority') }}</dt><dd><input v-model.number="form.priority" class="input" type="number" min="0" max="100" /></dd>
        </div>
        <div style="display:flex;gap:.5rem">
          <button class="btn" @click="create">{{ t('providers.save') }}</button>
          <button class="btn ghost" @click="showAdd=false">{{ t('providers.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- Table -->
    <table class="table">
      <thead>
        <tr>
          <th>Provider</th>
          <th>{{ t('providers.label') }}</th>
          <th>Auth</th>
          <th>{{ t('providers.priority') }}</th>
          <th>Status</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="a in filtered()" :key="a.id">
          <td>
            <span class="chip">{{ a.provider_id }}</span>
          </td>
          <td class="font-medium">{{ a.label }}</td>
          <td><span class="chip">{{ a.auth_type }}</span></td>
          <td>{{ a.priority ?? '—' }}</td>
          <td>
            <span
              :class="a.active ? 'st active' : 'st disabled'"
              style="cursor:pointer"
              @click="toggle(a)"
              :title="a.active ? 'Klik untuk nonaktifkan' : 'Klik untuk aktifkan'"
            >
              {{ a.active ? 'aktif' : 'nonaktif' }}
            </span>
          </td>
          <td>
            <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem;color:#b45151" @click="del(a)">🗑</button>
          </td>
        </tr>
        <tr v-if="filtered().length===0">
          <td colspan="6" class="note" style="text-align:center;padding:.5rem">
            {{ showAdd ? '' : 'Belum ada akun media — klik "+ Akun Media"' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
.kv { display:grid; grid-template-columns:120px 1fr; gap:.3rem .8rem; }
.kv dt { color:var(--jkr-text-muted);font-size:.8rem;text-align:right;padding-top:.3rem; }
.kv dd { margin:0; }
</style>
