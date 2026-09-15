<script setup lang="ts">
import { ref, onMounted } from 'vue'
const connections = ref<any[]>([])
const showing = ref(false)
const form = ref({ provider_id: '', name: '', secret: '', auth_type: 'api_key' })

async function load() {
  const r = await fetch('/api/dashboard/connections').then(x=>x.json())
  connections.value = r.connections || []
}
async function toggle(id: number) {
  await fetch(`/api/dashboard/connections/${id}/toggle`, { method:'PATCH' })
  await load()
}
async function create() {
  await fetch('/api/dashboard/connections', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(form.value) })
  form.value = { provider_id:'', name:'', secret:'', auth_type:'api_key' }
  showing.value = false
  await load()
}
onMounted(load)
</script>
<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-xl font-bold">Connections</h2>
      <button @click="showing=!showing" class="btn-primary">+ Add Connection</button>
    </div>
    <div v-if="showing" class="bg-gray-900 rounded-lg p-4 mb-4 border border-gray-800">
      <h3 class="font-semibold mb-3">New Connection</h3>
      <div class="space-y-3">
        <input v-model="form.provider_id" placeholder="Provider ID (numeric)" class="input" />
        <input v-model="form.name" placeholder="Connection name" class="input" />
        <select v-model="form.auth_type" class="input">
          <option>api_key</option><option>oauth</option>
        </select>
        <input v-model="form.secret" placeholder="API key / secret (optional)" class="input" type="password" />
      </div>
      <div class="mt-3 flex gap-2">
        <button @click="create" class="btn-primary">Save</button>
        <button @click="showing=false" class="btn-secondary">Cancel</button>
      </div>
    </div>
    <div class="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-800 text-gray-400"><tr>
          <th class="text-left p-3">Name</th><th class="text-left p-3">Provider</th>
          <th class="text-left p-3">Auth</th><th class="text-center p-3">Status</th><th class="text-right p-3"></th>
        </tr></thead>
        <tbody>
          <tr v-for="c in connections" :key="c.id" class="border-t border-gray-800">
            <td class="p-3 font-medium">{{ c.name }}</td>
            <td class="p-3 text-gray-400">{{ c.provider_id }}</td>
            <td class="p-3"><span class="badge">{{ c.auth_type }}</span></td>
            <td class="p-3 text-center">
              <button @click="toggle(c.id)" :class="c.disabled?'btn-disabled':'btn-active'" class="px-2 py-1 rounded text-xs">
                {{ c.disabled ? 'Disabled' : 'Active' }}
              </button>
            </td>
            <td class="p-3 text-right text-gray-600 text-xs">{{ new Date(c.created_at*1000).toLocaleDateString() }}</td>
          </tr>
          <tr v-if="connections.length===0"><td colspan="5" class="p-6 text-center text-gray-600">No connections yet</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<style scoped>
.input { @apply bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm w-full; }
.btn-primary { @apply bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium; }
.btn-secondary { @apply bg-gray-700 hover:bg-gray-600 text-gray-200 px-4 py-2 rounded text-sm; }
.badge { @apply bg-gray-800 text-gray-300 px-2 py-0.5 rounded text-xs; }
.btn-active { @apply bg-green-600 text-white px-2 py-1 rounded text-xs; }
.btn-disabled { @apply bg-gray-700 text-gray-400 px-2 py-1 rounded text-xs; }
</style>
