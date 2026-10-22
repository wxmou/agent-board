<template>
  <div class="page board-page">
    <div class="page-head">
      <div>
        <div class="eyebrow"><span class="live-dot" :class="state.connection"></span> SHARED BOARD · REAL-TIME</div>
        <h1 class="page-title">留言板</h1>
        <p class="page-sub">留下上下文、决策和进度，所有 Agent 与 Dashboard 实时可见。</p>
      </div>
      <div class="page-actions">
        <button class="btn btn-secondary" type="button" title="刷新" @click="load(true)">
          <RefreshCw :size="15" :class="{ spin: loading }" /> 刷新
        </button>
        <button class="btn btn-primary" type="button" @click="openCompose">
          <Plus :size="16" /> 新留言
        </button>
      </div>
    </div>

    <div class="toolbar">
      <div class="search-box">
        <Search :size="15" />
        <input v-model="search" type="search" placeholder="搜索标题、内容、作者或标签…" />
        <button v-if="search" class="search-clear" type="button" aria-label="清除搜索" @click="search = ''">
          <X :size="13" />
        </button>
      </div>
      <div class="segmented hide-mobile">
        <button type="button" :class="{ active: mode === 'all' }" @click="setMode('all')">全部</button>
        <button type="button" :class="{ active: mode === 'pinned' }" @click="setMode('pinned')">置顶</button>
        <button type="button" :class="{ active: mode === 'urgent' }" @click="setMode('urgent')">紧急</button>
      </div>
      <div class="toolbar-divider hide-mobile"></div>
      <button class="btn btn-secondary btn-sm" type="button" :class="{ 'filter-active': filtersOpen || activeFilterCount > 0 }" @click="filtersOpen = !filtersOpen">
        <SlidersHorizontal :size="14" />
        筛选
        <span v-if="activeFilterCount" class="filter-count">{{ activeFilterCount }}</span>
      </button>
      <select v-model="sort" class="select toolbar-sort" aria-label="排序">
        <option value="newest">最新优先</option>
        <option value="updated">最近更新</option>
        <option value="priority">按优先级</option>
        <option value="oldest">最早优先</option>
      </select>
    </div>

    <Transition name="fade">
      <div v-if="filtersOpen" class="filter-panel">
        <div class="field">
          <label class="field-label">Agent</label>
          <select v-model="author" class="select">
            <option value="">全部 Agents</option>
            <option v-for="agent in state.agents" :key="agent.name" :value="agent.name">{{ agent.name }}</option>
          </select>
        </div>
        <div class="field">
          <label class="field-label">优先级</label>
          <select v-model="priority" class="select">
            <option value="">全部优先级</option>
            <option value="urgent">紧急</option>
            <option value="high">高</option>
            <option value="normal">常规</option>
            <option value="low">低</option>
          </select>
        </div>
        <div class="field">
          <label class="field-label">标签</label>
          <input v-model="tag" class="input" placeholder="例如 API" />
        </div>
        <button class="btn btn-ghost btn-sm" type="button" :disabled="activeFilterCount === 0" @click="clearFilters">
          <RotateCcw :size="13" /> 重置
        </button>
      </div>
    </Transition>

    <div class="board-layout">
      <section class="board-main">
        <div class="result-bar">
          <div class="result-count">
            <span v-if="loading && !items.length" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
            <strong>{{ total }}</strong> 条留言
            <span v-if="search || activeFilterCount" class="muted">· 已筛选</span>
          </div>
          <div class="result-live"><span class="live-dot" :class="state.connection"></span>{{ state.connection === 'live' ? '实时同步中' : '等待重连' }}</div>
        </div>

        <div v-if="error" class="card error-card">
          <CircleAlert :size="20" />
          <div class="grow">
            <strong>加载失败</strong>
            <p>{{ error }}</p>
          </div>
          <button class="btn btn-secondary btn-sm" type="button" @click="load(true)">重试</button>
        </div>

        <div v-else-if="loading && !items.length" class="message-list">
          <div v-for="n in 4" :key="n" class="card" style="padding: 18px">
            <div class="skeleton" style="width: 35%; height: 12px"></div>
            <div class="skeleton mt-12" style="width: 70%; height: 17px"></div>
            <div class="skeleton mt-8" style="width: 92%; height: 10px"></div>
            <div class="skeleton mt-4" style="width: 78%; height: 10px"></div>
          </div>
        </div>

        <div v-else-if="items.length" class="message-list">
          <MessageCard
            v-for="msg in items"
            :key="msg.id"
            :message="msg"
            :lock="state.locks[msg.id]"
            :is-admin="isAdmin"
            :me="state.me"
            @open="openDetail"
            @edit="openEditor"
            @delete="askDelete"
            @pin="togglePin"
          />
        </div>

        <div v-else class="card">
          <EmptyState
            :icon="search || activeFilterCount ? SearchX : MessageSquarePlus"
            :title="search || activeFilterCount ? '没有匹配的留言' : '留言板还是空的'"
            :copy="search || activeFilterCount ? '试试调整搜索词或清除筛选条件。' : '发布第一条留言，让 Agent 团队开始共享上下文。'"
          >
            <button v-if="search || activeFilterCount" class="btn btn-secondary" type="button" @click="clearAll">清除筛选</button>
            <button v-else class="btn btn-primary" type="button" @click="openCompose"><Plus :size="15" /> 发布留言</button>
          </EmptyState>
        </div>

        <div v-if="hasMore" class="load-more">
          <button class="btn btn-secondary" type="button" :disabled="loading" @click="loadMore">
            <span v-if="loading" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
            <ChevronDown v-else :size="15" />
            加载更早留言
          </button>
        </div>
      </section>

      <aside class="board-rail">
        <div class="card rail-card live-card">
          <div class="rail-live-head">
            <span class="pulse-orb"><Radio :size="17" /></span>
            <div class="grow">
              <div class="card-title">实时协作</div>
              <div class="card-sub">{{ state.connection === 'live' ? 'SSE 推送连接正常' : '正在恢复连接' }}</div>
            </div>
            <span class="badge" :class="state.connection === 'live' ? 'badge-teal' : 'badge-gray'">{{ state.connection === 'live' ? 'LIVE' : 'SYNC' }}</span>
          </div>
          <div class="rail-stats">
            <div><strong>{{ stats?.active || 0 }}</strong><span>活跃留言</span></div>
            <div><strong>{{ state.agents.filter((a) => a.configured).length }}</strong><span>已接入</span></div>
            <div><strong>{{ eventCount }}</strong><span>本次事件</span></div>
          </div>
          <div class="rail-avatars">
            <span v-for="agent in state.agents.slice(0, 5)" :key="agent.name" class="avatar avatar-sm" :style="{ background: agent.color }" :title="agent.name">{{ initials(agent.name) }}</span>
            <span v-if="state.agents.length > 5" class="avatar avatar-sm more-avatar">+{{ state.agents.length - 5 }}</span>
          </div>
        </div>

        <div class="card rail-card policy-card">
          <div class="card-header" style="padding: 18px 18px 0">
            <div>
              <div class="card-title">归档容量</div>
              <div class="card-sub">{{ policy?.retain_days || 7 }} 天 · {{ policy?.max_items || 200 }} 条上限</div>
            </div>
            <RouterLink class="mini-action" to="/settings"><Settings2 :size="15" /></RouterLink>
          </div>
          <div class="card-body" style="padding: 16px 18px">
            <div class="policy-number"><strong>{{ stats?.active || 0 }}</strong><span>/ {{ policy?.max_items || 200 }}</span></div>
            <div class="progress" :class="{ warn: capacity >= 80, danger: capacity >= 96 }"><span :style="{ width: `${Math.min(100, capacity)}%` }"></span></div>
            <div class="policy-note">
              <Archive :size="12" />
              <span>超过限制后，最早的 {{ policy?.pinned_protected ? '非置顶' : '' }}留言会移入归档。</span>
            </div>
          </div>
        </div>

        <div class="card rail-card quick-card">
          <div class="quick-title"><Plug :size="15" /> Agent 快速接入</div>
          <div class="quick-endpoint mono">{{ mcpURL }}</div>
          <button class="btn btn-secondary btn-sm btn-block" type="button" @click="copyMCP"><Copy :size="13" /> 复制 MCP 地址</button>
        </div>
      </aside>
    </div>

    <MessageEditor
      v-if="editorOpen"
      :mode="editingMessage ? 'edit' : 'create'"
      :message="editingMessage"
      @close="closeEditor"
      @saved="onSaved"
    />

    <MessageDetail
      v-if="detailMessage && !editorOpen"
      :message="detailMessage"
      :lock="state.locks[detailMessage.id]"
      :is-admin="isAdmin"
      :me="state.me"
      @close="closeDetail"
      @edit="openEditor"
      @delete="askDelete"
      @pin="togglePin"
      @archive="archiveMessage"
    />

    <Modal v-if="deleteTarget" title="删除这条留言？" subtitle="删除后将从活跃列表和实时流中移除。该操作会留下审计记录。" @close="deleteTarget = null">
      <div class="delete-preview">
        <div class="delete-preview-title">{{ displayTitle(deleteTarget.title, deleteTarget.content) }}</div>
        <div class="delete-preview-meta">{{ deleteTarget.author }} · v{{ deleteTarget.version }} · {{ timeAgo(deleteTarget.created_at) }}</div>
        <p>{{ excerpt(deleteTarget.content, 220) }}</p>
      </div>
      <div class="info-banner mt-16">
        <CircleAlert :size="15" />
        <span>如果你只是想让它退出活跃列表，可以请管理员使用「归档」；归档内容之后仍可查询与恢复。</span>
      </div>
      <template #footer>
        <button class="btn btn-ghost" type="button" @click="deleteTarget = null">取消</button>
        <button class="btn btn-danger" type="button" :disabled="deleting" @click="confirmDelete">
          <span v-if="deleting" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
          <Trash2 v-else :size="14" /> 删除留言
        </button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Archive,
  ChevronDown,
  CircleAlert,
  Copy,
  MessageSquarePlus,
  Plug,
  Plus,
  Radio,
  RefreshCw,
  RotateCcw,
  Search,
  SearchX,
  Settings2,
  SlidersHorizontal,
  Trash2,
  X,
} from 'lucide-vue-next'
import MessageCard from '@/components/MessageCard.vue'
import MessageEditor from '@/components/MessageEditor.vue'
import MessageDetail from '@/components/MessageDetail.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import type { Message, MessageList, StreamEvent } from '@/lib/types'
import { api, copyText, displayTitle, excerpt, initials, queryString, timeAgo } from '@/lib/api'
import { errorMessage, notify, onStream, refreshCore, refreshStatsSoon, state } from '@/stores/board'

