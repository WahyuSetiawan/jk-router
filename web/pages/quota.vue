<script setup lang="ts">
import { ref, onMounted } from 'vue'
interface Conn {
  id: number
  name: string
  provider_name: string
  state: string
}
interface QuotaRow {
  id: number
  quota_limit: number
  quota_window_seconds: number
  quota_reset_at: number
  used: number
}

const connections = ref<Conn[]>([])
const quotas = ref<Record<number, QuotaRow>>({})
const editingId = ref<number | null>(null)
const editLimit = ref(0)
const editWindow = ref(86400)

async function load() {
  const [cRes, qRes] = await Promise.all([
    fetch('/api/dashboard/connections').then(r => r.json()),
    fetch('/api/dashboard/usage/stats').then(r => r.json()).catch(() => ({ stats: [] })),
  ])
  connections.value = cRes.connections || []
  // Precompute quota usage from usage_log (last window per account)
  const stats: any[] = qRes.stats || []
  const byAccount: Record<number, { in: number; out: number }> = {}
  for (const s of stats) {
    const aid = s.account_id
    if (!aid) continue
    if (!byAccount[aid]) byAccount[aid] = { in: 0, out: 0 }
    byAccount[aid].in += s.tok_in ?? 0
    byAccount[aid].out += s.tok_out ?? 0
  }
  // Load per-account quota settings
  for (const c of connections.value) {
    const r = await fetch(`/api/dashboard/connections/${c.id}/quota`).then(x => x.json()).catch(() => null)
    if (r) {
      quotas.value[c.id] = {
        id: c.id,
        quota_limit: r.quota_limit ?? 0,
        quota_window_seconds: r.quota_window_seconds ?? 86400,
        quota_reset_at: r.quota_reset_at ?? 0,
        used: byAccount[c.id]?.in + byAccount[c.id]?.out ?? 0,
      }
    }
  }
}

function startEdit(id: number) {
  editingId.value = id
  const q = quotas.value[id]
  editLimit.value = q?.quota_limit ?? 0
  editWindow.value = q?.quota_window_seconds ?? 86400
}

async function saveQuota(id: number) {
  await fetch(`/api/dashboard/connections/${id}/quota`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ quota_limit: editLimit.value, quota_window_seconds: editWindow.value }),
  })
  editingId.value = null
  await load()
}

function resetWindow(id: number) {
  quotas.value[id] = { ...quotas.value[id], quota_reset_at: Date.now() / 1000 + quotas.value[id].quota_window_seconds }
}

function pct(q: QuotaRow) {
  if (!q.quota_limit || q.quota_limit === 0) return 0
  return Math.min(100, Math.round((q.used / q.quota_limit) * 100))
}

function timeLeft(ts: number) {
  const sec = Math.max(0, ts - Date.now() / 1000)
  if (sec < 3600) return `${Math.round(sec / 60)}m`
  return `${Math.round(sec / 3600)}h${Math.round((sec % 3600) / 60)}m`
}

onMounted(load)
</script>

<template>
  <div>
    <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin-bottom:1rem">Quota Management</h2>
    <p style="color:var(--jkr-sub1);font-size:.8rem;margin-bottom:1.5rem">Set per-account token limits with a sliding window. Accounts exceeding their quota are automatically disabled until the window resets.</p>

    <div class="card" v-if="connections.length === 0" style="text-align:center;padding:2rem;color:var(--jkr-sub1)">No accounts configured. Add accounts in <NuxtLink to="/providers" style="color:var(--jkr-blue)">Providers</NuxtLink>.</div>

    <div v-for="c in connections" :key="c.id" class="card" style="margin-bottom:.75rem">
      <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:.5rem">
        <div>
          <span style="font-weight:600;color:var(--jkr-text)">{{ c.name }}</span>
          <span style="color:var(--jkr-sub1);font-size:.75rem;margin-left:.5rem">{{ c.provider_name }}</span>
          <span :class="`st ${c.state==='active'?'ok':c.state==='cooling_down'?'warn':'err'}`" style="margin-left:.5rem">{{ c.state }}</span>
        </div>
        <button v-if="!editingId" class="btn btn-sm" @click="startEdit(c.id)">Set Quota</button>
        <template v-else-if="editingId === c.id">
          <input v-model.number="editLimit" type="number" class="input" style="width:100px" placeholder="Limit (0=off)" />
          <input v-model.number="editWindow" type="number" class="input" style="width:100px" placeholder="Window (sec)" />
          <button class="btn btn-sm" style="background:var(--jkr-green)" @click="saveQuota(c.id)">Save</button>
          <button class="btn btn-sm" style="background:var(--jkr-crust)" @click="editingId=null">Cancel</button>
        </template>
      </div>

      <div v-if="quotas[c.id]" style="margin-top:.75rem">
        <div style="display:flex;justify-content:space-between;font-size:.75rem;color:var(--jkr-sub1);margin-bottom:.25rem">
          <span>Used: {{ quotas[c.id].used }} tokens</span>
          <span v-if="quotas[c.id].quota_limit > 0">Limit: {{ quotas[c.id].quota_limit }} · {{ pct(quotas[c.id]) }}%</span>
          <span v-else>Unlimited</span>
          <span v-if="quotas[c.id].quota_reset_at > 0">Reset in: {{ timeLeft(quotas[c.id].quota_reset_at) }}</span>
        </div>
        <div v-if="quotas[c.id].quota_limit > 0" style="height:6px;background:var(--jkr-mantle);border-radius:3px;overflow:hidden">
          <div :style="{width:pct(quotas[c.id])+'%',height:'100%',background:pct(quotas[c.id])>90?'var(--jkr-red)':pct(quotas[c.id])>70?'var(--jkr-yellow)':'var(--jkr-green)',transition:'width .3s'}"></div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.st { display:inline-block;padding:.15rem .45rem;border-radius:4px;font-size:.7rem;font-weight:600;text-transform:uppercase; }
.st.ok { background:rgba(166,227,161,.15);color:var(--jkr-green); }
.st.warn { background:rgba(249,226,175,.15);color:var(--jkr-yellow); }
.st.err { background:rgba(243,139,168,.15);color:var(--jkr-red); }
.btn-sm { padding:.25rem .6rem;font-size:.75rem; }
</style>
