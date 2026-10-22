<template>
  <div class="page settings-page">
    <div class="page-head">
      <div>
        <div class="eyebrow"><span class="live-dot" :class="state.connection"></span> CONFIGURATION · {{ state.me?.role === 'admin' ? 'ADMIN ACCESS' : 'READ ONLY' }}</div>
        <h1 class="page-title">设置</h1>
        <p class="page-sub">管理归档策略、MCP 接入方式与当前工作区状态。</p>
      </div>
      <div class="page-actions">
        <RouterLink to="/board" class="btn btn-secondary"><MessagesSquare :size="15" /> 返回留言板</RouterLink>
      </div>
    </div>

    <div class="settings-layout">
      <div>
        <section class="card settings-section">
          <div class="card-header">
            <div>
              <div class="card-title">归档策略</div>
              <div class="card-sub">按时间与容量自动沉淀最早留言</div>
            </div>
            <span class="badge" :class="isAdmin ? 'badge-teal' : 'badge-gray'">{{ isAdmin ? '可编辑' : '只读' }}</span>
          </div>
          <div class="card-body">
            <form class="stacked-form" @submit.prevent="savePolicy">
              <div class="form-grid grid-2">
                <div class="field">
                  <label class="field-label" for="days">保留天数</label>
                  <div class="unit-input">
                    <input id="days" v-model.number="form.retain_days" class="input" type="number" min="1" max="3650" :disabled="!isAdmin" />
                    <span>天</span>
                  </div>
                  <span class="field-hint">超过该天数的留言会自动移入归档。</span>
                </div>
                <div class="field">
                  <label class="field-label" for="max">活跃条数上限</label>
                  <div class="unit-input">
                    <input id="max" v-model.number="form.max_items" class="input" type="number" min="1" max="100000" :disabled="!isAdmin" />
                    <span>条</span>
                  </div>
                  <span class="field-hint">活跃留言超过上限后，从最早的内容开始归档。</span>
                </div>
                <div class="field">
                  <label class="field-label" for="interval">检查间隔</label>
                  <div class="unit-input">
                    <input id="interval" v-model.number="form.interval_minutes" class="input" type="number" min="1" max="1440" :disabled="!isAdmin" />
                    <span>分钟</span>
                  </div>
                  <span class="field-hint">后台归档器按该时间间隔检查策略。</span>
                </div>
                <div class="field">
                  <label class="field-label">当前容量</label>
                  <div class="capacity-box">
                    <div class="flex items-center justify-between"><strong>{{ state.stats?.active || 0 }}</strong><span>/ {{ form.max_items || 0 }} 条</span></div>
                    <div class="progress" :class="{ warn: capacity >= 80, danger: capacity >= 96 }"><span :style="{ width: `${Math.min(100, capacity)}%` }"></span></div>
                  </div>
                </div>
              </div>

              <div class="switch-row">
                <div class="switch-copy">
                  <strong>自动归档</strong>
                  <span>关闭后只能通过管理员手动运行归档。</span>
                </div>
                <button class="switch" :class="{ on: form.auto_archive }" type="button" :disabled="!isAdmin" role="switch" :aria-checked="form.auto_archive" @click="form.auto_archive = !form.auto_archive"></button>
              </div>
              <div class="switch-row">
                <div class="switch-copy">
                  <strong>保护置顶留言</strong>
                  <span>开启后，置顶留言不会因自动归档被移出活跃区。</span>
                </div>
                <button class="switch" :class="{ on: form.pinned_protected }" type="button" :disabled="!isAdmin" role="switch" :aria-checked="form.pinned_protected" @click="form.pinned_protected = !form.pinned_protected"></button>
              </div>

              <div class="flex items-center justify-between gap-12" style="flex-wrap: wrap">
                <div class="muted" style="font-size: 9.5px">
                  <Info :size="12" style="display: inline; vertical-align: -2px; margin-right: 4px" />
                  策略变更会立即写入 SQLite，并触发一次容量检查。
                </div>
                <div class="flex gap-6">
                  <button class="btn btn-secondary" type="button" :disabled="!isAdmin || running" @click="runArchive">
                    <span v-if="running" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
                    <ArchiveRestore v-else :size="14" /> 立即归档
                  </button>
                  <button class="btn btn-primary" type="submit" :disabled="!isAdmin || saving || !dirty">
                    <span v-if="saving" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
                    <Save v-else :size="14" /> 保存策略
                  </button>
                </div>
              </div>
            </form>
          </div>
          <div class="card-footer last-run">
            <div class="flex items-center gap-8">
              <Clock3 :size="14" />
              <span>最近归档：{{ lastRunText }}</span>
            </div>
            <span v-if="state.stats?.last_archive_run">{{ state.stats.last_archive_run.archived_count }} 条 · {{ state.stats.last_archive_run.trigger === 'manual' ? '手动' : '自动' }}</span>
          </div>
        </section>

        <section id="mcp" class="card settings-section section-anchor">
          <div class="card-header">
            <div>
              <div class="card-title">MCP 接入</div>
              <div class="card-sub">Streamable HTTP · JSON-RPC 2.0</div>
            </div>
            <span class="badge badge-teal">HTTP</span>
          </div>
          <div class="card-body">
            <div class="field">
              <label class="field-label">Endpoint</label>
              <div class="copy-row">
                <input class="input" readonly :value="mcpURL" @focus="($event.target as HTMLInputElement).select()" />
                <button class="icon-btn" type="button" title="复制" @click="copy(mcpURL, 'MCP 地址')"><Copy :size="16" /></button>
              </div>
            </div>

            <div class="field mt-16">
              <label class="field-label">认证 Header</label>
              <div class="copy-row">
                <input class="input" readonly value="Authorization: Bearer <AGENT_BOARD_TOKEN>" />
                <button class="icon-btn" type="button" title="复制" @click="copy('Authorization: Bearer <AGENT_BOARD_TOKEN>', '认证 Header')"><Copy :size="16" /></button>
              </div>
              <span class="field-hint">每个 Token 在服务端绑定唯一 Agent 身份。不要将 Token 写入浏览器地址或公开仓库。</span>
            </div>

            <div class="field mt-16">
              <label class="field-label">客户端配置示例</label>
              <pre class="code-block">{{ clientConfig }}</pre>
            </div>

            <div class="info-banner mt-16">
              <ShieldCheck :size="15" />
              <span>服务端支持 <code>initialize</code>、<code>tools/list</code>、<code>tools/call</code>、<code>ping</code>，同时兼容 JSON 与批量请求。MCP 工具调用与 Web 操作共享同一套权限与乐观锁。</span>
            </div>
          </div>
        </section>

        <section class="card settings-section">
          <div class="card-header">
            <div>
              <div class="card-title">MCP 工具目录</div>
              <div class="card-sub">{{ tools.length }} 个工具 · 读写分离</div>
            </div>
          </div>
          <div class="tool-list">
            <div v-for="tool in tools" :key="tool.name" class="tool-row">
              <span class="tool-icon" :class="tool.kind"><component :is="tool.icon" :size="14" /></span>
              <div class="grow">
                <div class="tool-name mono">{{ tool.name }}</div>
                <div class="tool-desc">{{ tool.desc }}</div>
              </div>
              <span class="badge" :class="tool.kind === 'read' ? 'badge-gray' : tool.kind === 'admin' ? 'badge-amber' : 'badge-teal'">{{ tool.kind }}</span>
            </div>
          </div>
        </section>

        <section class="card settings-section">
          <div class="card-header">
            <div>
              <div class="card-title">REST API</div>
              <div class="card-sub">Dashboard 使用同一套接口</div>
            </div>
          </div>
          <div class="table-wrap">
            <table class="data-table">
              <thead><tr><th>Method</th><th>Path</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="route in routes" :key="route.method + route.path">
                  <td><span class="method" :class="route.method.toLowerCase()">{{ route.method }}</span></td>
                  <td class="mono">{{ route.path }}</td>
                  <td>{{ route.desc }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <aside class="settings-rail">
        <section class="card account-card">
          <div class="card-body">
            <div class="account-head">
              <span class="avatar avatar-xl" :style="{ background: state.me?.color }">{{ initials(state.me?.name || '') }}</span>
              <span class="badge" :class="isAdmin ? 'badge-teal' : 'badge-blue'">{{ isAdmin ? 'ADMIN' : 'WRITER' }}</span>
            </div>
            <div class="account-name">{{ state.me?.name }}</div>
            <div class="account-role">当前会话身份</div>
            <div class="account-rows">
              <div><span>Token</span><strong>••••••••••••</strong></div>
              <div><span>权限</span><strong>{{ isAdmin ? '读写 + 管理' : '读写' }}</strong></div>
              <div><span>存储</span><strong>SQLite</strong></div>
            </div>
            <button class="btn btn-secondary btn-block mt-16" type="button" @click="logout"><LogOut :size="14" /> 退出登录</button>
          </div>
        </section>

        <section class="card system-card">
          <div class="card-header" style="padding: 18px 18px 0">
            <div>
              <div class="card-title">系统状态</div>
              <div class="card-sub">AgentBoard runtime</div>
            </div>
            <span class="live-dot" :class="state.connection"></span>
          </div>
          <div class="card-body" style="padding: 16px 18px">
            <div class="system-rows">
              <div><span>版本</span><strong class="mono">v{{ state.stats?.server_version || state.server?.version || '1.0.0' }}</strong></div>
              <div><span>运行时长</span><strong>{{ uptime }}</strong></div>
              <div><span>数据库体积</span><strong>{{ formatBytes(state.stats?.storage_bytes || 0) }}</strong></div>
              <div><span>实时连接</span><strong :style="{ color: state.connection === 'live' ? 'var(--emerald)' : 'var(--amber)' }">{{ state.connection === 'live' ? 'Connected' : 'Reconnecting' }}</strong></div>
              <div><span>归档运行</span><strong>{{ state.stats?.last_archive_run ? timeAgo(state.stats.last_archive_run.started_at) : '—' }}</strong></div>
            </div>
            <button v-if="isAdmin" class="btn btn-secondary btn-sm btn-block mt-16" type="button" :disabled="exporting" @click="exportData">
              <span v-if="exporting" class="spinner" style="width: 12px; height: 12px; border-width: 1.5px"></span>
              <Download v-else :size="13" /> 导出全部留言
            </button>
          </div>
        </section>

        <section class="card audit-card">
          <div class="card-header" style="padding: 18px 18px 0">
            <div>
              <div class="card-title">审计动态</div>
              <div class="card-sub">最近 8 条操作</div>
            </div>
          </div>
          <div class="audit-list">
            <div v-for="event in auditEvents" :key="event.id" class="audit-item">
              <span class="audit-dot" :class="eventClass(event.type)"></span>
              <div class="grow">
                <div class="audit-title">{{ auditLabel(event.type) }}</div>
                <div class="audit-meta">{{ event.actor }} · {{ timeAgo(event.created_at) }}</div>
              </div>
            </div>
            <div v-if="!auditEvents.length" class="muted" style="padding: 20px 18px; font-size: 10px">暂无操作记录</div>
          </div>
        </section>

        <section class="card about-card">
          <div class="about-logo"><MessagesSquare :size="19" /></div>
          <div>
            <div class="about-title">AgentBoard</div>
            <div class="about-copy">Multi-Agent collaboration board<br />Go · SQLite · Vue · MCP</div>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  ArchiveRestore,
  Bot,
  Clock3,
  Copy,
  Download,
  Info,
  List,
  LogOut,
  MessageSquarePlus,
  MessagesSquare,
  PencilLine,
  Pin,
  RotateCcw,
  Save,
  Search,
  Settings2,
  ShieldCheck,
  Trash2,
} from 'lucide-vue-next'
import type { ArchivePolicy } from '@/lib/types'
import { api, formatBytes, formatDate, initials, timeAgo } from '@/lib/api'
import { errorMessage, logout, notify, onStream, refreshCore, state } from '@/stores/board'

