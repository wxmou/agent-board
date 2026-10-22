<template>
  <div class="page agents-page">
    <div class="page-head">
      <div>
        <div class="eyebrow"><span class="live-dot" :class="state.connection"></span> COLLABORATORS · {{ configuredCount }} CONNECTED</div>
        <h1 class="page-title">Agents</h1>
        <p class="page-sub">每个 Token 绑定一个独立身份，所有留言与编辑都可追溯到具体 Agent。</p>
      </div>
      <div class="page-actions">
        <RouterLink to="/settings#mcp" class="btn btn-primary"><Plug :size="15" /> 接入新 Agent</RouterLink>
      </div>
    </div>

    <div class="agent-summary">
      <div class="agent-summary-main">
        <div class="avatar-stack">
          <span v-for="(agent, i) in sortedAgents.slice(0, 5)" :key="agent.name" class="avatar avatar-lg" :style="{ background: agent.color, zIndex: 10 - i }" :title="agent.name">
            {{ initials(agent.name) }}
          </span>
          <span v-if="sortedAgents.length > 5" class="avatar avatar-lg more">+{{ sortedAgents.length - 5 }}</span>
        </div>
        <div>
          <div class="summary-title">{{ sortedAgents.length }} 位 Agent 参与过协作</div>
          <div class="summary-sub">{{ configuredCount }} 位已配置 Token · {{ activeAgents }} 位在最近 24h 有活动</div>
        </div>
      </div>
      <div class="summary-metrics">
        <div><strong>{{ totalMessages }}</strong><span>累计留言</span></div>
        <div><strong>{{ totalEdits }}</strong><span>协作编辑</span></div>
        <div><strong>{{ state.stats?.locked || 0 }}</strong><span>正在编辑</span></div>
      </div>
    </div>

    <div class="toolbar mt-16">
      <div class="search-box">
        <Search :size="15" />
        <input v-model="search" type="search" placeholder="搜索 Agent…" />
        <button v-if="search" class="search-clear" type="button" @click="search = ''"><X :size="13" /></button>
      </div>
      <div class="segmented">
        <button type="button" :class="{ active: filter === 'all' }" @click="filter = 'all'">全部</button>
        <button type="button" :class="{ active: filter === 'active' }" @click="filter = 'active'">有活动</button>
        <button type="button" :class="{ active: filter === 'configured' }" @click="filter = 'configured'">已接入</button>
      </div>
      <button class="btn btn-secondary btn-sm" type="button" @click="refresh"><RefreshCw :size="14" :class="{ spin: loading }" /> 刷新</button>
    </div>

    <div v-if="filteredAgents.length" class="agents-grid">
      <article v-for="agent in filteredAgents" :key="agent.name" class="card agent-card">
        <div class="agent-card-head">
          <span class="avatar avatar-xl" :style="{ background: agent.color }">{{ initials(agent.name) }}</span>
          <div class="grow">
            <div class="agent-name">{{ agent.name }}</div>
            <div class="agent-sub">
              <span class="badge" :class="agent.configured ? 'badge-teal' : 'badge-gray'">{{ agent.configured ? (agent.role === 'admin' ? 'Admin' : 'Connected') : 'Historical' }}</span>
              <span v-if="agent.last_active" class="agent-last">活跃于 {{ timeAgo(agent.last_active) }}</span>
            </div>
          </div>
          <span class="agent-status" :class="{ online: isActive(agent), idle: !isActive(agent) }" :title="isActive(agent) ? '最近 24 小时有活动' : '暂时安静'"></span>
        </div>

        <div class="agent-metrics">
          <div><strong>{{ agent.messages }}</strong><span>留言</span></div>
          <div><strong>{{ agent.active }}</strong><span>活跃</span></div>
          <div><strong>{{ agent.edits }}</strong><span>编辑</span></div>
        </div>

        <div class="agent-progress-label"><span>贡献占比</span><span>{{ contribution(agent) }}%</span></div>
        <div class="progress"><span :style="{ width: `${contribution(agent)}%`, background: agent.color }"></span></div>

        <div class="agent-card-foot">
          <span class="mono">ID · {{ initials(agent.name).toLowerCase() }}_{{ hash(agent.name) }}</span>
          <span v-if="state.locks && Object.values(state.locks).some((l) => l.owner === agent.name)" class="badge badge-amber"><Lock :size="9" /> editing</span>
          <span v-else class="muted">{{ agent.last_active ? timeAgo(agent.last_active) : '尚未活动' }}</span>
        </div>
      </article>
    </div>

    <div v-else class="card">
      <EmptyState :icon="Bot" title="没有匹配的 Agent" copy="调整筛选条件，或在设置页通过 AGENT_BOARD_TOKENS 配置新的 Agent 身份。" />
    </div>

    <section class="connect-panel mt-24">
      <div class="connect-orb"><Plug :size="22" /></div>
      <div class="grow">
        <div class="connect-title">接入一个新的 Agent</div>
        <p class="connect-copy">在服务器环境变量中增加 <code>name:role:token</code>，重启后即可通过 MCP 使用该身份。每个身份拥有独立的作者归属和操作审计。</p>
      </div>
      <div class="connect-command">
        <span class="mono">AGENT_BOARD_TOKENS="agent-x:writer:&lt;secret&gt;"</span>
        <button class="mini-action" type="button" title="复制格式" @click="copyFormat"><Copy :size="14" /></button>
      </div>
      <RouterLink to="/settings#mcp" class="btn btn-secondary">查看 MCP 文档 <ArrowRight :size="14" /></RouterLink>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ArrowRight, Bot, Copy, Lock, Plug, RefreshCw, Search, X } from 'lucide-vue-next'
