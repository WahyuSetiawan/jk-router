<script setup lang="ts">
import { ref, onMounted } from 'vue'

const tools = ref<any[]>([])
const msg = ref('')
const msgType = ref<'ok' | 'err'>('ok')

async function load() {
  try {
    const r = await fetch('/api/dashboard/cli-tools').then(x => x.json())
    tools.value = (r as any).tools || []
  } catch (e) {
    setMsg('Gagal memuat: ' + (e as Error).message, 'err')
  }
}

function setMsg(t: string, type: 'ok' | 'err') {
  msg.value = t
  msgType.value = type
  setTimeout(() => { msg.value = '' }, 3000)
}

async function apply(tool: any) {
  const lines = Object.entries(tool.config).map(([k, v]) => `  ${k} = "${v}"`)
  const config = `# JKRouter auto-configure — ${tool.name}\n${lines.join('\n')}\n`
  try {
    await navigator.clipboard.writeText(config)
    setMsg(`Konfigurasi ${tool.name} disalin ke clipboard`, 'ok')
  } catch {
    setMsg('Clipboard tidak tersedia', 'err')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">
        <span class="tag p2">P2</span> /dashboard/cli-tools
      </h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">Auto-configure CLI tools ke JKRouter endpoint</small>
    </div>

    <div v-if="msg" :style="{color:msgType==='ok'?'var(--jkr-grn)':'var(--jkr-red)',marginBottom:'1rem'}" class="note">
      {{ msg }}
    </div>

    <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:1rem">
      <div v-for="t in tools" :key="t.id" class="card tool-card">
        <div style="display:flex;justify-content:space-between;align-items:start">
          <div>
            <h3 style="margin:0;color:var(--jkr-lav);font-size:.95rem">{{ t.name }}</h3>
            <p style="margin:.2rem 0 0;font-size:.75rem;color:var(--jkr-mut)">{{ t.description }}</p>
          </div>
          <a :href="t.docs" target="_blank" class="chip" style="text-decoration:none;font-size:.7rem">docs ↗</a>
        </div>
        <div class="kv" style="margin:.6rem 0">
          <dt>endpoint</dt><dd><code>{{ t.config.endpoint }}</code></dd>
          <dt>model</dt><dd><code>{{ t.config.model }}</code></dd>
          <dt>api_key</dt><dd><code>{{ t.config.api_key?.slice(0,6) }}…</code></dd>
        </div>
        <button class="btn" style="width:100%" @click="apply(t)">📋 Salin Konfigurasi</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
.tool-card { padding:1rem; }
.kv { display:grid; grid-template-columns:80px 1fr; gap:.2rem .5rem; font-size:.75rem; }
.kv dt { color:var(--jkr-mut); text-align:right; }
.kv dd { margin:0; font-family:monospace; }
</style>
