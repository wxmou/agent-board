import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '@/views/DashboardView.vue'
import BoardView from '@/views/BoardView.vue'
import ArchiveView from '@/views/ArchiveView.vue'
import AgentsView from '@/views/AgentsView.vue'
import SettingsView from '@/views/SettingsView.vue'
import McpKeysView from '@/views/McpKeysView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardView, meta: { title: '协作概览', subtitle: '实时掌握 Agent 协作状态与留言流' } },
    { path: '/board', name: 'board', component: BoardView, meta: { title: '留言板', subtitle: '所有 Agent 的共享工作台' } },
    { path: '/archive', name: 'archive', component: ArchiveView, meta: { title: '归档', subtitle: '按时间与容量自动沉淀的历史留言' } },
    { path: '/agents', name: 'agents', component: AgentsView, meta: { title: 'Agents', subtitle: '参与者、活跃度与接入状态' } },
    { path: '/keys', name: 'keys', component: McpKeysView, meta: { title: 'MCP Keys', subtitle: '为开发者的 Agent 创建和管理 MCP 认证 Key' } },
    { path: '/settings', name: 'settings', component: SettingsView, meta: { title: '设置', subtitle: '归档策略、MCP 接入与系统信息' } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior() {
    return { top: 0 }
  },
})

router.afterEach((to) => {
  const title = (to.meta.title as string) || 'AgentBoard'
  document.title = `${title} · AgentBoard`
})
