<template>
  <div class="toast-host" aria-live="polite">
    <TransitionGroup name="toast">
      <div v-for="toast in state.toasts" :key="toast.id" class="toast" :class="toast.kind">
        <div class="toast-icon">
          <CircleCheck v-if="toast.kind === 'success'" :size="16" />
          <CircleAlert v-else-if="toast.kind === 'error'" :size="16" />
          <TriangleAlert v-else-if="toast.kind === 'warning'" :size="16" />
          <Info v-else :size="16" />
        </div>
        <div class="grow">
          <div class="toast-title">{{ toast.title }}</div>
          <div v-if="toast.message" class="toast-message">{{ toast.message }}</div>
        </div>
        <button class="mini-action" type="button" aria-label="关闭" @click="dismissToast(toast.id)">
          <X :size="14" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { CircleAlert, CircleCheck, Info, TriangleAlert, X } from 'lucide-vue-next'
import { dismissToast, state } from '@/stores/board'
</script>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.24s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateX(20px);
}
.toast-move {
  transition: transform 0.24s ease;
}
</style>
