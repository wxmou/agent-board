<template>
  <div class="page keys-page">
    <div class="page-head">
      <div>
        <div class="eyebrow"><span class="pulse-dot"></span> ACCESS CONTROL · MCP ONLY</div>
        <h1 class="page-title">MCP Keys</h1>
        <p class="page-sub">为每位开发者的 Agent 创建独立认证 Key。Key 只在创建或轮换时显示一次，服务端仅保存哈希。</p>
      </div>
      <div class="page-actions">
        <button class="btn btn-secondary" type="button" :disabled="loading" @click="load">
          <RefreshCw :size="15" :class="{ spin: loading }" /> 刷新
        </button>
        <button v-if="isAdmin" class="btn btn-primary" type="button" @click="openCreate">
          <Plus :size="16" /> 创建 Key
        </button>
      </div>
    </div>

    <div v-if="!isAdmin" class="card">
      <EmptyState :icon="ShieldAlert" title="需要管理员权限" copy="MCP Key 列表与创建能力仅对 admin 角色开放。" />
    </div>

    <template v-else>
      <div class="key-stats">
        <div class="key-stat">
          <span class="key-stat-icon teal"><KeyRound :size="17" /></span>
          <div><strong>{{ keys.length }}</strong><span>Key 总数</span></div>
        </div>
        <div class="key-stat">
          <span class="key-stat-icon emerald"><ShieldCheck :size="17" /></span>
          <div><strong>{{ activeCount }}</strong><span>当前可用</span></div>
        </div>
        <div class="key-stat">
          <span class="key-stat-icon amber"><Timer :size="17" /></span>
          <div><strong>{{ expiringSoon }}</strong><span>14 天内到期</span></div>
        </div>
        <div class="key-stat">
          <span class="key-stat-icon blue"><Activity :size="17" /></span>
          <div><strong>{{ usedRecently }}</strong><span>24h 内使用</span></div>
        </div>
      </div>

      <div class="info-banner mt-16">
        <ShieldCheck :size="15" />
        <span>MCP Key 仅用于 <code>POST /mcp</code> 认证，不能登录 Web 控制台。撤销后立即失效；轮换会同时废止旧 Key。</span>
        <div class="endpoint-chip mono">{{ mcpURL }} <button class="mini-action" type="button" title="复制 MCP 地址" @click="copy(mcpURL, 'MCP 地址')"><Copy :size="12" /></button></div>
      </div>

      <div class="toolbar mt-16">
        <div class="search-box">
          <Search :size="15" />
          <input v-model="search" type="search" placeholder="搜索名称、创建人或前缀…" />
          <button v-if="search" class="search-clear" type="button" @click="search = ''"><X :size="13" /></button>
        </div>
        <div class="segmented">
          <button type="button" :class="{ active: filter === 'all' }" @click="filter = 'all'">全部</button>
          <button type="button" :class="{ active: filter === 'active' }" @click="filter = 'active'">可用</button>
          <button type="button" :class="{ active: filter === 'expired' }" @click="filter = 'expired'">已过期</button>
          <button type="button" :class="{ active: filter === 'revoked' }" @click="filter = 'revoked'">已撤销</button>
        </div>
      </div>

      <div v-if="loading && !keys.length" class="card" style="padding: 50px"><div class="loading-block"><span class="spinner"></span> 加载 Keys…</div></div>

      <div v-else-if="filteredKeys.length" class="card table-card">
        <div class="table-wrap">
          <table class="data-table key-table">
            <thead>
              <tr>
                <th>名称 / 前缀</th>
                <th>角色</th>
                <th>创建人</th>
                <th>最近使用</th>
                <th>到期时间</th>
                <th>状态</th>
                <th style="text-align: right">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="key in filteredKeys" :key="key.id" :class="{ revoked: key.status === 'revoked' }">
                <td>
                  <div class="key-name">{{ key.name }}</div>
                  <div class="key-prefix mono">{{ key.prefix }}<span>••••••••</span></div>
                </td>
                <td><span class="badge" :class="key.role === 'admin' ? 'badge-amber' : 'badge-teal'">{{ key.role }}</span></td>
                <td>{{ key.created_by }}</td>
                <td>
                  <span v-if="key.last_used_at">{{ timeAgo(key.last_used_at) }}</span>
                  <span v-else class="muted">从未</span>
                </td>
                <td>
                  <span v-if="!key.expires_at" class="muted">永不过期</span>
                  <span v-else :class="{ 'expiry-warn': isExpiring(key) }">{{ formatDate(key.expires_at, false) }}</span>
                </td>
                <td>
                  <span class="badge" :class="statusMeta[key.status].badge">{{ statusMeta[key.status].label }}</span>
                  <div v-if="key.status === 'revoked' && key.revoked_by" class="status-note">by {{ key.revoked_by }}</div>
                </td>
                <td>
                  <div class="row-actions">
                    <button v-if="key.status === 'active'" class="btn btn-ghost btn-sm" type="button" @click="openRotate(key)">
                      <RefreshCw :size="13" /> 轮换
                    </button>
                    <button v-if="key.status === 'active'" class="btn btn-ghost btn-sm danger" type="button" @click="revokeTarget = key">
                      <Ban :size="13" /> 撤销
                    </button>
                    <span v-else class="muted" style="font-size: 9.5px">—</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else class="card">
        <EmptyState :icon="KeyRound" title="还没有 MCP Key" copy="为开发者创建一个独立 Key，并配置到其 Agent 的 MCP headers 中。">
          <button class="btn btn-primary" type="button" @click="openCreate"><Plus :size="15" /> 创建第一个 Key</button>
        </EmptyState>
      </div>

      <div class="keys-footnote">
        <LockKeyhole :size="12" />
        <span>建议为每个开发者或 Agent 单独创建 Key；定期轮换，离职或停用时立即撤销。</span>
      </div>
    </template>

    <Modal v-if="createOpen" title="创建 MCP Key" subtitle="创建后完整 Key 只展示一次，请立即配置到 Agent。" @close="closeCreate">
      <form class="stacked-form" @submit.prevent="createKey">
        <div class="field">
          <label class="field-label" for="key-name">名称 <span class="required">*</span></label>
          <input id="key-name" v-model="form.name" class="input" maxlength="64" placeholder="例如 dev-alice / agent-frontend" autofocus />
          <span class="field-hint">这个名称会成为该 Agent 在留言与审计记录中的身份。</span>
        </div>

        <div class="field">
          <label class="field-label">角色</label>
          <div class="role-picker">
            <button type="button" :class="{ active: form.role === 'writer' }" @click="form.role = 'writer'">
              <span><strong>Writer</strong><small>读写、编辑、删除自己的留言</small></span>
            </button>
            <button type="button" :class="{ active: form.role === 'admin' }" @click="form.role = 'admin'">
              <span><strong>Admin</strong><small>额外管理归档、策略和 Key</small></span>
            </button>
          </div>
        </div>

        <div class="field">
          <label class="field-label">有效期</label>
          <div class="expiry-picker">
            <button v-for="option in expiryOptions" :key="option.value" type="button" :class="{ active: form.expiresDays === option.value }" @click="form.expiresDays = option.value">
              {{ option.label }}
            </button>
          </div>
        </div>

        <div class="info-banner">
          <ShieldCheck :size="14" />
          <span>服务端只保存 Key 的 SHA-256 哈希，之后无法再次查看完整内容。</span>
        </div>

        <div v-if="formError" class="form-error"><CircleAlert :size="14" /> {{ formError }}</div>
      </form>
      <template #footer>
        <button class="btn btn-ghost" type="button" @click="closeCreate">取消</button>
        <button class="btn btn-primary" type="button" :disabled="creating || !form.name.trim()" @click="createKey">
          <span v-if="creating" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
          <KeyRound v-else :size="14" /> 创建 Key
        </button>
      </template>
    </Modal>

    <Modal v-if="revealed" title="MCP Key 已创建" subtitle="这是唯一一次显示完整 Key。关闭后无法再次获取。" size="lg" :close-on-backdrop="false" @close="closeRevealed">
      <div class="reveal-banner">
        <div class="reveal-icon"><ShieldCheck :size="20" /></div>
        <div>
          <strong>{{ revealed.record.name }}</strong>
          <p>角色 {{ revealed.record.role }} · {{ revealed.record.expires_at ? `到期 ${formatDate(revealed.record.expires_at, false)}` : '永不过期' }}</p>
        </div>
      </div>
      <div class="field mt-16">
        <label class="field-label">完整 Key</label>
        <div class="reveal-input">
          <input class="input mono" :value="revealed.key" readonly @focus="($event.target as HTMLInputElement).select()" />
          <button class="btn btn-secondary" type="button" @click="copy(revealed.key, 'MCP Key')"><Copy :size="14" /> 复制</button>
        </div>
      </div>
      <div class="field mt-16">
        <label class="field-label">MCP Endpoint</label>
        <div class="reveal-input">
          <input class="input mono" :value="revealed.mcp_url" readonly @focus="($event.target as HTMLInputElement).select()" />
          <button class="btn btn-secondary" type="button" @click="copy(revealed.mcp_url, 'MCP 地址')"><Copy :size="14" /> 复制</button>
        </div>
      </div>
      <div class="code-block mt-16">Authorization: Bearer &lt;{{ revealed.record.prefix }}…&gt;</div>
      <div class="info-banner mt-16">
        <TriangleAlert :size="15" />
        <span>请将 Key 存入 Agent 的 Secret Manager，不要提交到仓库、日志或聊天记录。关闭本窗口后只能通过轮换生成新 Key。</span>
      </div>
      <template #footer>
        <button class="btn btn-primary" type="button" @click="closeRevealed">我已保存，关闭</button>
      </template>
    </Modal>

    <Modal v-if="revokeTarget" title="撤销 MCP Key？" subtitle="撤销后使用该 Key 的 Agent 会立即收到 401。" @close="revokeTarget = null">
      <div class="confirm-box">
        <div><span>名称</span><strong>{{ revokeTarget.name }}</strong></div>
        <div><span>前缀</span><strong class="mono">{{ revokeTarget.prefix }}…</strong></div>
        <div><span>角色</span><strong>{{ revokeTarget.role }}</strong></div>
      </div>
      <div class="info-banner mt-16">
        <TriangleAlert :size="15" />
        <span>此操作不可恢复。若只是更换 Key，请使用「轮换」，它会创建新 Key 并自动撤销旧 Key。</span>
      </div>
      <template #footer>
        <button class="btn btn-ghost" type="button" @click="revokeTarget = null">取消</button>
        <button class="btn btn-danger" type="button" :disabled="revoking" @click="confirmRevoke">
          <span v-if="revoking" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
          <Ban v-else :size="14" /> 确认撤销
        </button>
      </template>
    </Modal>

    <Modal v-if="rotateTarget" title="轮换 MCP Key" subtitle="旧 Key 会立即失效，新 Key 只显示一次。" @close="rotateTarget = null">
      <div class="confirm-box">
        <div><span>名称</span><strong>{{ rotateTarget.name }}</strong></div>
        <div><span>前缀</span><strong class="mono">{{ rotateTarget.prefix }}…</strong></div>
      </div>
      <div class="field mt-16">
        <label class="field-label">新 Key 有效期</label>
        <div class="expiry-picker">
          <button v-for="option in expiryOptions" :key="option.value" type="button" :class="{ active: rotateDays === option.value }" @click="rotateDays = option.value">
            {{ option.label }}
          </button>
        </div>
      </div>
      <template #footer>
        <button class="btn btn-ghost" type="button" @click="rotateTarget = null">取消</button>
        <button class="btn btn-primary" type="button" :disabled="rotating" @click="confirmRotate">
          <span v-if="rotating" class="spinner" style="width: 13px; height: 13px; border-width: 1.5px"></span>
          <RefreshCw v-else :size="14" /> 轮换并生成新 Key
        </button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  Activity,
  Ban,
  CircleAlert,
  Copy,
  KeyRound,
  LockKeyhole,
  Plus,
  RefreshCw,
  Search,
  ShieldAlert,
  ShieldCheck,
  Timer,
  TriangleAlert,
  X,
} from 'lucide-vue-next'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import type { MCPKey, MCPKeyCreated, StreamEvent } from '@/lib/types'
import { api, copyText, formatDate, timeAgo } from '@/lib/api'
import { errorMessage, notify, onStream, state } from '@/stores/board'

