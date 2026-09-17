<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, computed, onMounted, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
Chart.register(...registerables)

const activeTab = ref('usage') // 'usage' | 'logs'

const logs = ref<any[]>([])
const stats = ref<any>({ requests: 0, tokens_in: 0, tokens_out: 0, cost: 0, avg_ms: 0 })
const loading = ref(false)
const filterProvider = ref('')
const filterModel = ref('')
const filterStatus = ref('')
const filterCombo = ref('')
const page = ref(1)
const pageSize = 50
const chartInstance = ref<any>(null)

// t('usage.filter')ed logs
const filteredLogs = computed(() => {
  return logs.value.filter(l => {
    if (filterProvider.value && l.provider !== filterProvider.value) return false
    if (filterModel.value && !l.model?.toLowerCase().includes(filterModel.value.toLowerCase())) return false
    if (filterStatus.value && l.status !== filterStatus.value) return false
    if (filterCombo.value && !l.combo?.toLowerCase().includes(filterCombo.value.toLowerCase())) return false
    return true
  })
})

const providerOptions = computed(() => [...new Set(logs.value.map(l => l.provider).filter(Boolean))] as string[])
const modelOptions = computed(() => [...new Set(logs.value.map(l => l.model).filter(Boolean))] as string[])
const statusOptions = ['success', 'error', 'fallback', 'timeout']

// Provider summary for table: aggregated by provider
const providerSummary = computed(() => {
  const m = new Map<string, { count: number; latSum: number; costSum: number }>()
  filteredLogs.value.forEach((l: any) => {
    const k = l.provider || 'unknown'
    const e = m.get(k) || { count: 0, latSum: 0, costSum: 0 }
    e.count++
    e.latSum += (l.latency_ms || 0)
    e.costSum += (l.cost || 0)
    m.set(k, e)
  })
  return [...m.entries()].map(([k, v]) => ({
    provider: k,
    count: v.count,
    pct: filteredLogs.value.length ? (v.count / filteredLogs.value.length * 100).toFixed(1) : '0',
    avgLat: v.count ? Math.round(v.latSum / v.count) : 0,
    cost: v.costSum
  })).sort((a, b) => b.count - a.count)
})

// Chart data: aggregated by provider (stacked: success vs error)
const chartData = computed(() => {
  const byProvider = new Map<string, { ok: number; err: number }>()
  filteredLogs.value.forEach(l => {
    const k = l.provider || 'unknown'
    const e = byProvider.get(k) || { ok: 0, err: 0 }
    if (l.status === 'success') e.ok++
    else e.err++
    byProvider.set(k, e)
  })
  const sorted = [...byProvider.entries()].sort((a, b) => (b[1].ok + b[1].err) - (a[1].ok + a[1].err))
  return {
    labels: sorted.map(e => e[0]),
    datasets: [
      { label: 'success', data: sorted.map(e => e[1].ok), backgroundColor: '#a6e3a1', borderWidth: 0 },
      { label: 'error/fallback', data: sorted.map(e => e[1].err), backgroundColor: '#f38ba8', borderWidth: 0 },
    ]
  }
})

async function load(useTail = false) {
  loading.value = true
  try {
    const params = new URLSearchParams({ page: String(page.value), size: String(pageSize) })
    const [logsResp, statsResp] = await Promise.all([
      fetch(`/api/dashboard/usage?${params}`),
      fetch('/api/dashboard/usage/stats'),
    ])
    const logsData = await logsResp.json()
    const statsData = await statsResp.json()
    if (useTail) {
      logs.value = logsData.tail || []
    } else {
      logs.value = logsData.logs || []
      stats.value = statsData
    }
    // rebuild chart after data loads
    rebuildChart()
  } finally {
    loading.value = false
  }
}

