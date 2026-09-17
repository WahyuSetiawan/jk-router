<script setup lang="ts">
import { useI18n } from '~/composables/useI18n'
const { t } = useI18n()
import { ref, onMounted } from 'vue'
const form = ref({ current: '', new: '', confirm: '' })
const msg = ref('')
const msgType = ref<'ok' | 'err'>('ok')
const firstRun = ref(false)

async function load() {
  try {
    const r = await fetch('/api/dashboard/auth/status').then(x => x.json())
    firstRun.value = r.first_run ?? false
  } catch { firstRun.value = false }
}
async function save() {
  if (form.value.new !== form.value.confirm) {
    msg.value = 'Passwords do not match'
    msgType.value = 'err'
    return
  }
  if (!form.value.new || form.value.new.length < 6) {
    msg.value = 'Password must be at least 6 characters'
    msgType.value = 'err'
    return
  }
  msg.value = ''
  try {
    await fetch('/api/dashboard/auth/change-password', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ current: form.value.current, new: form.value.new })
    })
    msg.value = firstRun.value ? 'Password set — dashboard unlocked!' : 'Password changed'
    msgType.value = 'ok'
    form.value = { current: '', new: '', confirm: '' }
  } catch (e: any) {
    msg.value = e.message || 'Failed'
    msgType.value = 'err'
  }
}
onMounted(load)
</script>
<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0"><span class="tag p1">P1</span> /login → /dashboard/profile</h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">Dashboard auth lokal (mirror src/app/login 9Router, bcrypt)</small>
    </div>
    <div class="card" style="max-width:380px">
      <h3>{{ firstRun ? 'Set Dashboard Password' : t('profile.change_password') }}</h3>
      <div v-if="msg" :style="{color: msgType==='ok' ? 'var(--jkr-grn)' : 'var(--jkr-red)', marginBottom: '.8rem'}" class="note">{{ msg }}</div>
      <div class="kv" style="margin-bottom:.8rem">
        <dt>Current</dt><dd><input v-model="form.current" type="password" class="input" placeholder="••••••••" /></dd>
        <dt>New</dt><dd><input v-model="form.new" type="password" class="input" placeholder="••••••••" /></dd>
        <dt>Confirm</dt><dd><input v-model="form.confirm" type="password" class="input" placeholder="••••••••" /></dd>
      </div>
      <button class="btn" @click="save">{{ firstRun ? 'Save & Unlock' : 'Save' }}</button>
      <p class="note" style="margin-top:.8rem">
        {{ firstRun
          ? 'First-time setup: set a password to unlock the dashboard. Required when binding to 0.0.0.0.'
          : 'Flow pertama kali: setup wizard 1 halaman (password + port) di tempat login, sebelum dashboard unlock.' }}
      </p>
    </div>
  </div>
</template>