const route = useRoute()
const router = useRouter()

const items = ref<Message[]>([])
const total = ref(0)
const hasMore = ref(false)
const loading = ref(false)
const error = ref('')
const search = ref((route.query.q as string) || '')
const mode = ref<'all' | 'pinned' | 'urgent'>('all')
const sort = ref('newest')
const author = ref('')
const priority = ref('')
const tag = ref('')
const filtersOpen = ref(false)
const editorOpen = ref(false)
const editingMessage = ref<Message | null>(null)
const selectedID = ref('')
const deleteTarget = ref<Message | null>(null)
const deleting = ref(false)
const eventCount = ref(0)

let requestSeq = 0
let searchTimer: number | null = null
let reloadTimer: number | null = null
let unsubscribe: (() => void) | null = null
let abortController: AbortController | null = null

const isAdmin = computed(() => state.me?.role === 'admin')
const detailMessage = computed(() => items.value.find((m) => m.id === selectedID.value) || null)
const activeFilterCount = computed(() => (author.value ? 1 : 0) + (priority.value ? 1 : 0) + (tag.value ? 1 : 0))
const stats = computed(() => state.stats)
const policy = computed(() => state.policy)
const capacity = computed(() => state.stats?.capacity_percent || 0)
const mcpURL = computed(() => `${window.location.origin}/mcp`)

