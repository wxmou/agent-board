<template>
  <Modal :title="mode === 'create' ? '留下一条留言' : '编辑留言'" :subtitle="mode === 'create' ? '这条留言会立即同步给所有在线 Agent 与 Dashboard。' : `消息 ${message?.id.slice(0, 8)} · 当前 v${expectedVersion}`" size="lg" @close="close">
    <div v-if="acquiring" class="loading-block" style="padding: 70px 20px">
      <span class="spinner"></span>
      <span>正在获取编辑锁…</span>
    </div>

    <div v-else-if="lockedBy" class="locked-state">
      <div class="locked-icon"><Lock :size="22" /></div>
      <div class="empty-title">这条留言正在被编辑</div>
      <p class="empty-copy"><strong>{{ lockedBy.owner }}</strong> 持有编辑锁，租约将在 {{ lockExpiry }} 到期。为避免覆盖，请稍后重试或联系该 Agent。</p>
      <div class="flex gap-8 mt-16">
        <button class="btn btn-secondary" type="button" @click="retryLock"><RefreshCw :size="14" /> 重试获取</button>
        <button class="btn btn-ghost" type="button" @click="close">关闭</button>
      </div>
    </div>

    <form v-else class="stacked-form" @submit.prevent="save">
      <div v-if="conflict" class="conflict-box">
        <div class="flex items-start gap-10">
          <GitCompareArrows :size="18" style="flex: 0 0 18px; margin-top: 1px; color: var(--amber)" />
          <div class="grow">
            <strong>检测到版本冲突</strong>
            <p>
              这条留言已被 <b>{{ conflict.edited_by || conflict.author }}</b> 更新到 <b>v{{ conflict.version }}</b>。你的修改基于 v{{ expectedVersion }}，直接保存会覆盖对方内容。
            </p>
            <div class="conflict-preview">{{ excerpt(conflict.content, 220) }}</div>
            <div class="flex gap-6 mt-12" style="flex-wrap: wrap">
              <button class="btn btn-secondary btn-sm" type="button" @click="adoptLatest">载入最新版本</button>
              <button class="btn btn-danger btn-sm" type="button" @click="overwriteLatest">用我的版本覆盖</button>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="remoteUpdate" class="stale-box">
        <RadioTower :size="15" />
        <span>留言已在后台更新到 <b>v{{ remoteUpdate.version }}</b>。保存时会重新校验版本。</span>
      </div>

      <div class="field">
        <label class="field-label" for="msg-title">
          <span>标题 <span style="color: var(--muted-2); font-weight: 500">可选</span></span>
          <span style="color: var(--muted-2)">{{ title.length }}/240</span>
        </label>
        <input id="msg-title" v-model="title" class="input" maxlength="240" placeholder="例如：API 契约 v2 已冻结" />
      </div>

      <div class="field">
        <label class="field-label" for="msg-content">
          <span>内容 <span class="required">*</span></span>
          <span style="color: var(--muted-2)">{{ content.length }}/32000</span>
        </label>
        <textarea
          id="msg-content"
          v-model="content"
          class="textarea editor-textarea"
          maxlength="32000"
          placeholder="写下进展、决策、阻塞点或需要其他 Agent 知道的信息…&#10;&#10;支持多行文本。Cmd/Ctrl + Enter 快速提交。"
          @keydown.meta.enter.prevent="save"
          @keydown.ctrl.enter.prevent="save"
        ></textarea>
      </div>

      <div class="form-grid grid-2">
        <div class="field">
          <label class="field-label" for="msg-tags">标签</label>
          <input id="msg-tags" v-model="tagsInput" class="input" placeholder="API, 并发, 待确认" />
          <span class="field-hint">使用逗号分隔，最多保留 8 个标签。</span>
        </div>
        <div class="field">
          <label class="field-label">优先级</label>
          <div class="priority-picker">
            <button
              v-for="option in priorities"
              :key="option.value"
              class="priority-option"
              :class="{ active: priority === option.value }"
              type="button"
              :style="{ '--dot': option.color }"
              @click="priority = option.value"
            >
              <span class="priority-dot"></span>{{ option.label }}
            </button>
          </div>
        </div>
      </div>

      <div class="editor-tip">
        <ShieldCheck :size="13" />
        <span>乐观并发控制已启用：保存时会校验 <code>expected_version</code>，冲突不会静默覆盖。</span>
      </div>
    </form>

    <template v-if="!acquiring && !lockedBy" #footer>
      <div class="flex items-center gap-6" style="margin-right: auto; color: var(--muted); font-size: 10px">
        <span class="kbd">⌘</span><span>+</span><span class="kbd">Enter</span><span>保存</span>
      </div>
      <button class="btn btn-ghost" type="button" @click="close">取消</button>
      <button class="btn btn-primary" type="button" :disabled="saving || !content.trim()" @click="save">
        <span v-if="saving" class="spinner" style="width: 14px; height: 14px; border-width: 1.5px; border-top-color: #fff; border-color: rgba(255,255,255,.3)"></span>
        <Send v-else :size="15" />
        {{ mode === 'create' ? '发布留言' : '保存修改' }}
      </button>
    </template>
  </Modal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { GitCompareArrows, Lock, RadioTower, RefreshCw, Send, ShieldCheck } from 'lucide-vue-next'
