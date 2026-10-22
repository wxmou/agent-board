<template>
  <Teleport to="body">
    <div class="drawer-overlay" @click="$emit('close')"></div>
    <aside class="drawer" role="dialog" aria-modal="true" aria-label="留言详情">
      <div class="drawer-head">
        <div class="flex items-center gap-8">
          <span class="badge badge-teal">MESSAGE</span>
          <span class="mono" style="color: var(--muted); font-size: 9.5px">v{{ message.version }}</span>
          <span v-if="message.archived_at" class="badge badge-amber">已归档</span>
        </div>
        <div class="flex items-center gap-6">
          <button class="icon-btn plain" type="button" title="复制留言 ID" @click="copyID">
            <Copy :size="15" />
          </button>
          <button class="icon-btn plain" type="button" aria-label="关闭" @click="$emit('close')">
            <X :size="18" />
          </button>
        </div>
      </div>

      <div class="drawer-body">
        <div v-if="lock && lock.owner !== me?.name" class="info-banner" style="margin-bottom: 16px; border-color: rgba(245,158,11,.18); background: rgba(245,158,11,.07)">
          <Lock :size="15" style="color: var(--amber)" />
          <span><strong>{{ lock.owner }}</strong> 正在编辑这条留言。保存时会进行版本校验，不会静默覆盖。</span>
        </div>

        <div class="detail-title">{{ displayTitle(message.title, message.content) }}</div>
        <div class="detail-meta">
          <span class="avatar avatar-sm" :style="{ background: colorFromName(message.author) }">{{ initials(message.author) }}</span>
          <strong style="color: var(--text-soft)">{{ message.author }}</strong>
          <span>创建于 {{ formatDate(message.created_at) }}</span>
          <span v-if="message.edited_by">· {{ message.edited_by }} 编辑于 {{ formatDate(message.updated_at) }}</span>
        </div>

        <div class="flex items-center gap-6" style="margin-top: 16px; flex-wrap: wrap">
          <span class="badge" :class="priorityBadge">{{ priority.label }}优先级</span>
          <span v-for="tag in message.tags" :key="tag" class="tag">#{{ tag }}</span>
          <span v-if="message.pinned" class="badge badge-teal"><Pin :size="10" /> 已置顶</span>
        </div>

        <div class="detail-content">{{ message.content }}</div>

        <div v-if="message.archived_at" class="detail-section">
          <div class="detail-section-title">Archive</div>
          <div class="kv-list">
            <div class="kv-row"><span>归档时间</span><span>{{ formatDate(message.archived_at) }}</span></div>
            <div class="kv-row"><span>归档原因</span><span>{{ archiveReasonLabel(message.archived_reason) }}</span></div>
          </div>
        </div>

        <div class="detail-section">
          <div class="detail-section-title">Revision history</div>
          <div v-if="revisionsLoading" class="loading-block" style="padding: 20px"><span class="spinner"></span></div>
          <div v-else-if="revisions.length" class="revision-list">
            <div v-for="rev in revisions" :key="rev.version" class="revision-item" :class="{ open: expandedVersion === rev.version }">
              <button type="button" class="revision-head" @click="expandedVersion = expandedVersion === rev.version ? 0 : rev.version">
                <span class="revision-dot"></span>
                <span class="grow">
                  <strong>v{{ rev.version }}</strong>
                  <span> · {{ rev.author }} · {{ timeAgo(rev.created_at) }}</span>
                </span>
                <ChevronDown :size="14" :style="{ transform: expandedVersion === rev.version ? 'rotate(180deg)' : '', transition: 'transform .18s' }" />
              </button>
              <div v-if="expandedVersion === rev.version" class="revision-content">{{ rev.content }}</div>
            </div>
          </div>
          <div v-else class="muted" style="font-size: 11px">暂无历史版本</div>
        </div>

        <div class="detail-section">
          <div class="detail-section-title">Metadata</div>
          <div class="kv-list">
            <div class="kv-row"><span>消息 ID</span><span class="mono" style="font-size: 9.5px">{{ message.id }}</span></div>
            <div class="kv-row"><span>版本</span><span>v{{ message.version }}</span></div>
            <div class="kv-row"><span>最后更新</span><span>{{ formatDate(message.updated_at) }}</span></div>
            <div class="kv-row"><span>优先级</span><span>{{ priority.label }}</span></div>
          </div>
        </div>
      </div>

      <div class="drawer-foot">
        <div class="flex items-center gap-4">
          <button v-if="!message.archived_at" class="btn btn-ghost btn-sm" type="button" @click="$emit('pin', message)">
            <Pin :size="14" /> {{ message.pinned ? '取消置顶' : '置顶' }}
          </button>
          <button v-if="!message.archived_at && (isAdmin || message.author === me?.name)" class="btn btn-ghost btn-sm" type="button" style="color: var(--rose)" @click="$emit('delete', message)">
            <Trash2 :size="14" /> 删除
          </button>
        </div>
        <div class="flex items-center gap-6">
          <button v-if="message.archived_at && isAdmin" class="btn btn-secondary" type="button" @click="$emit('restore', message)">
            <RotateCcw :size="15" /> 恢复
          </button>
          <button v-if="!message.archived_at && isAdmin" class="btn btn-secondary" type="button" @click="$emit('archive', message)">
            <Archive :size="15" /> 归档
          </button>
          <button v-if="!message.archived_at" class="btn btn-primary" type="button" @click="$emit('edit', message)">
            <Pencil :size="15" /> 编辑
          </button>
        </div>
      </div>
    </aside>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Archive, ChevronDown, Copy, Lock, Pencil, Pin, RotateCcw, Trash2, X } from 'lucide-vue-next'
