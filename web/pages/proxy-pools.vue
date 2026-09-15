<script setup lang="ts">
import { ref, onMounted } from 'vue'
const pools = ref<any[]>([])
const showing = ref(false)
const form = ref({ label: '', description: '', proxy_list: '' })

async function load() {
  const r = await fetch('/api/dashboard/proxy-pools').then(x=>x.json())
  pools.value = r.proxy_pools || []
}
async function create() {
  if (!form.value.label) return
  await fetch('/api/dashboard/proxy-pools', { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(form.value) })
  form.value = { label:'', description:'', proxy_list:'' }
  showing.value = false
  await load()
}
async function del(id:number) {
  await fetch(`/api/dashboard/proxy-pools/${id}`, { method:'DELETE' })
  await load()
}
onMounted(load)
</script>
<template>
  <div>
    <div class="flex justify-between items-center mb-4">
      <h2 class="text-xl font-bold">Proxy Pools</h2>
      <button @click="showing=!showing" class="btn-primary">+ New Pool</button>
    </div>
    <div v-if="showing" class="bg-gray-900 rounded-lg p-4 mb-4 border border-gray-800">
      <h3 class="font-semibold mb-3">New Proxy Pool</h3>
      <div class="space-y-3">
        <input v-model="form.label" placeholder="Pool name" class="input" />
        <input v-model="form.description" placeholder="Description" class="input" />
        <textarea v-model="form.proxy_list" placeholder='Proxy list (one per line: http://host:port or socks5://host:port)' class="input" rows="4" />
      </div>
      <div class="mt-3 flex gap-2">
        <button @click="create" class="btn-primary">Save</button>
        <button @click="showing=false" class="btn-secondary">Cancel</button>
      </div>
    </div>
    <div class="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-800 text-gray-400"><tr>
          <th class="text-left p-3">Label</th><th class="text-left p-3">Status</th>
          <th class="text-left p-3">Proxies</th><th class="text-right p-3"></th>
        </tr></thead>
        <tbody>
          <tr v-for="p in pools" :key="p.id" class="border-t border-gray-800">
            <td class="p-3 font-medium">{{ p.label }}</td>
            <td class="p-3"><span :class="{'text-green-400':p.status==='healthy'||p.status==='idle','text-red-400':p.status==='deactivated'}">{{ p.status||'idle' }}</span></td>
            <td class="p-3 font-mono text-xs max-w-xs truncate">{{ p.proxy_list?.slice(1,-1)?.split(',').join(', ') }}</td>
            <td class="p-3 text-right"><button @click="del(p.id)" class="btn-danger btn-sm">Delete</button></td>
          </tr>
          <tr v-if="pools.length===0"><td colspan="4" class="p-6 text-center text-gray-600">No proxy pools</td></tr>
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
</style>
