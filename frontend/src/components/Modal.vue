<template>
  <Teleport to="body">
    <div class="modal-overlay" @mousedown.self="onBackdrop">
      <div class="modal-panel" :class="size" role="dialog" aria-modal="true" tabindex="-1" ref="panel">
        <div class="modal-head">
          <div>
            <div class="modal-title">{{ title }}</div>
            <div v-if="subtitle" class="modal-sub">{{ subtitle }}</div>
          </div>
          <button class="icon-btn plain" type="button" aria-label="关闭" @click="$emit('close')">
            <X :size="18" />
          </button>
        </div>
        <div class="modal-body">
          <slot />
        </div>
        <div v-if="$slots.footer" class="modal-foot">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { X } from 'lucide-vue-next'

const props = withDefaults(defineProps<{ title: string; subtitle?: string; size?: 'sm' | 'lg' | 'xl'; closeOnBackdrop?: boolean }>(), {
  size: 'sm',
  closeOnBackdrop: true,
})
const emit = defineEmits<{ close: [] }>()
const panel = ref<HTMLElement | null>(null)
let previousOverflow = ''

function onBackdrop() {
  if (props.closeOnBackdrop) emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => {
  previousOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  document.addEventListener('keydown', onKeydown)
  requestAnimationFrame(() => panel.value?.focus())
})

onBeforeUnmount(() => {
  document.body.style.overflow = previousOverflow
  document.removeEventListener('keydown', onKeydown)
})
</script>