import type { EditLock, Message, Principal, Revision } from '@/lib/types'
import { api } from '@/lib/api'
import { colorFromName, displayTitle, formatDate, initials, priorityMeta, timeAgo } from '@/lib/api'
import { errorMessage, notify } from '@/stores/board'

const props = defineProps<{
  message: Message
  lock?: EditLock
  isAdmin?: boolean
  me?: Principal | null
}>()

const emit = defineEmits<{
  close: []
  edit: [message: Message]
  delete: [message: Message]
  pin: [message: Message]
  restore: [message: Message]
  archive: [message: Message]
}>()

const revisions = ref<Revision[]>([])
const revisionsLoading = ref(false)
const expandedVersion = ref(0)
let previousOverflow = ''

const priority = computed(() => priorityMeta[props.message.priority] || priorityMeta.normal)
const priorityBadge = computed(() => (props.message.priority === 'urgent' ? 'badge-red' : props.message.priority === 'high' ? 'badge-amber' : 'badge-teal'))

function archiveReasonLabel(reason?: string) {
  if (reason === 'retention_days') return '超过保留天数'
  if (reason === 'max_items') return '超过活跃条数上限'
  if (reason === 'manual') return '手动归档'
  return reason || '未知'
}

async function loadRevisions() {
  revisionsLoading.value = true
  try {
    const res = await api.get<{ items: Revision[] }>(`/api/v1/messages/${props.message.id}/revisions`)
    revisions.value = res.items
    expandedVersion.value = res.items[0]?.version || 0
  } catch (err) {
    notify('error', '加载历史版本失败', errorMessage(err))
  } finally {
    revisionsLoading.value = false
  }
}

async function copyID() {
  try {
    await navigator.clipboard.writeText(props.message.id)
    notify('success', '已复制消息 ID')
  } catch {
    notify('error', '复制失败', '浏览器不允许访问剪贴板')
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.message.id,
  () => loadRevisions(),
  { immediate: true },
)

onMounted(() => {
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.body.style.overflow = previousOverflow
  document.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.revision-list {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.revision-item {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-soft);
  overflow: hidden;
}
.revision-head {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 9px;
  padding: 10px 11px;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 10.5px;
  text-align: left;
  cursor: pointer;
}
.revision-head strong {
  color: var(--text-soft);
  font-family: var(--font-mono);
  font-size: 10px;
}
.revision-dot {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  border-radius: 50%;
  background: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-soft);
}
.revision-content {
  max-height: 220px;
  overflow: auto;
  padding: 0 11px 11px 26px;
  color: var(--text-soft);
  font-size: 11px;
  line-height: 1.65;
  white-space: pre-wrap;
}
.revision-item.open {
  border-color: rgba(20, 184, 166, 0.24);
  background: var(--surface-solid);
}
</style>
