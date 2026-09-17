<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, onMounted } from 'vue'
const pools = ref<any[]>([])
const accounts = ref<any[]>([])
const showing = ref(false)
const form = ref({ name: '', proxy_url: '', ptype: 'http', no_proxy: '', strict_proxy: false, is_active: true })

// Deploy state
const deployType = ref<'vercel'|'cloudflare'|'deno'>('vercel')
const deployModal = ref(false)
const deployName = ref('')
const vercelToken = ref('')
const cfAccount = ref('')
const cfToken = ref('')
const denoToken = ref('')
const denoOrg = ref('')
const deploying = ref(false)
const deployResult = ref<{ url: string; poolId: number } | null>(null)
const deployError = ref('')

async function load() {
  const [p, a] = await Promise.all([
    fetch('/api/dashboard/proxy-pools').then(x => x.json()),
    fetch('/api/dashboard/connections').then(x => x.json()),
  ])
  pools.value = p.proxy_pools || []
  accounts.value = a.connections || []
}
async function create() {
  if (!form.value.name) return
  await fetch('/api/dashboard/proxy-pools', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: form.value.name, proxy_url: form.value.proxy_url, ptype: form.value.ptype,
      no_proxy: form.value.no_proxy, strict_proxy: form.value.strict_proxy ? 1 : 0, is_active: form.value.is_active
    })
  })
  form.value = { name: '', proxy_url: '', ptype: 'http', no_proxy: '', strict_proxy: false, is_active: true }
  showing.value = false
  await load()
}
async function del(id: number) {
  if (!confirm(t('proxypools.delete') + '?')) return
  await fetch(`/api/dashboard/proxy-pools/${id}`, { method: 'DELETE' })
  await load()
}
function boundCount(poolId: number | null) {
  return accounts.value.filter(a => a.proxy_pool_id === poolId).length
}
function testStatusClass(p: any) {
  if (p.test_status === 'healthy') return 'st active'
  if (p.test_status === 'deactivated') return 'st disabled'
  return 'st cooling'
}
function testStatusText(p: any) {
  if (p.test_status === 'healthy') return 'active'
  if (p.test_status === 'deactivated') return 'inactive'
  return p.last_tested ? 'testing…' : 'idle'
}
function openDeploy(type: 'vercel'|'cloudflare'|'deno') {
  deployType.value = type
  deployName.value = ''
  vercelToken.value = ''
  cfAccount.value = ''
  cfToken.value = ''
  denoToken.value = ''
  denoOrg.value = ''
  deployResult.value = null
  deployError.value = ''
  deployModal.value = true
}
async function doDeploy() {
  deploying.value = true
  deployError.value = ''
  deployResult.value = null
  try {
    const payload: any = { name: deployName.value || undefined }
    if (deployType.value === 'vercel') payload.vercelToken = vercelToken.value
    else if (deployType.value === 'cloudflare') { payload.accountId = cfAccount.value; payload.apiToken = cfToken.value }
    else { payload.denoToken = denoToken.value; payload.orgDomain = denoOrg.value }
    const ep = `/api/dashboard/proxy-pools/deploy/${deployType.value}`
    const resp = await fetch(ep, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const data = await resp.json()
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`)
    deployResult.value = { url: data.deploy_url, poolId: data.proxy_pool_id }
  } catch (e: any) {
    deployError.value = e.message || String(e)
  } finally {
    deploying.value = false
  }
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /dashboard/proxy-pools</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">CRUD pool + health-test + binding (per akun, bukan global)</small>
      <button class="btn" @click="showing=true">+ New Pool</button>
    </div>
    <div v-if="showing" class="card" style="margin-bottom:1rem">
      <h3>New Proxy Pool</h3>
      <div class="kv" style="margin-bottom:.8rem">
        <dt>Name</dt><dd><input v-model="form.name" class="input" placeholder="e.g. fast-us-1" /></dd>
        <dt>Type</dt><dd>
          <select v-model="form.ptype" class="select">
            <option value="http">http</option><option value="socks5">socks5</option><option value="relay">relay</option>
          </select>
        </dd>
        <dt>Proxy URL</dt><dd><input v-model="form.proxy_url" class="input" placeholder="http://host:port or socks5://host:port" /></dd>
        <dt>No Proxy</dt><dd><input v-model="form.no_proxy" class="input" placeholder="localhost,10.0.0.0/8" /></dd>
        <dt>Strict Proxy</dt><dd><label style="display:flex;align-items:center;gap:.5rem"><input type="checkbox" v-model="form.strict_proxy" /> Yes</label></dd>
        <dt>Active</dt><dd><label style="display:flex;align-items:center;gap:.5rem"><input type="checkbox" v-model="form.is_active" /> Yes</label></dd>
      </div>
      <div style="display:flex;gap:.5rem">
        <button class="btn" @click="create">Save</button>
        <button class="btn ghost" @click="showing=false">Cancel</button>
      </div>
    </div>
    <div class="grid3">
      <div v-for="p in pools" :key="p.id" class="card" style="margin-bottom:0">
        <div style="display:flex;justify-content:space-between;align-items:start;margin-bottom:.4rem">
          <h3 style="color:var(--jkr-lav);font-size:.95rem;margin:0">{{ p.name }}</h3>
          <span :class="testStatusClass(p)" style="font-size:.7rem">{{ testStatusText(p) }}</span>
        </div>
        <div class="kv">
          <dt>Type</dt><dd>{{ p.ptype }}</dd>
          <dt>URL</dt><dd class="font-mono" style="font-size:.75rem">{{ p.proxy_url || '—' }}</dd>
          <dt>noProxy</dt><dd>{{ p.no_proxy || '—' }}</dd>
          <dt>Strict</dt><dd>{{ p.strict_proxy ? '✓' : '—' }}</dd>
          <dt>Akun Terikat</dt><dd>
            <span class="st active" style="font-size:.7rem">{{ boundCount(p.id) }} akun</span>
          </dd>
        </div>
        <div style="display:flex;gap:.5rem;margin-top:.5rem;flex-wrap:wrap">
          <button class="btn ghost btn-sm" @click="load">t('proxypools.test')</button>
          <button class="btn ghost btn-sm" style="color:var(--jkr-red)" @click="del(p.id)">Delete</button>
          <button v-if="p.ptype === 'relay'" class="btn ghost btn-sm" style="color:var(--jkr-green)"
            @click="openDeploy('vercel')" title="Redeploy to Vercel">↗ Vercel</button>
          <button v-if="p.ptype === 'relay'" class="btn ghost btn-sm" style="color:var(--jkr-green)"
            @click="openDeploy('cloudflare')" title="Redeploy to Cloudflare">↗ CF</button>
          <button v-if="p.ptype === 'relay'" class="btn ghost btn-sm" style="color:var(--jkr-green)"
            @click="openDeploy('deno')" title="Redeploy to Deno">↗ Deno</button>
        </div>
        <p class="note" style="font-size:.7rem">last test {{ p.last_tested ? new Date(p.last_tested * 1000).toLocaleString() : 'never' }}</p>
      </div>
    </div>
    <div v-if="pools.length===0 && !showing" class="note" style="text-align:center;padding:2rem">No proxy pools configured</div>

    <!-- Deploy Modal -->
    <div v-if="deployModal" class="modal-overlay" @click.self="deployModal=false">
      <div class="card" style="max-width:420px;margin:2rem auto">
        <h3>{{ deployType === 'vercel' ? 'Deploy to Vercel' : deployType === 'cloudflare' ? 'Deploy to Cloudflare' : 'Deploy to Deno' }} Relay</h3>
        <div class="kv" style="margin-bottom:.8rem">
          <dt>App Name</dt><dd><input v-model="deployName" class="input" placeholder="auto-generated if empty" /></dd>
          <template v-if="deployType==='vercel'">
            <dt>Vercel Token</dt><dd><input v-model="vercelToken" class="input" type="password" placeholder="vc_* token" /></dd>
          </template>
          <template v-else-if="deployType==='cloudflare'">
            <dt>Account ID</dt><dd><input v-model="cfAccount" class="input" placeholder="cloudflare account id" /></dd>
            <dt>API Token</dt><dd><input v-model="cfToken" class="input" type="password" placeholder="Cloudflare API token" /></dd>
          </template>
          <template v-else>
            <dt>Deno Token</dt><dd><input v-model="denoToken" class="input" type="password" placeholder="Deno Deploy token" /></dd>
            <dt>Org Domain</dt><dd><input v-model="denoOrg" class="input" placeholder="e.g. myorg.deno.dev" /></dd>
          </template>
        </div>
        <div v-if="deployError" style="color:var(--jkr-red);font-size:.8rem;margin-bottom:.5rem">{{ deployError }}</div>
        <div v-if="deployResult" style="color:var(--jkr-green);font-size:.8rem;margin-bottom:.5rem">
          ✅ Deployed! Pool ID: {{ deployResult.poolId }}<br>
          <a :href="deployResult.url" target="_blank" style="color:var(--jkr-lav)">{{ deployResult.url }}</a>
        </div>
        <div style="display:flex;gap:.5rem">
          <button class="btn" :disabled="deploying" @click="doDeploy">{{ deploying ? '⏳ Deploying…' : 'Deploy' }}</button>
          <button class="btn ghost" @click="deployModal=false">Cancel</button>
        </div>
        <p class="note" style="font-size:.65rem;margin-top:.5rem">
          {{ deployType === 'vercel'
            ? 'Requires Vercel CLI token (vc_*)'
            : deployType === 'cloudflare'
              ? 'Requires Cloudflare Account ID + API Token with Workers edit permission'
              : 'Requires Deno Deploy token + org domain' }}
        </p>
      </div>
    </div>
  </div>
</template>
<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
.btn-sm { font-size:.7rem;padding:.15rem .4rem; }
.btn-danger { background:var(--jkr-red); color:var(--jkr-crust); border:none; padding:.25rem .5rem; border-radius:4px; font-size:.7rem; cursor:pointer; }
.font-mono { font-family:ui-monospace,'Cascadia Code',monospace; }
.modal-overlay { position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.5);display:flex;align-items:flex-start;justify-content:center;z-index:100;padding-top:5vh; }
</style>