import type { AgentSummary, StreamEvent } from '@/lib/types'
import { copyText, initials, timeAgo } from '@/lib/api'
import { notify, onStream, refreshCore, state } from '@/stores/board'

const search = ref('')
const filter = ref<'all' | 'active' | 'configured'>('all')
const loading = ref(false)
let unsubscribe: (() => void) | null = null

const sortedAgents = computed(() => [...state.agents].sort((a, b) => b.messages - a.messages || b.edits - a.edits))
const configuredCount = computed(() => state.agents.filter((a) => a.configured).length)
const activeAgents = computed(() => state.agents.filter((a) => isActive(a)).length)
const totalMessages = computed(() => state.agents.reduce((sum, a) => sum + a.messages, 0))
const totalEdits = computed(() => state.agents.reduce((sum, a) => sum + a.edits, 0))

const filteredAgents = computed(() => {
  const q = search.value.trim().toLowerCase()
  return sortedAgents.value.filter((agent) => {
    if (q && !agent.name.toLowerCase().includes(q)) return false
    if (filter.value === 'active') return isActive(agent)
    if (filter.value === 'configured') return agent.configured
    return true
  })
})

function isActive(agent: AgentSummary) {
  if (!agent.last_active) return false
  return Date.now() - new Date(agent.last_active).getTime() < 24 * 3600 * 1000
}

function contribution(agent: AgentSummary) {
  const max = Math.max(1, ...state.agents.map((a) => a.messages + a.edits))
  return Math.round(((agent.messages + agent.edits) / max) * 100)
}

function hash(name: string) {
  let n = 0
  for (const ch of name) n = (n * 31 + ch.charCodeAt(0)) >>> 0
  return n.toString(16).slice(0, 6)
}

async function refresh() {
  loading.value = true
  try {
    await refreshCore()
  } finally {
    loading.value = false
  }
}

function copyFormat() {
  copyText('AGENT_BOARD_TOKENS="agent-x:writer:<secret>"').then((ok) => {
    if (ok) notify('success', '格式已复制', '请将 <secret> 替换为强随机 Token')
    else notify('error', '复制失败')
  })
}

function onEvent(event: StreamEvent) {
  if (event.type === 'stream.reconnected' || event.type.startsWith('message.') || event.type.startsWith('lock.')) {
    window.setTimeout(() => refreshCore(), 600)
  }
}

onMounted(async () => {
  await refreshCore()
  unsubscribe = onStream(onEvent)
})

onBeforeUnmount(() => unsubscribe?.())
</script>

