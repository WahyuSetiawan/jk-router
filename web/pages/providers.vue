<script setup lang="ts">
import { ref, onMounted } from 'vue'
const providers = ref<any[]>([])
const showAdd = ref(false)
const form = ref({ name: '', label: '', auth_type: 'api_key', base_url: '' })
const search = ref('')

async function load() {
  const r = await fetch('/api/dashboard/providers').then(x => x.json())
  providers.value = r.providers || []
}
async function create() {
  if (!form.value.name) return
  await fetch('/api/dashboard/providers', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: form.value.name, label: form.value.label, auth_type: form.value.auth_type, base_url: form.value.base_url })
  })
  form.value = { name: '', label: '', auth_type: 'api_key', base_url: '' }
  showAdd.value = false
  await load()
}
const filtered = () => {
  if (!search.value) return providers.value
  const s = search.value.toLowerCase()
  return providers.value.filter(p => p.name.toLowerCase().includes(s) || p.accounts?.some(a => a.label.toLowerCase().includes(s)))
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">Providers</h2>
      <div style="display:flex;gap:.5rem;align-items:center">
        <input v-model="search" class="search" style="flex:1;max-width:240px" placeholder="Cari provider…" />
        <button class="btn" @click="showAdd=true">+ Add API Key</button>
        <button class="btn ghost" @click="showAdd=true;form.auth_type='oauth'">+ Connect OAuth</button>
      </div>
    </div>
    <div v-if="showAdd" class="card" style="margin-bottom:1rem">
      <h3>New Provider Account</h3>
      <div class="kv" style="margin-bottom:.8rem">
        <dt>Provider Name</dt><dd><input v-model="form.name" class="input" placeholder="e.g. openai" /></dd>
        <dt>Account Label</dt><dd><input v-model="form.label" class="input" placeholder="e.g. work-mail" /></dd>
        <dt>Auth Type</dt><dd>
          <select v-model="form.auth_type" class="select">
            <option value="api_key">API Key</option><option value="oauth">OAuth</option>
          </select>
        </dd>
        <dt>Base URL</dt><dd><input v-model="form.base_url" class="input" placeholder="https://api.openai.com" /></dd>
      </div>
      <div style="display:flex;gap:.5rem">
        <button class="btn" @click="create">Save</button>
        <button class="btn ghost" @click="showAdd=false">Cancel</button>
      </div>
    </div>
    <template v-for="p in filtered()" :key="p.id">
      <div class="card">
        <h3>{{ p.name }} <span class="st active">{{ p.accounts?.length ?? 0 }} akun</span></h3>
        <table class="table">
          <thead><tr><th>Name</th><th>Type</th><th>Priority</th><th>State</th></tr></thead>
          <tbody>
            <tr v-for="a in p.accounts" :key="a.id">
              <td class="font-medium">{{ a.label }}</td>
              <td><span class="chip">{{ a.auth_type }}</span></td>
              <td>{{ a.priority ?? '—' }}</td>
              <td>
                <span v-if="a.state==='active'" class="st active">active</span>
                <span v-else-if="a.state==='cooling_down'" class="st cooling">cooling_down</span>
                <span v-else class="st disabled">disabled</span>
              </td>
            </tr>
            <tr v-if="!p.accounts?.length"><td colspan="4" class="note" style="text-align:center;padding:.5rem">No accounts — add one above</td></tr>
          </tbody>
        </table>
      </div>
    </template>
    <div v-if="filtered().length===0 && !showAdd" class="note" style="text-align:center;padding:2rem">No providers configured</div>
  </div>
</template>
<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
</style>