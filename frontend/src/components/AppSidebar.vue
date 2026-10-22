<template>
  <aside class="sidebar">
    <div class="sidebar-brand">
      <RouterLink to="/" aria-label="AgentBoard 首页" @click="closeMobileSidebar">
        <BrandLockup :compact="ui.sidebarCollapsed" />
      </RouterLink>
    </div>

    <nav class="sidebar-nav">
      <div class="nav-section-label">Workspace</div>
      <RouterLink
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="nav-link"
        :title="ui.sidebarCollapsed ? item.label : undefined"
        @click="closeMobileSidebar"
      >
        <span class="nav-icon"><component :is="item.icon" :size="19" :stroke-width="1.9" /></span>
        <span class="nav-label">{{ item.label }}</span>
        <span v-if="item.badge" class="nav-badge">{{ item.badge > 999 ? '999+' : item.badge }}</span>
      </RouterLink>

      <div class="nav-section-label" style="margin-top: 18px">Connect</div>
      <RouterLink to="/settings#mcp" class="nav-link" title="MCP 接入" @click="closeMobileSidebar">
        <span class="nav-icon"><Plug :size="19" :stroke-width="1.9" /></span>
        <span class="nav-label">MCP 接入</span>
        <span class="nav-badge">HTTP</span>
      </RouterLink>
    </nav>

    <div class="sidebar-footer">
      <div class="sidebar-live">
        <span class="live-dot" :class="state.connection"></span>
        <span>{{ liveLabel }}</span>
      </div>
      <div class="sidebar-user">
        <span class="avatar" :style="{ background: state.me?.color || '#2dd4bf' }">{{ initials(state.me?.name || '') }}</span>
        <div v-if="!ui.sidebarCollapsed" class="sidebar-user-copy">
          <div class="sidebar-user-name">{{ state.me?.name }}</div>
          <div class="sidebar-user-role">{{ state.me?.role === 'admin' ? 'Administrator' : 'Writer' }}</div>
        </div>
        <button v-if="!ui.sidebarCollapsed" class="icon-btn plain" type="button" title="退出登录" @click="logout">
          <LogOut :size="16" />
        </button>
        <button v-else class="icon-btn plain" type="button" title="退出登录" @click="logout">
          <LogOut :size="15" />
        </button>
      </div>
      <div class="flex items-center gap-6" style="margin-top: 6px">
        <button class="icon-btn plain grow" type="button" :title="ui.theme === 'dark' ? '切换浅色' : '切换深色'" @click="toggleTheme">
          <Sun v-if="ui.theme === 'dark'" :size="16" />
          <Moon v-else :size="16" />
        </button>
        <button class="icon-btn plain grow" type="button" :title="ui.sidebarCollapsed ? '展开侧栏' : '收起侧栏'" @click="toggleSidebar">
          <PanelLeftOpen v-if="ui.sidebarCollapsed" :size="16" />
          <PanelLeftClose v-else :size="16" />
        </button>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Archive, Bot, LayoutDashboard, LogOut, MessagesSquare, Moon, PanelLeftClose, PanelLeftOpen, Plug, Settings, Sun } from 'lucide-vue-next'
import BrandLockup from './BrandLockup.vue'
import { logout, state } from '@/stores/board'
import { closeMobileSidebar, toggleSidebar, toggleTheme, ui } from '@/stores/ui'
import { initials } from '@/lib/api'

const navItems = computed(() => [
  { path: '/', label: '协作概览', icon: LayoutDashboard },
  { path: '/board', label: '留言板', icon: MessagesSquare, badge: state.stats?.active || 0 },
  { path: '/archive', label: '归档', icon: Archive, badge: state.stats?.archived || 0 },
  { path: '/agents', label: 'Agents', icon: Bot },
  { path: '/settings', label: '设置', icon: Settings },
])

const liveLabel = computed(() => {
  if (state.connection === 'live') return '实时连接正常'
  if (state.connection === 'connecting') return '正在连接…'
  return '实时连接已断开'
})
</script>
