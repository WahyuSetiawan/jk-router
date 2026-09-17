<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, computed, onMounted } from 'vue'

const providers = ref<any[]>([])
const proxyPools = ref<{id: number; name: string}[]>([])
const showAddAccount = ref(false)
const showAddProvider = ref(false)
const search = ref('')

// Per-provider expand state
const expanded = ref<Record<string, boolean>>({})

// Form state
const providerForm = ref({ name: '', label: '', auth_type: 'api_key', base_url: '' })
const accountForm = ref({
  provider_id: '', label: '', auth_type: 'api_key', secret: '',
  priority: 0, proxy_pool_id: null as number | null,
})
const editingAccount = ref<number | null>(null)
const editForm = ref<Record<number, any>>({})

async function load() {
  const [pr, pp] = await Promise.all([
    fetch('/api/dashboard/providers').then(r => r.json()),
    fetch('/api/dashboard/proxy-pools').then(r => r.json()),
  ])
  providers.value = pr.providers || []
  proxyPools.value = pp.proxy_pools || []
}

function openAddAccount(pid: string) {
  accountForm.value = { provider_id: pid, label: '', auth_type: 'api_key', secret: '', priority: 0, proxy_pool_id: null }
  showAddAccount.value = true
}

async function createAccount() {
  const { provider_id, label, auth_type, secret, priority, proxy_pool_id } = accountForm.value
  if (!label) return
  const body: any = { provider_id, label, auth_type, priority, proxy_pool_id }
  if (auth_type === 'api_key' && secret) body.secret = secret
  await fetch('/api/dashboard/connections', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
  })
  showAddAccount.value = false
  await load()
}

async function createProvider() {
  const { name, label, auth_type, base_url } = providerForm.value
  if (!name) return
  await fetch('/api/dashboard/providers', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, label, auth_type, base_url })
  })
  providerForm.value = { name: '', label: '', auth_type: 'api_key', base_url: '' }
  showAddProvider.value = false
  await load()
}

function toggleProvider(pid: string) {
  expanded.value[pid] = !expanded.value[pid]
}

function startEdit(a: any) {
  editingAccount.value = a.id
  editForm.value[a.id] = { ...a }
}

async function saveEdit(a: any) {
  const f = editForm.value[a.id]
  await fetch(`/api/dashboard/connections/${a.id}`, {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: f.label, proxy_pool_id: f.proxy_pool_id || null })
  })
  editingAccount.value = null
  await load()
}

function cancelEdit() { editingAccount.value = null }

async function deleteAccount(id: number) {
  if (!confirm(t('providers.delete') + '?')) return
  await fetch(`/api/dashboard/connections/${id}`, { method: 'DELETE' })
  await load()
}

async function toggleState(a: any) {
  const next = a.state === 'active' ? 'disabled' : 'active'
  await fetch(`/api/dashboard/connections/${a.id}/toggle`, { method: 'PATCH' })
  await load()
}

// Quota calculation for OAuth accounts
function getQuotaStatus(a: any): { pct: number; used: number; limit: number; resetIn: string } | null {
  if (a.quota_limit <= 0) return null
  const used = a.quota_used || 0
  const pct = Math.min(100, Math.round((used / a.quota_limit) * 100))
  const resetAt = a.quota_reset_at || 0
  const resetIn = resetAt > 0 ? formatTimeLeft(resetAt - Math.floor(Date.now() / 1000)) : '—'
  return { pct, used, limit: a.quota_limit, resetIn }
}

function formatTimeLeft(sec: number): string {
  if (sec <= 0) return 'berakhir'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) return `${h}j ${m}m`
  return `${m}m`
}

// OAuth refresh status
function getOAuthStatus(a: any): { masked: string; countdown: string; needsRefresh: boolean } | null {
  if (a.auth_type !== 'oauth') return null
  const masked = '●●●●●●●●' + (a.label ? ' ' + a.label.slice(-4) : '')
  const expiresAt = a.expires_at || 0
  const now = Math.floor(Date.now() / 1000)
  const remains = expiresAt - now
  const needsRefresh = remains < 300 // 5 min
  const countdown = remains > 0 ? formatTimeLeft(remains) : 'berakhir'
  return { masked, countdown, needsRefresh }
}

