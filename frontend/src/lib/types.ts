export interface Principal {
  name: string
  role: 'admin' | 'writer'
  color: string
}

export interface Message {
  id: string
  title: string
  content: string
  author: string
  edited_by?: string
  tags: string[]
  priority: 'urgent' | 'high' | 'normal' | 'low'
  pinned: boolean
  version: number
  client_request_id?: string
  created_at: string
  updated_at: string
  archived_at?: string
  archived_reason?: string
  deleted_at?: string
}

export interface EditLock {
  message_id: string
  owner: string
  token: string
  expires_at: string
  created_at: string
}

export interface ArchivePolicy {
  retain_days: number
  max_items: number
  auto_archive: boolean
  interval_minutes: number
  pinned_protected: boolean
}

export interface ArchiveRun {
  id: number
  trigger: string
  started_at: string
  finished_at?: string
  archived_count: number
  retention_count: number
  overflow_count: number
  remaining: number
  error?: string
}

export interface AgentSummary {
  name: string
  color: string
  role?: string
  messages: number
  active: number
  edits: number
  last_active?: string
  configured: boolean
}

export interface DayPoint {
  date: string
  created: number
  edited: number
  archived: number
}

export interface BoardEvent {
  id: number
  type: string
  message_id?: string
  title?: string
  actor: string
  detail?: string
  created_at: string
}

export interface Stats {
  active: number
  archived: number
  deleted: number
  agents: number
  pinned: number
  created_today: number
  edits_24h: number
  total_all_time: number
  locked: number
  storage_bytes: number
  trend: DayPoint[]
  top_agents: AgentSummary[]
  recent_events: BoardEvent[]
  policy: ArchivePolicy
  last_archive_run?: ArchiveRun
  generated_at: string
  uptime_seconds: number
  server_version: string
  active_capacity: number
  capacity_percent: number
}

export interface MessageList {
  items: Message[]
  total: number
  limit: number
  offset: number
  next_cursor?: string
  has_more: boolean
}

export interface Revision {
  id: string
  title: string
  content: string
  author: string
  edited_by?: string
  version: number
  created_at: string
  updated_at: string
}

export interface StreamEvent {
  type: string
  data: any
  timestamp: string
}

export interface AuthInfo {
  principal: Principal
  server: { version: string; mcp_url: string; uptime_seconds: number }
  capabilities: { archive: boolean; realtime: boolean; mcp: boolean }
}
