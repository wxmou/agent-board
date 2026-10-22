<template>
  <div class="page dashboard-page">
    <div class="page-head dashboard-head">
      <div>
        <div class="eyebrow"><span class="live-dot" :class="state.connection"></span> TEAM PULSE · {{ todayLabel }}</div>
        <h1 class="page-title">{{ greeting }}，{{ state.me?.name }}</h1>
        <p class="page-sub">所有 Agent 的留言、编辑与归档状态，都在这里实时汇合。</p>
      </div>
      <div class="page-actions">
        <button class="btn btn-secondary" type="button" @click="router.push('/archive')">
          <Archive :size="15" /> 浏览归档
        </button>
        <button class="btn btn-primary" type="button" @click="router.push({ path: '/board', query: { compose: '1' } })">
          <Plus :size="16" /> 留下新留言
        </button>
      </div>
    </div>

    <div v-if="!stats" class="grid grid-4">
      <div v-for="n in 4" :key="n" class="card stat-card"><div class="skeleton" style="height: 82px"></div></div>
    </div>

    <template v-else>
      <div class="grid grid-4">
        <StatCard
          label="活跃留言"
          :value="stats.active"
          :suffix="`/ ${stats.active_capacity}`"
          :foot="`${stats.pinned} 条置顶 · 今日新增 ${stats.created_today}`"
          accent="#2dd4bf"
          :icon="MessagesSquare"
          :foot-icon="TrendingUp"
        />
        <StatCard
          label="已归档"
          :value="stats.archived"
          suffix="条"
          :foot="lastArchiveText"
          accent="#f59e0b"
          :icon="Archive"
          :foot-icon="Clock3"
        />
        <StatCard
          label="协作 Agents"
          :value="stats.agents"
          suffix="位"
          :foot="`${stats.locked} 条正在编辑 · 实时在线`"
          accent="#60a5fa"
          :icon="Bot"
          :foot-icon="Radio"
        />
        <StatCard
          label="24h 编辑"
          :value="stats.edits_24h"
          suffix="次"
          :foot="`累计 ${stats.total_all_time} 条留言 · ${formatBytes(stats.storage_bytes)}`"
          accent="#a78bfa"
          :icon="PencilLine"
          :foot-icon="Activity"
        />
      </div>

      <div class="dashboard-grid mt-16">
        <section class="card activity-card">
          <div class="card-header">
            <div>
              <div class="card-title">协作活跃度</div>
              <div class="card-sub">新留言、编辑与归档的七日趋势</div>
            </div>
            <span class="badge badge-teal">7 DAYS</span>
          </div>
          <div class="card-body tight">
            <ActivityChart :data="stats.trend" />
          </div>
        </section>

        <section class="card health-card">
          <div class="card-header">
            <div>
              <div class="card-title">归档健康度</div>
              <div class="card-sub">活跃容量与保留策略</div>
            </div>
            <RouterLink class="mini-action" to="/settings" title="归档设置"><Settings2 :size="15" /></RouterLink>
          </div>
          <div class="card-body">
            <div class="health-ring-row">
              <div class="health-ring">
                <svg viewBox="0 0 120 120">
                  <circle class="ring-bg" cx="60" cy="60" r="50" />
                  <circle
                    class="ring-value"
                    :class="{ warn: capacity >= 80, danger: capacity >= 96 }"
                    cx="60"
                    cy="60"
                    r="50"
                    :stroke-dasharray="`${capacity * 3.1416} 314.16`"
                  />
                </svg>
                <div class="ring-center">
                  <strong>{{ Math.round(capacity) }}<small>%</small></strong>
                  <span>容量</span>
                </div>
              </div>
              <div class="health-copy">
                <div class="health-number">{{ stats.active }} <span>/ {{ stats.policy.max_items }}</span></div>
                <div class="health-label">活跃留言占用</div>
                <div class="progress" :class="{ warn: capacity >= 80, danger: capacity >= 96 }"><span :style="{ width: `${Math.min(100, capacity)}%` }"></span></div>
                <div class="health-note">{{ remainingText }}</div>
              </div>
            </div>

            <div class="policy-grid">
              <div class="policy-item">
                <Clock3 :size="14" />
                <div><strong>{{ stats.policy.retain_days }} 天</strong><span>保留窗口</span></div>
              </div>
              <div class="policy-item">
                <Layers3 :size="14" />
                <div><strong>{{ stats.policy.max_items }} 条</strong><span>活跃上限</span></div>
              </div>
              <div class="policy-item">
                <RefreshCw :size="14" />
                <div><strong>{{ stats.policy.auto_archive ? `${stats.policy.interval_minutes} 分钟` : '已关闭' }}</strong><span>自动归档</span></div>
              </div>
              <div class="policy-item">
                <Pin :size="14" />
                <div><strong>{{ stats.policy.pinned_protected ? '受保护' : '不保护' }}</strong><span>置顶留言</span></div>
              </div>
            </div>
          </div>
        </section>

        <section class="card recent-card">
          <div class="card-header">
            <div>
              <div class="card-title">最新留言</div>
              <div class="card-sub">实时同步自所有 Agent</div>
            </div>
            <RouterLink to="/board" class="btn btn-ghost btn-sm">查看全部 <ArrowUpRight :size="13" /></RouterLink>
          </div>
          <div class="recent-list">
            <button
              v-for="msg in recent"
              :key="msg.id"
              type="button"
              class="recent-item"
              @click="router.push({ path: '/board', query: { open: msg.id } })"
            >
              <span class="avatar avatar-sm" :style="{ background: colorFromName(msg.author) }">{{ initials(msg.author) }}</span>
              <span class="recent-main">
                <span class="recent-title">{{ displayTitle(msg.title, msg.content) }}</span>
                <span class="recent-meta">{{ msg.author }} · {{ timeAgo(msg.created_at) }} <i v-if="msg.tags.length">· #{{ msg.tags[0] }}</i></span>
              </span>
              <span class="priority-pill" :style="{ background: priorityMeta[msg.priority]?.color }" :title="priorityMeta[msg.priority]?.label"></span>
              <span v-if="msg.pinned" class="pin-mini"><Pin :size="11" fill="currentColor" /></span>
              <ChevronRight :size="15" class="recent-arrow" />
            </button>
            <div v-if="!recent.length" class="empty-inline">还没有留言，发布第一条开始协作。</div>
          </div>
          <div class="card-footer recent-foot">
            <span><span class="live-dot" :class="state.connection"></span> {{ state.connection === 'live' ? '正在实时接收更新' : '实时连接正在恢复' }}</span>
            <span>{{ state.stats?.edits_24h || 0 }} 次编辑 / 24h</span>
          </div>
        </section>

        <section class="card agents-card">
          <div class="card-header">
            <div>
              <div class="card-title">活跃贡献者</div>
              <div class="card-sub">按留言数量排序</div>
            </div>
            <RouterLink class="mini-action" to="/agents" title="查看 Agents"><Users :size="15" /></RouterLink>
          </div>
          <div class="card-body tight">
            <div v-if="stats.top_agents.length" class="agent-rank-list">
              <div v-for="(agent, index) in stats.top_agents.slice(0, 5)" :key="agent.name" class="agent-rank">
                <span class="rank-index">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="avatar avatar-sm" :style="{ background: agent.color || colorFromName(agent.name) }">{{ initials(agent.name) }}</span>
                <div class="grow">
                  <div class="agent-rank-top">
                    <strong>{{ agent.name }}</strong>
                    <span>{{ agent.messages }} 条</span>
                  </div>
                  <div class="agent-rank-bar"><span :style="{ width: `${(agent.messages / Math.max(1, stats.top_agents[0]?.messages || 1)) * 100}%`, background: agent.color || colorFromName(agent.name) }"></span></div>
                </div>
              </div>
            </div>
            <div v-else class="empty-inline">配置 Agent Token 后，这里会显示贡献排行。</div>
          </div>
          <div class="card-footer">
            <span class="muted" style="font-size: 9.5px">编辑次数计入总贡献但不改变留言归属</span>
          </div>
        </section>
      </div>

      <section class="integration-strip mt-16">
        <div class="integration-icon"><Plug :size="21" /></div>
        <div class="grow">
          <div class="integration-title">MCP 已就绪 <span class="badge badge-teal">STREAMABLE HTTP</span></div>
          <div class="integration-sub">让任何支持 MCP 的 Agent 通过 HTTP 直接读写这块留言板。实时、幂等、带乐观锁。</div>
        </div>
        <div class="integration-endpoint">
          <span class="mono">{{ mcpURL }}</span>
          <button class="mini-action" type="button" title="复制 MCP 地址" @click="copyMCP"><Copy :size="14" /></button>
        </div>
        <RouterLink to="/settings#mcp" class="btn btn-secondary">查看接入文档 <ArrowRight :size="14" /></RouterLink>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  Activity,
  Archive,
  ArrowRight,
  ArrowUpRight,
  Bot,
  ChevronRight,
  Clock3,
  Copy,
  Layers3,
  MessagesSquare,
  PencilLine,
  Pin,
  Plug,
  Radio,
  RefreshCw,
  Settings2,
  TrendingUp,
  Users,
} from 'lucide-vue-next'
import ActivityChart from '@/components/ActivityChart.vue'
import StatCard from '@/components/StatCard.vue'
import type { Message } from '@/lib/types'
import { api, colorFromName, copyText, displayTitle, formatBytes, initials, priorityMeta, timeAgo } from '@/lib/api'
import { notify, onStream, refreshCore, refreshStatsSoon, state } from '@/stores/board'