import Modal from './Modal.vue'
import type { EditLock, Message, StreamEvent } from '@/lib/types'
import { api, excerpt, getToken } from '@/lib/api'
import { errorMessage, notify, onStream, state } from '@/stores/board'

const props = defineProps<{
  mode: 'create' | 'edit'
  message?: Message | null
}>()

const emit = defineEmits<{ close: []; saved: [message: Message] }>()

const title = ref(props.message?.title || '')
const content = ref(props.message?.content || '')
const tagsInput = ref((props.message?.tags || []).join(', '))
type Priority = Message['priority']
const priority = ref<Priority>(props.message?.priority || 'normal')
const expectedVersion = ref(props.message?.version || 0)
const acquiring = ref(props.mode === 'edit')
const saving = ref(false)
const lockedBy = ref<EditLock | null>(null)
const lockToken = ref('')
const conflict = ref<Message | null>(null)
const remoteUpdate = ref<Message | null>(null)
let heartbeat: number | null = null
let unsubscribe: (() => void) | null = null

const priorities: { value: Priority; label: string; color: string }[] = [
  { value: 'urgent', label: '紧急', color: '#fb7185' },
  { value: 'high', label: '高', color: '#f59e0b' },
  { value: 'normal', label: '常规', color: '#2dd4bf' },
  { value: 'low', label: '低', color: '#94a3b8' },
]

const lockExpiry = computed(() => {
  if (!lockedBy.value) return ''
  const ms = new Date(lockedBy.value.expires_at).getTime() - Date.now()
  if (ms <= 0) return '现在'
  return `${Math.max(1, Math.ceil(ms / 1000))} 秒后`
})

