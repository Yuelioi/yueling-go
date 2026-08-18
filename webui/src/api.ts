export interface GroupInfo {
  group_id: number
  group_name: string
}

export interface PluginEntry {
  id: number
  name: string
  group: string
  desc: string
  usage: string
  commands: string[] | null
}

export interface AffinityRow {
  ID: number
  UserID: number
  GroupID: number
  Nickname: string
  Score: number
  LastReason: string
  UpdatedAt: number
}

export interface AffinityTier {
  min_score: number
  name: string
  prompt: string
  updated_at?: number
}

export interface AffinitySettings {
  ok: true
  custom: boolean
  tiers: AffinityTier[]
  initial: number
  min_score: number
  max_score: number
  block_below: number
  max_tiers: number
  max_name_chars: number
  max_prompt_chars: number
}

export interface GroupMessagePayload {
  text?: string
  at_user_ids?: number[]
  images?: string[]
}

export interface GroupAIStyle {
  ok: true
  group_id: number
  style_prompt: string
  custom: boolean
  default_scope: boolean
  default_style_prompt: string
  max_chars: number
}

export interface MemoryRow {
  ID: number
  UserID: number
  Content: string
  Category: string
  Source: string
  Confidence: number
  Importance: number
  Score: number
  CreatedAt: number
  UpdatedAt: number
  LastAccessed: number
}

export interface DailyDigest {
  ID: number
  GroupID: number
  CreatedBy: number
  SendTime: string
  CronExpr: string
  MessageCount: number
  Enabled: boolean
}

export interface FeedSubscription {
  id: number
  group_id: number
  url: string
  name: string
  created_by: number
  enabled: boolean
  created_at: number
  updated_at: number
  consecutive_failures: number
  last_error: string
  last_checked_at: number
  last_success_at: number
  next_check_at: number
}

export interface FeedSettings {
  group_id: number
  quiet_enabled: boolean
  quiet_start: string
  quiet_end: string
  updated_at: number
}

export interface FeedCheckResult {
  checked: number
  updated: number
  items: number
  delivered: number
  queued: number
  failed: number
}

export interface KnowledgeEntry {
  id: number
  group_id: number
  title: string
  content: string
  source_url: string
  created_by: number
  created_at: number
  updated_at: number
  shortcuts: KnowledgeShortcut[]
}

export interface KnowledgeShortcut {
  id: number
  knowledge_id: number
  group_id: number
  trigger: string
  created_at: number
}

export interface OverviewData {
  ok: true
  bot_connected: boolean
  group_count: number
  plugin_count: number
  affinity_count: number
  low_affinity_count: number
  memory_count: number
  memory_user_count: number
  digest_count: number
  feed_count: number
  knowledge_count: number
  recent_affinity: AffinityRow[]
  recent_memories: MemoryRow[]
}

export interface CommandUsageRank {
  command: string
  plugin_id: number
  calls: number
  unique_users: number
  last_used_at: number
}

export interface CommandUsageDay {
  date: string
  calls: number
  unique_users: number
}

export interface GroupCommandUsageStats {
  group_id: number
  days: number
  total_calls: number
  unique_users: number
  active_commands: number
  top_commands: CommandUsageRank[]
  daily: CommandUsageDay[]
}

export interface CommandUsageResponse {
  ok: true
  stats: GroupCommandUsageStats
  plugin_names: Record<string, string>
}

export class UnauthorizedError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'UnauthorizedError'
  }
}

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init.headers || {}) },
    ...init,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    if (res.status === 401) {
      throw new UnauthorizedError(data.error || '请先登录')
    }
    throw new Error(data.error || `HTTP ${res.status}`)
  }
  return data as T
}

