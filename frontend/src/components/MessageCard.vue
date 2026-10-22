<template>
  <article
    class="message-card"
    :class="{ pinned: message.pinned, archived }"
    :style="{ '--priority-color': priority.color }"
    @click="$emit('open', message)"
  >
    <div class="message-top">
      <div class="message-author">
        <span class="avatar avatar-sm" :style="{ background: colorFromName(message.author) }">{{ initials(message.author) }}</span>
        <div>
          <div class="message-author-name">{{ message.author }}</div>
          <div class="message-meta">
            <span>{{ timeAgo(message.created_at) }}</span>
            <span>·</span>
            <span>v{{ message.version }}</span>
            <span v-if="message.edited_by" class="edited-hint">· 由 {{ message.edited_by }} 编辑</span>
          </div>
        </div>
      </div>
      <div class="flex items-center gap-6">
        <span v-if="lock && lock.owner !== me?.name" class="lock-chip" :title="`${lock.owner} 正在编辑`">
          <Lock :size="10" /> {{ lock.owner }}
        </span>
        <span v-if="message.priority !== 'normal'" class="badge" :class="priority.badge">{{ priority.label }}</span>
        <span v-if="archived" class="badge badge-amber">已归档</span>
      </div>
    </div>

    <div class="message-title-row">
      <span v-if="message.pinned" class="pin-mark"><Pin :size="11" fill="currentColor" /></span>
      <h3>{{ displayTitle(message.title, message.content) }}</h3>
    </div>
    <p class="message-excerpt">{{ excerpt(message.content, 230) }}</p>

    <div class="message-bottom">
      <div class="message-tags">
        <span v-for="tag in message.tags.slice(0, 4)" :key="tag" class="tag">#{{ tag }}</span>
        <span v-if="message.tags.length > 4" class="tag">+{{ message.tags.length - 4 }}</span>
      </div>
      <div class="message-actions">
        <button class="mini-action" type="button" title="查看详情" @click.stop="$emit('open', message)">
          <Eye :size="14" />
        </button>
        <button v-if="archived && isAdmin" class="mini-action" type="button" title="恢复留言" @click.stop="$emit('restore', message)">
          <RotateCcw :size="14" />
        </button>
        <button v-if="!archived" class="mini-action" type="button" :title="message.pinned ? '取消置顶' : '置顶'" @click.stop="$emit('pin', message)">
          <Pin :size="14" />
        </button>
        <button v-if="!archived" class="mini-action" type="button" title="编辑" @click.stop="$emit('edit', message)">
          <Pencil :size="14" />
        </button>
        <button v-if="!archived && (isAdmin || message.author === me?.name)" class="mini-action danger" type="button" title="删除" @click.stop="$emit('delete', message)">
          <Trash2 :size="14" />
        </button>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Eye, Lock, Pencil, Pin, RotateCcw, Trash2 } from 'lucide-vue-next'
import type { EditLock, Message, Principal } from '@/lib/types'
import { colorFromName, displayTitle, excerpt, initials, priorityMeta, timeAgo } from '@/lib/api'

const props = defineProps<{
  message: Message
  lock?: EditLock
  archived?: boolean
  isAdmin?: boolean
  me?: Principal | null
}>()

defineEmits<{
  open: [message: Message]
  edit: [message: Message]
  delete: [message: Message]
  pin: [message: Message]
  restore: [message: Message]
}>()

const priority = computed(() => {
  const meta = priorityMeta[props.message.priority] || priorityMeta.normal
  return { ...meta, badge: meta.label === '紧急' ? 'badge-red' : meta.label === '高' ? 'badge-amber' : 'badge-teal' }
})
</script>

<style scoped>
.edited-hint {
  color: var(--muted-2);
}
</style>
