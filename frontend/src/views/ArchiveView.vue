<template>
  <div class="page archive-page">
    <div class="page-head">
      <div>
        <div class="eyebrow"><span class="dot"></span> LONG-TERM MEMORY · {{ policy?.retain_days || 7 }}D WINDOW</div>
        <h1 class="page-title">归档</h1>
        <p class="page-sub">自动沉淀的历史留言仍可搜索、审阅和恢复，不占用活跃协作空间。</p>
      </div>
      <div class="page-actions">
        <RouterLink to="/settings" class="btn btn-secondary"><Settings2 :size="15" /> 归档策略</RouterLink>
        <button v-if="isAdmin" class="btn btn-primary" type="button" @click="confirmArchiveOpen = true">
          <ArchiveRestore :size="15" /> 立即归档
        </button>
      </div>
    </div>

    <div class="archive-summary">
      <div class="archive-stat">
        <span class="archive-stat-icon amber"><Archive :size="17" /></span>
        <div><strong>{{ state.stats?.archived || 0 }}</strong><span>归档留言</span></div>
      </div>
      <div class="archive-stat">
        <span class="archive-stat-icon teal"><Layers3 :size="17" /></span>
        <div><strong>{{ policy?.max_items || 200 }}</strong><span>活跃上限</span></div>
      </div>
      <div class="archive-stat">
        <span class="archive-stat-icon blue"><Clock3 :size="17" /></span>
        <div><strong>{{ policy?.retain_days || 7 }} 天</strong><span>保留窗口</span></div>
      </div>
      <div class="archive-stat archive-policy">
        <div class="grow">
          <div class="archive-policy-label">下次自动清理</div>
          <div class="archive-policy-value">{{ policy?.auto_archive ? `每 ${policy.interval_minutes} 分钟检查` : '自动归档已关闭' }}</div>
        </div>
        <span class="switch" :class="{ on: policy?.auto_archive }" role="presentation"></span>
      </div>
    </div>

    <div class="toolbar mt-16">
      <div class="search-box">
        <Search :size="15" />
        <input v-model="search" type="search" placeholder="搜索归档留言…" />
        <button v-if="search" class="search-clear" type="button" @click="search = ''"><X :size="13" /></button>
      </div>
      <select v-model="author" class="select" style="width: 170px; height: 37px">
        <option value="">全部 Agents</option>
        <option v-for="agent in state.agents" :key="agent.name" :value="agent.name">{{ agent.name }}</option>
      </select>
      <select v-model="reason" class="select" style="width: 160px; height: 37px">
        <option value="">全部归档原因</option>
        <option value="retention_days">超过保留天数</option>
        <option value="max_items">超过条数上限</option>
        <option value="manual">手动归档</option>
      </select>
      <button class="btn btn-secondary btn-sm" type="button" @click="load(true)"><RefreshCw :size="14" :class="{ spin: loading }" /> 刷新</button>
    </div>

    <div class="result-bar">
      <span><strong>{{ total }}</strong> 条归档记录</span>
      <span>{{ state.connection === 'live' ? '实时同步' : '等待重连' }}</span>
    </div>

    <div v-if="loading && !items.length" class="card" style="padding: 40px"><div class="loading-block"><span class="spinner"></span> 加载归档…</div></div>

    <div v-else-if="items.length" class="message-list">
      <MessageCard
        v-for="msg in items"
        :key="msg.id"
        :message="msg"
        archived
        :is-admin="isAdmin"
        :me="state.me"
        @open="selectedID = msg.id"
        @restore="restore"
      />
    </div>

    <div v-else class="card">
      <EmptyState :icon="Archive" title="归档区是空的" copy="当留言超过保留天数或活跃条数上限时，最早的内容会自动移动到这里。" />
    </div>

    <div v-if="hasMore" class="load-more">
      <button class="btn btn-secondary" type="button" :disabled="loading" @click="loadMore">
        <span v-if="loading" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
        <ChevronDown v-else :size="15" /> 加载更多归档
      </button>
    </div>

    <section class="card runs-card mt-24">
      <div class="card-header">
        <div>
          <div class="card-title">归档运行记录</div>
          <div class="card-sub">自动与手动归档的审计轨迹</div>
        </div>
        <span class="badge badge-gray">{{ runs.length }} RUNS</span>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr><th>时间</th><th>触发方式</th><th>归档</th><th>超龄</th><th>超量</th><th>剩余活跃</th><th>耗时</th></tr>
          </thead>
          <tbody>
            <tr v-for="run in runs" :key="run.id">
              <td>{{ formatDate(run.started_at) }}</td>
              <td><span class="badge" :class="run.trigger === 'manual' ? 'badge-blue' : 'badge-gray'">{{ run.trigger === 'manual' ? '手动' : '自动' }}</span></td>
              <td><strong style="color: var(--text)">{{ run.archived_count }}</strong></td>
              <td>{{ run.retention_count }}</td>
              <td>{{ run.overflow_count }}</td>
              <td>{{ run.remaining }}</td>
              <td class="mono muted">{{ runDuration(run) }}</td>
            </tr>
            <tr v-if="!runs.length"><td colspan="7" style="text-align: center; color: var(--muted); padding: 28px">暂无运行记录</td></tr>
          </tbody>
        </table>
      </div>
    </section>

    <MessageDetail
      v-if="detailMessage"
      :message="detailMessage"
      :is-admin="isAdmin"
      :me="state.me"
      @close="selectedID = ''"
      @restore="restore"
      @edit="() => {}"
      @delete="() => {}"
      @pin="() => {}"
      @archive="() => {}"
    />

    <Modal v-if="confirmArchiveOpen" title="立即运行归档？" subtitle="将按当前策略把最早的超龄或超量留言移入归档。" @close="confirmArchiveOpen = false">
      <div class="archive-confirm">
        <div class="archive-confirm-row"><span>保留天数</span><strong>{{ policy?.retain_days }} 天</strong></div>
        <div class="archive-confirm-row"><span>活跃上限</span><strong>{{ policy?.max_items }} 条</strong></div>
        <div class="archive-confirm-row"><span>当前活跃</span><strong>{{ state.stats?.active }} 条</strong></div>
        <div class="archive-confirm-row"><span>置顶保护</span><strong>{{ policy?.pinned_protected ? '开启' : '关闭' }}</strong></div>
      </div>
      <div class="info-banner mt-16">
        <Info :size="15" />
        <span>归档后的留言仍可在此页面搜索与恢复，不会删除数据。</span>
      </div>
      <template #footer>
        <button class="btn btn-ghost" type="button" @click="confirmArchiveOpen = false">取消</button>
        <button class="btn btn-primary" type="button" :disabled="running" @click="runArchive">
          <span v-if="running" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
          <ArchiveRestore v-else :size="14" /> 运行归档
        </button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Archive, ArchiveRestore, ChevronDown, Clock3, Info, Layers3, RefreshCw, Search, Settings2, X } from 'lucide-vue-next'