const keys = ref<MCPKey[]>([])
const loading = ref(false)
const creating = ref(false)
const revoking = ref(false)
const rotating = ref(false)
const search = ref('')
const filter = ref<'all' | 'active' | 'expired' | 'revoked'>('all')
const createOpen = ref(false)
const revealed = ref<MCPKeyCreated | null>(null)
const revokeTarget = ref<MCPKey | null>(null)
const rotateTarget = ref<MCPKey | null>(null)
const rotateDays = ref(90)
const formError = ref('')
const form = reactive({ name: '', role: 'writer' as 'writer' | 'admin', expiresDays: 90 })
let unsubscribe: (() => void) | null = null

const isAdmin = computed(() => state.me?.role === 'admin')
const mcpURL = computed(() => `${window.location.origin}/mcp`)
const activeCount = computed(() => keys.value.filter((k) => k.status === 'active').length)
const expiringSoon = computed(() => keys.value.filter((k) => isExpiring(k) && k.status === 'active').length)
const usedRecently = computed(() => keys.value.filter((k) => k.last_used_at && Date.now() - new Date(k.last_used_at).getTime() < 24 * 3600 * 1000).length)
const filteredKeys = computed(() => {
  const q = search.value.trim().toLowerCase()
  return keys.value.filter((key) => {
    if (filter.value !== 'all' && key.status !== filter.value) return false
    if (!q) return true
    return [key.name, key.created_by, key.prefix].some((value) => value.toLowerCase().includes(q))
  })
})

