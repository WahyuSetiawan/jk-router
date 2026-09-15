<script setup lang="ts">
import { ref, onMounted } from 'vue'
const stats = ref({ total: 0, success: 0, errors: 0, avgLatency: 0, costUSD: 0 })
const recent = ref<any[]>([])

async function load() {
  const [u, c] = await Promise.all([
    fetch('/api/dashboard/usage').then(r => r.json()),
    fetch('/api/dashboard/usage/cost').then(r => r.json()),
  ])
  const usage = u.usage || []
  stats.value = {
    total: usage.length,
    success: usage.filter(x => x.status === 'success').length,
    errors: usage.filter(x => x.status === 'error').length,
    avgLatency: usage.length ? Math.round(usage.reduce((s:number,x:number)=>s+x.latency_ms,0)/usage.length) : 0,
    costUSD: (c.costs||[]).reduce((s:number,x:any)=>s+x.cost_usd,0).toFixed(4),
  }
  recent.value = usage.slice(0, 10)
}
onMounted(load)
</script>
<template>
  <div>
    <h2 class="text-xl font-bold mb-4">Dashboard</h2>
    <div class="grid grid-cols-4 gap-4 mb-6">
      <div class="stat-card">
        <div class="stat-label">Total Requests</div>
        <div class="stat-value">{{ stats.total }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Success</div>
        <div class="stat-value text-green-400">{{ stats.success }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Errors</div>
        <div class="stat-value text-red-400">{{ stats.errors }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Est. Cost</div>
        <div class="stat-value text-yellow-400">${{ stats.costUSD }}</div>
      </div>
    </div>
    <div class="bg-gray-900 rounded-lg p-4">
      <h3 class="font-semibold mb-3">Recent Requests</h3>
      <table class="w-full text-sm">
        <thead><tr class="text-gray-500 border-b border-gray-800">
          <th class="text-left pb-2">Time</th><th class="text-left pb-2">Model</th>
          <th class="text-left pb-2">Provider</th><th class="text-left pb-2">Status</th>
          <th class="text-left pb-2">Latency</th><th class="text-right pb-2">Tokens</th>
        </tr></thead>
        <tbody>
          <tr v-for="r in recent" :key="r.request_id" class="border-b border-gray-800 last:border-0">
            <td class="py-2 text-gray-400">{{ r.ts?.slice(5) }}</td>
            <td class="py-2 font-mono text-xs">{{ r.model }}</td>
            <td class="py-2">{{ r.provider }}</td>
            <td class="py-2"><span :class="r.status==='success'?'text-green-400':'text-red-400'">{{ r.status }}</span></td>
            <td class="py-2">{{ r.latency_ms }}ms</td>
            <td class="py-2 text-right font-mono text-xs">{{ r.tok_in }}+{{ r.tok_out }}</td>
          </tr>
          <tr v-if="recent.length===0"><td colspan="6" class="py-4 text-center text-gray-600">No usage data yet</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
<style scoped>
.stat-card { @apply bg-gray-900 rounded-lg p-4 border border-gray-800; }
.stat-label { @apply text-xs text-gray-500 uppercase tracking-wide; }
.stat-value { @apply text-2xl font-bold mt-1; }
</style>
