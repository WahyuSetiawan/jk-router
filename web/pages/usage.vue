<script setup lang="ts">
import { ref, onMounted } from 'vue'
type Tab = 'usage' | 'logs'
const activeTab = ref<Tab>('usage')
const logs = ref<any[]>([])
const usage = ref<any[]>([])
const filter = ref({ provider: '', model: '', date: '' })

async function loadUsage() {
  const params = new URLSearchParams()
  if (filter.value.provider) params.set('provider', filter.value.provider)
  if (filter.value.model) params.set('model', filter.value.model)
  if (filter.value.date) params.set('date', filter.value.date)
  const r = await fetch(`/api/dashboard/usage?${params}`).then(x => x.json())
  usage.value = r.usage || []
}
async function loadLogs() {
  const r = await fetch('/api/dashboard/usage/tail', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }).then(x => x.json())
  logs.value = r.logs || []
}
async function switchTab(tab: Tab) {
  activeTab.value = tab
  if (tab === 'usage') await loadUsage()
  else await loadLogs()
}
onMounted(loadUsage)
</script>
<template>
  <div>
    <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin-bottom:1rem">Usage & Logs</h2>
    <div class="tabs">
      <span :class="{ active: activeTab === 'usage' }" @click="switchTab('usage')">Usage</span>
      <span :class="{ active: activeTab === 'logs' }" @click="switchTab('logs')">Logs (recent)</span>
    </div>
    <template v-if="activeTab === 'usage'">
      <div class="flex gap-3 mb-4" style="display:flex;gap:.75rem;margin-bottom:1rem;flex-wrap:wrap">
        <input v-model="filter.provider" class="input" style="width:180px" placeholder="Filter provider" />
        <input v-model="filter.model" class="input" style="width:180px" placeholder="Filter model" />
        <input v-model="filter.date" type="date" class="input" style="width:160px" />
        <button class="btn" @click="loadUsage">Filter</button>
      </div>
      <div class="card">
        <table class="table">
          <thead><tr><th>Time</th><th>Request</th><th>Model</th><th>Provider</th><th>Status</th><th>Latency</th><th>Tokens</th></tr></thead>
          <tbody>
            <tr v-for="r in usage" :key="r.request_id">
              <td style="color:var(--jkr-mut);font-size:.75rem">{{ r.ts?.slice(11,19) }}</td>
              <td class="font-mono" style="font-size:.7rem">{{ r.request_id?.slice(0,12) }}…</td>
              <td class="font-mono" style="font-size:.75rem">{{ r.model }}</td>
              <td>{{ r.provider }}</td>
              <td><span :class="r.status==='success'?'st ok':'st err'">{{ r.status }}</span></td>
              <td>{{ r.latency_ms }}ms</td>
              <td class="font-mono" style="font-size:.75rem">{{ r.tok_in ?? 0 }}+{{ r.tok_out ?? 0 }}</td>
            </tr>
            <tr v-if="usage.length===0"><td colspan="7" class="note" style="text-align:center;padding:1rem">No usage data</td></tr>
          </tbody>
        </table>
      </div>
    </template>
    <template v-else>
      <div class="card">
        <table class="table">
          <thead><tr><th>Time</th><th>Level</th><th>Message</th></tr></thead>
          <tbody>
            <tr v-for="l in logs" :key="l.idx">
              <td style="color:var(--jkr-mut);font-size:.75rem">{{ l.ts }}</td>
              <td><span class="chip">{{ l.level }}</span></td>
              <td class="font-mono" style="font-size:.75rem">{{ l.msg }}</td>
            </tr>
            <tr v-if="logs.length===0"><td colspan="3" class="note" style="text-align:center;padding:1rem">No logs</td></tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>