import MessageCard from '@/components/MessageCard.vue'
import MessageDetail from '@/components/MessageDetail.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import type { ArchiveRun, Message, MessageList, StreamEvent } from '@/lib/types'
import { api, formatDate, queryString } from '@/lib/api'
import { errorMessage, notify, onStream, refreshCore, refreshStatsSoon, state } from '@/stores/board'

const items = ref<Message[]>([])
const total = ref(0)
const hasMore = ref(false)
const loading = ref(false)
const search = ref('')
const author = ref('')
const reason = ref('')
const selectedID = ref('')
const runs = ref<ArchiveRun[]>([])
const confirmArchiveOpen = ref(false)
const running = ref(false)
let searchTimer: number | null = null
let reloadTimer: number | null = null
let unsubscribe: (() => void) | null = null

const isAdmin = computed(() => state.me?.role === 'admin')
const policy = computed(() => state.policy)
const detailMessage = computed(() => items.value.find((m) => m.id === selectedID.value) || null)

const filters = computed(() => ({
  status: 'archived',
  q: search.value.trim() || undefined,
  author: author.value || undefined,
  reason: reason.value || undefined,
  sort: 'oldest',
}))

async function load(reset = false) {
  if (reset) items.value = []
  loading.value = true
  try {
    const res = await api.get<MessageList>(`/api/v1/messages${queryString({ ...filters.value, limit: 25, offset: reset ? 0 : items.value.length })}`)
    const incoming = res.items
    items.value = reset ? incoming : [...items.value, ...incoming.filter((m) => !items.value.some((x) => x.id === m.id))]
    total.value = res.total
    hasMore.value = res.has_more
  } catch (err) {
    notify('error', '加载归档失败', errorMessage(err))
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (loading.value || !hasMore.value) return
  await load(false)
}

async function loadRuns() {
  try {
    const res = await api.get<{ items: ArchiveRun[] }>('/api/v1/archive/runs?limit=8')
    runs.value = res.items
  } catch {
    // Runs history is supplementary.
  }
}

async function restore(message: Message) {
  try {
    await api.post(`/api/v1/messages/${message.id}/restore`)
    items.value = items.value.filter((m) => m.id !== message.id)
    total.value = Math.max(0, total.value - 1)
    selectedID.value = ''
    notify('success', '留言已恢复', '已回到活跃留言板')
    refreshStatsSoon()
  } catch (err) {
    notify('error', '恢复失败', errorMessage(err))
  }
}

async function runArchive() {
  running.value = true
  try {
    const result = await api.post<{ archived_count: number; retention_count: number; overflow_count: number }>('/api/v1/archive/run')
    confirmArchiveOpen.value = false
    if (result.archived_count > 0) {
      notify('success', `已归档 ${result.archived_count} 条留言`, `超龄 ${result.retention_count} 条 · 超量 ${result.overflow_count} 条`)
    } else {
      notify('info', '无需归档', '当前所有留言都在策略范围内')
    }
    await Promise.all([load(true), loadRuns(), refreshStatsSoon()])
  } catch (err) {
    notify('error', '归档运行失败', errorMessage(err))
  } finally {
    running.value = false
  }
}

function scheduleReload() {
  if (reloadTimer) window.clearTimeout(reloadTimer)
  reloadTimer = window.setTimeout(() => {
    load(true)
    loadRuns()
  }, 650)
}

function handleStream(event: StreamEvent) {
  if (event.type === 'stream.reconnected' || event.type === 'message.archived' || event.type === 'message.restored' || event.type === 'archive.run') scheduleReload()
}

function runDuration(run: ArchiveRun) {
  if (!run.finished_at) return '—'
  return `${Math.max(0, new Date(run.finished_at).getTime() - new Date(run.started_at).getTime())}ms`
}

watch(search, () => {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => load(true), 320)
})
watch([author, reason], () => load(true))

