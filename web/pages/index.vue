<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
const stats = ref({ requests: 0, tokensIn: 0, cost: 0, p95Latency: 0 })
const recent = ref<any[]>([])
const loading = ref(true)

async function load() {
  try {
    const [u, c] = await Promise.all([
      fetch('/api/dashboard/usage?limit=20').then(r => r.json()),
      fetch('/api/dashboard/usage/stats').then(r => r.json()).catch(() => ({ requests: 0, tokens_in: 0, cost: 0, avg_ms: 0 })),
    ])
    const list = u.usage || []
    recent.value = list
    if (c.requests !== undefined) {
      stats.value = { requests: c.requests, tokensIn: c.tokens_in, cost: c.cost, p95Latency: c.avg_ms }
    } else {
      stats.value = {
        requests: list.length,
        tokensIn: list.reduce((s: number, x: any) => s + (x.tok_in || 0), 0),
        cost: list.reduce((s: number, x: any) => s + (x.cost_usd || 0), 0).toFixed(4),
        p95Latency: 0,
      }
    }
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> {{ t('dashboard.title') }}</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">{{ t('dashboard.subtitle') }}</small>
    </div>
    <div v-if="loading" class="note" style="padding:2rem;text-align:center">{{ t('dashboard.loading') }}</div>
    <template v-else>
      <div class="grid4" style="margin-bottom:1rem">
        <div class="card stat"><div class="n">{{ stats.requests }}</div><div class="l">{{ t('dashboard.requests') }}</div></div>
        <div class="card stat"><div class="n">{{ stats.tokensIn.toLocaleString() }}</div><div class="l">{{ t('dashboard.tokens') }}</div></div>
        <div class="card stat"><div class="n">${{ typeof stats.cost === 'number' ? stats.cost.toFixed(4) : stats.cost }}</div><div class="l">{{ t('dashboard.cost') }}</div></div>
        <div class="card stat"><div class="n">{{ stats.p95Latency }}ms</div><div class="l">{{ t('dashboard.latency') }}</div></div>
      </div>
      <div class="card" style="margin-top:.5rem">
        <h3>{{ t('dashboard.recent') }}</h3>
        <table class="table">
          <thead><tr>
            <th>{{ t('usage.date') }}</th><th>Model</th><th>Provider</th><th>Status</th><th>Latency</th><th>Tokens</th>
          </tr></thead>
          <tbody>
            <tr v-for="r in recent" :key="r.request_id">
              <td style="color:var(--jkr-mut);font-size:.75rem">{{ r.ts?.slice(11,19) }}</td>
              <td class="font-mono" style="font-size:.75rem">{{ r.model }}</td>
              <td>{{ r.provider }}</td>
              <td><span :class="r.status==='success'?'st ok':'st err'">{{ r.status }}</span></td>
              <td>{{ r.latency_ms }}ms</td>
              <td class="font-mono" style="font-size:.75rem">{{ r.tok_in ?? 0 }}+{{ r.tok_out ?? 0 }}</td>
            </tr>
            <tr v-if="recent.length===0"><td colspan="6" class="note" style="text-align:center;padding:1rem">{{ t('dashboard.loading') }}</td></tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>
