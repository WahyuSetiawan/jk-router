<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'

const connections = ref<any[]>([])
const loading = ref(false)
const refreshing = ref<Record<number, boolean>>({})

interface QuotaData {
  id: number
  label: string
  provider: string
  quota_limit: number
  quota_window_seconds: number
  quota_reset_at: number
  used: number
}

const quotaData = ref<QuotaData[]>([])

async function load() {
  loading.value = true
  try {
    const r = await fetch('/api/dashboard/connections').then(x => x.json())
    connections.value = r.connections || []
    
    // Fetch quota for each account
    const results = await Promise.all(
      connections.value.map(async (c: any) => {
        try {
          const qr = await fetch(`/api/dashboard/connections/${c.id}/quota`)
          if (!qr.ok) return null
          return qr.json()
        } catch { return null }
      })
    )
    
    quotaData.value = connections.value.map((c, i) => ({
      id: c.id,
      label: c.name,
      provider: c.provider_name,
      ...(results[i] || { quota_limit: 0, quota_window_seconds: 86400, quota_reset_at: 0, used: 0 })
    })).filter(q => q.quota_limit > 0)
  } finally {
    loading.value = false
  }
}

function pct(q: QuotaData): number {
  if (!q.quota_limit) return 0
  return Math.min(100, Math.round((q.used / q.quota_limit) * 100))
}

function timeLeft(q: QuotaData): string {
  const now = Math.floor(Date.now() / 1000)
  const sec = (q.quota_reset_at || 0) - now
  if (sec <= 0) return 'berakhir'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) return `${h}j ${m}m`
  return `${m}m`
}

function formatWindow(sec: number): string {
  if (sec >= 86400) return `${sec / 86400} hari`
  if (sec >= 3600) return `${sec / 3600} jam`
  return `${sec / 60} menit`
}

async function refreshQuota(id: number) {
  refreshing.value[id] = true
  try {
    await load()
  } finally {
    delete refreshing.value[id]
  }
}

async function saveQuota(id: number, limit: number, windowSec: number) {
  await fetch(`/api/dashboard/connections/${id}/quota`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ quota_limit: limit, quota_window_seconds: windowSec })
  })
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">Quota Manager</h2>
      <button class="btn ghost" :disabled="loading" @click="load()">↻ Refresh</button>
    </div>

    <div v-if="loading && quotaData.length===0" class="note" style="text-align:center;padding:2rem">Memuat quota…</div>
    
    <div v-else-if="quotaData.length===0" class="note" style="text-align:center;padding:2rem">
      Belum ada akun dengan quota yang dikonfigurasi.
      <br /><span class="note" style="font-size:.75rem">Set quota_limit > 0 pada akun di halaman Providers untuk mengaktifkan tracking.</span>
    </div>

    <div v-else class="grid3">
      <div v-for="q in quotaData" :key="q.id" class="card">
        <div style="display:flex;justify-content:space-between;align-items:start;margin-bottom:.4rem">
          <div>
            <h3 style="color:var(--jkr-lav);font-size:.95rem;margin:0">{{ q.label }}</h3>
            <p class="note" style="font-size:.7rem;margin:.1rem 0 0">{{ q.provider }}</p>
          </div>
          <button class="btn ghost" style="padding:.1rem .3rem;font-size:.7rem" :disabled="!!refreshing[q.id]" @click="refreshQuota(q.id)">
            {{ refreshing[q.id] ? '↻' : '↺' }}
          </button>
        </div>

        <!-- Progress bar -->
        <div style="margin:.5rem 0">
          <div style="display:flex;justify-content:space-between;margin-bottom:.2rem">
            <span class="note" style="font-size:.7rem">Penggunaan</span>
            <span class="note" style="font-size:.7rem">{{ q.used.toLocaleString() }} / {{ q.quota_limit.toLocaleString() }} tok</span>
          </div>
          <div style="height:8px;background:var(--jkr-surface2);border-radius:4px;overflow:hidden">
            <div :style="{width:pct(q)+'%',background:pct(q)>80?'#b45151':pct(q)>60?'#e5a10e':'var(--jkr-emerald)',height:'100%',borderRadius:'4px',transition:'width .3s'}" style="height:100%;border-radius:4px;transition:width .3s ease"></div>
          </div>
          <div style="text-align:right;margin-top:.1rem">
            <span class="note" style="font-size:.65rem;color:{{ pct(q)>80?'var(--jkr-red)':'var(--jkr-text-muted)' }}">{{ pct(q) }}%</span>
          </div>
        </div>

        <!-- Time left -->
        <div style="display:flex;gap:.5rem;margin-bottom:.5rem">
          <span class="chip" style="font-size:.65rem">⏱ reset: {{ timeLeft(q) }}</span>
          <span class="chip" style="font-size:.65rem">📏 window: {{ formatWindow(q.quota_window_seconds) }}</span>
        </div>

        <!-- Edit quota -->
        <div class="kv" style="margin-top:.5rem">
          <dt style="font-size:.7rem">Daily Limit (tokens)</dt>
          <dd>
            <input type="number" class="input" style="width:100px" :value="q.quota_limit" 
              @change="saveQuota(q.id, parseInt(($event.target as HTMLInputElement).value) || 0, q.quota_window_seconds)" />
          </dd>
          <dt style="font-size:.7rem">Window (menit)</dt>
          <dd>
            <input type="number" class="input" style="width:80px" :value="Math.round(q.quota_window_seconds / 60)" 
              @change="saveQuota(q.id, q.quota_limit, parseInt(($event.target as HTMLInputElement).value) * 60)" />
          </dd>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
</style>