onMounted(async () => {
  await Promise.allSettled([refreshCore(), load(true), loadRuns()])
  unsubscribe = onStream(handleStream)
})

onBeforeUnmount(() => {
  unsubscribe?.()
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
  color: var(--amber);
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.14em;
}
.eyebrow .dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--amber);
}
.archive-summary {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1.35fr;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: var(--shadow-card);
  backdrop-filter: blur(16px);
}
.archive-stat {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 17px 19px;
  border-right: 1px solid var(--border);
}
.archive-stat:last-child {
  border-right: none;
}
.archive-stat-icon {
  display: grid;
  width: 35px;
  height: 35px;
  flex: 0 0 35px;
  place-items: center;
  border-radius: 10px;
}
.archive-stat-icon.amber {
  background: rgba(245, 158, 11, 0.1);
  color: var(--amber);
}
.archive-stat-icon.teal {
  background: var(--primary-soft);
  color: var(--primary);
}
.archive-stat-icon.blue {
  background: rgba(59, 130, 246, 0.1);
  color: #60a5fa;
}
.archive-stat strong {
  display: block;
  font-size: 17px;
  font-weight: 720;
  letter-spacing: -0.03em;
  line-height: 1.1;
}
.archive-stat span {
  display: block;
  margin-top: 2px;
  color: var(--muted);
  font-size: 8.5px;
}
.archive-policy-label {
  color: var(--muted);
  font-size: 8.5px;
}
.archive-policy-value {
  margin-top: 2px;
  color: var(--text-soft);
  font-size: 10.5px;
  font-weight: 620;
}
.result-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  padding: 0 3px;
  color: var(--muted);
  font-size: 10px;
}
.result-bar strong {
  color: var(--text-soft);
}
.load-more {
  display: flex;
  justify-content: center;
  padding: 18px 0;
}
.runs-card {
  margin-top: 24px;
}
.archive-confirm {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 11px;
}
.archive-confirm-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 13px;
  border-bottom: 1px solid var(--border);
  color: var(--muted);
  font-size: 11px;
}
.archive-confirm-row:last-child {
  border-bottom: none;
}
.archive-confirm-row strong {
  color: var(--text-soft);
  font-weight: 620;
}
@media (max-width: 1000px) {
  .archive-summary {
    grid-template-columns: 1fr 1fr;
  }
  .archive-stat:nth-child(2) {
    border-right: none;
  }
  .archive-stat:nth-child(-n + 2) {
    border-bottom: 1px solid var(--border);
  }
}
@media (max-width: 640px) {
  .archive-summary {
    grid-template-columns: 1fr;
  }
  .archive-stat {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
}
</style>
