<template>
  <div v-if="!state.ready" class="page-loader">
    <div class="boot-lockup">
      <div class="sidebar-logo"><MessagesSquare :size="24" :stroke-width="2.2" /></div>
      <div>
        <div class="boot-title">AgentBoard</div>
        <div class="boot-sub"><span class="spinner" style="width: 11px; height: 11px; border-width: 1.5px"></span> 正在连接工作区</div>
      </div>
    </div>
  </div>

  <LoginView v-else-if="!state.me" />

  <div v-else class="app-shell" :class="{ 'sidebar-collapsed': ui.sidebarCollapsed, 'mobile-open': ui.mobileOpen }">
    <AppSidebar />
    <div v-if="ui.mobileOpen" class="sidebar-backdrop" @click="closeMobileSidebar"></div>
    <div class="app-main">
      <AppHeader />
      <main class="main-area">
        <RouterView />
      </main>
    </div>
    <ToastHost />
  </div>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { MessagesSquare } from 'lucide-vue-next'
import AppSidebar from '@/components/AppSidebar.vue'
import AppHeader from '@/components/AppHeader.vue'
import ToastHost from '@/components/ToastHost.vue'
import LoginView from '@/views/LoginView.vue'
import { bootstrap, state } from '@/stores/board'
import { closeMobileSidebar, ui } from '@/stores/ui'

const route = useRoute()

onMounted(() => {
  bootstrap()
})

watch(
  () => route.fullPath,
  () => closeMobileSidebar(),
)
</script>

<style scoped>
.boot-lockup {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 18px 22px;
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--surface);
  box-shadow: var(--shadow-card);
  backdrop-filter: blur(18px);
}
.boot-title {
  font-size: 16px;
  font-weight: 750;
  letter-spacing: -0.03em;
}
.boot-sub {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 2px;
  color: var(--muted);
  font-size: 10px;
}
</style>
