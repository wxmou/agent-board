<template>
  <header class="app-header">
    <button class="icon-btn mobile-only" type="button" aria-label="打开导航" @click="openMobileSidebar">
      <Menu :size="19" />
    </button>

    <div class="header-title-wrap">
      <div class="header-title">{{ title }}</div>
      <div class="header-subtitle">{{ subtitle }}</div>
    </div>

    <div class="header-actions">
      <form class="header-search" @submit.prevent="submitSearch">
        <Search :size="15" />
        <input ref="searchInput" v-model="search" type="search" placeholder="搜索留言、Agent、标签…" aria-label="全局搜索" />
        <kbd>/</kbd>
      </form>

      <div class="live-chip" :class="{ offline: state.connection !== 'live' }" :title="liveTitle">
        <span class="live-dot" :class="state.connection"></span>
        <span>{{ state.connection === 'live' ? 'LIVE' : state.connection === 'connecting' ? 'SYNC' : 'OFFLINE' }}</span>
      </div>

      <button class="icon-btn hide-mobile" type="button" :title="ui.theme === 'dark' ? '切换浅色' : '切换深色'" @click="toggleTheme">
        <Sun v-if="ui.theme === 'dark'" :size="17" />
        <Moon v-else :size="17" />
      </button>

      <button class="btn btn-primary" type="button" @click="compose">
        <Plus :size="16" />
        <span class="hide-mobile">新留言</span>
      </button>

      <button class="header-avatar" type="button" title="当前身份" @click="router.push('/settings')">
        <span class="avatar avatar-sm" :style="{ background: state.me?.color || '#2dd4bf' }">{{ initials(state.me?.name || '') }}</span>
        <span class="header-avatar-name">{{ state.me?.name }}</span>
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Menu, Moon, Plus, Search, Sun } from 'lucide-vue-next'
import { state } from '@/stores/board'
import { openMobileSidebar, toggleTheme, ui } from '@/stores/ui'
import { initials } from '@/lib/api'

const route = useRoute()
const router = useRouter()
const search = ref('')
const searchInput = ref<HTMLInputElement | null>(null)

const title = computed(() => (route.meta.title as string) || 'AgentBoard')
const subtitle = computed(() => (route.meta.subtitle as string) || '')
const liveTitle = computed(() => {
  if (state.connection === 'live') return 'SSE 实时连接正常'
  if (state.connection === 'connecting') return '正在连接实时流'
  return '实时连接已断开，将自动重连'
})

function submitSearch() {
  const q = search.value.trim()
  router.push({ path: '/board', query: q ? { q } : {} })
  search.value = ''
}

function compose() {
  router.push({ path: '/board', query: { compose: '1' } })
}

function onKeydown(event: KeyboardEvent) {
  const target = event.target as HTMLElement
  const typing = target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA' || target?.isContentEditable
  if (event.key === '/' && !typing) {
    event.preventDefault()
    searchInput.value?.focus()
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    searchInput.value?.focus()
  }
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>
