<script setup lang="ts">
import { ref, onMounted } from 'vue'
const providers = ref<any[]>([])
const showing = ref(false)
const form = ref({ label: '', base_url: '', auth_type: 'api_key' })

async function load() {
  const r = await fetch('/api/dashboard/providers').then(x=>x.json())
  providers.value = r.providers || []
}
async function create() {
  await fetch('/api/dashboard/providers', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(form.value) })
  form.value = { label:'', base_url:'', auth_type:'api_key' }
  showing.value = false
  await load()
}
onMounted(load)
</script>
<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-xl font-bold">Providers</h2>
      <button @click="showing=!showing" class="btn-primary">+ Add Provider</button>
    </div>
    <div v-if="showing" class="bg-gray-900 rounded-lg p-4 mb-4 border border-gray-800">
      <h3 class="font-semibold mb-3">New Provider</h3>
      <div class="grid grid-cols-3 gap-3">
        <input v-model="form.label" placeholder="Label (e.g. my-openai)" class="input" />
        <input v-model="form.base_url" placeholder="Base URL" class="input" />
        <select v-model="form.auth_type" class="input">
          <option>api_key</option><option>oauth</option>
        </select>
      </div>
      <div class="mt-3 flex gap-2">
        <button @click="create" class="btn-primary">Save</button>
        <button @click="showing=false" class="btn-secondary">Cancel</button>
      </div>
    </div>
    <div class="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-800 text-gray-400"><tr>
          <th class="text-left p-3">Label</th><th class="text-left p-3">Auth</th>
          <th class="text-left p-3">Base URL</th><th class="text-right p-3"></th>
        </tr></thead>
        <tbody>
          <tr v-for="p in providers" :key="p.id" class="border-t border-gray-800">
            <td class="p-3 font-medium">{{ p.label }}</td>
            <td class="p-3"><span class="badge">{{ p.auth_type }}</span></td>
            <td class="p-3 font-mono text-xs">{{ p.base_url }}</td>
            <td class="p-3 text-right"><button class="btn-danger btn-sm">Delete</button></td>
          </tr>
          <tr v-if="providers.length===0"><td colspan="4" class="p-6 text-center text-gray-600">No providers configured</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<style scoped>
.input { @apply bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm w-full; }
.btn-primary { @apply bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium; }
.btn-secondary { @apply bg-gray-700 hover:bg-gray-600 text-gray-200 px-4 py-2 rounded text-sm; }
.btn-danger { @apply bg-red-600 hover:bg-red-700 text-white px-3 py-1 rounded text-xs; }
.btn-sm { @apply px-2 py-1 text-xs; }
.badge { @apply bg-gray-800 text-gray-300 px-2 py-0.5 rounded text-xs; }
</style>