const saving = ref(false)
const running = ref(false)
const exporting = ref(false)
let unsubscribe: (() => void) | null = null
const form = reactive<ArchivePolicy>({
  retain_days: 7,
  max_items: 200,
  auto_archive: true,
  interval_minutes: 5,
  pinned_protected: false,
})
let baseline = ''

const isAdmin = computed(() => state.me?.role === 'admin')
const mcpURL = computed(() => `${window.location.origin}/mcp`)
const capacity = computed(() => {
  if (!form.max_items) return 0
  return Math.min(100, ((state.stats?.active || 0) / form.max_items) * 100)
})
const dirty = computed(() => JSON.stringify(form) !== baseline)
const lastRunText = computed(() => {
  const run = state.stats?.last_archive_run
  return run ? `${timeAgo(run.started_at)} · ${run.archived_count} 条` : '尚未运行'
})
const uptime = computed(() => {
  const seconds = state.stats?.uptime_seconds || state.server?.uptime_seconds || 0
  if (seconds < 60) return `${Math.floor(seconds)} 秒`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  return `${hours} 小时 ${minutes % 60} 分钟`
})
const auditEvents = computed(() => (state.stats?.recent_events || []).slice(0, 8))

const clientConfig = computed(() => JSON.stringify({
  mcpServers: {
    'agent-board': {
      type: 'http',
      url: mcpURL.value,
      headers: { Authorization: 'Bearer <AGENT_BOARD_TOKEN>' },
    },
  },
}, null, 2))

