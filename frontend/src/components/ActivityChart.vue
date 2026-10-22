<template>
  <div class="activity-chart">
    <div class="chart-legend">
      <span><i class="legend-dot" style="background: #2dd4bf"></i>新留言</span>
      <span><i class="legend-dot" style="background: #60a5fa"></i>编辑</span>
      <span><i class="legend-dot" style="background: #fbbf24"></i>归档</span>
    </div>
    <div class="plot">
      <div class="y-axis">
        <span v-for="tick in ticks" :key="tick">{{ tick }}</span>
      </div>
      <div class="plot-inner">
        <div class="grid-lines">
          <span v-for="n in 4" :key="n"></span>
        </div>
        <div class="bars">
          <div v-for="(day, i) in data" :key="day.date" class="bar-group">
            <div class="bar-stack">
              <span
                class="bar created"
                :style="{ height: height(day.created), animationDelay: `${i * 45}ms` }"
                :title="`${day.date} 新留言 ${day.created} 条`"
              ></span>
              <span
                class="bar edited"
                :style="{ height: height(day.edited), animationDelay: `${i * 45 + 60}ms` }"
                :title="`${day.date} 编辑 ${day.edited} 次`"
              ></span>
              <span
                class="bar archived"
                :style="{ height: height(day.archived), animationDelay: `${i * 45 + 120}ms` }"
                :title="`${day.date} 归档 ${day.archived} 条`"
              ></span>
            </div>
            <div class="bar-label">
              <strong>{{ weekday(day.date) }}</strong>
              <span>{{ monthDay(day.date) }}</span>
            </div>
          </div>
          <div v-if="!data.length" class="chart-empty">暂无活动数据</div>
        </div>
      </div>
    </div>
    <div class="chart-foot">
      <span>过去 7 天</span>
      <span>峰值 {{ maxValue }} 条 / 天</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { DayPoint } from '@/lib/types'

const props = defineProps<{ data: DayPoint[] }>()

const maxValue = computed(() => Math.max(1, ...props.data.flatMap((d) => [d.created, d.edited, d.archived])))
const ticks = computed(() => {
  const max = Math.ceil(maxValue.value / 4) * 4 || 4
  return [max, Math.round(max * 0.66), Math.round(max * 0.33), 0]
})

function height(value: number) {
  const max = Math.ceil(maxValue.value / 4) * 4 || 4
  return `${Math.max(value > 0 ? 5 : 1.5, (value / max) * 100)}%`
}

function weekday(value: string) {
  const date = new Date(`${value}T00:00:00`)
  return ['日', '一', '二', '三', '四', '五', '六'][date.getDay()]
}

function monthDay(value: string) {
  return value.slice(5).replace('-', '/')
}
</script>

<style scoped>
.activity-chart {
  padding-top: 4px;
}
.plot {
  display: flex;
  gap: 10px;
  margin-top: 18px;
}
.y-axis {
  display: flex;
  height: 170px;
  flex: 0 0 26px;
  flex-direction: column;
  justify-content: space-between;
  padding-bottom: 31px;
  color: var(--muted-2);
  font-family: var(--font-mono);
  font-size: 8.5px;
  text-align: right;
}
.plot-inner {
  position: relative;
  flex: 1;
  min-width: 0;
}
.grid-lines {
  position: absolute;
  inset: 0 0 31px 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  pointer-events: none;
}
.grid-lines span {
  height: 1px;
  background: var(--border);
  opacity: 0.55;
}
.bars {
  position: relative;
  display: flex;
  height: 170px;
  align-items: flex-end;
  justify-content: space-around;
  gap: 8px;
}
.bar-group {
  display: flex;
  height: 100%;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
}
.bar-stack {
  display: flex;
  width: 100%;
  max-width: 62px;
  height: 132px;
  align-items: flex-end;
  justify-content: center;
  gap: 3px;
  padding: 0 2px;
}
.bar {
  width: 7px;
  min-height: 2px;
  border-radius: 4px 4px 2px 2px;
  transform-origin: bottom;
  animation: bar-grow 0.65s cubic-bezier(0.16, 1, 0.3, 1) both;
}
.bar.created {
  background: linear-gradient(180deg, #2dd4bf, #0d9488);
  box-shadow: 0 2px 8px rgba(20, 184, 166, 0.2);
}
.bar.edited {
  background: linear-gradient(180deg, #60a5fa, #2563eb);
  opacity: 0.85;
}
.bar.archived {
  background: linear-gradient(180deg, #fbbf24, #d97706);
  opacity: 0.72;
}
.bar-label {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--muted);
  font-size: 9px;
  line-height: 1;
  white-space: nowrap;
}
.bar-label strong {
  color: var(--text-soft);
  font-weight: 620;
}
.bar-label span {
  color: var(--muted-2);
  font-family: var(--font-mono);
  font-size: 8px;
}
.chart-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
  color: var(--muted-2);
  font-size: 9px;
}
.chart-empty {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: var(--muted);
  font-size: 11px;
}
@keyframes bar-grow {
  from {
    transform: scaleY(0.08);
    opacity: 0.2;
  }
  to {
    transform: scaleY(1);
    opacity: 1;
  }
}
</style>