const statusMeta: Record<MCPKey['status'], { label: string; badge: string }> = {
  active: { label: '可用', badge: 'badge-teal' },
  expired: { label: '已过期', badge: 'badge-gray' },
  revoked: { label: '已撤销', badge: 'badge-red' },
}

const expiryOptions = [
  { value: 30, label: '30 天' },
  { value: 90, label: '90 天' },
  { value: 180, label: '180 天' },
  { value: 365, label: '1 年' },
  { value: 0, label: '永不过期' },
]

function isExpiring(key: MCPKey) {
  if (!key.expires_at) return false
  const diff = new Date(key.expires_at).getTime() - Date.now()
  return diff > 0 && diff < 14 * 24 * 3600 * 1000
}

async function load() {
  loading.value = true
  try {
    const res = await api.get<{ items: MCPKey[] }>('/api/v1/mcp-keys')
    keys.value = res.items
  } catch (err) {
    notify('error', '加载 MCP Keys 失败', errorMessage(err))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.name = ''
  form.role = 'writer'
  form.expiresDays = 90
  formError.value = ''
  createOpen.value = true
}

function closeCreate() {
  if (creating.value) return
  createOpen.value = false
}

async function createKey() {
  if (creating.value || !form.name.trim()) return
  creating.value = true
  formError.value = ''
  try {
    const result = await api.post<MCPKeyCreated>('/api/v1/mcp-keys', {
      name: form.name.trim(),
      role: form.role,
      expires_days: form.expiresDays,
    })
    createOpen.value = false
    revealed.value = result
    notify('success', 'MCP Key 已创建', '请立即复制并配置到 Agent')
    await load()
  } catch (err: any) {
    formError.value = errorMessage(err)
    if (err?.status === 409) formError.value = '已存在同名且未撤销的 Key'
  } finally {
    creating.value = false
  }
}

function closeRevealed() {
  revealed.value = null
}

async function confirmRevoke() {
  if (!revokeTarget.value) return
  revoking.value = true
  try {
    await api.delete(`/api/v1/mcp-keys/${revokeTarget.value.id}`)
    notify('success', 'Key 已撤销', `${revokeTarget.value.name} 的 Agent 将无法继续调用 MCP`)
    revokeTarget.value = null
    await load()
  } catch (err) {
    notify('error', '撤销失败', errorMessage(err))
  } finally {
    revoking.value = false
  }
}

function openRotate(key: MCPKey) {
  rotateTarget.value = key
  rotateDays.value = 90
}

async function confirmRotate() {
  if (!rotateTarget.value) return
  rotating.value = true
  try {
    const result = await api.post<MCPKeyCreated>(`/api/v1/mcp-keys/${rotateTarget.value.id}/rotate`, { expires_days: rotateDays.value })
    rotateTarget.value = null
    revealed.value = result
    notify('success', 'Key 已轮换', '旧 Key 已立即失效')
    await load()
  } catch (err) {
    notify('error', '轮换失败', errorMessage(err))
  } finally {
    rotating.value = false
  }
}

async function copy(value: string, label: string) {
  const ok = await copyText(value)
  if (ok) notify('success', `${label}已复制`)
  else notify('warning', '复制失败', '请手动选中文本复制')
}

function onKeyEvent(event: StreamEvent) {
  if (event.type.startsWith('mcp_key.')) load()
}

onMounted(async () => {
  if (!isAdmin.value) return
  await load()
  unsubscribe = onStream(onKeyEvent)
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
.pulse-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-soft);
}
.key-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: var(--shadow-card);
  backdrop-filter: blur(16px);
}
.key-stat {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 17px 19px;
  border-right: 1px solid var(--border);
}
.key-stat:last-child {
  border-right: none;
}
.key-stat-icon {
  display: grid;
  width: 35px;
  height: 35px;
  flex: 0 0 35px;
  place-items: center;
  border-radius: 10px;
}
.key-stat-icon.teal { background: var(--primary-soft); color: var(--primary); }
.key-stat-icon.emerald { background: rgba(16, 185, 129, 0.1); color: var(--emerald); }
.key-stat-icon.amber { background: rgba(245, 158, 11, 0.1); color: var(--amber); }
.key-stat-icon.blue { background: rgba(59, 130, 246, 0.1); color: #60a5fa; }
.key-stat strong {
  display: block;
  font-size: 20px;
  font-weight: 740;
  letter-spacing: -0.04em;
  line-height: 1.1;
}
.key-stat span {
  display: block;
  margin-top: 2px;
  color: var(--muted);
  font-size: 8.5px;
}
.endpoint-chip {
  display: flex;
  align-items: center;
  gap: 3px;
  margin-left: auto;
  padding: 5px 6px 5px 9px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-soft);
  color: var(--muted);
  font-size: 8.5px;
  white-space: nowrap;
}
.table-card {
  overflow: hidden;
}
.key-table tbody tr.revoked {
  opacity: 0.55;
}
.key-name {
  color: var(--text);
  font-size: 12px;
  font-weight: 650;
}
.key-prefix {
  margin-top: 2px;
  color: var(--muted);
  font-size: 9px;
}
.key-prefix span {
  color: var(--muted-2);
}
.status-note {
  margin-top: 2px;
  color: var(--muted-2);
  font-size: 8px;
}
.expiry-warn {
  color: var(--amber);
}
.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
}
.btn-ghost.danger {
  color: var(--rose);
}
.keys-footnote {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 14px;
  color: var(--muted-2);
  font-size: 9.5px;
}
.role-picker {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 7px;
}
.role-picker button {
  padding: 11px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-soft);
  color: var(--muted);
  text-align: left;
  cursor: pointer;
}
.role-picker button.active {
  border-color: rgba(20, 184, 166, 0.34);
  background: var(--primary-soft);
  color: var(--text);
  box-shadow: 0 0 0 2px rgba(20, 184, 166, 0.08);
}
.role-picker strong {
  display: block;
  font-size: 11px;
}
.role-picker small {
  display: block;
  margin-top: 2px;
  color: var(--muted);
  font-size: 8.5px;
  line-height: 1.45;
}
.expiry-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
.expiry-picker button {
  height: 31px;
  padding: 0 11px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-soft);
  color: var(--muted);
  font-size: 10px;
  cursor: pointer;
}
.expiry-picker button.active {
  border-color: rgba(20, 184, 166, 0.34);
  background: var(--primary-soft);
  color: var(--primary);
}
.form-error {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 11px;
  border: 1px solid rgba(244, 63, 94, 0.2);
  border-radius: 9px;
  background: rgba(244, 63, 94, 0.07);
  color: #fb7185;
  font-size: 10px;
}
.reveal-banner {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 13px;
  border: 1px solid rgba(20, 184, 166, 0.2);
  border-radius: 11px;
  background: var(--primary-soft);
}
.reveal-icon {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border-radius: 10px;
  background: rgba(20, 184, 166, 0.14);
  color: var(--primary);
}
.reveal-banner strong {
  font-size: 12px;
}
.reveal-banner p {
  margin-top: 2px;
  color: var(--muted);
  font-size: 9.5px;
}
.reveal-input {
  display: flex;
  gap: 7px;
}
.reveal-input .input {
  font-size: 11px;
}
.confirm-box {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 10px;
}
.confirm-box div {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 10px 13px;
  border-bottom: 1px solid var(--border);
  font-size: 11px;
}
.confirm-box div:last-child {
  border-bottom: none;
}
.confirm-box span {
  color: var(--muted);
}
.confirm-box strong {
  color: var(--text-soft);
  font-weight: 600;
}
@media (max-width: 900px) {
  .key-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .key-stat:nth-child(2) {
    border-right: none;
  }
  .key-stat:nth-child(-n + 2) {
    border-bottom: 1px solid var(--border);
  }
  .endpoint-chip {
    display: none;
  }
}
@media (max-width: 640px) {
  .key-stats {
    grid-template-columns: 1fr;
  }
  .key-stat {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
  .role-picker {
    grid-template-columns: 1fr;
  }
  .reveal-input {
    flex-direction: column;
  }
}
</style>