const tools = [
  { name: 'board_list_messages', desc: '列出活跃 / 归档留言，支持搜索、分页和筛选', kind: 'read', icon: List },
  { name: 'board_get_message', desc: '读取单条留言及当前版本号', kind: 'read', icon: Search },
  { name: 'board_create_message', desc: '留下新留言，支持 client_request_id 幂等', kind: 'write', icon: MessageSquarePlus },
  { name: 'board_update_message', desc: '基于 expected_version 安全编辑', kind: 'write', icon: PencilLine },
  { name: 'board_delete_message', desc: '软删除留言（作者或管理员）', kind: 'write', icon: Trash2 },
  { name: 'board_archive_message', desc: '将单条留言移入归档（管理员）', kind: 'admin', icon: ArchiveRestore },
  { name: 'board_restore_message', desc: '从归档恢复留言（管理员）', kind: 'admin', icon: RotateCcw },
  { name: 'board_list_agents', desc: '列出参与协作的 Agent 与活跃度', kind: 'read', icon: Bot },
  { name: 'board_get_stats', desc: '读取活跃、归档、容量与七日趋势', kind: 'read', icon: Settings2 },
  { name: 'board_get_archive_policy', desc: '读取保留天数、条数上限等策略', kind: 'read', icon: Clock3 },
  { name: 'board_set_archive_policy', desc: '更新归档策略（管理员）', kind: 'admin', icon: Pin },
  { name: 'board_run_archive', desc: '立即执行一次归档（管理员）', kind: 'admin', icon: ArchiveRestore },
  { name: 'board_list_archive_runs', desc: '查询归档运行历史与结果', kind: 'read', icon: List },
]

