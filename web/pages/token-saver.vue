<template>
  <div>
    <h2 style="color:var(--jkr-lav);margin-bottom:1rem">{{ t('token_saver.title') }}</h2>
    <p class="note" style="margin-bottom:1rem">{{ t('token_saver.desc') }}</p>

    <!-- Filters -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">{{ t('token_saver.filters') }}</h3>
      <div class="kv">
        <div v-for="f in filters" :key="f.id" class="filter-row">
          <div>
            <strong style="color:var(--jkr-txt)">{{ f.label }}</strong>
            <span class="note" style="margin-left:.3rem">{{ f.desc }}</span>
          </div>
          <label style="display:flex;align-items:center;gap:.3rem;cursor:pointer">
            <input type="checkbox" :checked="enabled.includes(f.id)" @change="toggle(f.id)" />
            <span class="note">{{ enabled.includes(f.id) ? 'on' : 'off' }}</span>
          </label>
        </div>
      </div>
    </div>

    <!-- Headroom config -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">{{ t('token_saver.headroom_cfg') }}</h3>
      <div class="kv" style="flex-direction:row;align-items:center;gap:.5rem;flex-wrap:wrap">
        <dt>{{ t('token_saver.max_tokens_per_msg') }}</dt>
        <dd>
          <input type="number" class="input" style="width:80px" v-model.number="headroomTokens" min="100" max="8000" step="100" />
          <span class="note" style="margin-left:.3rem">{{ t('token_saver.tokens') }}</span>
        </dd>
      </div>
    </div>

    <!-- Save -->
    <div class="card" style="margin-bottom:.8rem">
      <button class="btn" :disabled="saving" @click="saveAll">{{ t('token_saver.save') }}</button>
      <span class="note" style="margin-left:.5rem">{{ saveMsg }}</span>
    </div>

    <!-- Status -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">{{ t('token_saver.status') }}</h3>
      <div class="kv">
        <dt>{{ t('token_saver.active_filters') }}</dt>
        <dd><span class="note">{{ activeFilters.length > 0 ? activeFilters.join(', ') : '—' }}</span></dd>
        <dt>{{ t('token_saver.last_test') }}</dt>
        <dd>
          <button class="btn btn-sm" :disabled="testing" @click="runTest">{{ t('token_saver.test') }}</button>
          <span class="note" style="margin-left:.5rem">{{ testResult }}</span>
        </dd>
      </div>
    </div>

    <!-- Savings log -->
    <div class="card">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">{{ t('token_saver.log') }}</h3>
      <pre class="note" style="max-height:200px;overflow-y:auto;font-size:.7rem">{{ log }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
import { ref, onMounted, computed } from 'vue'
const { t } = useI18n()

const FILTERS = [
  { id: 'caveman', label: 'caveman', desc: 'strip verbose prefixes (please be concise…)' },
  { id: 'ponytail', label: 'ponytail', desc: 'strip comments, collapse whitespace' },
  { id: 'headroom', label: 'headroom', desc: 'truncate long messages' },
  { id: 'system-inject', label: 'system-inject', desc: 'prepend concise behavior hint' },
] as const

const enabled = ref<string[]>([])
const headroomTokens = ref(2000)
const testing = ref(false)
const testResult = ref('')
const log = ref('')
const saving = ref(false)
const saveMsg = ref('')

async function load() {
  try {
    const r = await fetch('/api/dashboard/settings').then(x => x.json())
    if (r.settings?.rtkFilters) {
      try { enabled.value = JSON.parse(r.settings.rtkFilters) } catch { /* ignore */ }
    }
    if (r.settings?.headroomTokens) {
      try { headroomTokens.value = parseInt(r.settings.headroomTokens) || 2000 } catch { /* ignore */ }
    }
  } catch { /* use defaults */ }
}

async function saveAll() {
  saving.value = true
  saveMsg.value = ''
  try {
    await fetch('/api/dashboard/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ 
        rtkFilters: JSON.stringify(enabled.value),
        headroomTokens: headroomTokens.value,
      }),
    })
    saveMsg.value = '✓ tersimpan'
  } catch (e) {
    saveMsg.value = '✗ gagal'
  } finally {
    saving.value = false
  }
}

function toggle(id: string) {
  const i = enabled.value.indexOf(id)
  if (i >= 0) enabled.value.splice(i, 1)
  else enabled.value.push(id)
}

const activeFilters = computed(() => enabled.value.filter(id => id !== 'headroom' || headroomTokens.value > 0))

async function runTest() {
  testing.value = true
  testResult.value = ''
  try {
    const before = JSON.stringify({ messages: [{ role: 'user', content: 'Please respond as concisely as possible. Hello world, tell me about Go programming language features and history.' }] })
    const payload = { ...JSON.parse(before), rtkFilters: enabled.value, headroomTokens: headroomTokens.value }
    const r = await fetch('/api/dashboard/translator/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    }).then(x => x.json())
    const after = r.filtered || before
    const saved = before.length - after.length
    const pct = before.length > 0 ? Math.round((saved / before.length) * 100) : 0
    const ts = new Date().toLocaleTimeString()
    log.value = `[${ts}] filters=${enabled.value.join(',') || 'none'} delta=${saved}B (${pct}%) → ${after.length}B\n` + log.value
    testResult.value = `${saved}B saved (${pct}%)`
  } catch (e: any) {
    testResult.value = 'error: ' + (e.message || 'unknown')
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.kv { display: flex; flex-direction: column; gap: .5rem; }
.filter-row { display: flex; justify-content: space-between; align-items: center; padding: .3rem 0; border-bottom: 1px solid var(--jkr-s1, #313244); }
.filter-row:last-child { border-bottom: none; }
.btn-sm { font-size: .7rem; padding: .15rem .4rem; }
</style>
