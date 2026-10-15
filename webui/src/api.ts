export interface GroupAdministration extends GroupInfo {
  connected: boolean
  disabled_plugins: number[]
  custom_style: boolean
  style_prompt: string
  join_mode: string
  feed_count: number
  failing_feeds: number
  knowledge_count: number
  digest_time: string
}
export interface FeedHealth {
  id: number
  group_id: number
  status: string
  message: string
  checked_at: number
}

export interface JoinReviewConfig {
  mode: 'inherit' | 'override' | 'disabled'
  allow: string[]
  deny: string[]
}

export interface JoinReviewState {
  group_id: number
  config: JoinReviewConfig
  global: JoinReviewConfig
  effective: JoinReviewConfig
}

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
  translate_to_chinese: boolean
  created_at: number
  updated_at: number
  consecutive_failures: number
  last_error: string
  last_checked_at: number
  last_success_at: number
  next_check_at: number
}

export interface FeedSettings {
  poll_interval_minutes: number
  group_id: number
  quiet_enabled: boolean
  quiet_start: string
  quiet_end: string
  item_max_chars: number
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

export interface ChatInsightWord {
  text: string
  count: number
}

export interface ChatInsightUser {
  user_id: number
  nickname: string
  count: number
  phrases: ChatInsightWord[]
  words: ChatInsightWord[]
}

export interface ChatInsightsResponse {
  ok: true
  group_id: number
  period: string
  period_label: string
  start_at: number
  end_at: number
  retention_days: number
  retention_policy: 'indefinite'
  summary: {
    total: number
    text_total: number
    participants: number
  }
  words: ChatInsightWord[]
  users: ChatInsightUser[]
}

export interface ChatHistoryStats {
  total: number
  matched: number
  oldest_at: number
  newest_at: number
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

export interface BilibiliSettings {
  enabled: boolean
  poll_seconds: number
  notify_users: number[]
  reply_mode: 'off' | 'keyword' | 'ai' | 'hybrid'
  keywords: { keyword: string; reply: string; exact: boolean }[]
  reply_cooldown_seconds: number
  reply_per_minute: number
  ai_prompt: string
  uid: number
  recent_count: number
}

export interface BilibiliResponse {
  ok: true
  settings: BilibiliSettings
  status: string
  login_configured: boolean
  contacts: { number: number; uid: number; name: string; preview: string; last_message_at: number }[]
}

export interface ImageCategory { name: string; aliases: string[] | null; external: boolean }
export interface ImageAsset { id: string; name: string }
export interface ImageAssetPage { assets: ImageAsset[]; category: string; total: number; page: number; page_size: number }
export function imagePreviewURL(category: string,id: string,thumbnail = false) {
 return `/api/webui/image-assets/${encodeURIComponent(id)}/preview?${new URLSearchParams({category,...(thumbnail ? {thumbnail:'1'} : {})})}`
}

export interface ModelUsageStats {
  profile_id: string
  feature: string
 group_id: number
 model: string
 requests: number
 prompt_tokens: number
 completion_tokens: number
 cache_reported_requests: number
 cache_reported_prompt_tokens: number
 cached_tokens: number
 uncached_tokens: number
 duration_ms: number
 cache_hit_rate: number | null
}

export const api = {
 modelUsage(groupID: number | null, days: number) {
  const query = new URLSearchParams({ days: String(days) })
  if (groupID !== null) query.set('group_id', String(groupID))
  return request<{ok: true; stats: ModelUsageStats[]}>(`/api/webui/ai/model-usage?${query}`)
 },
 imageCategories() { return request<{categories:ImageCategory[]}>('/api/webui/image-assets/categories') },
 imageAssets(category:string,query:string,page:number) {
  return request<ImageAssetPage>(`/api/webui/image-assets?${new URLSearchParams({category,query,page:String(page)})}`)
 },
 deleteImageAsset(category:string,id:string) {
  return request<{ok:true;name:string}>(`/api/webui/image-assets/${encodeURIComponent(id)}?${new URLSearchParams({category})}`,{method:'DELETE'})
 },
  aiFeatures() { return request<{features: AIFeature[]}>('/api/webui/ai/features') },
  testAIProfile(id: string, capability: string) { return request<{status: string; message: string; duration_ms: number}>(`/api/webui/ai/profiles/${encodeURIComponent(id)}/test`, {method:'POST', body:JSON.stringify({capability})}) },
  codexStatus(id: string) { return request<{imported: boolean; account_id?: string}>(`/api/webui/ai/connections/${encodeURIComponent(id)}/auth`) },
  importCodex(id: string, contents: string, replace: boolean) { return request<{imported: boolean; account_id?: string}>(`/api/webui/ai/connections/${encodeURIComponent(id)}/auth?replace=${replace}`, {method:'PUT',body:contents}) },
  configuration() { return request<RuntimeConfiguration>('/api/webui/config') },
  saveConfiguration(revision: number, section: string, values: Record<string, unknown>, clear_secrets: string[]) {
    return request<{ ok: true; reload: boolean; config: RuntimeConfiguration }>('/api/webui/config', {
      method: 'PUT', body: JSON.stringify({ revision, section, values, clear_secrets }),
    })
  },
  bilibili() {
    return request<BilibiliResponse>('/api/webui/bilibili')
  },
  saveBilibili(settings: BilibiliSettings) {
    return request<BilibiliResponse>('/api/webui/bilibili/settings', {
      method: 'PUT', body: JSON.stringify(settings),
    })
  },

  joinReview(groupID: number) {
    return request<JoinReviewState>(`/api/webui/join-review/${groupID === 0 ? 'default' : groupID}`)
  },
  setJoinReview(groupID: number, config: JoinReviewConfig) {
    return request<JoinReviewState>(`/api/webui/join-review/${groupID === 0 ? 'default' : groupID}`, {
      method: 'PUT', body: JSON.stringify(config),
    })
  },
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
  chatInsights(groupID: number, period: string) {
    return request<ChatInsightsResponse>(`/api/webui/chat-insights?group_id=${groupID}&period=${encodeURIComponent(period)}`)
  },
  chatHistory(groupID: number, beforeAt?: number) {
    const suffix = beforeAt ? `?before_at=${beforeAt}` : ''
    return request<{ ok: true; group_id: number; before_at: number; stats: ChatHistoryStats }>(`/api/webui/groups/${groupID}/chat-history${suffix}`)
  },
  deleteChatHistory(groupID: number, payload: { before_at?: number; all?: boolean }) {
    return request<{ ok: true; group_id: number; deleted: number; remaining: number }>(`/api/webui/groups/${groupID}/chat-history`, {
      method: 'DELETE',
      body: JSON.stringify(payload),
    })
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
  chatOverview(period: string) {
    return request<{ ok: true; groups: { group_id: number; total: number; text_total: number; participants: number }[] }>(`/api/webui/chat-overview?period=${encodeURIComponent(period)}`)
  },
  async managedGroups() {
    const res = await api.groupAdministration()
    return { ok: true as const, groups: res.groups.filter(row => row.group_id > 0) }
  },
  groupAdministration() {
    return request<{ ok: true; groups: GroupAdministration[]; warning: string }>('/api/webui/group-administration')
  },
  diagnoseFeeds(groupID: number | null = null, id = 0) {
    return request<{ ok: true; results: FeedHealth[] }>('/api/webui/feeds/diagnose', {
      method: 'POST', body: JSON.stringify({ group_id: groupID ?? 0, id }),
    })
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
  setFeedTranslation(groupID: number, feedID: number, translateToChinese: boolean) {
    return request<{ ok: true; feed: FeedSubscription }>(`/api/webui/groups/${groupID}/feeds/${feedID}`, {
      method: 'PUT',
      body: JSON.stringify({ translate_to_chinese: translateToChinese }),
    })
  },
  checkFeeds() {
    return request<{ ok: true; result: FeedCheckResult }>(`/api/webui/feeds/check`, { method: 'POST' })
  },
  feedSettings(groupID: number) {
    return request<{ ok: true; settings: FeedSettings; pending_count: number }>(`/api/webui/groups/${groupID}/feeds/settings`)
  },
  setFeedSettings(groupID: number, settings: Pick<FeedSettings, 'quiet_enabled' | 'quiet_start' | 'quiet_end' | 'item_max_chars' | 'poll_interval_minutes'>) {
    return request<{ ok: true; settings: FeedSettings; pending_count: number }>(`/api/webui/groups/${groupID}/feeds/settings`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },
  knowledge(groupID: number | null) {
    return request<{ ok: true; knowledge: KnowledgeEntry[] }>(`/api/webui/knowledge${groupID === null ? "" : `?group_id=${groupID}`}`)
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

export interface ImageEntry {
  folder: string; call: string[]; add: string; kind: string; arg: boolean | null
  url: string; pick: string; base: string
}
export interface AIConnection {
 id: string; name: string; kind: 'openai' | 'codex'; base_url: string; api_key: string; key_configured: boolean; clear_key?: boolean
}
export interface AIFeature { id: string; name: string; capabilities: string[] }
export interface AIModelProfile {
 connection_id?: string
 capabilities?: string[] | null
 disabled?: boolean
 max_tokens?: number
 timeout_seconds?: number
 reasoning_effort?: string
  id: string
  name: string
  base_url: string
  model: string
  api_key: string
  vision: boolean
  key_configured: boolean
  clear_key?: boolean
}

export interface ConfigurationField {
  path: string; label: string; kind: string; value: string | number | boolean | number[] | ImageEntry[] | AIModelProfile[] | AIConnection[] | Record<string, string> | null
  secret: boolean; configured: boolean; reload: boolean
}
export interface RuntimeConfiguration {
  applied_revision: number
  restart_pending: boolean
  revision: number
  sections: { id: string; title: string; fields: ConfigurationField[] }[]
  bootstrap: { database_dsn_configured: boolean; data_dir: string; note: string }
}
