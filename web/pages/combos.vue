<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, onMounted } from 'vue'

const combos = ref<any[]>([])
const providers = ref<any[]>([])
const showAdd = ref(false)
const form = ref({ name: '', description: '', model_ids: '', strategy: 'fallback' })
const search = ref('')
const refreshStatus = ref<string>('')

// Capability-aware routing state
const capabilities = ref<string[]>([])
const showCaps = ref(false)

async function load() {
  const [c, p] = await Promise.all([
    fetch('/api/dashboard/combos').then(r => r.json()),
    fetch('/api/dashboard/providers').then(r => r.json()),
  ])
  combos.value = c.combos || []
  providers.value = p.providers || []
}

function openAdd() {
  form.value = { name: '', description: '', model_ids: '', strategy: 'fallback' }
  showAdd.value = true
}

async function create() {
  if (!form.value.name) return
  await fetch('/api/dashboard/combos', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: form.value.name,
      description: form.value.description,
      model_ids: form.value.model_ids,
      strategy: form.value.strategy,
    })
  })
  form.value = { name: '', description: '', model_ids: '', strategy: 'fallback' }
  showAdd.value = false
  await load()
}

async function remove(id: number) {
  if (!confirm(t('combos.delete') + '?')) return
  await fetch(`/api/dashboard/combos/${id}`, { method: 'DELETE' })
  await load()
}

async function refreshModels() {
  refreshStatus.value = 'Menyinkronisasi…'
  try {
    const resp = await fetch('/api/dashboard/providers/refresh-all', { method: 'POST' })
    const data = await resp.json()
    refreshStatus.value = data.ok ? '✅ Model terbaru dimuat' : `❌ ${data.error || 'gagal'}`
  } catch {
    refreshStatus.value = '❌ Koneksi gagal'
  }
  setTimeout(() => { refreshStatus.value = '' }, 5000)
}

function getModelList(id: string): string[] {
  try {
    return JSON.parse(id)
  } catch {
    return id.split(',').map(s => s.trim())
  }
}

function getProviderName(pid: string): string {
  const found = providers.value.find(p => p.id === pid)
  return found ? found.name : pid
}

function getAccountForModel(combo: any, model: string): any {
  // Search through all providers' accounts to find which one handles this model
  for (const p of providers.value) {
    for (const a of p.accounts || []) {
      // Check if this account's provider supports the model
      if (combo.model_ids?.includes(model) || getModelList(combo.model_ids).includes(model)) {
        return { provider: p.name, account: a.label }
      }
    }
  }
  return null
}

function stateClass(s: string) {
  return s === 'active' ? 'st active' : s === 'cooling_down' ? 'st cooling' : 'st disabled'
}

function stepNumber(i: number) {
  return i + 1
}

