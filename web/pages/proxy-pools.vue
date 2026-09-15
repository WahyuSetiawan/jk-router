<script setup lang="ts">
import { ref, onMounted } from 'vue'
const pools = ref<any[]>([])
const showing = ref(false)
const form = ref({ name: '', proxy_url: '', ptype: 'http', no_proxy: '', strict_proxy: false, is_active: true })

async function load() {
  const r = await fetch('/api/dashboard/proxy-pools').then(x => x.json())
  pools.value = r.proxy_pools || []
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
  await fetch(`/api/dashboard/proxy-pools/${id}`, { method: 'DELETE' })
  await load()
}
const stateClass = (p: any) => {
  if (p.test_status === 'healthy' || p.test_status === 'idle') return 'st ok'
  if (p.test_status === 'deactivated') return 'st err'
  return 'st ok'
}
const stateText = (p: any) => p.test_status || 'idle'
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">Proxy Pools</h2>
      <button class="btn" @click="showing=true">+ New Pool</button>
    </div>
    <div v-if="showing" class="card">
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
        <h3>{{ p.name }} <span :class="stateClass(p)">{{ stateText(p) }}</span></h3>
        <div class="kv">
          <dt>Type</dt><dd>{{ p.ptype }}</dd>
          <dt>URL</dt><dd class="font-mono" style="font-size:.75rem">{{ p.proxy_url || '—' }}</dd>
          <dt>noProxy</dt><dd>{{ p.no_proxy || '—' }}</dd>
          <dt>Strict</dt><dd>{{ p.strict_proxy ? '✓' : '—' }}</dd>
        </div>
        <div style="display:flex;gap:.5rem;margin-top:.5rem">
          <button class="btn ghost btn-sm">Test</button>
          <button class="btn-danger btn-sm" @click="del(p.id)">Delete</button>
        </div>
        <p class="note">last test {{ p.last_tested ? new Date(p.last_tested * 1000).toLocaleString() : 'never' }}</p>
      </div>
    </div>
    <div v-if="pools.length===0 && !showing" class="note" style="text-align:center;padding:2rem">No proxy pools configured</div>
  </div>
</template>
<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
.btn-danger { background:var(--jkr-red); color:var(--jkr-crust); border:none; padding:.25rem .5rem; border-radius:4px; font-size:.7rem; cursor:pointer; }
.font-mono { font-family:ui-monospace,'Cascadia Code',monospace; }
</style>