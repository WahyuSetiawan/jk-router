<script setup lang="ts">
import { ref, onMounted } from 'vue'
const keys = ref<any[]>([])
const showing = ref(false)
const newKey = ref('')
const copied = ref<number | null>(null)

async function load() {
  const r = await fetch('/api/dashboard/api-keys').then(x => x.json())
  keys.value = r.keys || []
}
async function create() {
  const r = await fetch('/api/dashboard/api-keys', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label: 'user-key-' + Date.now() })
  })
  const j = await r.json()
  newKey.value = j.key || ''
  showing.value = false
  await load()
}
async function revoke(id: number) {
  await fetch(`/api/dashboard/api-keys/${id}/revoke`, { method: 'POST' })
  await load()
}
async function copyKey(id: number, rawKey: string) {
  await navigator.clipboard.writeText(rawKey)
  copied.value = id
  setTimeout(() => { copied.value = null }, 1500)
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">API Keys</h2>
      <button class="btn" @click="showing=true">+ Generate</button>
    </div>
    <p class="note" style="margin-bottom:1rem">Client pakai key ini ke /v1 — bukan key provider upstream.</p>
    <div v-if="newKey" class="card" style="background:#f9e2af22;border-color:var(--jkr-yel)">
      <p style="color:var(--jkr-yel);font-size:.8rem;margin-bottom:.5rem">Save this key — it won't be shown again:</p>
      <code style="color:var(--jkr-txt);font-size:.85rem;word-break:break-all">{{ newKey }}</code>
    </div>
    <div v-if="showing" class="card">
      <button class="btn" @click="create">Generate</button>
      <button class="btn ghost" style="margin-left:.5rem" @click="showing=false">Cancel</button>
    </div>
    <div class="card">
      <table class="table">
        <thead><tr><th>Name</th><th>Key</th><th>Status</th><th>Created</th><th></th></tr></thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td class="font-medium">{{ k.label || '(no label)' }}</td>
            <td class="font-mono" style="font-size:.75rem">{{ k.key_hash?.slice(0,12) }}…</td>
            <td><span :class="k.revoked?'st disabled':'st active'">{{ k.revoked ? 'Revoked' : 'Active' }}</span></td>
            <td style="color:var(--jkr-mut);font-size:.75rem">{{ new Date(k.created_at * 1000).toLocaleDateString() }}</td>
            <td style="text-align:right">
              <button v-if="!k.revoked" class="btn ghost btn-sm" style="margin-right:.3rem" @click="copyKey(k.id, k.key_display || '')">Copy</button>
              <button v-if="!k.revoked" class="btn-danger btn-sm" @click="revoke(k.id)">Revoke</button>
            </td>
          </tr>
          <tr v-if="keys.length===0"><td colspan="5" class="note" style="text-align:center;padding:1rem">No API keys</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:.5rem; }
.btn-danger { background:var(--jkr-red); color:var(--jkr-crust); border:none; padding:.25rem .5rem; border-radius:4px; font-size:.7rem; cursor:pointer; }
</style>