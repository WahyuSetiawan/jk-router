<script setup lang="ts">
import { ref, onMounted } from 'vue'
const combos = ref<any[]>([])
const showing = ref(false)
const form = ref({ name: '', description: '', model_ids: [] as string[] })

async function load() {
  const r = await fetch('/api/dashboard/combos').then(x=>x.json())
  combos.value = r.combos || []
}
async function create() {
  if (!form.value.name) return
  await fetch('/api/dashboard/combos', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(form.value) })
  form.value = { name:'', description:'', model_ids:[] }
  showing.value = false
  await load()
}
async function del(id:number) {
  await fetch(`/api/dashboard/combos/${id}`, { method:'DELETE' })
  await load()
}
onMounted(load)
</script>
<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-xl font-bold">Combos</h2>
      <button @click="showing=!showing" class="btn-primary">+ New Combo</button>
    </div>
    <div v-if="showing" class="bg-gray-900 rounded-lg p-4 mb-4 border border-gray-800">
      <h3 class="font-semibold mb-3">New Combo</h3>
      <div class="space-y-3">
        <input v-model="form.name" placeholder="Combo name (e.g. fast-vision)" class="input" />
        <input v-model="form.description" placeholder="Description" class="input" />
        <input v-model="form.model_ids" placeholder='Model IDs, comma-separated (e.g. gpt-4o,claude-sonnet-4)' class="input" />
      </div>
      <div class="mt-3 flex gap-2">
        <button @click="create" class="btn-primary">Save</button>
        <button @click="showing=false" class="btn-secondary">Cancel</button>
      </div>
    </div>
    <div class="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-800 text-gray-400"><tr>
          <th class="text-left p-3">Name</th><th class="text-left p-3">Strategy</th>
          <th class="text-left p-3">Models</th><th class="text-right p-3"></th>
        </tr></thead>
        <tbody>
          <tr v-for="c in combos" :key="c.id" class="border-t border-gray-800">
            <td class="p-3 font-medium">{{ c.name }}</td>
            <td class="p-3"><span class="badge">{{ c.strategy || 'fallback' }}</span></td>
            <td class="p-3 font-mono text-xs">{{ c.model_ids?.slice(1,-1)?.split(',').join(', ') }}</td>
            <td class="p-3 text-right"><button @click="del(c.id)" class="btn-danger btn-sm">Delete</button></td>
          </tr>
          <tr v-if="combos.length===0"><td colspan="4" class="p-6 text-center text-gray-600">No combos yet</td></tr>
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
