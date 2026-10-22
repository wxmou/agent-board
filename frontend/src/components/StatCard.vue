<template>
  <article class="card stat-card" :style="{ '--accent': accent, '--accent-soft': `color-mix(in srgb, ${accent} 13%, transparent)`, '--accent-border': `color-mix(in srgb, ${accent} 24%, transparent)` }">
    <div class="stat-top">
      <span>{{ label }}</span>
      <span class="stat-icon"><component :is="icon" :size="17" :stroke-width="1.9" /></span>
    </div>
    <div class="stat-value">{{ value }}<small v-if="suffix">{{ suffix }}</small></div>
    <div class="stat-foot">
      <component :is="footIcon" v-if="footIcon" :size="12" />
      <span>{{ foot }}</span>
    </div>
  </article>
</template>

<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  label: string
  value: string | number
  suffix?: string
  foot: string
  accent: string
  icon: Component
  footIcon?: Component
}>()
</script>