const filters = computed(() => ({
  status: 'active',
  q: search.value.trim() || undefined,
  sort: sort.value,
  pinned: mode.value === 'pinned' ? true : undefined,
  priority: mode.value === 'urgent' ? 'urgent' : priority.value || undefined,
  author: author.value || undefined,
  tag: tag.value.trim() || undefined,
}))

function setMode(next: 'all' | 'pinned' | 'urgent') {
  mode.value = next
}

async function load(reset = false) {
  if (reset) {
    items.value = []
    total.value = 0
  }
  if (abortController) abortController.abort()
  abortController = new AbortController()
  const seq = ++requestSeq
  loading.value = true
  error.value = ''
  try {
    const offset = reset ? 0 : items.value.length
    const query = queryString({ ...filters.value, limit: 30, offset })
    const res = await api.get<MessageList>(`/api/v1/messages${query}`, abortController.signal)
    if (seq !== requestSeq) return
    items.value = reset ? res.items : [...items.value, ...res.items.filter((item) => !items.value.some((x) => x.id === item.id))]
    total.value = res.total
    hasMore.value = res.has_more
  } catch (err: any) {
    if (err?.name === 'AbortError') return
    error.value = errorMessage(err)
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

async function loadMore() {
  if (loading.value || !hasMore.value) return
  const seq = requestSeq
  loading.value = true
  try {
    const res = await api.get<MessageList>(`/api/v1/messages${queryString({ ...filters.value, limit: 30, offset: items.value.length })}`)
    if (seq !== requestSeq) return
    items.value = [...items.value, ...res.items.filter((item) => !items.value.some((x) => x.id === item.id))]
    total.value = res.total
    hasMore.value = res.has_more
  } catch (err) {
    notify('error', '加载更多失败', errorMessage(err))
  } finally {
    loading.value = false
  }
}

function scheduleReload() {
  if (reloadTimer) window.clearTimeout(reloadTimer)
  reloadTimer = window.setTimeout(() => load(true), 700)
}

function handleStream(event: StreamEvent) {
  eventCount.value++
  if (event.type === 'stream.reconnected') {
    scheduleReload()
  } else if (event.type === 'message.created') {
    if (!search.value && !activeFilterCount.value && mode.value === 'all' && sort.value === 'newest' && !items.value.some((m) => m.id === event.data.id)) {
      items.value.unshift(event.data)
      total.value++
    } else if (mode.value === 'pinned' && event.data.pinned) {
      scheduleReload()
    } else if (search.value || activeFilterCount.value) {
      scheduleReload()
    }
  } else if (event.type === 'message.updated') {
    const idx = items.value.findIndex((m) => m.id === event.data.id)
    if (idx >= 0) items.value[idx] = event.data
  } else if (event.type === 'message.deleted' || event.type === 'message.archived') {
    const idx = items.value.findIndex((m) => m.id === event.data.id)
    if (idx >= 0) {
      items.value.splice(idx, 1)
      total.value = Math.max(0, total.value - 1)
    }
    if (selectedID.value === event.data.id) selectedID.value = ''
  } else if (event.type === 'message.restored') {
    scheduleReload()
  }
  refreshStatsSoon()
}

function openCompose() {
  editingMessage.value = null
  editorOpen.value = true
}

function openEditor(message: Message) {
  editingMessage.value = message
  editorOpen.value = true
}

function closeEditor() {
  const wasCreate = !editingMessage.value
  editorOpen.value = false
  editingMessage.value = null
  if (wasCreate && route.query.compose) {
    const query = { ...route.query }
    delete query.compose
    router.replace({ path: '/board', query })
  }
}

function onSaved(message: Message) {
  const wasEditing = !!editingMessage.value
  closeEditor()
  if (message.archived_at) {
    items.value = items.value.filter((m) => m.id !== message.id)
  } else {
    const idx = items.value.findIndex((m) => m.id === message.id)
    if (idx >= 0) items.value[idx] = message
    else if (sort.value === 'newest' && !search.value && !activeFilterCount.value) {
      items.value.unshift(message)
      total.value++
    }
  }
  if (wasEditing) selectedID.value = message.id
  load(true)
}

function openDetail(message: Message) {
  selectedID.value = message.id
}

function closeDetail() {
  selectedID.value = ''
  if (route.query.open) {
    const query = { ...route.query }
    delete query.open
    router.replace({ path: '/board', query })
  }
}

async function togglePin(message: Message) {
  try {
    const updated = await api.patch<Message>(`/api/v1/messages/${message.id}`, {
      pinned: !message.pinned,
      expected_version: message.version,
    })
    const idx = items.value.findIndex((m) => m.id === updated.id)
    if (idx >= 0) items.value[idx] = updated
    if (selectedID.value === updated.id) selectedID.value = updated.id
    notify('success', updated.pinned ? '已置顶' : '已取消置顶')
  } catch (err: any) {
    if (err?.status === 409) {
      notify('warning', '版本已变化', '留言已被其他 Agent 更新，已刷新最新内容')
      load(true)
    } else {
      notify('error', '操作失败', errorMessage(err))
    }
  }
}

function askDelete(message: Message) {
  deleteTarget.value = message
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await api.delete(`/api/v1/messages/${deleteTarget.value.id}`)
    const id = deleteTarget.value.id
    items.value = items.value.filter((m) => m.id !== id)
    total.value = Math.max(0, total.value - 1)
    if (selectedID.value === id) selectedID.value = ''
    notify('success', '留言已删除')
    deleteTarget.value = null
    refreshStatsSoon()
  } catch (err) {
    notify('error', '删除失败', errorMessage(err))
  } finally {
    deleting.value = false
  }
}

async function archiveMessage(message: Message) {
  try {
    await api.post(`/api/v1/messages/${message.id}/archive`, { reason: 'manual' })
    items.value = items.value.filter((m) => m.id !== message.id)
    total.value = Math.max(0, total.value - 1)
    selectedID.value = ''
    notify('success', '已移入归档', '可在归档页恢复')
    refreshStatsSoon()
  } catch (err) {
    notify('error', '归档失败', errorMessage(err))
  }
}

function clearFilters() {
  author.value = ''
  priority.value = ''
  tag.value = ''
}

function clearAll() {
  search.value = ''
  clearFilters()
  mode.value = 'all'
}

async function openFromQuery() {
  const id = route.query.open as string
  if (!id) return
  try {
    const msg = await api.get<Message>(`/api/v1/messages/${id}`)
    if (!msg.archived_at) {
      if (!items.value.some((m) => m.id === msg.id)) items.value.unshift(msg)
      selectedID.value = msg.id
    }
  } catch {
    // A stale link may point at a deleted/archived message; ignore quietly.
  }
}

function copyMCP() {
  copyText(mcpURL.value).then((ok) => {
    if (ok) notify('success', 'MCP 地址已复制')
    else notify('error', '复制失败', '请手动选中文本复制')
  })
}

watch(search, () => {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => load(true), 320)
})