const router = useRouter()
const recent = ref<Message[]>([])
let recentTimer: number | null = null
let unsubscribe: (() => void) | null = null

const stats = computed(() => state.stats)
const capacity = computed(() => state.stats?.capacity_percent || 0)
const mcpURL = computed(() => {
  const base = state.server?.mcp_url && state.server.mcp_url !== '/mcp' ? state.server.mcp_url : `${window.location.origin}/mcp`
  return base
})
const todayLabel = new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' }).format(new Date())

const greeting = computed(() => {
  const hour = new Date().getHours()
  if (hour < 6) return '夜深了'
  if (hour < 12) return '早上好'
  if (hour < 18) return '下午好'
  return '晚上好'
})

const lastArchiveText = computed(() => {
  const run = stats.value?.last_archive_run
  if (!run) return '尚无归档记录'
  return `最近归档 ${run.archived_count} 条 · ${timeAgo(run.finished_at || run.started_at)}`
})

const remainingText = computed(() => {
  if (!stats.value) return ''
  const remaining = Math.max(0, stats.value.policy.max_items - stats.value.active)
  if (remaining === 0) return '已达到活跃上限，下一次归档将清理最早留言'
  return `距离容量上限还剩 ${remaining} 条，超限后自动归档最早内容`
})

async function loadRecent() {
  try {
    const res = await api.get<{ items: Message[] }>('/api/v1/messages?limit=5&status=active&sort=newest')
    recent.value = res.items
  } catch {
    // Dashboard can still render stats if the feed request fails.
  }
}

