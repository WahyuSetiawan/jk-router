<template>
  <div class="app">
    <nav class="nav">
      <div class="logo">jkROUTER</div>
      <div class="nav-sec">{{ t('nav.routing') }}</div>
      <NuxtLink to="/providers" class="nav-link" :class="{ active: route.path.startsWith('/providers') }">{{ t('nav.providers') }}</NuxtLink>
      <NuxtLink to="/combos" class="nav-link" :class="{ active: route.path.startsWith('/combos') }">{{ t('nav.combos') }}</NuxtLink>
      <NuxtLink to="/endpoint" class="nav-link" :class="{ active: route.path.startsWith('/endpoint') }">{{ t('nav.endpoint') }}</NuxtLink>
      <div class="nav-sec">{{ t('nav.infrastructure') }}</div>
      <NuxtLink to="/proxy-pools" class="nav-link" :class="{ active: route.path.startsWith('/proxy-pools') }">{{ t('nav.proxy-pools') }}</NuxtLink>
      <NuxtLink to="/usage" class="nav-link" :class="{ active: route.path.startsWith('/usage') || route.path.startsWith('/logs') }">{{ t('nav.usage') }}</NuxtLink>
      <NuxtLink to="/quota" class="nav-link" :class="{ active: route.path.startsWith('/quota') }">{{ t('nav.quota') }}</NuxtLink>
      <div class="nav-sec">{{ t('nav.system') }}</div>
      <NuxtLink to="/api-keys" class="nav-link" :class="{ active: route.path.startsWith('/api-keys') }">{{ t('nav.api-keys') }}</NuxtLink>
      <NuxtLink to="/media-providers" class="nav-link" :class="{ active: route.path.startsWith('/media-providers') }">{{ t('nav.media-providers') }}</NuxtLink>
      <NuxtLink to="/basic-chat" class="nav-link" :class="{ active: route.path.startsWith('/basic-chat') }">{{ t('nav.basic-chat') }}</NuxtLink>
      <NuxtLink to="/translator" class="nav-link" :class="{ active: route.path.startsWith('/translator') }">{{ t('nav.translator') }}</NuxtLink>
      <NuxtLink to="/cli-tools" class="nav-link" :class="{ active: route.path.startsWith('/cli-tools') }">{{ t('nav.cli-tools') }}</NuxtLink>
      <NuxtLink to="/settings" class="nav-link" :class="{ active: route.path.startsWith('/settings') }">{{ t('nav.settings') }}</NuxtLink>
      <NuxtLink to="/token-saver" class="nav-link" :class="{ active: route.path.startsWith('/token-saver') }">{{ t('nav.token-saver') }}</NuxtLink>
      <NuxtLink to="/profile" class="nav-link" :class="{ active: route.path.startsWith('/profile') }">{{ t('nav.profile') }}</NuxtLink>
      <div class="lang-switch">
        <button :class="{ active: locale === 'id' }" @click="setLocale('id')">ID</button>
        <button :class="{ active: locale === 'en' }" @click="setLocale('en')">EN</button>
      </div>
    </nav>
    <div class="main">
      <NuxtPage />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from '~/composables/useI18n'
const route = useRoute()
const { t, setLocale, locale } = useI18n()
</script>

<style scoped>
.app { display: grid; grid-template-columns: 190px 1fr; min-height: 100vh; background: var(--jkr-base); }
.nav { background: var(--jkr-mantle); border-right: 1px solid var(--jkr-brd); padding: 1rem .6rem; display: flex; flex-direction: column; gap: .15rem; position: sticky; top: 0; height: 100vh; overflow-y: auto; }
.logo { font-weight: 700; color: var(--jkr-blue); font-size: 1rem; padding: 0 .5rem .8rem; border-bottom: 1px solid var(--jkr-brd); margin-bottom: .6rem; }
.nav-sec { font-size: .65rem; text-transform: uppercase; letter-spacing: .08em; color: var(--jkr-mut); padding: .5rem .5rem .2rem; margin-top: .3rem; }
.nav-link { display: block; padding: .35rem .5rem; border-radius: 4px; color: var(--jkr-sub); font-size: .82rem; text-decoration: none; }
.nav-link:hover { background: var(--jkr-base); color: var(--jkr-txt); }
.nav-link.active { background: var(--jkr-surface); color: var(--jkr-lav); }
.main { padding: 1.5rem 2rem; background: var(--jkr-base); min-height: 100vh; }
.lang-switch { margin-top: auto; padding-top: .8rem; border-top: 1px solid var(--jkr-brd); display: flex; gap: .3rem; }
.lang-switch button { flex: 1; padding: .25rem; border: 1px solid var(--jkr-brd); border-radius: 4px; background: transparent; color: var(--jkr-mut); font-size: .7rem; cursor: pointer; }
.lang-switch button.active { background: var(--jkr-surface); color: var(--jkr-lav); border-color: var(--jkr-lav); }
</style>
