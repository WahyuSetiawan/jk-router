<script setup lang="ts">
import { ref, onMounted } from 'vue'
const settings = ref({ port: 20128, bind: '127.0.0.1', dataDir: '~/.jkrouter' })
const saving = ref(false)
const msg = ref('')
const msgType = ref<'ok' | 'err'>('ok')
const rtkStatus = ref<{ enabled: boolean; windowSec: number; saved: number }>({ enabled: true, windowSec: 86400, saved: 0 })

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
    if (r.settings?.rtkWindowSec) {
      rtkStatus.value.windowSec = r.settings.rtkWindowSec
    }
  } catch { /* use defaults */ }
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
    <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin-bottom:1rem">Settings</h2>
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

    <!-- Backup & Restore -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Backup & Restore</h3>
      <div class="kv">
        <dt>Export config</dt><dd><button class="btn ghost btn-sm" @click="exportConfig">Export JSON</button></dd>
        <dt>Import config</dt><dd>
          <label class="btn ghost btn-sm" style="cursor:pointer;display:inline-block">
            Import JSON
            <input ref="importInput" type="file" accept=".json" class="hidden" @change="importConfig" style="display:none" />
          </label>
        </dd>
      </div>
      <p class="note" style="font-size:.75rem;margin-top:.3rem">Export/import portable config (providers, combos, pools, keys). Does not include usage logs.</p>
    </div>

    <!-- Resilience Defaults -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Resilience Defaults</h3>
      <div class="kv">
        <dt>Cooldown (429)</dt><dd class="note" style="font-size:.8rem">60s default when Retry-After header absent</dd>
        <dt>Circuit Breaker</dt><dd class="note" style="font-size:.8rem">Auto-disable account after 5 consecutive failures</dd>
        <dt>WAL checkpoint</dt><dd class="note" style="font-size:.8rem">5m interval to flush SQLite WAL</dd>
        <dt>Log channel</dt><dd class="note" style="font-size:.8rem">4096 buffer; drops oldest + increments counter when full</dd>
        <dt>Stream guard</dt><dd class="note" style="font-size:.8rem">Atomic bool prevents concurrent writes during streaming</dd>
      </div>
    </div>

    <!-- RTK Token Saver -->
    <div class="card" style="margin-bottom:.8rem">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.5rem">
        <h3 style="color:var(--jkr-lav);font-size:.95rem;margin:0">RTK Token Saver</h3>
        <label style="display:flex;align-items:center;gap:.4rem;cursor:pointer">
          <input type="checkbox" :checked="rtkStatus.enabled" @change="rtkStatus.enabled = ($event.target as HTMLInputElement).checked" />
          <span class="note" style="font-size:.75rem">{{ rtkStatus.enabled ? 'Aktif' : 'Nonaktif' }}</span>
        </label>
      </div>
      <div class="kv">
        <dt>Status</dt><dd>
          <span :class="rtkStatus.enabled ? 'st active' : 'st disabled'" style="font-size:.75rem">
            {{ rtkStatus.enabled ? 'Tracking enabled' : 'Disabled' }}
          </span>
        </dd>
        <dt>Sliding Window</dt><dd class="note" style="font-size:.8rem">{{ formatWindow(rtkStatus.windowSec) }} (default 24h)</dd>
        <dt>Tokens Tracked</dt><dd class="note" style="font-size:.8rem">{{ rtkStatus.saved }} akun dengan token tersimpan</dd>
      </div>
      <p class="note" style="font-size:.7rem;margin-top:.3rem">
        Setiap request sukses menyimpan token ke store. Saat quota terlewati, akun otomatis dinonaktifkan
        sampai window berikutnya. Mencegah pemborosan cost pada akun yang sudah limit.
      </p>
    </div>

    <!-- Capacity Adapter -->
    <div class="card" style="margin-bottom:.8rem">
      <h3 style="color:var(--jkr-lav);font-size:.95rem">Capacity Adapter</h3>
      <p class="note" style="font-size:.8rem;margin-bottom:.5rem">
        Fallback models when combo lacks required capability. Auto-applied per request.
      </p>
      <div class="kv" style="flex-direction:column;gap:.4rem">
        <div v-for="k in capKeys" :key="k" style="display:flex;align-items:center;gap:1rem">
          <dt style="min-width:120px;text-transform:capitalize;font-size:.8rem">{{ k.replace('_',' ') }}</dt>
          <dd style="display:flex;align-items:center;gap:.5rem">
            <label style="display:flex;align-items:center;gap:.3rem;cursor:pointer">
              <input type="radio" :name="k" value="off" :checked="!capAdapter[k]" @change="setCap(k,'off')" />
              <span class="note" style="font-size:.75rem">off</span>
            </label>
            <select v-model="capAdapter[k]" class="select" style="width:220px" @change="setCap(k, $event.target.value)">
              <option value="off">off</option>
              <option v-for="m in capModels" :value="m">{{ m }}</option>
            </select>
          </dd>
        </div>
      </div>
    </div>
  </div>
</template>
<style scoped>
.hidden { display: none; }
.kv { display: flex; flex-direction: column; gap: .4rem; }
.btn-sm { font-size:.7rem;padding:.15rem .4rem; }
</style>
