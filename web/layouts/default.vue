<template>
  <div class="app">
    <nav class="nav">
      <div class="logo">jkROUTER</div>
      <div class="nav-sec">Routing</div>
      <NuxtLink to="/providers" class="nav-link" :class="{ active: route.path.startsWith('/providers') }">Providers</NuxtLink>
      <NuxtLink to="/combos" class="nav-link" :class="{ active: route.path.startsWith('/combos') }">Combos</NuxtLink>
      <NuxtLink to="/endpoint" class="nav-link" :class="{ active: route.path.startsWith('/endpoint') }">Endpoint</NuxtLink>
      <div class="nav-sec">Infrastructure</div>
      <NuxtLink to="/proxy-pools" class="nav-link" :class="{ active: route.path.startsWith('/proxy-pools') }">Proxy Pools</NuxtLink>
      <NuxtLink to="/usage" class="nav-link" :class="{ active: route.path.startsWith('/usage') }">Usage & Logs</NuxtLink>
      <div class="nav-sec">System</div>
      <NuxtLink to="/api-keys" class="nav-link" :class="{ active: route.path.startsWith('/api-keys') }">API Keys</NuxtLink>
      <NuxtLink to="/settings" class="nav-link" :class="{ active: route.path.startsWith('/settings') }">Settings</NuxtLink>
      <NuxtLink to="/profile" class="nav-link" :class="{ active: route.path.startsWith('/profile') }">Profile</NuxtLink>
    </nav>
    <div class="main">
      <NuxtPage />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
const route = useRoute()
</script>

<style scoped>
.app { display: grid; grid-template-columns: 190px 1fr; min-height: 100vh; background: var(--jkr-base); }
.nav { background: var(--jkr-mantle); border-right: 1px solid var(--jkr-brd); padding: 1rem .6rem; display: flex; flex-direction: column; gap: .15rem; position: sticky; top: 0; height: 100vh; overflow-y: auto; }
.logo { font-weight: 700; color: var(--jkr-blue); font-size: 1rem; padding: 0 .5rem .8rem; border-bottom: 1px solid var(--jkr-brd); margin-bottom: .6rem; }
.main { padding: 1.5rem 2rem; background: var(--jkr-base); min-height: 100vh; }
</style>