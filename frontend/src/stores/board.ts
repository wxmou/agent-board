import { reactive } from 'vue'
import { api, getToken, setToken, type ApiError } from '@/lib/api'
import type { AgentSummary, ArchivePolicy, AuthInfo, EditLock, Principal, Stats, StreamEvent } from '@/lib/types'

export interface Toast {
  id: number
  kind: 'success' | 'error' | 'info' | 'warning'
  title: string
  message?: string
}

type StreamListener = (event: StreamEvent) => void

export const state = reactive({
  ready: false,
  checking: false,
  me: null as Principal | null,
  server: null as AuthInfo['server'] | null,
  stats: null as Stats | null,
  policy: null as ArchivePolicy | null,
  agents: [] as AgentSummary[],
  locks: {} as Record<string, EditLock>,
  toasts: [] as Toast[],
  live: false,
  connection: 'connecting' as 'connecting' | 'live' | 'offline',
  lastEventAt: null as Date | null,
  lastEvent: null as StreamEvent | null,
})

const listeners = new Set<StreamListener>()
let toastID = 1
let streamAbort: AbortController | null = null
let reconnectTimer: number | null = null
let reconnectAttempt = 0
let stopped = true
let statsTimer: number | null = null
let streamReadyOnce = false

export function onStream(listener: StreamListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function notify(kind: Toast['kind'], title: string, message?: string, duration = 4200) {
  const id = toastID++
  state.toasts.push({ id, kind, title, message })
  window.setTimeout(() => dismissToast(id), duration)
}

export function dismissToast(id: number) {
  const index = state.toasts.findIndex((t) => t.id === id)
  if (index >= 0) state.toasts.splice(index, 1)
}

export async function bootstrap() {
  if (!getToken()) {
    state.ready = true
    return
  }
  state.checking = true
  try {
    const me = await api.get<AuthInfo>('/api/v1/auth/me')
    state.me = me.principal
    state.server = me.server
    state.ready = true
    await refreshCore()
    connectStream()
  } catch (err: any) {
    if (err?.status === 401) setToken('')
    state.me = null
    state.ready = true
  } finally {
    state.checking = false
  }
}

export async function login(token: string) {
  const trimmed = token.trim()
  if (!trimmed) throw new Error('请输入 API Token')
  setToken(trimmed)
  try {
    const me = await api.get<AuthInfo>('/api/v1/auth/me')
    state.me = me.principal
    state.server = me.server
    state.ready = true
    await refreshCore()
    connectStream()
  } catch (err) {
    setToken('')
    state.me = null
    throw err
  }
}

export function logout() {
  stopped = true
  disconnectStream()
  setToken('')
  state.me = null
  state.stats = null
  state.policy = null
  state.agents = []
  state.locks = {}
  state.live = false
  state.connection = 'offline'
  state.lastEventAt = null
  streamReadyOnce = false
}

export async function refreshCore() {
  const results = await Promise.allSettled([
    api.get<Stats>('/api/v1/stats'),
    api.get<ArchivePolicy>('/api/v1/settings/archive'),
    api.get<{ items: AgentSummary[] }>('/api/v1/agents'),
    api.get<{ items: EditLock[] }>('/api/v1/locks'),
  ])
  if (results[0].status === 'fulfilled') {
    state.stats = results[0].value
    state.policy = results[0].value.policy
  }
  if (results[1].status === 'fulfilled') state.policy = results[1].value
  if (results[2].status === 'fulfilled') state.agents = results[2].value.items
  if (results[3].status === 'fulfilled') {
    const map: Record<string, EditLock> = {}
    for (const lock of results[3].value.items) map[lock.message_id] = lock
    state.locks = map
  }
}

export function refreshStatsSoon() {
  if (statsTimer) window.clearTimeout(statsTimer)
  statsTimer = window.setTimeout(() => {
    api.get<Stats>('/api/v1/stats').then((stats) => {
      state.stats = stats
      state.policy = stats.policy
    }).catch(() => {})
    api.get<{ items: AgentSummary[] }>('/api/v1/agents').then((res) => {
      state.agents = res.items
    }).catch(() => {})
  }, 500)
}

function connectStream() {
  stopped = false
  if (reconnectTimer) {
    window.clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (streamAbort) streamAbort.abort()
  const controller = new AbortController()
  streamAbort = controller
  state.connection = 'connecting'
  ;(async () => {
    try {
      const response = await fetch('/api/v1/stream', {
        headers: { Authorization: `Bearer ${getToken()}`, Accept: 'text/event-stream' },
        signal: controller.signal,
        cache: 'no-store',
      })
      if (response.status === 401) {
        window.dispatchEvent(new CustomEvent('ab:unauthorized'))
        return
      }
      if (!response.ok || !response.body) throw new Error(`stream failed: ${response.status}`)
      state.live = true
      state.connection = 'live'
      reconnectAttempt = 0
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (!stopped) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
        let boundary = buffer.indexOf('\n\n')
        while (boundary >= 0) {
          const block = buffer.slice(0, boundary)
          buffer = buffer.slice(boundary + 2)
          handleSSEBlock(block)
          boundary = buffer.indexOf('\n\n')
        }
      }
      if (!stopped) throw new Error('stream ended')
    } catch (err: any) {
      if (stopped || err?.name === 'AbortError') return
      state.live = false
      state.connection = 'offline'
      scheduleReconnect()
    }
  })()
}

function scheduleReconnect() {
  if (stopped) return
  reconnectAttempt++
  const delay = Math.min(1000 * Math.pow(1.7, reconnectAttempt), 15000)
  reconnectTimer = window.setTimeout(() => {
    if (!stopped) connectStream()
  }, delay)
}

function disconnectStream() {
  if (streamAbort) {
    streamAbort.abort()
    streamAbort = null
  }
  if (reconnectTimer) {
    window.clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  state.live = false
  state.connection = 'offline'
}

function handleSSEBlock(block: string) {
  if (!block.trim() || block.startsWith(':')) return
  let eventName = 'message'
  const dataLines: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) eventName = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart())
  }
  if (!dataLines.length) return
  let data: any = null
  try {
    data = JSON.parse(dataLines.join('\n'))
  } catch {
    return
  }
  if (eventName === 'ready') {
    state.connection = 'live'
    state.live = true
    if (streamReadyOnce) {
      const event: StreamEvent = { type: 'stream.reconnected', data, timestamp: new Date().toISOString() }
      state.lastEvent = event
      state.lastEventAt = new Date()
      for (const listener of listeners) {
        try {
          listener(event)
        } catch (err) {
          console.error('stream listener failed', err)
        }
      }
    }
    streamReadyOnce = true
    return
  }
  const event: StreamEvent = { type: eventName, data, timestamp: new Date().toISOString() }
  state.lastEvent = event
  state.lastEventAt = new Date()
  applyLockEvent(event)
  if ((eventName === 'message.created' || eventName === 'message.updated') && data?.author && data.author !== state.me?.name) {
    notify('info', eventName === 'message.created' ? `${data.author} 留下了新留言` : `${data.author} 更新了留言`, data.title || '')
  }
  if (eventName === 'archive.run' && data?.archived_count > 0) {
    notify('info', '自动归档完成', `已将 ${data.archived_count} 条留言移入归档`)
    refreshStatsSoon()
  }
  if (eventName.startsWith('message.') || eventName === 'policy.updated') refreshStatsSoon()
  for (const listener of listeners) {
    try {
      listener(event)
    } catch (err) {
      console.error('stream listener failed', err)
    }
  }
}

function applyLockEvent(event: StreamEvent) {
  if (event.type === 'lock.acquired' && event.data?.message_id) {
    state.locks[event.data.message_id] = {
      message_id: event.data.message_id,
      owner: event.data.owner,
      token: '',
      expires_at: event.data.expires_at,
      created_at: new Date().toISOString(),
    }
  } else if (event.type === 'lock.released' && event.data?.message_id) {
    delete state.locks[event.data.message_id]
  } else if (event.type === 'message.archived' && event.data?.id) {
    delete state.locks[event.data.id]
  }
}

window.addEventListener('ab:unauthorized', () => {
  if (!state.me && !getToken()) return
  logout()
  notify('warning', '登录已失效', '请重新输入 API Token')
})

export function errorMessage(err: unknown): string {
  const e = err as ApiError
  if (e?.message) return e.message
  return '操作失败，请稍后重试'
}