function stateClass(s: string) {
  return s === 'active' ? 'st active' : s === 'cooling_down' ? 'st cooling' : 'st disabled'
}

function stateLabel(s: string) {
  return s === 'active' ? 'aktif' : s === 'cooling_down' ? 'pendinginan' : 'dinonaktifkan'
}

function authChip(t: string) {
  const cls = t === 'oauth' ? 'chip oauth' : 'chip'
  return `<span class="${cls}">${t}</span>`
}

const filtered = () => {
  if (!search.value) return providers.value
  const s = search.value.toLowerCase()
  return providers.value.filter(p =>
    p.name.toLowerCase().includes(s) || p.accounts?.some(a => a.label.toLowerCase().includes(s))
  )
}

onMounted(load)
</script>

<template>
  <div>
    <!-- Header -->
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /dashboard/providers</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">CRUD provider + connections (akun) + bind proxy pool</small>
      <div style="display:flex;gap:.5rem;align-items:center;flex-wrap:wrap">
        <input v-model="search" class="search" style="flex:1;max-width:200px" placeholder="Cari provider…" />
        <button class="btn" @click="showAddProvider=true">+ Provider</button>
        <button class="btn ghost" @click="load()">↻ Refresh</button>
      </div>
    </div>

    <!-- Add Provider Modal -->
    <div v-if="showAddProvider" class="modal-overlay" @click.self="showAddProvider=false">
      <div class="modal">
        <h3>{{ t('providers.add_provider') }}</h3>
        <div class="kv" style="margin-bottom:.8rem">
          <dt>Nama ID</dt><dd><input v-model="providerForm.name" class="input" placeholder="e.g. openai" /></dd>
          <dt>{{ t('providers.label') }}</dt><dd><input v-model="providerForm.label" class="input" placeholder="e.g. Work Account" /></dd>
          <dt>Auth Type</dt><dd>
            <select v-model="providerForm.auth_type" class="select">
              <option value="api_key">API Key</option><option value="oauth">OAuth</option>
            </select>
          </dd>
          <dt>Base URL</dt><dd><input v-model="providerForm.base_url" class="input" placeholder="https://api.openai.com" /></dd>
        </div>
        <div style="display:flex;gap:.5rem">
          <button class="btn" @click="createProvider">{{ t('providers.save') }}</button>
          <button class="btn ghost" @click="showAddProvider=false">{{ t('providers.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- Add Account Modal -->
    <div v-if="showAddAccount" class="modal-overlay" @click.self="showAddAccount=false">
      <div class="modal">
        <h3>{{ t('providers.add_account') }}</h3>
        <div class="kv" style="margin-bottom:.8rem">
          <dt>{{ t('providers.label') }}</dt><dd><input v-model="accountForm.label" class="input" placeholder="e.g. work-mail" /></dd>
          <dt>Auth Type</dt><dd>
            <select v-model="accountForm.auth_type" class="select">
              <option value="api_key">API Key</option><option value="oauth">OAuth</option>
            </select>
          </dd>
          <dt v-if="accountForm.auth_type==='api_key'">API Secret</dt>
          <dd v-if="accountForm.auth_type==='api_key'"><input v-model="accountForm.secret" class="input" placeholder="sk-..." /></dd>
          <dt>{{ t('providers.proxy_pool') }}</dt><dd>
            <select v-model="accountForm.proxy_pool_id" class="select">
              <option :value="null">{{ t('providers.none') }}</option>
              <option v-for="pp in proxyPools" :key="pp.id" :value="pp.id">{{ pp.name }}</option>
            </select>
          </dd>
          <dt>{{ t('providers.priority') }}</dt><dd><input v-model.number="accountForm.priority" class="input" type="number" min="0" max="100" /></dd>
        </div>
        <div style="display:flex;gap:.5rem">
          <button class="btn" @click="createAccount">{{ t('providers.save') }}</button>
          <button class="btn ghost" @click="showAddAccount=false">{{ t('providers.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- Provider Cards -->
    <template v-for="p in filtered()" :key="p.id">
      <div class="card">
        <!-- Provider header -->
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.6rem">
          <div style="display:flex;align-items:center;gap:.5rem">
            <h3 style="color:var(--jkr-lav);margin:0;font-size:1rem">{{ p.name }}</h3>
            <span class="st active" style="font-size:.7rem">{{ p.accounts?.length ?? 0 }} akun</span>
          </div>
          <button class="btn ghost" style="font-size:.75rem;padding:.2rem .5rem" @click="openAddAccount(p.id)">+ Akun</button>
        </div>

        <!-- Capability badges -->
        <div style="display:flex;gap:.3rem;flex-wrap:wrap;margin-bottom:.5rem">
          <span class="chip" style="font-size:.65rem;background:var(--jkr-surface2)">vision</span>
          <span class="chip" style="font-size:.65rem;background:var(--jkr-surface2)">text</span>
        </div>

        <!-- Account Table -->
        <table class="table">
          <thead>
            <tr>
              <th></th>
              <th>Name</th>
              <th>Type</th>
              <th>Proxy Pool</th>
              <th>Priority</th>
              <th>State</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="a in p.accounts" :key="a.id">
              <!-- Expand toggle -->
              <td>
                <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem" @click="toggleProvider(p.id + '-' + a.id)">
                  {{ expanded[p.id + '-' + a.id] ? '▼' : '▶' }}
                </button>
              </td>

              <!-- Name -->
              <td>
                <template v-if="editingAccount === a.id">
                  <input v-model="editForm[a.id].label" class="input" style="width:100px" />
                </template>
                <template v-else>
                  <span class="font-medium">{{ a.label }}</span>
                  <!-- OAuth token display -->
                  <div v-if="getOAuthStatus(a)" class="oauth-token" style="margin-top:.2rem">
                    <span class="chip" style="font-size:.65rem">🔑 {{ getOAuthStatus(a)!.masked }}</span>
                    <span v-if="getOAuthStatus(a)!.needsRefresh" class="chip" style="font-size:.65rem;background:#b45151">refresh↑</span>
                    <span class="note" style="font-size:.65rem;margin-left:.3rem">⏱ {{ getOAuthStatus(a)!.countdown }}</span>
                  </div>
                </template>
              </td>

              <!-- Auth Type -->
              <td>
                <span class="chip" :class="a.auth_type==='oauth'?'oauth':''">{{ a.auth_type }}</span>
              </td>

              <!-- Proxy Pool -->
              <td>
                <template v-if="editingAccount === a.id">
                  <select v-model="editForm[a.id].proxy_pool_id" class="select" style="width:120px">
                    <option :value="null">{{ t('providers.none') }}</option>
                    <option v-for="pp in proxyPools" :key="pp.id" :value="pp.id">{{ pp.name }}</option>
                  </select>
                </template>
                <template v-else>
                  <span v-if="a.proxy_pool_name" class="chip">{{ a.proxy_pool_name }}</span>
                  <span v-else class="note">—</span>
                </template>
              </td>

              <!-- Priority -->
              <td>{{ a.priority ?? '—' }}</td>

              <!-- State -->
              <td>
                <span v-if="editingAccount !== a.id" :class="stateClass(a.state)" style="cursor:pointer" @click="toggleState(a)">
                  {{ stateLabel(a.state) }}
                </span>
              </td>

              <!-- Actions -->
              <td>
                <template v-if="editingAccount === a.id">
                  <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem" @click="saveEdit(a)">✓</button>
                  <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem" @click="cancelEdit">✗</button>
                </template>
                <template v-else>
                  <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem" @click="startEdit(a)">✏️</button>
                  <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem;color:#b45151" @click="deleteAccount(a.id)">🗑</button>
                </template>
              </td>
            </tr>
            <tr v-if="!p.accounts?.length">
              <td colspan="7" class="note" style="text-align:center;padding:.5rem">
                Belum ada akun — klik "+ Akun"
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <div v-if="filtered().length===0 && !showAddProvider" class="note" style="text-align:center;padding:2rem">
      Belum ada provider. Klik "+ Provider" untuk memulai.
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
.oauth-token { display:flex; align-items:center; gap:.3rem; }
.quota-bar { display:flex; align-items:center; gap:.3rem; }
.quota-track { flex-shrink:0; }
</style>
