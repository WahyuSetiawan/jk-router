<script setup lang="ts">
import { ref, onMounted } from 'vue'
const keys = ref<any[]>([])
const showing = ref(false)
const newKey = ref('')

async function load() {
  const r = await fetch('/api/dashboard/api-keys').then(x=>x.json())
  keys.value = r.keys || []
}
async function create() {
  const r = await fetch('/api/dashboard/api-keys', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({ label: 'user-key-' + Date.now() }) })
  const j = await r.json()
  newKey.value = j.key || ''
  showing.value = false
  await load()
}
async function revoke(id:number) {
  await fetch(`/api/dashboard/api-keys/${id}/revoke`, { method:'POST' })
  await load()
}
onMounted(load)
</script>
<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-xl font-bold">API Keys</h2>
      <button @click="showing=!showing" class="btn-primary">+ Generate Key</button>
    </div>
    <div v-if="newKey" class="bg-yellow-900/30 border border-yellow-700 rounded-lg p-4 mb-4">
      <p class="text-yellow-300 text-sm mb-2">Save this key — it won't be shown again:</p>
      <code class="text-yellow-200 text-sm break-all">{{ newKey }}</code>
    </div>
    <div v-if="showing" class="bg-gray-900 rounded-lg p-4 mb-4 border border-gray-800">
      <button @click="create" class="btn-primary">Generate</button>
      <button @click="showing=false" class="btn-secondary ml-2">Cancel</button>
    </div>
    <div class="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-800 text-gray-400"><tr>
          <th class="text-left p-3">Label</th><th class="text-left p-3">Key Hash</th>
          <th class="text-left p-3">Status</th><th class="text-left p-3">Created</th><th class="text-right p-3"></th>
        </tr></thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id" class="border-t border-gray-800">
            <td class="p-3 font-medium">{{ k.label || '(no label)' }}</td>
            <td class="p-3 font-mono text-xs">{{ k.key_hash?.slice(0,16) }}...</td>
            <td class="p-3"><span :class="k.revoked?'text-red-400':'text-green-400'">{{ k.revoked?'Revoked':'Active' }}</span></td>
            <td class="p-3 text-gray-400 text-xs">{{ new Date(k.created_at*1000).toLocaleDateString() }}</td>
            <td class="p-3 text-right"><button v-if="!k.revoked" @click="revoke(k.id)" class="btn-danger btn-sm">Revoke</button></td>
          </tr>
          <tr v-if="keys.length===0"><td colspan="5" class="p-6 text-center text-gray-600">No API keys</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<style scoped>
.btn-primary { @apply bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium; }
.btn-secondary { @apply bg-gray-700 hover:bg-gray-600 text-gray-200 px-4 py-2 rounded text-sm; }
.btn-danger { @apply bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-xs; }
.btn-sm { @apply px-2 py-1 text-xs; }
</style>
