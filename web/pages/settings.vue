<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, onMounted } from 'vue'
const settings = ref({ port: 20128, bind: '127.0.0.1', dataDir: '~/.jkrouter' })
const saving = ref(false)
const msg = ref('')
const msgType = ref<'ok' | 'err'>('ok')
const rtkStatus = ref<{ enabled: boolean; windowSec: number; saved: number }>({ enabled: true, windowSec: 86400, saved: 0 })
// Per-filter toggles: ['caveman','ponytail','headroom','system-inject']
const rtkFilters = ref<string[]>([])
const rtkFilterLabels: Record<string,string> = {
  caveman: 'caveman',
  ponytail: 'ponytail',
  headroom: 'headroom',
  'system-inject': 'system-inject',
}
const rtkFilterDesc: Record<string,string> = {
  caveman: 'strip verbose prefixes',
  ponytail: 'strip comments/whitespace',
  headroom: 'truncate long msgs',
  'system-inject': 'inject concise hint',
}
function toggleRtkFilter(name: string) {
  const i = rtkFilters.value.indexOf(name)
  if (i >= 0) rtkFilters.value.splice(i, 1)
  else rtkFilters.value.push(name)
}
const walInterval = ref(300)   // seconds
const logBuffer = ref(4096)     // entries
const cooldown429 = ref(60)     // seconds
const circuitBreaker = ref(5)   // consecutive failures
const backups = ref<{filename:string,size:number,mod_time:string,sha256:string}[]>([])
const restoring = ref(false)
const restoringFile = ref('')

// Capacity adapter: { vision: "model-id", pdf: "", ... } or null
const capAdapter = ref<Record<string, string>>({})

