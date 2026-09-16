<script setup lang="ts">
import { ref, onMounted } from 'vue'

const getPort = () => {
  if (typeof window !== 'undefined') return window.location.port || '20128'
  return '20128'
}
const port = getPort()
const baseUrl = `http://localhost:${port}`
const v1Url = `${baseUrl}/v1`
const apiInfo = ref({ key: '', keyFull: '' })

async function copy(text: string, btn: HTMLElement) {
  await navigator.clipboard.writeText(text)
  const orig = btn.textContent
  btn.textContent = 'Copied!'
  setTimeout(() => { btn.textContent = orig }, 1500)
}

onMounted(async () => {
  try {
    const r = await fetch('/api/dashboard/bootstrap-key').then(x => x.json()).catch(() => ({}))
    if (r.key) apiInfo.value.keyFull = r.key
  } catch { /* placeholder */ }
  if (apiInfo.value.keyFull) {
    const k = apiInfo.value.keyFull
    apiInfo.value.key = k.slice(0, 4) + '••••••••' + k.slice(-2)
  } else {
    apiInfo.value.key = 'jk_••••••••'
  }
})
</script>
<template>
  <div>
    <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin-bottom:1rem">Endpoint</h2>
    <p class="note" style="margin-bottom:1rem">Info copy-paste untuk AI CLI tools.</p>

    <div class="card">
      <h3>OpenAI-compatible</h3>
      <div class="kv">
        <dt>Base URL</dt><dd><code style="color:var(--jkr-yel)">{{ v1Url }}</code></dd>
        <dt>API Key</dt><dd>
          <code style="color:var(--jkr-yel)">{{ apiInfo.key }}</code>
          <button class="btn ghost btn-sm" style="margin-left:.5rem" @click="copy(apiInfo.keyFull || apiInfo.key, $event.currentTarget)">copy</button>
        </dd>
      </div>
    </div>

    <div class="row" style="gap:1rem;margin-top:1rem">
      <div class="card" style="flex:1;margin-bottom:0">
        <h3>Claude Code</h3>
        <pre>ANTHROPIC_BASE_URL={{ baseUrl }}
ANTHROPIC_API_KEY={{ apiInfo.keyFull || 'jk_••••••••' }}
ANTHROPIC_MODEL=kr/claude-sonnet-4.5</pre>
      </div>
      <div class="card" style="flex:1;margin-bottom:0">
        <h3>Cursor / OpenClaw</h3>
        <pre>{ "endpoint": "{{ v1Url }}",
  "apiKey": "{{ apiInfo.keyFull || 'jk_••••••••' }}",
  "model": "kr/gemini-2.5-flash-lite" }</pre>
      </div>
      <div class="card" style="flex:1;margin-bottom:0">
        <h3>curl smoke test</h3>
        <pre>curl -s {{ baseUrl }}/v1/chat/completions \
  -H "Authorization: Bearer {{ apiInfo.keyFull || 'jk_••••••••' }}" \
  -d '{"model":"kr/claude-sonnet-4.5","messages":[{"role":"user","content":"ping"}],"stream":true}'</pre>
      </div>
    </div>
  </div>
</template>