async function tail() {
  loading.value = true
  try {
    const resp = await fetch('/api/dashboard/usage/tail', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' })
    const data = await resp.json()
    logs.value = data.tail || []
    rebuildChart()
  } finally {
    loading.value = false
  }
}

function rebuildChart() {
  // Destroy previous instance
  if (chartInstance.value) {
    chartInstance.value.destroy()
    chartInstance.value = null
  }
  const canvas = document.getElementById('usageChart') as HTMLCanvasElement
  if (!canvas || chartData.value.labels.length === 0) return
  chartInstance.value = new Chart(canvas, {
    type: 'bar',
    data: chartData.value,
    options: {
      responsive: true,
      plugins: { legend: { display: false } },
      scales: {
        x: { ticks: { color: '#a8b4c4', font: { size: 11 } }, grid: { display: false } },
        y: { ticks: { color: '#a8b4c4' }, grid: { color: 'rgba(168,180,196,0.15)' }, beginAtZero: true }
      }
    }
  })
}

watch(activeTab, (tab) => {
  if (tab === 'usage') {
    setTimeout(rebuildChart, 50)
  }
})

function formatTs(ts: string) {
  if (!ts) return '—'
  return new Date(ts).toLocaleString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function costColor(c: number) {
  if (c <= 0) return 'var(--jkr-text-muted)'
  return c < 0.01 ? 'var(--jkr-emerald)' : 'var(--jkr-red)'
}

function statusClass(s: string) {
  return s === 'success' ? 'st active' : s === 'error' ? 'st disabled' : 'st cooling'
}

onMounted(() => load())
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /dashboard/usage</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">Per-request log (WAL + writer goroutine) + chart ringan</small>
    </div>

    <!-- Tabs -->
    <div style="display:flex;gap:0;margin-bottom:1rem;border-bottom:1px solid var(--jkr-brd);font-size:.8rem">
      <span :class="{active: activeTab==='usage'}" class="tab-item" @click="activeTab='usage'">Usage</span>
      <span :class="{active: activeTab==='logs'}" class="tab-item" @click="activeTab='logs'">Logs (live stream)</span>
      <span :class="{active: activeTab==='quota'}" class="tab-item" @click="activeTab='quota'">Quota <span class="tag p2" style="margin-left:.3rem">P2</span></span>
    </div>

    <!-- ===== USAGE TAB ===== -->
    <template v-if="activeTab === 'usage'">
      <!-- Stats Grid -->
      <div class="grid4" style="margin-bottom:1rem">
        <div class="card stat"><div class="n">{{ stats.requests || '—' }}</div><div class="l">requests / 7d</div></div>
        <div class="card stat"><div class="n">{{ (stats.tokens_in || 0).toLocaleString() }}</div><div class="l">tokens in</div></div>
        <div class="card stat"><div class="n">${{ (stats.cost || 0).toFixed(4) }}</div><div class="l">cost / 7d</div></div>
        <div class="card stat"><div class="n">{{ stats.avg_ms ? stats.avg_ms + 'ms' : '—' }}</div><div class="l">avg latency</div></div>
      </div>

      <div style="display:flex;gap:.5rem;align-items:center;flex-wrap:wrap;margin-bottom:.8rem">
        <button class="btn ghost" @click="load()">↻ Load</button>
        <button class="btn ghost" @click="tail()">⚡ Tail</button>
      </div>
      <div class="card" style="margin-bottom:1rem">
        <div style="display:flex;gap:.5rem;flex-wrap:wrap;align-items:center">
          <select v-model="filterProvider" class="select" style="width:130px">
            <option value="">Semua Provider</option>
            <option v-for="p in providerOptions" :key="p" :value="p">{{ p }}</option>
          </select>
          <input v-model="filterModel" class="input" style="width:150px" placeholder="Filter model…" />
          <select v-model="filterStatus" class="select" style="width:110px">
            <option value="">Semua Status</option>
            <option v-for="s in statusOptions" :key="s" :value="s">{{ s }}</option>
          </select>
          <input v-model="filterCombo" class="input" style="width:150px" placeholder="Filter combo…" />
          <button class="btn ghost" style="font-size:.75rem;padding:.2rem .5rem" @click="() => { filterProvider=''; filterModel=''; filterStatus=''; filterCombo='' }">Reset</button>
          <span class="note" style="font-size:.75rem;margin-left:auto">{{ filteredLogs.length }} baris</span>
        </div>
      </div>

      <!-- Chart -->
      <div class="card" style="margin-top:.8rem">
        <h3>Chart: tokens per hari, per provider</h3>
        <div class="bar" style="height:80px"><i style="width:84%"></i></div>
        <p class="note">chart.js bar/line. Filter: provider / model / hari / status.</p>
        <div style="height:200px;position:relative;margin-top:.5rem">
          <canvas id="usageChart"></canvas>
        </div>
        <div v-if="filteredLogs.length===0" class="note" style="text-align:center;padding:.5rem;font-size:.8rem">Belum ada data</div>
      </div>

      <!-- Summary table -->
      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem">
          <h3 style="color:var(--jkr-lav);font-size:.9rem;margin:0">Ringkasan</h3>
        </div>
        <div style="overflow-x:auto">
          <table class="table">
            <thead>
              <tr>
                <th>Provider</th>
                <th>Requests</th>
                <th>%</th>
                <th>Avg Latency (ms)</th>
                <th>Total Cost ($)</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(g,i) in providerSummary" :key="i">
                <td><span class="chip" style="font-size:.7rem">{{ g.provider }}</span></td>
                <td class="note">{{ g.count }}</td>
                <td class="note">{{ g.pct }}%</td>
                <td class="note">{{ g.avgLat }}ms</td>
                <td class="note" :style="{color: costColor(g.cost)}">${g.cost.toFixed(4)}</td>
              </tr>
              <tr v-if="filteredLogs.length===0">
                <td colspan="5" class="note" style="text-align:center;padding:1rem">Tidak ada data</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <!-- ===== LOGS TAB ===== -->
    <template v-else-if="activeTab === 'logs'">
      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem">
          <h3 style="color:var(--jkr-lav);font-size:.9rem;margin:0">Request detail (expand row)</h3>
          <span class="note" style="font-size:.75rem">{{ loading ? 'Memuat…' : filteredLogs.length + ' entries' }}</span>
        </div>
        <div style="overflow-x:auto">
          <table class="table">
            <thead>
              <tr>
                <th>time</th>
                <th>client→model</th>
                <th>upstream (provider/akun)</th>
                <th>state</th>
                <th>tok in/out</th>
                <th>ms</th>
                <th>status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="l in filteredLogs" :key="l.request_id">
                <td class="note" style="white-space:nowrap;font-size:.75rem">{{ formatTs(l.ts) }}</td>
                <td class="font-mono" style="font-size:.75rem">{{ l.combo || '—' }}</td>
                <td class="note" style="font-size:.75rem">{{ l.provider }} · {{ l.account || '—' }}</td>
                <td><span :class="l.status==='success'?'st ok':l.status==='fallback'?'st cooling':'st err'" style="font-size:.7rem">{{ l.status }}</span></td>
                <td class="note" style="font-size:.75rem">{{ l.tok_in ?? '—' }} / {{ l.tok_out ?? '—' }}</td>
                <td class="note" style="font-size:.75rem">{{ l.latency_ms || '—' }}</td>
                <td class="note" style="font-size:.75rem">{{ l.http_status || '—' }}</td>
              </tr>
              <tr v-if="filteredLogs.length===0 && !loading">
                <td colspan="7" class="note" style="text-align:center;padding:1rem">Tidak ada data penggunaan</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="note">Failover tercatat sebagai baris terpisah — log.txt mirror di ~/.jkrouter.</p>
      </div>
    </template>
    <template v-else-if="activeTab === 'quota'">
      <div class="note" style="padding:2rem;text-align:center">
        <span class="tag p2">P2</span> Quota per-akun — hal. /quota
      </div>
    </template>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
.tab-item { color: var(--jkr-mut); padding: .3rem 0; cursor: pointer; border-bottom: 2px solid transparent; font-size: .85rem; }
.tab-item.active { color: var(--jkr-blue); border-bottom-color: var(--jkr-blue); }
.tab-item:hover:not(.active) { color: var(--jkr-txt); }
</style>