async function load() {
  try {
    const r = await fetch('/api/dashboard/settings').then(x => x.json())
    if (r.settings) settings.value = { ...settings.value, ...r.settings }
    if (r.settings?.capacityAdapter) {
      try { capAdapter.value = JSON.parse(r.settings.capacityAdapter) } catch { /* ignore */ }
    }
    // Load RTK status from settings
    if (r.settings?.rtkEnabled !== undefined) {
      rtkStatus.value.enabled = r.settings.rtkEnabled
    }
    if (r.settings?.rtkFilters) {
      try { rtkFilters.value = JSON.parse(r.settings.rtkFilters) } catch { /* ignore */ }
    }
    if (r.settings?.rtkWindowSec) {
      rtkStatus.value.windowSec = r.settings.rtkWindowSec
    }
    if (r.settings?.wal_interval) walInterval.value = parseInt(r.settings.wal_interval) || 300
    if (r.settings?.log_buffer) logBuffer.value = parseInt(r.settings.log_buffer) || 4096
    if (r.settings?.cooldown_429) cooldown429.value = parseInt(r.settings.cooldown_429) || 60
    loadBackups()
  } catch { /* use defaults */ }
}
async function loadBackups() {
  try {
    const r = await fetch('/api/dashboard/backups').then(x => x.json())
    backups.value = r.backups || []
  } catch { /* ignore */ }
}
async function doRestore(b: typeof backups.value[0]) {
  if (!confirm(`Restore from ${b.filename}? This will replace the current database.`)) return
  restoring.value = true
  restoringFile.value = b.filename
  msg.value = ''
  try {
    await fetch('/api/dashboard/backups/restore', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ filename: b.filename, sha256: b.sha256 })
    })
    msg.value = `Restored from ${b.filename}`
    msgType.value = 'ok'
    await loadBackups()
  } catch (e: any) {
    msg.value = 'Restore failed: ' + (e.message || 'unknown')
    msgType.value = 'err'
  } finally {
    restoring.value = false
    restoringFile.value = ''
  }
}
async function save() {
  saving.value = true
  msg.value = ''
  try {
    const payload: Record<string, unknown> = { ...settings.value }
    if (Object.keys(capAdapter.value).length > 0) {
      payload.capacityAdapter = JSON.stringify(capAdapter.value)
    } else {
      payload.capacityAdapter = ''
    }
    payload.walInterval = String(walInterval.value)
    payload.logBuffer = String(logBuffer.value)
    payload.cooldown429 = String(cooldown429.value)
    payload.circuitBreaker = String(circuitBreaker.value)
    payload.rtkFilters = JSON.stringify(rtkFilters.value)
    await fetch('/api/dashboard/settings', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    msg.value = 'Settings saved'
    msgType.value = 'ok'
  } catch (e: any) {
    msg.value = 'Failed: ' + (e.message || 'unknown error')
    msgType.value = 'err'
  } finally {
    saving.value = false
  }
}
async function exportConfig() {
  try {
    const r = await fetch('/api/dashboard/config/export').then(x => x.json())
    const blob = new Blob([JSON.stringify(r.config, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'jkrouter-config.json'
    a.click()
    msg.value = 'Config exported'
    msgType.value = 'ok'
  } catch (e: any) {
    msg.value = 'Export failed: ' + (e.message || 'unknown')
    msgType.value = 'err'
  }
}
const importInput = ref<HTMLInputElement | null>(null)
async function importConfig(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async () => {
    try {
      const data = JSON.parse(reader.result as string)
      await fetch('/api/dashboard/config/import', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
      msg.value = 'Config imported successfully'
      msgType.value = 'ok'
      await load()
    } catch {
      msg.value = 'Invalid config file'
      msgType.value = 'err'
    }
  }
  reader.readAsText(file)
}

const capModels = ['oc/mimo-v2.5-free', 'openrouter/anthropic/claude-3-haiku', 'openrouter/google/gemini-2.0-flash-lite']
const capKeys = ['vision', 'pdf', 'audio_input', 'video_input'] as const

function setCap(key: string, model: string) {
  if (model === 'off') {
    const { [key]: _, ...rest } = capAdapter.value
    capAdapter.value = rest
  } else {
    capAdapter.value = { ...capAdapter.value, [key]: model }
  }
}

function formatWindow(sec: number): string {
  if (sec >= 86400) return `${sec / 86400} hari`
  if (sec >= 3600) return `${sec / 3600} jam`
  return `${sec / 60} menit`
}

onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /dashboard/settings</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">System config</small>
    </div>
    <div v-if="msg" :style="{color: msgType==='ok' ? 'var(--jkr-grn)' : 'var(--jkr-red)', marginBottom: '1rem'}" class="note">{{ msg }}</div>

    <!-- Server -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Server</h3>
      <div class="kv" style="margin-bottom:.5rem">
        <dt>Port</dt><dd><input v-model.number="settings.port" type="number" class="input" style="width:100px" min="1000" max="65535" /></dd>
        <dt>Bind</dt><dd>
          <select v-model="settings.bind" class="select" style="width:160px">
            <option value="127.0.0.1">127.0.0.1</option>
            <option value="0.0.0.0">0.0.0.0</option>
          </select>
        </dd>
        <dt>DATA_DIR</dt><dd><input v-model="settings.dataDir" class="input" style="width:240px" /></dd>
      </div>
      <button class="btn" @click="save" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
    </div>

    <!-- t('settings.backup') -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Backup & Restore (§4.3)</h3>
      <div class="kv">
        <dt>Auto snapshot</dt><dd>sebelum migrasi DB · retensi 10 · <span style="color:var(--jkr-grn)">{{ backups.length ? 'latest: ' + new Date(backups[0].mod_time).toLocaleString() : 'belum ada' }}</span></dd>
        <dt>Restore</dt><dd>
          <select v-if="backups.length" class="select" style="width:200px;display:inline-block">
            <option value="">Pilih snapshot…</option>
            <option v-for="b in backups" :key="b.filename" :value="b.filename">{{ b.filename }}</option>
          </select>
          <span v-else class="note">tidak ada snapshot</span>
          <button class="btn ghost btn-sm" style="margin-left:.3rem" :disabled="!backups.length || restoring" @click="doRestore(backups[0])">restore</button>
          <span class="note" style="font-size:.7rem;margin-left:.3rem">(verifikasi SHA-256)</span>
        </dd>
        <dt>Export config</dt><dd>
          <button class="btn ghost btn-sm" @click="exportConfig">export</button>
          <label class="btn ghost btn-sm" style="cursor:pointer;display:inline-block;margin-left:.3rem">
            import
            <input ref="importInput" type="file" accept=".json" class="hidden" @change="importConfig" style="display:none" />
          </label>
        </dd>
      </div>
      <p class="note">Upgrade di atas 9Router (yang hanya punya safety backup pre-migration tanpa jalur restore).</p>
      <div v-if="backups.length" style="margin-top:.6rem;border-top:1px solid var(--jkr-brd);padding-top:.5rem">
        <p class="note" style="font-size:.8rem;margin-bottom:.3rem">DB snapshots (auto before each migration, max 10)</p>
        <table style="width:100%;font-size:.8rem;border-collapse:collapse">
          <thead>
            <tr style="color:var(--jkr-fg-muted)"><th style="text-align:left;padding:.2rem 0">File</th><th>Size</th><th>Time</th><th></th></tr>
          </thead>
          <tbody>
          <tr v-for="b in backups" :key="b.filename" style="border-top:1px solid var(--jkr-brd)">
            <td style="padding:.25rem 0"><code style="font-size:.75rem">{{ b.filename }}</code></td>
            <td class="note">{{ (b.size/1024).toFixed(0) }} KB</td>
            <td class="note">{{ new Date(b.mod_time).toLocaleString() }}</td>
            <td><button class="btn ghost btn-sm" :disabled="restoring" @click="doRestore(b)" style="font-size:.7rem">restore</button></td>
          </tr>
          </tbody>
        </table>
        <p v-if="restoring" class="note" style="font-size:.75rem;margin-top:.3rem;color:var(--jkr-yellow)">Restoring {{ restoringFile }}… do not restart server.</p>
      </div>
    </div>

    <!-- Resilience Defaults -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Resilience defaults (§4.1)</h3>
      <div class="kv">
        <dt>cooldown fallback 429</dt><dd>
          <input type="number" class="input" style="width:80px" :value="cooldown429" @change="cooldown429 = parseInt(($event.target as HTMLInputElement).value) || 60" />
          <span class="note" style="font-size:.7rem;margin-left:.3rem">saat header Retry-After absen</span>
        </dd>
        <dt>WAL checkpoint interval</dt><dd>
          <input type="number" class="input" style="width:80px" :value="walInterval" @change="walInterval = parseInt(($event.target as HTMLInputElement).value) || 300" />
          <span class="note" style="font-size:.7rem;margin-left:.3rem">detik</span>
        </dd>
        <dt>log channel size</dt><dd>
          <input type="number" class="input" style="width:80px" :value="logBuffer" @change="logBuffer = parseInt(($event.target as HTMLInputElement).value) || 4096" />
          <span class="note" style="font-size:.7rem;margin-left:.3rem">(drop+counter saat full)</span>
        </dd>
        <dt>Circuit Breaker</dt><dd>
          <input type="number" class="input" style="width:80px" :value="circuitBreaker" @change="circuitBreaker = parseInt(($event.target as HTMLInputElement).value) || 5" />
          <span class="note" style="font-size:.7rem;margin-left:.3rem">konsekutif gagal → disable akun</span>
        </dd>
      </div>
    </div>

    <!-- RTK Token Saver -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">RTK token saver <span class="tag p2">P2</span></h3>
      <p class="note" style="margin-bottom:.5rem">Filter request bodies before sending to reduce token usage.</p>
      <div class="kv">
        <dt v-for="name in ['caveman','ponytail','headroom','system-inject']" :key="name">
          {{ rtkFilterLabels[name] || name }}
          <span class="note" style="margin-left:.3rem;font-size:.65rem">{{ rtkFilterDesc[name] }}</span>
        </dt>
        <dd v-for="name in ['caveman','ponytail','headroom','system-inject']" :key="name">
          <label style="display:flex;align-items:center;gap:.4rem;cursor:pointer">
            <input type="checkbox" :checked="rtkFilters.includes(name)" @change="toggleRtkFilter(name)" />
            <span class="note" style="font-size:.75rem">{{ rtkFilters.includes(name) ? 'on' : 'off' }}</span>
          </label>
        </dd>
      </div>
    </div>

    <!-- t('settings.capacity_adapter') -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Capacity Adapter (§4.2) <span class="tag p1">P1</span></h3>
      <div class="kv">
        <dt>vision</dt><dd>
          <select v-model="capAdapter['vision']" class="select" style="width:220px;display:inline-block">
            <option value="off">off</option>
            <option v-for="m in capModels" :value="m">{{ m }}</option>
          </select>
          <span class="note" style="font-size:.7rem;margin-left:.3rem">strategy: fallback · pool: default oc/mimo-v2.5-free</span>
        </dd>
        <dt>pdf</dt><dd>
          <select v-model="capAdapter['pdf']" class="select" style="width:220px;display:inline-block">
            <option value="off">off</option>
            <option v-for="m in capModels" :value="m">{{ m }}</option>
          </select>
        </dd>
        <dt>audio input</dt><dd>
          <select v-model="capAdapter['audio_input']" class="select" style="width:220px;display:inline-block">
            <option value="off">off</option>
            <option v-for="m in capModels" :value="m">{{ m }}</option>
          </select>
        </dd>
        <dt>video input</dt><dd>
          <select v-model="capAdapter['video_input']" class="select" style="width:220px;display:inline-block">
            <option value="off">off</option>
            <option v-for="m in capModels" :value="m">{{ m }}</option>
          </select>
        </dd>
      </div>
      <p class="note">On saat combo user tidak punya model yang support modality itu, pool adapter di-prepend ke urutan coba (autoSwitch).</p>
    </div>
  </div>
</template>
<style scoped>
.hidden { display: none; }
.kv { display: flex; flex-direction: column; gap: .4rem; }
.btn-sm { font-size:.7rem;padding:.15rem .4rem; }
</style>
