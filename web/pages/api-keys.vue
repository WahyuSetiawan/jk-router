<script setup lang="ts">
import { ref, onMounted } from 'vue'
const keys = ref<any[]>([])
const showingNew = ref(false)
const newFullKey = ref('')
const copied = ref<number | null>(null)

async function load() {
  const r = await fetch('/api/dashboard/api-keys').then(x => x.json())
  keys.value = r.keys || []
}
async function generate() {
  const r = await fetch('/api/dashboard/api-keys', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label: 'user-key-' + Date.now() })
  })
  const j = await r.json()
  newFullKey.value = j.key || ''
  showingNew.value = true
  await load()
}
async function revoke(id: number) {
  await fetch(`/api/dashboard/api-keys/${id}/revoke`, { method: 'POST' })
  await load()
}
async function copyKey(id: number, rawKey: string) {
  if (!rawKey) return
  await navigator.clipboard.writeText(rawKey)
  copied.value = id
  setTimeout(() => { copied.value = null }, 1500)
}
function maskKey(key: string): string {
  if (!key || key.length < 8) return '••••••••'
  return key.slice(0, 4) + '••••••••' + key.slice(-2)
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">API Keys</h2>
    </div>
    <p class="note" style="margin-bottom:1rem">Client pakai key ini ke /v1 — bukan key provider upstream.</p>

    <div v-if="showingNew && newFullKey" class="card" style="background:#f9e2af22;border-color:var(--jkr-yel);margin-bottom:1rem">
      <p style="color:var(--jkr-yel);font-size:.8rem;margin-bottom:.5rem">Save this key — it won't be shown again:</p>
      <div class="row" style="gap:.5rem;align-items:center">
        <code style="color:var(--jkr-txt);font-size:.85rem;word-break:break-all;flex:1">{{ newFullKey }}</code>
        <button class="btn ghost btn-sm" @click="copyKey(-1, newFullKey)">
          {{ copied === -1 ? 'Copied!' : 'Copy' }}
        </button>
        <button class="btn ghost btn-sm" @click="showingNew = false">✕</button>
      </div>
    </div>

    <div style="margin-bottom:.8rem;text-align:right">
      <button class="btn" @click="generate">+ Generate</button>
    </div>

    <div class="card">
      <table class="table">
        <thead><tr><th>Name</th><th>Key</th><th>Last used</th><th>Requests</th><th></th></tr></thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td class="font-medium">{{ k.label || '(no label)' }}</td>
            <td class="font-mono" style="font-size:.75rem;color:var(--jkr-yel)">{{ maskKey(k.key_display || '') }}</td>
            <td style="color:var(--jkr-mut);font-size:.75rem">—</td>
            <td style="color:var(--jkr-mut);font-size:.75rem">—</td>
            <td style="text-align:right">
              <button
                class="btn ghost btn-sm"
                style="margin-right:.3rem"
                :disabled="!!k.revoked"
                @click="copyKey(k.id, k.key_display || '')"
              >
                {{ copied === k.id ? 'Copied!' : 'Copy' }}
              </button>
              <button
                class="btn-danger btn-sm"
                @click="revoke(k.id)"
                :disabled="k.revoked"
              >
                {{ k.revoked ? 'Revoked' : 'Revoke' }}
              </button>
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
.btn-danger:disabled { opacity:.5; cursor:not-allowed; }
.btn-sm { padding:.15rem .4rem; font-size:.7rem; }
.row { display:flex; align-items:center; }
</style>
