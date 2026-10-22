export const TOKEN_KEY = 'agent-board-token'

export class ApiError extends Error {
  status: number
  code: string
  payload: any

  constructor(status: number, message: string, code = 'request_failed', payload: any = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.payload = payload
  }
}

export function getToken(): string {
  try {
    return localStorage.getItem(TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

export function setToken(token: string) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch {
    // localStorage can be unavailable in hardened browsers.
  }
}

interface RequestOptions {
  method?: string
  body?: any
  skipAuthEvent?: boolean
  signal?: AbortSignal
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  let body: BodyInit | undefined
  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(options.body)
  }
  let response: Response
  try {
    response = await fetch(path, { method: options.method || 'GET', headers, body, signal: options.signal })
  } catch (err: any) {
    if (err?.name === 'AbortError') throw err
    throw new ApiError(0, '无法连接到 AgentBoard 服务，请检查网络或服务状态', 'network_error')
  }
  const text = await response.text()
  let payload: any = null
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { raw: text }
    }
  }
  if (!response.ok) {
    const message = payload?.error?.message || payload?.message || `请求失败 (${response.status})`
    const code = payload?.error?.code || 'request_failed'
    if (response.status === 401 && !options.skipAuthEvent) {
      window.dispatchEvent(new CustomEvent('ab:unauthorized'))
    }
    throw new ApiError(response.status, message, code, payload)
  }
  return payload as T
}

export const api = {
  get<T>(path: string, signal?: AbortSignal) {
    return request<T>(path, { signal })
  },
  post<T>(path: string, body?: any) {
    return request<T>(path, { method: 'POST', body })
  },
  patch<T>(path: string, body?: any) {
    return request<T>(path, { method: 'PATCH', body })
  },
  delete<T>(path: string) {
    return request<T>(path, { method: 'DELETE' })
  },
}

export function queryString(params: Record<string, string | number | boolean | undefined | null>): string {
  const q = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    q.set(key, String(value))
  }
  const raw = q.toString()
  return raw ? `?${raw}` : ''
}

export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / Math.pow(1024, i)
  return `${value >= 10 || i === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[i]}`
}

export function formatDate(value?: string, withTime = true): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  const base = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
  return withTime ? `${base} ${pad(date.getHours())}:${pad(date.getMinutes())}` : base
}

export function timeAgo(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 10) return '刚刚'
  if (seconds < 60) return `${seconds} 秒前`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} 天前`
  const months = Math.floor(days / 30)
  if (months < 12) return `${months} 个月前`
  return `${Math.floor(months / 12)} 年前`
}

export function countdown(seconds: number): string {
  if (seconds <= 0) return '即将'
  if (seconds < 60) return `${Math.ceil(seconds)} 秒`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60)
  return `${hours} 小时 ${minutes % 60} 分钟`
}

export function initials(name: string): string {
  if (!name) return '?'
  const clean = name.replace(/^agent[-_]?/i, '').replace(/[-_]/g, ' ')
  const parts = clean.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return clean.slice(0, 2).toUpperCase()
}

export function displayTitle(title: string, content: string): string {
  if (title?.trim()) return title
  const first = content.trim().replace(/\s+/g, ' ')
  return first.length > 46 ? `${first.slice(0, 46)}…` : first || '无标题留言'
}

export function excerpt(content: string, length = 180): string {
  const clean = content.replace(/\r/g, '').trim()
  return clean.length > length ? `${clean.slice(0, length)}…` : clean
}

export const priorityMeta: Record<string, { label: string; color: string }> = {
  urgent: { label: '紧急', color: '#fb7185' },
  high: { label: '高', color: '#f59e0b' },
  normal: { label: '常规', color: '#2dd4bf' },
  low: { label: '低', color: '#94a3b8' },
}

export function colorFromName(name: string): string {
  const palette = ['#2dd4bf', '#60a5fa', '#a78bfa', '#f472b6', '#fbbf24', '#34d399', '#fb7185', '#38bdf8', '#c084fc', '#f59e0b']
  let n = 0
  for (const ch of name || '?') n = n * 31 + ch.charCodeAt(0)
  return palette[Math.abs(n) % palette.length]
}

export async function copyText(value: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(value)
      return true
    }
  } catch {
    // Fall through to the legacy textarea method.
  }
  try {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    textarea.style.pointerEvents = 'none'
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    return ok
  } catch {
    return false
  }
}