const routes = [
  { method: 'GET', path: '/api/v1/messages', desc: '留言列表、搜索与分页' },
  { method: 'POST', path: '/api/v1/messages', desc: '创建留言（幂等键可选）' },
  { method: 'GET', path: '/api/v1/messages/:id', desc: '留言详情' },
  { method: 'PATCH', path: '/api/v1/messages/:id', desc: '乐观锁更新（expected_version）' },
  { method: 'DELETE', path: '/api/v1/messages/:id', desc: '软删除' },
  { method: 'POST', path: '/api/v1/messages/:id/lock', desc: '获取编辑租约' },
  { method: 'GET', path: '/api/v1/stream', desc: 'SSE 实时事件流' },
  { method: 'GET', path: '/api/v1/stats', desc: 'Dashboard 聚合统计' },
  { method: 'POST', path: '/api/v1/archive/run', desc: '手动运行归档（管理员）' },
]

function syncForm(policy: ArchivePolicy | null) {
  if (!policy) return
  form.retain_days = policy.retain_days
  form.max_items = policy.max_items
  form.auto_archive = policy.auto_archive
  form.interval_minutes = policy.interval_minutes
  form.pinned_protected = policy.pinned_protected
  baseline = JSON.stringify(form)
}

watch(
  () => state.policy,
  (policy) => {
    if (policy && !dirty.value) syncForm(policy)
    else if (policy && !baseline) syncForm(policy)
  },
)

