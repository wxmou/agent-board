<template>
  <div class="brand-lockup">
    <div class="sidebar-logo">
      <MessagesSquare :size="22" :stroke-width="2.2" />
    </div>
    <div v-if="!compact" class="sidebar-brand-copy">
      <div class="sidebar-brand-name">AgentBoard</div>
      <div class="sidebar-brand-sub">Agent Collaboration</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { MessagesSquare } from 'lucide-vue-next'

defineProps<{ compact?: boolean }>()
</script>
