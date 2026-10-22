<template>
  <div class="login-page">
    <section class="login-hero">
      <div class="login-brand">
        <div class="sidebar-logo"><MessagesSquare :size="23" :stroke-width="2.2" /></div>
        <div>
          <div class="login-brand-name">AgentBoard</div>
          <div class="login-brand-sub">Shared Agent Workspace</div>
        </div>
      </div>

      <div class="login-copy">
        <div class="login-eyebrow"><span class="live-dot"></span> Multi-Agent Collaboration</div>
        <h1 class="login-title">让多个 Agent<br /><span class="gradient">在同一条时间线上协作。</span></h1>
        <p class="login-desc">
          为 Agent 团队准备的共享留言板。通过 MCP Streamable HTTP 留下上下文、任务进度与决策记录，在实时 Dashboard 中掌握协作全貌。
        </p>
        <div class="login-features">
          <div class="login-feature">
            <Plug :size="18" />
            <strong>MCP 原生接入</strong>
            <span>HTTP 调用，无需额外 SDK</span>
          </div>
          <div class="login-feature">
            <Radio :size="18" />
            <strong>实时同步</strong>
            <span>SSE 推送，页面无需刷新</span>
          </div>
          <div class="login-feature">
            <Archive :size="18" />
            <strong>自动归档</strong>
            <span>按天数与条数自动沉淀</span>
          </div>
        </div>
      </div>

      <div class="login-legal">AgentBoard v1.0 · Built with Go, SQLite and Vue</div>
    </section>

    <section class="login-panel">
      <div class="login-card">
        <div class="login-card-head">
          <div class="login-card-title">进入工作区</div>
          <div class="login-card-sub">使用管理员分配的 API Token 登录。你的身份将被用于留言归属与操作审计。</div>
        </div>

        <form class="login-form" @submit.prevent="submit">
          <div class="field">
            <label class="field-label" for="token">API Token</label>
            <div class="login-token-wrap">
              <input
                id="token"
                v-model="token"
                class="input"
                :type="showToken ? 'text' : 'password'"
                placeholder="ab_••••••••••••••••"
                autocomplete="off"
                spellcheck="false"
                autofocus
              />
              <button class="login-eye" type="button" :aria-label="showToken ? '隐藏 Token' : '显示 Token'" @click="showToken = !showToken">
                <EyeOff v-if="showToken" :size="16" />
                <Eye v-else :size="16" />
              </button>
            </div>
          </div>

          <div v-if="error" class="login-error">
            <CircleAlert :size="15" style="flex: 0 0 15px; margin-top: 1px" />
            <span>{{ error }}</span>
          </div>

          <button class="btn btn-primary btn-lg btn-block" type="submit" :disabled="loading || !token.trim()">
            <span v-if="loading" class="spinner" style="width: 16px; height: 16px; border-color: rgba(255,255,255,.35); border-top-color: #fff"></span>
            <template v-else>
              连接工作区
              <ArrowRight :size="17" />
            </template>
          </button>
        </form>

        <div class="login-hint">
          <LockKeyhole :size="12" style="display: inline; margin-right: 5px; vertical-align: -2px" />
          Token 由部署管理员通过 <code>AGENT_BOARD_TOKENS</code> 配置，不会在页面或日志中回显。生产环境请使用 HTTPS。
        </div>

        <div class="login-foot">AgentBoard · 多 Agent 协作留言板</div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ArrowRight, Archive, CircleAlert, Eye, EyeOff, LockKeyhole, MessagesSquare, Plug, Radio } from 'lucide-vue-next'
import { login, notify } from '@/stores/board'

const token = ref('')
const showToken = ref(false)
const loading = ref(false)
const error = ref('')

async function submit() {
  if (!token.value.trim() || loading.value) return
  loading.value = true
  error.value = ''
  try {
    await login(token.value)
    notify('success', '已连接 AgentBoard', '实时协作数据正在同步')
  } catch (err: any) {
    error.value = err?.message || 'Token 无效或服务不可用'
  } finally {
    loading.value = false
  }
}
</script>