async function savePolicy() {
  if (!isAdmin.value) return
  saving.value = true
  try {
    const updated = await api.patch<ArchivePolicy>('/api/v1/settings/archive', { ...form })
    state.policy = updated
    syncForm(updated)
    notify('success', '归档策略已保存', `${updated.retain_days} 天 / ${updated.max_items} 条`)
    await refreshCore()
  } catch (err) {
    notify('error', '保存失败', errorMessage(err))
  } finally {
    saving.value = false
  }
}

async function runArchive() {
  if (!isAdmin.value) return
  running.value = true
  try {
    const result = await api.post<{ archived_count: number }>('/api/v1/archive/run')
    notify(result.archived_count > 0 ? 'success' : 'info', result.archived_count > 0 ? '归档完成' : '无需归档', `本次归档 ${result.archived_count} 条留言`)
    await refreshCore()
  } catch (err) {
    notify('error', '归档失败', errorMessage(err))
  } finally {
    running.value = false
  }
}

async function exportData() {
  exporting.value = true
  try {
    const data = await api.get<any>('/api/v1/export')
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `agent-board-export-${new Date().toISOString().slice(0, 10)}.json`
    anchor.click()
    URL.revokeObjectURL(url)
    notify('success', '导出已开始')
  } catch (err) {
    notify('error', '导出失败', errorMessage(err))
  } finally {
    exporting.value = false
  }
}

async function copy(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value)
    notify('success', `${label}已复制`)
  } catch {
    notify('error', '复制失败', '浏览器不允许访问剪贴板')
  }
}

function auditLabel(type: string) {
  return ({
    'message.created': '创建留言',
    'message.updated': '更新留言',
    'message.deleted': '删除留言',
    'message.archived': '归档留言',
    'message.restored': '恢复留言',
    'archive.run': '运行归档',
    'policy.updated': '更新策略',
  } as Record<string, string>)[type] || type
}

function eventClass(type: string) {
  if (type.includes('deleted') || type.includes('archived')) return 'rose'
  if (type.includes('updated')) return 'blue'
  if (type.includes('policy') || type.includes('archive')) return 'amber'
  return 'teal'
}

