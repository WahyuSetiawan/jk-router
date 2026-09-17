<script setup lang="ts">
import { ref, onMounted, nextTick } from 'vue'

const apiKey = ref('')
const model = ref('')
const models = ref<string[]>([])
const messages = ref<{role: string; content: string}[]>([])
const input = ref('')
const loading = ref(false)
const history = ref<{role: string; content: string; model?: string; latency_ms?: number}[]>([])
const error = ref('')
const responseInfo = ref('')
const connected = ref(false)

async function loadModels() {
  try {
    const r = await fetch('/v1/models').then(x => x.json())
    models.value = (r as any).data?.map((m: any) => m.id) || []
    if (models.value.length > 0) model.value = models.value[0]
  } catch (e) {
    error.value = '加载模型失败'
  }
}

async function sendMessage() {
  if (!input.value.trim() || !apiKey.value) return
  loading.value = true
  error.value = ''
  responseInfo.value = ''
  
  const userMsg = input.value.trim()
  messages.value = [...messages.value, { role: 'user', content: userMsg }]
  input.value = ''
  
  try {
    const resp = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${apiKey.value}`
      },
      body: JSON.stringify({
        model: model.value,
        messages: [{ role: 'user', content: userMsg }],
        stream: false
      })
    })
    
    const data = await resp.json()
    if (!resp.ok) throw new Error(data.error?.message || '请求失败')
    
    const choice = data.choices?.[0]
    const assistantMsg = choice?.message?.content || ''
    messages.value = [...messages.value, { role: 'assistant', content: assistantMsg }]
    
    const usage = data.usage
    responseInfo.value = `模型: ${data.model} | 延迟: ${Math.round(choice?.finish_reason === 'length' ? 0 : (data.body?._latency || 0))}ms`
    
    history.value = [...history.value, {
      role: 'user', content: userMsg,
      model: data.model,
      latency_ms: Math.round((Date.now() - startTime) / 1000 * 1000)
    }]
    
    // Store last API key for future sessions
    if (!localStorage.getItem('jkr_api_key')) {
      localStorage.setItem('jkr_api_key', apiKey.value)
    }
    
  } catch (e) {
    error.value = (e as Error).message
    messages.value = messages.value.slice(0, -1) // Remove failed user message
  } finally {
    loading.value = false
  }
}

let startTime = 0

async function startConversation() {
  startTime = Date.now()
  await sendMessage()
}

onMounted(async () => {
  const savedKey = localStorage.getItem('jkr_api_key')
  if (savedKey) apiKey.value = savedKey
  await loadModels()
})
</script>

<template>
  <div>
    <div class="page-head">
      <h2 style="color:var(--jkr-lav);font-size:1.1rem;margin:0">
        <span class="tag p2">P2</span> /dashboard/basic-chat
      </h2>
      <small style="color:var(--jkr-mut);font-size:.75rem">在线测试路由与模型响应</small>
    </div>

    <div class="card" style="margin-bottom:1rem">
      <div style="display:flex;gap:.5rem;align-items:center">
        <input v-model="apiKey" type="text" class="code-input" placeholder="输入 API Key（首次需从API Keys页面创建）"
          style="flex:1" />
        <select v-model="model" class="select">
          <option v-for="m in models" :key="m" :value="m">{{ m }}</option>
        </select>
      </div>
      <p v-if="error" class="note" style="color:var(--jkr-red)">{{ error }}</p>
    </div>

    <div class="card" style="flex:1;min-height:300px;max-height:50vh;overflow-y:auto;margin-bottom:1rem">
      <div v-if="messages.length === 0" class="note" style="text-align:center;padding:2rem;color:var(--jkr-mut)">
        开始对话…
      </div>
      <div v-for="(msg, i) in messages" :key="i" :style="{marginBottom:'1rem', textAlign: msg.role==='user'?'right':'left'}">
        <span :style="{
          display:'inline-block', padding:'.5rem .8rem', borderRadius:'8px',
          maxWidth:'80%', fontSize:'.85rem', lineHeight:'1.4',
          background: msg.role==='user'?'var(--jkr-lav)':'var(--jkr-base)',
          color: msg.role==='user'?'var(--jkr-mantle)':'var(--jkr-txt)'
        }">{{ msg.content }}</span>
      </div>
      <div v-if="loading" class="note" style="text-align:center;color:var(--jkr-mut)">思考中…</div>
    </div>

    <div style="display:flex;gap:.5rem">
      <input v-model="input" class="code-input" placeholder="输入消息…" @keyup.enter="startConversation"
        style="flex:1" :disabled="loading" />
      <button class="btn" :disabled="loading || !input.trim()" @click="startConversation">发送</button>
    </div>
    <p v-if="responseInfo" style="font-size:.7rem;color:var(--jkr-mut);margin-top:.3rem">{{ responseInfo }}</p>
  </div>
</template>

<style scoped>
.page-head { display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; }
.card { background:var(--jkr-mantle); border:1px solid var(--jkr-brd); border-radius:8px; padding:1rem; }
.code-input { width:100%; background:var(--jkr-base); color:var(--jkr-txt); border:1px solid var(--jkr-brd);
  border-radius:4px; padding:.5rem; font-family:monospace; font-size:.8rem; resize:vertical; box-sizing:border-box; }
.select { background:var(--jkr-base); color:var(--jkr-txt); border:1px solid var(--jkr-brd);
  border-radius:4px; padding:.3rem .5rem; font-size:.8rem; min-width:200px; }
.note { margin:.5rem 0; font-size:.8rem; }
</style>
