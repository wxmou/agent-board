<template>
  <div class="empty-state">
    <div class="empty-icon"><component :is="icon" :size="22" :stroke-width="1.7" /></div>
    <div class="empty-title">{{ title }}</div>
    <p class="empty-copy">{{ copy }}</p>
    <div v-if="$slots.default" class="mt-16"><slot /></div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ icon: Component; title: string; copy: string }>()
</script>