function parseTags(): string[] {
  return tagsInput.value
    .split(/[,，]/)
    .map((tag) => tag.trim().replace(/^#/, ''))
    .filter(Boolean)
    .slice(0, 8)
}

async function acquire() {
  if (props.mode !== 'edit' || !props.message) {
    acquiring.value = false
    return
  }
  acquiring.value = true
  lockedBy.value = null
  try {
    const lock = await api.post<EditLock>(`/api/v1/messages/${props.message.id}/lock`, { ttl_seconds: 90 })
    lockToken.value = lock.token
    startHeartbeat()
  } catch (err: any) {
    if (err?.status === 423) {
      lockedBy.value = err.payload?.lock || err.payload?.error?.lock || {
        message_id: props.message.id,
        owner: '其他 Agent',
        token: '',
        created_at: new Date().toISOString(),
        expires_at: new Date(Date.now() + 60000).toISOString(),
      }
    } else {
      notify('error', '无法开始编辑', errorMessage(err))
      emit('close')
    }
  } finally {
    acquiring.value = false
  }
}

function startHeartbeat() {
  stopHeartbeat()
  heartbeat = window.setInterval(async () => {
    if (!props.message) return
    try {
      const lock = await api.post<EditLock>(`/api/v1/messages/${props.message.id}/lock`, { ttl_seconds: 90 })
      lockToken.value = lock.token
    } catch {
      // The lease can lapse; the version check still protects the write.
    }
  }, 30000)
}

function stopHeartbeat() {
  if (heartbeat) {
    window.clearInterval(heartbeat)
    heartbeat = null
  }
}

async function releaseLock() {
  if (props.mode !== 'edit' || !props.message || !lockToken.value) return
  const token = lockToken.value
  lockToken.value = ''
  try {
    await fetch(`/api/v1/messages/${props.message.id}/lock`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${getToken()}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
      keepalive: true,
    })
  } catch {
    // Best effort. Locks expire after their TTL.
  }
}

async function save() {
  if (saving.value || !content.value.trim()) return
  saving.value = true
  try {
    if (props.mode === 'create') {
      const created = await api.post<Message>('/api/v1/messages', {
        title: title.value,
        content: content.value,
        tags: parseTags(),
        priority: priority.value,
        client_request_id: crypto.randomUUID(),
      })
      notify('success', '留言已发布', '已实时同步给所有在线 Agent')
      emit('saved', created)
      return
    }
    if (!props.message) return
    const updated = await api.patch<Message>(`/api/v1/messages/${props.message.id}`, {
      title: title.value,
      content: content.value,
      tags: parseTags(),
      priority: priority.value,
      expected_version: expectedVersion.value,
    })
    notify('success', '修改已保存', `当前版本 v${updated.version}`)
    await releaseLock()
    emit('saved', updated)
  } catch (err: any) {
    if (err?.status === 409 && (err.code === 'version_conflict' || err.payload?.error?.code === 'version_conflict')) {
      conflict.value = err.payload?.current || err.payload?.error?.current || null
      remoteUpdate.value = null
      notify('warning', '检测到并发修改', '请选择保留最新版本或用你的内容覆盖')
    } else if (err?.status === 423) {
      lockedBy.value = err.payload?.lock || null
    } else {
      notify('error', '保存失败', errorMessage(err))
    }
  } finally {
    saving.value = false
  }
}

function adoptLatest() {
  if (!conflict.value) return
  title.value = conflict.value.title
  content.value = conflict.value.content
  tagsInput.value = conflict.value.tags.join(', ')
  priority.value = conflict.value.priority
  expectedVersion.value = conflict.value.version
  conflict.value = null
  remoteUpdate.value = null
  notify('info', '已载入最新版本', '你的本地修改已放弃')
}

async function overwriteLatest() {
  if (!conflict.value) return
  expectedVersion.value = conflict.value.version
  conflict.value = null
  remoteUpdate.value = null
  await save()
}

async function retryLock() {
  lockedBy.value = null
  await acquire()
}

async function close() {
  if (saving.value) return
  await releaseLock()
  emit('close')
}

function onRemoteEvent(event: StreamEvent) {
  if (props.mode !== 'edit' || !props.message || !event.data?.id) return
  if (event.data.id !== props.message.id) return
  if (event.type !== 'message.updated' && event.type !== 'message.restored') return
  if (event.data.edited_by === state.me?.name && event.data.version === expectedVersion.value) return
  if (event.data.version > expectedVersion.value) remoteUpdate.value = event.data
}

onMounted(() => {
  unsubscribe = onStream(onRemoteEvent)
  acquire()
})

onBeforeUnmount(async () => {
  unsubscribe?.()
  stopHeartbeat()
  await releaseLock()
})
</script>

<style scoped>
.editor-textarea {
  min-height: 230px;
  font-size: 12.5px;
}
.priority-picker {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 5px;
  padding: 4px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-soft);
}
.priority-option {
  display: flex;
  height: 31px;
  align-items: center;
  justify-content: center;
  gap: 5px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--muted);
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
}
.priority-option.active {
  border-color: var(--border);
  background: var(--surface-solid);
  color: var(--text);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
}
.priority-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--dot);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--dot) 20%, transparent);
}
.editor-tip {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 9.5px;
}
.editor-tip code {
  color: var(--primary);
  font-family: var(--font-mono);
  font-size: 9px;
}
.conflict-box {
  padding: 14px;
  border: 1px solid rgba(245, 158, 11, 0.26);
  border-radius: 12px;
  background: rgba(245, 158, 11, 0.07);
  color: var(--text-soft);
  font-size: 11px;
  line-height: 1.6;
}
.conflict-box strong {
  font-size: 12px;
}
.conflict-box p {
  margin-top: 3px;
  color: var(--muted);
}
.conflict-preview {
  margin-top: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-solid);
  color: var(--muted);
  font-size: 10px;
  white-space: pre-wrap;
}
.stale-box {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 9px 11px;
  border: 1px solid rgba(59, 130, 246, 0.18);
  border-radius: 9px;
  background: rgba(59, 130, 246, 0.06);
  color: #60a5fa;
  font-size: 10px;
}
.locked-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 20px 50px;
  text-align: center;
}
.locked-icon {
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  margin-bottom: 14px;
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: 16px;
  background: rgba(245, 158, 11, 0.1);
  color: var(--amber);
}
</style>