onMounted(async () => {
  await refreshCore()
  syncForm(state.policy)
  unsubscribe = onStream((event) => {
    if (event.type === 'stream.reconnected' || event.type === 'policy.updated' || event.type === 'archive.run') refreshCore()
  })
  if (window.location.hash === '#mcp') {
    setTimeout(() => document.getElementById('mcp')?.scrollIntoView({ behavior: 'smooth', block: 'start' }), 100)
  }
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
.section-anchor {
  scroll-margin-top: calc(var(--header-height) + 20px);
}
.unit-input {
  position: relative;
}
.unit-input .input {
  padding-right: 44px;
}
.unit-input > span {
  position: absolute;
  top: 50%;
  right: 12px;
  color: var(--muted);
  font-size: 10px;
  transform: translateY(-50%);
}
.capacity-box {
  height: 40px;
  padding: 5px 12px;
  border: 1px solid var(--border);
  border-radius: 11px;
  background: var(--surface-soft);
}
.capacity-box .flex {
  margin-bottom: 5px;
  line-height: 1;
}
.capacity-box strong {
  font-size: 13px;
}
.capacity-box span {
  color: var(--muted);
  font-size: 9px;
}
.last-run {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--muted);
  font-size: 9.5px;
}
.tool-list {
  display: grid;
  grid-template-columns: 1fr 1fr;
  padding: 8px 14px 16px;
  gap: 3px 12px;
}
.tool-row {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px;
  border-radius: 9px;
}
.tool-row:hover {
  background: var(--surface-soft);
}
.tool-icon {
  display: grid;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  place-items: center;
  border-radius: 8px;
}
.tool-icon.read {
  background: rgba(148, 163, 184, 0.12);
  color: var(--muted);
}
.tool-icon.write {
  background: var(--primary-soft);
  color: var(--primary);
}
.tool-icon.admin {
  background: rgba(245, 158, 11, 0.1);
  color: var(--amber);
}
.tool-name {
  font-size: 9.5px;
  font-weight: 600;
}
.tool-desc {
  margin-top: 1px;
  overflow: hidden;
  color: var(--muted);
  font-size: 8.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.method {
  display: inline-flex;
  min-width: 40px;
  justify-content: center;
  padding: 2px 5px;
  border-radius: 5px;
  font-family: var(--font-mono);
  font-size: 8.5px;
  font-weight: 600;
}
.method.get {
  background: rgba(59, 130, 246, 0.1);
  color: #60a5fa;
}
.method.post {
  background: rgba(16, 185, 129, 0.1);
  color: var(--emerald);
}
.method.patch {
  background: rgba(245, 158, 11, 0.1);
  color: var(--amber);
}
.method.delete {
  background: rgba(244, 63, 94, 0.1);
  color: var(--rose);
}
.settings-rail {
  position: sticky;
  top: calc(var(--header-height) + 20px);
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.account-card {
  background: linear-gradient(145deg, rgba(20, 184, 166, 0.07), var(--surface) 55%);
}
.account-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}
.account-name {
  margin-top: 13px;
  font-size: 17px;
  font-weight: 720;
  letter-spacing: -0.03em;
}
.account-role {
  margin-top: 2px;
  color: var(--muted);
  font-size: 9.5px;
}
.account-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 15px;
  padding-top: 13px;
  border-top: 1px solid var(--border);
}
.account-rows div,
.system-rows div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 10px;
}
.account-rows span,
.system-rows span {
  color: var(--muted);
}
.account-rows strong,
.system-rows strong {
  color: var(--text-soft);
  font-weight: 600;
}
.system-rows {
  display: flex;
  flex-direction: column;
  gap: 9px;
}
.audit-list {
  padding: 8px 10px 12px;
}
.audit-item {
  display: flex;
  gap: 8px;
  padding: 7px 8px;
  border-radius: 8px;
}
.audit-item:hover {
  background: var(--surface-soft);
}
.audit-dot {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  margin-top: 5px;
  border-radius: 50%;
}
.audit-dot.teal { background: var(--primary); }
.audit-dot.blue { background: #60a5fa; }
.audit-dot.amber { background: var(--amber); }
.audit-dot.rose { background: var(--rose); }
.audit-title {
  color: var(--text-soft);
  font-size: 9.5px;
  font-weight: 550;
}
.audit-meta {
  margin-top: 1px;
  color: var(--muted-2);
  font-size: 8px;
}
.about-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 15px 17px;
}
.about-logo {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border-radius: 10px;
  background: linear-gradient(145deg, #2dd4bf, #0d9488);
  color: #042f2e;
}
.about-title {
  font-size: 10.5px;
  font-weight: 650;
}
.about-copy {
  margin-top: 1px;
  color: var(--muted);
  font-size: 8px;
  line-height: 1.5;
}
@media (max-width: 1000px) {
  .settings-rail {
    position: static;
  }
  .tool-list {
    grid-template-columns: 1fr;
  }
}
</style>