export const api = {
  login(password: string) {
    return request<{ ok: true }>('/api/webui/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    })
  },
  me() {
    return request<{ ok: true; authenticated: boolean }>('/api/webui/auth/me')
  },
  logout() {
    return request<{ ok: true }>('/api/webui/auth/logout', { method: 'POST' })
  },
  groups() {
    return request<{ ok: true; groups: GroupInfo[] }>('/api/webui/groups')
  },
  overview() {
    return request<OverviewData>('/api/webui/overview')
  },
  commandUsage(groupID: number, days: number) {
    return request<CommandUsageResponse>(`/api/webui/command-usage?group_id=${groupID}&days=${days}`)
  },
  async plugins() {
    const res = await request<{ ok: true; plugins: PluginEntry[] }>('/api/webui/plugins')
    return {
      ...res,
      plugins: res.plugins.map((plugin) => ({
        ...plugin,
        commands: Array.isArray(plugin.commands) ? plugin.commands : [],
      })),
    }
  },
  groupPlugins(groupID: number) {
    return request<{ ok: true; disabled: Record<string, boolean> }>(`/api/webui/groups/${groupID}/plugins`)
  },
  setGroupPlugin(groupID: number, pluginID: number, disabled: boolean) {
    return request<{ ok: true }>(`/api/webui/groups/${groupID}/plugins/${pluginID}`, {
      method: 'PUT',
      body: JSON.stringify({ disabled }),
    })
  },
  applyPluginAll(pluginID: number, groupIDs: number[], disabled: boolean) {
    return request<{ ok: true }>(`/api/webui/plugins/${pluginID}/apply-all`, {
      method: 'POST',
      body: JSON.stringify({ group_ids: groupIDs, disabled }),
    })
  },
  sendGroupMessage(groupID: number, payload: string | GroupMessagePayload) {
    const body = typeof payload === 'string' ? { text: payload } : payload
    return request<{ ok: true; message_id: number }>(`/api/webui/groups/${groupID}/messages`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  groupAIStyle(groupID: number) {
    const path = groupID === 0 ? '/api/webui/ai-style/default' : `/api/webui/groups/${groupID}/ai-style`
    return request<GroupAIStyle>(path)
  },
  setGroupAIStyle(groupID: number, stylePrompt: string) {
    const path = groupID === 0 ? '/api/webui/ai-style/default' : `/api/webui/groups/${groupID}/ai-style`
    return request<GroupAIStyle>(path, {
      method: 'PUT',
      body: JSON.stringify({ style_prompt: stylePrompt }),
    })
  },
  resetGroupAIStyle(groupID: number) {
    const path = groupID === 0 ? '/api/webui/ai-style/default' : `/api/webui/groups/${groupID}/ai-style`
    return request<GroupAIStyle>(path, { method: 'DELETE' })
  },
  affinity(groupID: number | null, q: string) {
    const params = new URLSearchParams()
    if (groupID) params.set('group_id', String(groupID))
    if (q) params.set('q', q)
    return request<{ ok: true; affinity: AffinityRow[]; block_below: number }>(`/api/webui/affinity?${params}`)
  },
  affinitySettings() {
    return request<AffinitySettings>('/api/webui/affinity/settings')
  },
  setAffinitySettings(tiers: AffinityTier[]) {
    return request<AffinitySettings>('/api/webui/affinity/settings', {
      method: 'PUT',
      body: JSON.stringify({ tiers }),
    })
  },
  resetAffinitySettings() {
    return request<AffinitySettings>('/api/webui/affinity/settings', { method: 'DELETE' })
  },
  setAffinityScore(id: number, score: number) {
    return request<{ ok: true; affinity: AffinityRow }>(`/api/webui/affinity/${id}/score`, {
      method: 'PUT',
      body: JSON.stringify({ score }),
    })
  },
  adjustAffinity(id: number, delta: number) {
    return request<{ ok: true; affinity: AffinityRow }>(`/api/webui/affinity/${id}/adjust`, {
      method: 'POST',
      body: JSON.stringify({ delta }),
    })
  },
  resetAffinity(id: number) {
    return request<{ ok: true; affinity: AffinityRow }>(`/api/webui/affinity/${id}/reset`, { method: 'POST' })
  },
  memories(q: string) {
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    return request<{ ok: true; memories: MemoryRow[] }>(`/api/webui/memories?${params}`)
  },
  deleteMemory(id: number) {
    return request<{ ok: true }>(`/api/webui/memories/${id}`, { method: 'DELETE' })
  },
  clearUserMemories(userID: number) {
    return request<{ ok: true; deleted: number }>(`/api/webui/memories/users/${userID}`, { method: 'DELETE' })
  },
  digests() {
    return request<{ ok: true; digests: DailyDigest[] }>('/api/webui/digests')
  },
  setDigest(groupID: number, sendTime: string, messageCount: number) {
    return request<{ ok: true; digest: DailyDigest }>(`/api/webui/groups/${groupID}/digest`, {
      method: 'PUT',
      body: JSON.stringify({ send_time: sendTime, message_count: messageCount }),
    })
  },
  deleteDigest(groupID: number) {
    return request<{ ok: true }>(`/api/webui/groups/${groupID}/digest`, { method: 'DELETE' })
  },
  feeds() {
    return request<{ ok: true; feeds: FeedSubscription[] }>('/api/webui/feeds')
  },
  addFeed(groupID: number, url: string, name: string) {
    return request<{ ok: true; feed: FeedSubscription; latest_title: string }>(`/api/webui/groups/${groupID}/feeds`, {
      method: 'POST',
      body: JSON.stringify({ url, name }),
    })
  },
  addPlatformFeed(groupID: number, platform: string, target: string, name: string) {
    return request<{ ok: true; feed: FeedSubscription; latest_title: string }>(`/api/webui/groups/${groupID}/feeds/platform`, {
      method: 'POST',
      body: JSON.stringify({ platform, target, name }),
    })
  },
  deleteFeed(groupID: number, feedID: number) {
    return request<{ ok: true }>(`/api/webui/groups/${groupID}/feeds/${feedID}`, { method: 'DELETE' })
  },
  setFeedEnabled(groupID: number, feedID: number, enabled: boolean) {
    return request<{ ok: true; feed: FeedSubscription }>(`/api/webui/groups/${groupID}/feeds/${feedID}`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    })
  },
  checkFeeds(groupID: number) {
    return request<{ ok: true; result: FeedCheckResult }>(`/api/webui/groups/${groupID}/feeds/check`, { method: 'POST' })
  },
  feedSettings(groupID: number) {
    return request<{ ok: true; settings: FeedSettings; pending_count: number }>(`/api/webui/groups/${groupID}/feeds/settings`)
  },
  setFeedSettings(groupID: number, quietEnabled: boolean, quietStart: string, quietEnd: string) {
    return request<{ ok: true; settings: FeedSettings; pending_count: number }>(`/api/webui/groups/${groupID}/feeds/settings`, {
      method: 'PUT',
      body: JSON.stringify({ quiet_enabled: quietEnabled, quiet_start: quietStart, quiet_end: quietEnd }),
    })
  },
  knowledge(groupID: number) {
    return request<{ ok: true; knowledge: KnowledgeEntry[] }>(`/api/webui/knowledge?group_id=${groupID}`)
  },
  addKnowledge(groupID: number, payload: { title: string; content?: string; url?: string; shortcuts?: string[] }) {
    const path = groupID === 0 ? '/api/webui/knowledge/shared' : `/api/webui/groups/${groupID}/knowledge`
    return request<{ ok: true; knowledge: KnowledgeEntry }>(path, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  deleteKnowledge(groupID: number, knowledgeID: number) {
    const path = groupID === 0
      ? `/api/webui/knowledge/shared/${knowledgeID}`
      : `/api/webui/groups/${groupID}/knowledge/${knowledgeID}`
    return request<{ ok: true }>(path, { method: 'DELETE' })
  },
  setKnowledgeShortcuts(groupID: number, knowledgeID: number, shortcuts: string[]) {
    const path = groupID === 0
      ? `/api/webui/knowledge/shared/${knowledgeID}/shortcuts`
      : `/api/webui/groups/${groupID}/knowledge/${knowledgeID}/shortcuts`
    return request<{ ok: true; shortcuts: KnowledgeShortcut[] }>(path, {
      method: 'PUT',
      body: JSON.stringify({ shortcuts }),
    })
  },
  searchKnowledge(groupID: number, q: string) {
    const params = new URLSearchParams({ q })
    const path = groupID === 0
      ? '/api/webui/knowledge/shared/search'
      : `/api/webui/groups/${groupID}/knowledge/search`
    return request<{ ok: true; knowledge: KnowledgeEntry[] }>(`${path}?${params}`)
  },
}
