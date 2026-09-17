<script setup lang="ts">
import { ref, onMounted } from 'vue'

const pairs = ref<{ pair: string; format_from: string; format_to: string }[]>([])
const selectedPair = ref('openai:anthropic')
const payload = ref('{\n  "model": "gpt-4",\n  "messages": [{"role": "user", "content": "Hello world"}],\n  "stream": false\n}')
const result = ref('')
const error = ref('')
const loading = ref(false)
const direction = ref<'forward' | 'reverse'>('forward')

async function loadPairs() {
  try {
    const r = await fetch('/api/dashboard/translator/pairs').then(x => x.json())
    pairs.value = (r as any).pairs || []
    if (pairs.value.length > 0) selectedPair.value = pairs.value[0].pair
  } catch (e) {
    error.value = 'Gagal memuat pairs: ' + (e as Error).message
  }
}

function toggleDirection() {
  const p = pairs.value.find(p => p.pair === selectedPair.value)
  if (p) {
    selectedPair.value = `${p.format_to}:${p.format_from}`
  }
}

async function preview() {
  error.value = ''
  result.value = ''
  loading.value = true
  try {
    // Validate JSON
    JSON.parse(payload.value)
    const r = await fetch('/api/dashboard/translator/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ pair: selectedPair.value, payload: payload.value }),
    })
    const data = await r.json()
    if (!r.ok) throw new Error((data as any).error || 'preview failed')
    result.value = (data as any).payload || JSON.stringify(data, null, 2)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

onMounted(loadPairs)
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">
        <span class="tag p2">P2</span> /dashboard/translator
      </h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">Debug format translation (openai ↔ anthropic)</small>
    </div>

    <div style="display:grid;grid-template-columns:1fr 1fr;gap:1rem">
      <!-- Left: input -->
      <div class="card">
        <h3 style="margin:0 0 .5rem;font-size:.9rem;color:var(--jkr-lav)">Input</h3>
        <div style="display:flex;gap:.5rem;margin-bottom:.5rem;align-items:center">
          <select v-model="selectedPair" class="select" style="flex:1">
            <option v-for="p in pairs" :key="p.pair" :value="p.pair">
              {{ p.format_from }} → {{ p.format_to }}
            </option>
          </select>
          <button class="btn ghost" @click="toggleDirection" title="Tukar arah">⇄</button>
        </div>
        <textarea v-model="payload" class="code-input" rows="12" spellcheck="false"
          placeholder='{"model":"gpt-4","messages":[...]}'></textarea>
        <div style="margin-top:.5rem;display:flex;gap:.5rem">
          <button class="btn" :disabled="loading" @click="preview">
            {{ loading ? '…' : 'Preview Translate' }}
          </button>
        </div>
        <p v-if="error" class="note" style="color:var(--jkr-red);margin-top:.5rem">{{ error }}</p>
      </div>

      <!-- Right: output -->
      <div class="card">
        <h3 style="margin:0 0 .5rem;font-size:.9rem;color:var(--jkr-lav)">Output ({{ selectedPair }})</h3>
        <pre class="code-output" v-if="result">{{ result }}</pre>
        <p v-else class="note" style="padding:2rem;text-align:center;color:var(--jkr-mut)">
          {{ loading ? 'Menerjemahkan…' : 'Hasil terjemahan akan muncul di sini' }}
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
.card { background:var(--jkr-mantle); border:1px solid var(--jkr-brd); border-radius:8px; padding:1rem; }
.code-input { width:100%; background:var(--jkr-base); color:var(--jkr-txt); border:1px solid var(--jkr-brd);
  border-radius:4px; padding:.5rem; font-family:monospace; font-size:.8rem; resize:vertical; box-sizing:border-box; }
.code-output { background:var(--jkr-base); color:var(--jkr-grn); border:1px solid var(--jkr-brd);
  border-radius:4px; padding:.5rem; font-family:monospace; font-size:.8rem; white-space:pre-wrap;
  word-break:break-all; max-height:500px; overflow-y:auto; }
.select { background:var(--jkr-base); color:var(--jkr-txt); border:1px solid var(--jkr-brd);
  border-radius:4px; padding:.3rem .5rem; font-size:.8rem; }
</style>
