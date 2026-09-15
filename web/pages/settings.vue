<script setup lang="ts">
import { ref, onMounted } from 'vue'
const settings = ref({ port: 20128, bind: '127.0.0.1', dataDir: '~/.jkrouter' })
const saving = ref(false)
const msg = ref('')
const msgType = ref<'ok' | 'err'>('ok')

async function load() {
  try {
    const r = await fetch('/api/dashboard/settings').then(x => x.json())
    if (r.settings) settings.value = { ...settings.value, ...r.settings }
  } catch { /* use defaults */ }
}
async function save() {
  saving.value = true
  msg.value = ''
  try {
    await fetch('/api/dashboard/settings', {
      method: 'PUT', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
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
onMounted(load)
</script>
<template>
  <div>
    <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin-bottom:1rem">Settings</h2>
      <div v-if="msg" :style="{color: msgType==='ok' ? 'var(--jkr-grn)' : 'var(--jkr-red)', marginBottom: '1rem'}" class="note">{{ msg }}</div>
    <div class="card">
      <h3>Server</h3>
      <div class="kv">
        <dt>Port</dt><dd><input v-model.number="settings.port" type="number" class="input" style="width:100px" min="1000" max="65535" /></dd>
        <dt>Bind</dt><dd>
          <select v-model="settings.bind" class="select" style="width:160px">
            <option value="127.0.0.1">127.0.0.1</option>
            <option value="0.0.0.0">0.0.0.0</option>
          </select>
        </dd>
        <dt>DATA_DIR</dt><dd><input v-model="settings.dataDir" class="input" style="width:240px" /></dd>
      </div>
      <button class="btn" style="margin-top:.8rem" @click="save" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
    </div>
    <div class="card">
      <h3>Backup & Restore</h3>
      <div class="kv">
        <dt>Export config</dt><dd><button class="btn ghost btn-sm" @click="exportConfig">Export JSON</button></dd>
        <dt>Import config</dt><dd>
          <label class="btn ghost btn-sm" style="cursor:pointer;display:inline-block">
            Import JSON
            <input ref="importInput" type="file" accept=".json" class="hidden" @change="importConfig" style="display:none" />
          </label>
        </dd>
      </div>
      <p class="note">Export/import portable config (providers, combos, pools, keys). Does not include usage logs.</p>
    </div>
    <div class="card">
      <h3>Resilience Defaults</h3>
      <div class="kv">
        <dt>Cooldown (429)</dt><dd>60s (default when Retry-After absent)</dd>
        <dt>WAL checkpoint</dt><dd>5m interval</dd>
        <dt>Log channel</dt><dd>4096 buffer (drop+counter when full)</dd>
      </div>
    </div>
    <div class="card">
      <h3>Capacity Adapter</h3>
      <div class="kv">
        <dt>Vision</dt><dd><label style="display:flex;align-items:center;gap:.5rem"><input type="checkbox" disabled /> off</label></dd>
        <dt>PDF</dt><dd><label style="display:flex;align-items:center;gap:.5rem"><input type="checkbox" disabled /> off</label></dd>
        <dt>Audio input</dt><dd><label style="display:flex;align-items:center;gap:.5rem"><input type="checkbox" disabled /> off</label></dd>
        <dt>Video input</dt><dd><label style="display:flex;align-items:center;gap:.5rem"><input type="checkbox" disabled /> off</label></dd>
      </div>
      <p class="note">Auto-enabled when combo has no model supporting the requested modality.</p>
    </div>
  </div>
</template>
<style scoped>
.hidden { display: none; }
</style>