const filtered = () => {
  if (!search.value) return combos.value
  const s = search.value.toLowerCase()
  return combos.value.filter(c =>
    c.name.toLowerCase().includes(s) || c.description?.toLowerCase().includes(s)
  )
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /dashboard/combos</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">{{ t('combos.desc') }}</small>
      <div style="display:flex;gap:.5rem;align-items:center;flex-wrap:wrap">
        <input v-model="search" class="search" style="flex:1;max-width:200px" :placeholder="t('providers.search')" />
        <button class="btn ghost" :disabled="!!refreshStatus" @click="refreshModels">
          ↻ Refresh Models
        </button>
        <span v-if="refreshStatus" class="note" style="font-size:.75rem">{{ refreshStatus }}</span>
        <button class="btn" @click="openAdd">{{ t('combos.add') }}</button>
      </div>
    </div>

    <!-- Capability-Aware Routing Section -->
    <div class="card" style="margin-bottom:1rem">
      <div style="display:flex;justify-content:space-between;align-items:center;cursor:pointer" @click="showCaps=!showCaps">
        <h3 style="color:var(--jkr-lav);font-size:.95rem;margin:0">
          Capability-Aware Routing
          <span class="note" style="font-size:.7rem;margin-left:.5rem">Klik untuk detail</span>
        </h3>
        <span style="color:var(--jkr-text-muted);font-size:.8rem">{{ showCaps ? '▼' : '▶' }}</span>
      </div>
      <div v-if="showCaps" style="margin-top:.8rem">
        <p class="note" style="font-size:.8rem;margin-bottom:.5rem">
          Router akan memprioritaskan model dengan capability yang dibutuhkan sebelum fallback ke model lain.
        </p>
        <div style="display:flex;gap:.4rem;flex-wrap:wrap;margin-bottom:.5rem">
          <span class="chip" style="background:#3d59a4;color:#cad3f5;font-size:.7rem">vision</span>
          <span class="chip" style="background:#29a77c;color:#1e2030;font-size:.7rem">pdf</span>
          <span class="chip" style="background:#ef8b61;color:#1e2030;font-size:.7rem">audio</span>
          <span class="chip" style="background:#c6a0f6;color:#1e2030;font-size:.7rem">video</span>
        </div>
        <div class="note" style="font-size:.75rem">
          {{ t('combos.fallback_note') }} yang cocok akan tetap digunakan sebagai fallback terakhir.
          Pengurutan otomatis dilakukan oleh engine berdasarkan kebutuhan request.
        </div>
      </div>
    </div>

    <!-- Add Combo Modal -->
    <div v-if="showAdd" class="modal-overlay" @click.self="showAdd=false">
      <div class="modal">
        <h3>{{ t('combos.add') }}</h3>
        <div class="kv" style="margin-bottom:.8rem">
          <dt>Nama Combo</dt><dd><input v-model="form.name" class="input" placeholder="e.g. gpt+claude-fallback" /></dd>
          <dt>Deskripsi</dt><dd><input v-model="form.description" class="input" placeholder="e.g. GPT-4o → Claude Sonnet fallback" /></dd>
          <dt>Model List (JSON array)</dt><dd>
            <input v-model="form.model_ids" class="input" placeholder='["gpt-4o","claude-sonnet-4"]' />
          </dd>
          <dt>Strategy</dt><dd>
            <select v-model="form.strategy" class="select">
              <option value="fallback">Fallback (sequential)</option>
              <option value="parallel">Parallel (concurrent)</option>
              <option value="random">Random</option>
            </select>
          </dd>
        </div>
        <div style="display:flex;gap:.5rem">
          <button class="btn" @click="create">{{ t('providers.save') }}</button>
          <button class="btn ghost" @click="showAdd=false">{{ t('providers.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- Combo Cards -->
    <template v-for="c in filtered()" :key="c.id">
      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:start;margin-bottom:.5rem">
          <div>
            <h3 style="color:var(--jkr-lav);margin:0;font-size:1rem">{{ c.name }}</h3>
            <p class="note" style="font-size:.75rem;margin:.2rem 0 0">{{ c.description || '—' }}</p>
          </div>
          <div style="display:flex;gap:.3rem;align-items:center">
            <span class="chip" :style="{background:c.strategy==='parallel'?'#3d59a4':c.strategy==='random'?'#ef8b61':'var(--jkr-surface2)'}">{{ c.strategy }}</span>
            <button class="btn ghost" style="padding:.15rem .3rem;font-size:.7rem;color:#b45151" @click="remove(c.id)">🗑</button>
          </div>
        </div>

        <!-- Step-by-step fallback chain -->
        <div style="margin-top:.5rem">
          <div class="note" style="font-size:.75rem;margin-bottom:.4rem">Fallback Chain:</div>
          <div style="display:flex;flex-direction:column;gap:.3rem">
            <div v-for="(model, i) in getModelList(c.model_ids)" :key="i" style="display:flex;align-items:center;gap:.5rem">
              <!-- Step number -->
              <div class="step-num">{{ stepNumber(i) }}</div>
              <!-- Model badge -->
              <span class="chip" style="font-size:.75rem;flex:1">{{ model }}</span>
              <!-- Provider + Account -->
              <span class="note" style="font-size:.7rem">
                <template v-for="p in providers" :key="p.id">
                  <template v-for="a in (p.accounts || [])" :key="a.id">
                    <span v-if="c.model_ids?.includes(model) || getModelList(c.model_ids).includes(model)" class="st active" style="font-size:.65rem;margin-right:.3rem">
                      {{ p.name }} · {{ a.label }}
                    </span>
                  </template>
                </template>
              </span>
              <!-- Arrow to next -->
              <span v-if="i < getModelList(c.model_ids).length - 1" style="color:var(--jkr-text-muted);font-size:.7rem">→</span>
            </div>
          </div>
        </div>

        <!-- Stats -->
        <div style="display:flex;gap:1rem;margin-top:.6rem;padding-top:.5rem;border-top:1px solid var(--jkr-surface2)">
          <span class="note" style="font-size:.7rem">{{ getModelList(c.model_ids).length }} model</span>
          <span class="note" style="font-size:.7rem">{{ c.created_at ? new Date(c.created_at * 1000).toLocaleDateString('id-ID') : '—' }}</span>
        </div>
      </div>
    </template>

    <div v-if="filtered().length===0 && !showAdd" class="note" style="text-align:center;padding:2rem">
      {{ t('combos.none') }}
    </div>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; flex-wrap:wrap; gap:.5rem; }
.step-num {
  width:22px;height:22px;border-radius:50%;background:var(--jkr-surface2);
  display:flex;align-items:center;justify-content:center;
  font-size:.65rem;color:var(--jkr-text-muted);flex-shrink:0;
}
</style>