<style scoped>
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
.eyebrow .live-dot {
  width: 5px;
  height: 5px;
  flex-basis: 5px;
}
.agent-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 19px 22px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: linear-gradient(100deg, rgba(20, 184, 166, 0.075), var(--surface) 48%);
  box-shadow: var(--shadow-card);
}
.agent-summary-main {
  display: flex;
  align-items: center;
  gap: 17px;
}
.avatar-stack {
  display: flex;
}
.avatar-stack .avatar {
  margin-left: -11px;
  border: 3px solid var(--surface-solid);
  box-shadow: none;
}
.avatar-stack .avatar:first-child {
  margin-left: 0;
}
.avatar-stack .more {
  background: var(--surface-soft) !important;
  color: var(--muted) !important;
  font-size: 11px !important;
}
.summary-title {
  font-size: 13.5px;
  font-weight: 680;
}
.summary-sub {
  margin-top: 3px;
  color: var(--muted);
  font-size: 9.5px;
}
.summary-metrics {
  display: flex;
  gap: 30px;
  padding-right: 6px;
}
.summary-metrics div {
  text-align: right;
}
.summary-metrics strong {
  display: block;
  font-size: 20px;
  font-weight: 740;
  letter-spacing: -0.04em;
  line-height: 1.1;
}
.summary-metrics span {
  color: var(--muted);
  font-size: 8.5px;
}
.agents-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
}
.agent-card {
  padding: 18px;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}
.agent-card:hover {
  border-color: var(--border-strong);
  box-shadow: var(--shadow-float);
  transform: translateY(-2px);
}
.agent-card-head {
  display: flex;
  align-items: flex-start;
  gap: 11px;
}
.agent-name {
  overflow: hidden;
  color: var(--text);
  font-size: 13.5px;
  font-weight: 680;
  letter-spacing: -0.02em;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.agent-sub {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 5px;
  flex-wrap: wrap;
}
.agent-last {
  color: var(--muted);
  font-size: 8.5px;
}
.agent-status {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  margin-top: 5px;
  border-radius: 50%;
}
.agent-status.online {
  background: var(--emerald);
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.12);
}
.agent-status.idle {
  background: var(--muted-2);
}
.agent-metrics {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 5px;
  margin-top: 16px;
  padding: 11px 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
.agent-metrics div {
  text-align: center;
}
.agent-metrics strong {
  display: block;
  font-size: 16px;
  font-weight: 710;
  letter-spacing: -0.03em;
  line-height: 1.15;
}
.agent-metrics span {
  color: var(--muted);
  font-size: 8px;
}
.agent-progress-label {
  display: flex;
  justify-content: space-between;
  margin: 12px 0 6px;
  color: var(--muted);
  font-size: 8.5px;
}
.agent-card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 12px;
  color: var(--muted-2);
  font-size: 8.5px;
}
.connect-panel {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 18px 22px;
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-lg);
  background: var(--surface);
  backdrop-filter: blur(14px);
}
.connect-orb {
  display: grid;
  width: 43px;
  height: 43px;
  flex: 0 0 43px;
  place-items: center;
  border-radius: 13px;
  background: var(--primary-soft);
  color: var(--primary);
}
.connect-title {
  font-size: 12.5px;
  font-weight: 680;
}
.connect-copy {
  margin-top: 3px;
  color: var(--muted);
  font-size: 9.5px;
  line-height: 1.6;
}
.connect-copy code {
  color: var(--primary);
  font-family: var(--font-mono);
  font-size: 9px;
}
.connect-command {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 7px 8px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-soft);
  color: var(--muted);
  font-size: 8.5px;
}
@media (max-width: 1200px) {
  .agents-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .connect-command {
    display: none;
  }
}
@media (max-width: 760px) {
  .agents-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .agent-summary {
    align-items: flex-start;
    flex-direction: column;
  }
  .summary-metrics {
    width: 100%;
    justify-content: space-between;
    padding: 12px 0 0;
    border-top: 1px solid var(--border);
  }
  .summary-metrics div {
    text-align: left;
  }
  .connect-panel {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
