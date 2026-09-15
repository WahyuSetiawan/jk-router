<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
const usage = ref<any[]>([])
const filter = ref({ provider: '', model: '', date: '' })

async function load() {
  const params = new URLSearchParams()
  if (filter.value.provider) params.set('provider', filter.value.provider)
  if (filter.value.model) params.set('model', filter.value.model)
  if (filter.value.date) params.set('date', filter.value.date)
  const r = await fetch(`/api/dashboard/usage?${params}`).then(x=>x.json())
  usage.value = r.usage || []
}
const totalCost = computed(() => usage.value.reduce((s, r) => {
  // We need pricing data, but for now just show token counts
  return s + r.tok_in + r.tok_out
}, 0))

onMounted(load)
</script>
<template>
  <div>
    <h2 class="text-xl font-bold mb-4">Usage Log</h2>
    <div class="flex gap-3 mb-4">
      <input v-model="filter.provider" placeholder="Filter by provider" class="input" />
      <input v-model="filter.model" placeholder="Filter by model" class="input" />
      <input v-model="filter.date" type="date" class="input" />
      <button @click="load" class="btn-primary">Filter</button>
    </div>
    <div class="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-gray-800 text-gray-400"><tr>
          <th class="text-left p-3">Time</th><th class="text-left p-3">Request</th>
          <th class="text-left p-3">Model</th><th class="text-left p-3">Provider</th>
          <th class="text-left p-3">Status</th><th class="text-right p-3">Latency</th>
          <th class="text-right p-3">Tokens</th>
        </tr></thead>
        <tbody>
          <tr v-for="r in usage" :key="r.request_id" class="border-t border-gray-800">
            <td class="p-3 text-gray-400 text-xs">{{ r.ts?.slice(11,19) }}</td>
            <td class="p-3 font-mono text-xs">{{ r.request_id }}</td>
            <td class="p-3 font-mono text-xs">{{ r.model }}</td>
            <td class="p-3">{{ r.provider }}</td>
            <td class="p-3"><span :class="r.status==='success'?'text-green-400':'text-red-400'">{{ r.status }}</span></td>
            <td class="p-3 text-right">{{ r.latency_ms }}ms</td>
            <td class="p-3 text-right font-mono text-xs">{{ r.tok_in }}+{{ r.tok_out }}</td>
          </tr>
          <tr v-if="usage.length===0"><td colspan="7" class="p-6 text-center text-gray-600">No usage data</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<style scoped>
.input { @apply bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm; }
.btn-primary { @apply bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium; }
</style>
