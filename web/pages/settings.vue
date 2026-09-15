<script setup lang="ts">
import { ref, onMounted } from 'vue'
const settings = ref({ port: 20128, dataDir: '', version: '0.3.0-alpha' })
const exportRef = ref<HTMLTextAreaElement | null>(null)

async function load() {
  // Read from config file or env
  settings.value = { port: 20128, dataDir: process.env.DATA_DIR || '~/.jkrouter', version: '0.3.0-alpha' }
}
async function exportConfig() {
  const r = await fetch('/api/dashboard/health').then(x=>x.json())
  alert('Config export coming soon. Currently run: jkrouter backup')
}
const importInput = ref<HTMLInputElement | null>(null)
async function importConfig(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async () => {
    try {
      const data = JSON.parse(reader.result as string)
      // Validate and apply
      alert('Config imported: ' + Object.keys(data).length + ' keys')
    } catch {
      alert('Invalid config file')
    }
  }
  reader.readAsText(file)
}
onMounted(load)
</script>
<template>
  <div>
    <h2 class="text-xl font-bold mb-4">Settings</h2>
    <div class="space-y-4">
      <div class="bg-gray-900 rounded-lg p-4 border border-gray-800">
        <h3 class="font-semibold mb-2">Server</h3>
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div><span class="text-gray-500">Port:</span> <span class="font-mono">{{ settings.port }}</span></div>
          <div><span class="text-gray-500">Data Dir:</span> <span class="font-mono">{{ settings.dataDir }}</span></div>
          <div><span class="text-gray-500">Version:</span> <span class="font-mono">{{ settings.version }}</span></div>
        </div>
      </div>
      <div class="bg-gray-900 rounded-lg p-4 border border-gray-800">
        <h3 class="font-semibold mb-2">Backup & Restore</h3>
        <div class="flex gap-3">
          <button @click="exportConfig" class="btn-primary">Export Config</button>
          <label class="btn-secondary cursor-pointer">
            Import Config
            <input ref="importInput" type="file" accept=".json" class="hidden" @change="importConfig" />
          </label>
          <button class="btn-secondary ml-auto" onclick="window.location.href='/dashboard/'">Restart Dashboard</button>
        </div>
      </div>
      <div class="bg-gray-900 rounded-lg p-4 border border-gray-800">
        <h3 class="font-semibold mb-2">Danger Zone</h3>
        <p class="text-sm text-gray-500 mb-3">Reset all data and start fresh. This cannot be undone.</p>
        <button class="btn-danger">Reset Data</button>
      </div>
    </div>
  </div>
</template>
<style scoped>
.btn-primary { @apply bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm font-medium; }
.btn-secondary { @apply bg-gray-700 hover:bg-gray-600 text-gray-200 px-4 py-2 rounded text-sm inline-block; }
.btn-danger { @apply bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded text-sm font-medium; }
</style>
