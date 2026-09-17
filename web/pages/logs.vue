<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const logs = ref<any[]>([])
const loading = ref(false)
const live = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

async function load() {
  loading.value = true
  try {
    const resp = await fetch('/api/dashboard/usage/tail?n=200')
    const data = await resp.json()
    logs.value = data.tail || []
  } finally {
    loading.value = false
  }
}

function toggleLive() {
  live.value = !live.value
  if (live.value) {
    load()
    timer = setInterval(load, 2000)
  } else {
    if (timer) { clearInterval(timer); timer = null }
  }
}

onMounted(load)
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /dashboard/logs</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">Live tail request log — mirror halaman Usage & Logs 9Router</small>
      <div style="display:flex;gap:.5rem;align-items:center">
        <span :class="['st', live ? 'active' : 'disabled']" style="font-size:.7rem">{{ live ? '● LIVE' : '○ OFFLINE' }}</span>
        <button class="btn ghost" :disabled="loading" @click="load()">↻ Refresh</button>
        <button class="btn" :class="live ? '' : 'ghost'" @click="toggleLive()">
          {{ live ? 'Stop Live' : 'Live Tail' }}
        </button>
      </div>
    </div>

    <div v-if="loading && logs.length===0" class="note" style="text-align:center;padding:2rem">Memuat…</div>

    <div v-else-if="logs.length===0" class="note" style="text-align:center;padding:2rem">Belum ada log request.</div>

    <div v-else class="card" style="overflow-x:auto">
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
          </tr>
        </thead>
        <tbody>
          <tr v-for="l in logs" :key="l.request_id">
            <td class="note" style="white-space:nowrap;font-size:.75rem">{{ l.ts }}</td>
            <td><span class="chip" style="font-size:.65rem">{{ l.combo || '—' }}</span></td>
            <td class="font-medium" style="font-size:.8rem">{{ l.model || '—' }}</td>
            <td><span class="chip" style="font-size:.65rem">{{ l.provider || '—' }}</span></td>
            <td><span :class="l.status==='success'?'st active':l.status==='fallback'?'st cooling':'st disabled'" style="font-size:.7rem">{{ l.status }}</span></td>
            <td class="note" style="font-size:.75rem">{{ l.tok_in ?? '—' }}</td>
            <td class="note" style="font-size:.75rem">{{ l.tok_out ?? '—' }}</td>
            <td class="note" style="font-size:.75rem">{{ l.latency_ms ? l.latency_ms + 'ms' : '—' }}</td>
          </tr>
        </tbody>
      </table>
      <p class="note" style="font-size:.7rem;margin-top:.4rem">{{ logs.length }} baris terakhir</p>
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
</style>
