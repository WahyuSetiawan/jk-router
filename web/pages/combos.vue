<script setup lang="ts">
import { ref, onMounted } from 'vue'
const combos = ref<any[]>([])
const showing = ref(false)
const form = ref({ name: '', description: '', model_ids: '' as string, strategy: 'fallback' })

async function load() {
  const r = await fetch('/api/dashboard/combos').then(x => x.json())
  combos.value = r.combos || []
}
async function create() {
  if (!form.value.name) return
  // Parse comma-separated string into array
  const ids = form.value.model_ids.split(',').map(s => s.trim()).filter(Boolean)
  await fetch('/api/dashboard/combos', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: form.value.name, description: form.value.description, model_ids: ids, strategy: form.value.strategy })
  })
  form.value = { name: '', description: '', model_ids: '', strategy: 'fallback' }
  showing.value = false
  await load()
}
async function del(id: number) {
  await fetch(`/api/dashboard/combos/${id}`, { method: 'DELETE' })
  await load()
}
const modelList = (c: any) => {
  try { return JSON.parse(c.model_ids || '[]') } catch { return [] }
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">Combos</h2>
      <button class="btn" @click="showing=true">+ New Combo</button>
    </div>
    <div v-if="showing" class="card">
      <h3>New Combo</h3>
      <div class="kv" style="margin-bottom:.8rem">
        <dt>Name</dt><dd><input v-model="form.name" class="input" placeholder="e.g. fast-vision" /></dd>
        <dt>Strategy</dt><dd>
          <select v-model="form.strategy" class="select">
            <option value="fallback">fallback</option><option value="round_robin">round_robin</option>
          </select>
        </dd>
        <dt>Models</dt><dd><input v-model="form.model_ids" class="input" placeholder="gpt-4o, claude-sonnet-4, kimi-k2.5 (comma-separated)" /></dd>
        <dt>Description</dt><dd><input v-model="form.description" class="input" placeholder="Optional description" /></dd>
      </div>
      <div style="display:flex;gap:.5rem">
        <button class="btn" @click="create">Save</button>
        <button class="btn ghost" @click="showing=false">Cancel</button>
      </div>
    </div>
    <template v-for="c in combos" :key="c.id">
      <div class="card">
        <h3>{{ c.name }} <span class="st active">{{ c.strategy || 'fallback' }}</span></h3>
        <ol style="font-size:.8rem;padding-left:1.2rem;margin:0">
          <li v-for="m in modelList(c)" :key="m" style="margin-bottom:.2rem">{{ m }}</li>
        </ol>
        <p v-if="c.description" class="note">{{ c.description }}</p>
        <div style="text-align:right;margin-top:.5rem"><button class="btn-danger btn-sm" @click="del(c.id)">Delete</button></div>
      </div>
    </template>
    <div v-if="combos.length===0 && !showing" class="note" style="text-align:center;padding:2rem">No combos configured</div>
  </div>
</template>
<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
.btn-danger { background:var(--jkr-red); color:var(--jkr-crust); border:none; padding:.25rem .5rem; border-radius:4px; font-size:.7rem; cursor:pointer; }
.btn-sm { padding: .2rem .5rem; font-size: .7rem; }
</style>