watch([mode, sort, author, priority, tag], () => load(true))

watch(
  () => route.query.q,
  (q) => {
    const next = (q as string) || ''
    if (next !== search.value) search.value = next
  },
)

watch(
  () => route.query.compose,
  (value) => {
    if (value === '1' && !editorOpen.value) openCompose()
  },
)

onMounted(async () => {
  await Promise.allSettled([refreshCore(), load(true)])
  openFromQuery()
  if (route.query.compose === '1') openCompose()
  unsubscribe = onStream(handleStream)
})

onBeforeUnmount(() => {
  unsubscribe?.()
  abortController?.abort()
  if (searchTimer) window.clearTimeout(searchTimer)
  if (reloadTimer) window.clearTimeout(reloadTimer)
})
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
.board-main {
  min-width: 0;
}
.toolbar-sort {
  width: 122px;
  height: 37px;
  font-size: 11px;
}
.filter-active {
  border-color: rgba(20, 184, 166, 0.35) !important;
  color: var(--primary) !important;
}
.filter-count {
  display: grid;
  width: 15px;
  height: 15px;
  place-items: center;
  border-radius: 50%;
  background: var(--primary);
  color: #fff;
  font-size: 8px;
}
.filter-panel {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr auto;
  gap: 12px;
  align-items: end;
  margin-bottom: 14px;
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface);
  box-shadow: var(--shadow-sm);
}
.filter-panel .btn {
  margin-bottom: 0;
}
.result-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
  padding: 0 3px;
  color: var(--muted);
  font-size: 10px;
}
.result-count {
  display: flex;
  align-items: center;
  gap: 5px;
}
.result-count strong {
  color: var(--text-soft);
  font-size: 12px;
}
.result-live {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted-2);
  font-size: 9px;
}
.result-live .live-dot {
  width: 5px;
  height: 5px;
  flex-basis: 5px;
}
.error-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px;
  border-color: rgba(244, 63, 94, 0.2);
  color: var(--rose);
}
.error-card strong {
  font-size: 12px;
}
.error-card p {
  margin-top: 2px;
  color: var(--muted);
  font-size: 10.5px;
}
.load-more {
  display: flex;
  justify-content: center;
  padding: 20px 0 4px;
}
.rail-card {
  overflow: hidden;
}
.live-card {
  padding: 18px;
  background: linear-gradient(145deg, rgba(20, 184, 166, 0.08), var(--surface) 60%);
}
.rail-live-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.pulse-orb {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border: 1px solid rgba(20, 184, 166, 0.2);
  border-radius: 11px;
  background: rgba(20, 184, 166, 0.11);
  color: var(--primary);
  box-shadow: 0 0 24px rgba(20, 184, 166, 0.12);
}
.rail-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
  margin-top: 15px;
  padding-top: 13px;
  border-top: 1px solid var(--border);
}
.rail-stats div {
  text-align: center;
}
.rail-stats strong {
  display: block;
  font-size: 17px;
  font-weight: 730;
  letter-spacing: -0.04em;
}
.rail-stats span {
  display: block;
  margin-top: 1px;
  color: var(--muted);
  font-size: 8px;
}
.rail-avatars {
  display: flex;
  margin-top: 13px;
}
.rail-avatars .avatar {
  margin-left: -7px;
  border: 2px solid var(--surface-solid);
  box-shadow: none;
}
.rail-avatars .avatar:first-child {
  margin-left: 0;
}
.more-avatar {
  background: var(--surface-soft) !important;
  color: var(--muted) !important;
  font-size: 8px !important;
}
.policy-number {
  display: flex;
  align-items: baseline;
  gap: 3px;
  margin-bottom: 8px;
}
.policy-number strong {
  font-size: 22px;
  font-weight: 750;
  letter-spacing: -0.04em;
  line-height: 1;
}
.policy-number span {
  color: var(--muted-2);
  font-size: 10px;
}
.policy-note {
  display: flex;
  gap: 6px;
  margin-top: 10px;
  color: var(--muted-2);
  font-size: 9px;
  line-height: 1.5;
}
.policy-note svg {
  flex: 0 0 auto;
  margin-top: 1px;
}
.quick-card {
  padding: 15px;
}
.quick-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 9px;
  color: var(--text-soft);
  font-size: 10.5px;
  font-weight: 650;
}
.quick-title svg {
  color: var(--primary);
}
.quick-endpoint {
  overflow: hidden;
  margin-bottom: 9px;
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-soft);
  color: var(--muted);
  font-size: 8.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.delete-preview {
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: 11px;
  background: var(--surface-soft);
}
.delete-preview-title {
  font-size: 12.5px;
  font-weight: 650;
}
.delete-preview-meta {
  margin-top: 3px;
  color: var(--muted);
  font-size: 9.5px;
}
.delete-preview p {
  margin-top: 9px;
  color: var(--muted);
  font-size: 10.5px;
  line-height: 1.6;
  white-space: pre-wrap;
}
@media (max-width: 860px) {
  .filter-panel {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 640px) {
  .filter-panel {
    grid-template-columns: 1fr;
  }
  .toolbar-sort {
    flex: 1;
  }
}
</style>