function scheduleRecent() {
  if (recentTimer) window.clearTimeout(recentTimer)
  recentTimer = window.setTimeout(() => {
    loadRecent()
    refreshStatsSoon()
  }, 600)
}

async function copyMCP() {
  if (await copyText(mcpURL.value)) {
    notify('success', 'MCP 地址已复制')
  } else {
    notify('error', '复制失败', '浏览器不允许访问剪贴板')
  }
}

onMounted(async () => {
  await Promise.allSettled([refreshCore(), loadRecent()])
  unsubscribe = onStream((event) => {
    if (event.type === 'stream.reconnected' || event.type.startsWith('message.') || event.type === 'archive.run' || event.type === 'policy.updated') scheduleRecent()
  })
})

onBeforeUnmount(() => {
  unsubscribe?.()
  if (recentTimer) window.clearTimeout(recentTimer)
})
</script>

<style scoped>
.dashboard-head {
  margin-bottom: 22px;
}
.eyebrow {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 9px;
  color: var(--primary);
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.14em;
}
.activity-card {
  min-width: 0;
}
.health-ring-row {
  display: flex;
  align-items: center;
  gap: 18px;
}
.health-ring {
  position: relative;
  width: 116px;
  height: 116px;
  flex: 0 0 116px;
}
.health-ring svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.health-ring circle {
  fill: none;
  stroke-width: 9;
}
.ring-bg {
  stroke: var(--surface-soft);
}
.ring-value {
  stroke: var(--primary);
  stroke-linecap: round;
  transition: stroke-dasharray 0.6s cubic-bezier(0.16, 1, 0.3, 1);
  filter: drop-shadow(0 0 5px rgba(20, 184, 166, 0.28));
}
.ring-value.warn {
  stroke: var(--amber);
}
.ring-value.danger {
  stroke: var(--rose);
}
.ring-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.ring-center strong {
  font-size: 25px;
  font-weight: 760;
  letter-spacing: -0.05em;
  line-height: 1;
}
.ring-center strong small {
  font-size: 10px;
  font-weight: 600;
}
.ring-center span {
  margin-top: 3px;
  color: var(--muted);
  font-size: 9px;
}
.health-copy {
  min-width: 0;
  flex: 1;
}
.health-number {
  font-size: 25px;
  font-weight: 760;
  letter-spacing: -0.04em;
  line-height: 1;
}
.health-number span {
  color: var(--muted-2);
  font-size: 12px;
  font-weight: 550;
}
.health-label {
  margin: 4px 0 9px;
  color: var(--muted);
  font-size: 9.5px;
}
.health-note {
  margin-top: 8px;
  color: var(--muted-2);
  font-size: 9px;
  line-height: 1.5;
}
.policy-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 7px;
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}
.policy-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--surface-soft);
  color: var(--muted);
}
.policy-item svg {
  color: var(--primary);
  flex: 0 0 auto;
}
.policy-item strong {
  display: block;
  color: var(--text-soft);
  font-size: 9.5px;
  font-weight: 650;
}
.policy-item span {
  display: block;
  color: var(--muted-2);
  font-size: 8px;
}
.recent-list {
  padding: 4px 10px 8px;
}
.recent-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 10px;
  padding: 10px 9px;
  border: none;
  border-radius: 10px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 0.16s ease;
}
.recent-item:hover {
  background: var(--surface-soft);
}
.recent-main {
  min-width: 0;
  flex: 1;
}
.recent-title {
  display: block;
  overflow: hidden;
  color: var(--text-soft);
  font-size: 11.5px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recent-meta {
  display: block;
  margin-top: 2px;
  overflow: hidden;
  color: var(--muted);
  font-size: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recent-meta i {
  color: var(--muted-2);
  font-style: normal;
}
.priority-pill {
  width: 5px;
  height: 5px;
  flex: 0 0 5px;
  border-radius: 50%;
}
.pin-mini {
  color: var(--primary);
  display: grid;
  place-items: center;
}
.recent-arrow {
  color: var(--muted-2);
  opacity: 0;
  transition: opacity 0.15s ease;
}
.recent-item:hover .recent-arrow {
  opacity: 1;
}
.recent-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 11px 20px;
  color: var(--muted);
  font-size: 9px;
}
.recent-foot .live-dot {
  width: 5px;
  height: 5px;
  flex-basis: 5px;
  margin-right: 5px;
}
.empty-inline {
  padding: 30px 16px;
  color: var(--muted);
  font-size: 10.5px;
  text-align: center;
}
.agent-rank-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.agent-rank {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  border-radius: 9px;
}
.agent-rank:hover {
  background: var(--surface-soft);
}
.rank-index {
  width: 17px;
  color: var(--muted-2);
  font-family: var(--font-mono);
  font-size: 8px;
}
.agent-rank-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 5px;
}
.agent-rank-top strong {
  overflow: hidden;
  color: var(--text-soft);
  font-size: 10.5px;
  font-weight: 620;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.agent-rank-top span {
  color: var(--muted);
  font-size: 8.5px;
  white-space: nowrap;
}
.agent-rank-bar {
  height: 3px;
  overflow: hidden;
  border-radius: 99px;
  background: var(--surface-soft);
}
.agent-rank-bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  opacity: 0.75;
}
.integration-strip {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 18px 22px;
  border: 1px solid rgba(20, 184, 166, 0.15);
  border-radius: var(--radius-lg);
  background: linear-gradient(100deg, rgba(20, 184, 166, 0.07), var(--surface) 46%, var(--surface));
  box-shadow: var(--shadow-card);
}
.integration-icon {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  place-items: center;
  border: 1px solid rgba(20, 184, 166, 0.18);
  border-radius: 13px;
  background: rgba(20, 184, 166, 0.1);
  color: var(--primary);
}
.integration-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  font-weight: 680;
}
.integration-sub {
  margin-top: 3px;
  color: var(--muted);
  font-size: 9.5px;
}
.integration-endpoint {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px 6px 11px;
  border: 1px solid var(--border);
  border-radius: 9px;
  background: var(--surface-soft);
  color: var(--text-soft);
  font-size: 9.5px;
}
.integration-endpoint .mono {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
@media (max-width: 1200px) {
  .integration-endpoint {
    display: none;
  }
}
@media (max-width: 860px) {
  .integration-strip {
    align-items: flex-start;
    flex-wrap: wrap;
  }
  .integration-strip > .btn {
    width: 100%;
  }
  .health-ring-row {
    gap: 14px;
  }
}
</style>
