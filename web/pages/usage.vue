<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

const logs = ref<any[]>([])
const stats = ref<any>({ total_requests: 0, total_tok_in: 0, total_tok_out: 0, total_cost: 0, p95_latency: 0 })
const loading = ref(false)
const filterProvider = ref('')
const filterModel = ref('')
const filterStatus = ref('')
const filterCombo = ref('')
const page = ref(1)
const pageSize = 50

// Filtered logs
const filteredLogs = computed(() => {
  return logs.value.filter(l => {
    if (filterProvider.value && l.provider !== filterProvider.value) return false
    if (filterModel.value && !l.model?.toLowerCase().includes(filterModel.value.toLowerCase())) return false
    if (filterStatus.value && l.status !== filterStatus.value) return false
    if (filterCombo.value && !l.combo?.toLowerCase().includes(filterCombo.value.toLowerCase())) return false
    return true
  })
})

// Unique values for filters
const providerOptions = computed(() => [...new Set(logs.value.map(l => l.provider).filter(Boolean))] as string[])
const modelOptions = computed(() => [...new Set(logs.value.map(l => l.model).filter(Boolean))] as string[])
const statusOptions = ['success', 'error', 'fallback', 'timeout']

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
  } finally {
    loading.value = false
  }
}

function formatTs(ts: string) {
  if (!ts) return '—'
  return new Date(ts).toLocaleString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function costColor(c: number) {
  if (c <= 0) return 'var(--jkr-text-muted)'
  return c < 0.01 ? '#40916c' : '#b45151'
}

function statusClass(s: string) {
  return s === 'success' ? 'st active' : s === 'error' ? 'st disabled' : 'st cooling'
}

onMounted(() => load())
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">Usage</h2>
      <div style="display:flex;gap:.5rem;align-items:center;flex-wrap:wrap">
        <button class="btn ghost" @click="load()">↻ Load</button>
        <button class="btn ghost" @click="tail()">⚡ Tail Live</button>
      </div>
    </div>

    <!-- Stats Grid -->
    <div class="stats-grid" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:.6rem;margin-bottom:1rem">
      <div class="stat-card">
        <div class="note" style="font-size:.7rem">Requests (7d)</div>
        <div class="stat-val">{{ stats.total_requests || '—' }}</div>
      </div>
      <div class="stat-card">
        <div class="note" style="font-size:.7rem">Tokens In</div>
        <div class="stat-val">{{ (stats.total_tok_in || 0).toLocaleString() }}</div>
      </div>
      <div class="stat-card">
        <div class="note" style="font-size:.7rem">Tokens Out</div>
        <div class="stat-val">{{ (stats.total_tok_out || 0).toLocaleString() }}</div>
      </div>
      <div class="stat-card">
        <div class="note" style="font-size:.7rem">Total Cost ($)</div>
        <div class="stat-val" :style="{color: costColor(stats.total_cost || 0)}">
          ${{ (stats.total_cost || 0).toFixed(4) }}
        </div>
      </div>
      <div class="stat-card">
        <div class="note" style="font-size:.7rem">P95 Latency</div>
        <div class="stat-val">{{ stats.p95_latency ? stats.p95_latency + 'ms' : '—' }}</div>
      </div>
    </div>

    <!-- Filter Bar -->
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

    <!-- Chart Visualization -->
    <div class="card" style="margin-bottom:1rem">
      <h3 style="color:var(--jkr-lav);font-size:.9rem;margin:0 0 .5rem">Request Distribution (by Provider)</h3>
      <div style="display:flex;flex-direction:column;gap:.4rem">
        <div v-for="(group, idx) in (() => {
          const map = new Map()
          filteredLogs.value.forEach(l => {
            const k = l.provider || 'unknown'
            map.set(k, (map.get(k) || 0) + 1)
          })
          return [...map.entries()].sort((a, b) => b[1] - a[1])
        })()" :key="idx" style="display:flex;align-items:center;gap:.5rem">
          <span style="width:100px;font-size:.75rem;text-align:right;flex-shrink:0">{{ group[0] }}</span>
          <div class="chart-bar-track" style="flex:1;height:18px;background:var(--jkr-surface2);border-radius:3px;overflow:hidden">
            <div class="chart-bar-fill"
              :style="{width: filteredLogs.length ? Math.round(group[1] / filteredLogs.length * 100) : 0 + '%', background: ['#3d59a4','#7aa2f7','#9aa5ce','#c6a0f6','#ed9a8a','#a3d4a5'][idx % 6]}">
            </div>
          </div>
          <span class="note" style="width:40px;font-size:.7rem;flex-shrink:0">{{ group[1] }}</span>
        </div>
        <div v-if="filteredLogs.length===0" class="note" style="text-align:center;padding:.5rem;font-size:.8rem">Belum ada data</div>
      </div>
    </div>

    <!-- Logs Table -->
    <div class="card">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem">
        <h3 style="color:var(--jkr-lav);font-size:.9rem;margin:0">Request Log</h3>
        <span class="note" style="font-size:.75rem">{{ loading ? 'Memuat…' : filteredLogs.length + ' entries' }}</span>
      </div>
      <div style="overflow-x:auto">
        <table class="table">
          <thead>
            <tr>
              <th>Waktu</th>
              <th>Combo</th>
              <th>Model</th>
              <th>Provider</th>
              <th>Status</th>
              <th>Tok In</th>
              <th>Tok Out</th>
              <th>Latency</th>
              <th>Cost</th>
              <th>Adapter</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in filteredLogs" :key="l.request_id">
              <td class="note" style="white-space:nowrap;font-size:.75rem">{{ formatTs(l.ts) }}</td>
              <td><span class="chip" style="font-size:.65rem">{{ l.combo || '—' }}</span></td>
              <td class="font-medium" style="font-size:.8rem">{{ l.model || '—' }}</td>
              <td><span class="chip" style="font-size:.65rem">{{ l.provider || '—' }}</span></td>
              <td><span :class="statusClass(l.status)" style="font-size:.7rem">{{ l.status }}</span></td>
              <td class="note" style="font-size:.75rem">{{ l.tok_in ?? '—' }}</td>
              <td class="note" style="font-size:.75rem">{{ l.tok_out ?? '—' }}</td>
              <td class="note" style="font-size:.75rem">{{ l.latency_ms ? l.latency_ms + 'ms' : '—' }}</td>
              <td class="note" style="font-size:.75rem;color:var(--jkr-emerald)">{{ l.cost ? '$' + Number(l.cost).toFixed(4) : '—' }}</td>
              <td><span class="chip" style="font-size:.6rem">{{ l.adapter_used || '—' }}</span></td>
            </tr>
            <tr v-if="filteredLogs.length===0 && !loading">
              <td colspan="10" class="note" style="text-align:center;padding:1rem">Tidak ada data penggunaan</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
.stats-grid { grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:.6rem;margin-bottom:1rem; }
.stat-card { background:var(--jkr-surface2);padding:.6rem .8rem;border-radius:var(--jkr-radius); }
.stat-val { color:var(--jkr-floo);font-size:1.4rem;font-weight:600;margin-top:.2rem; }
.chart-bar-track { flex:1;height:18px;background:var(--jkr-surface2);border-radius:3px;overflow:hidden; }
.chart-bar-fill { height:100%;border-radius:3px;transition:width .3s ease; }
</style>
