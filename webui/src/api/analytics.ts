import { request } from './http.ts'
import type { AffinityRow } from './affinity.ts'
import type { MemoryRow } from './memory.ts'

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

export const analyticsApi = {
  overview(options?: Pick<RequestInit, 'signal'>) {
    return request<OverviewData>('/api/webui/overview', options)
  },
  commandUsage(
    groupID: number,
    days: number,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    return request<CommandUsageResponse>(
      `/api/webui/command-usage?group_id=${groupID}&days=${days}`,
      options,
    )
  },
  chatInsights(
    groupID: number,
    period: string,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    return request<ChatInsightsResponse>(
      `/api/webui/chat-insights?group_id=${groupID}&period=${encodeURIComponent(period)}`,
      options,
    )
  },
  chatHistory(
    groupID: number,
    beforeAt?: number,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    const suffix = beforeAt ? `?before_at=${beforeAt}` : ''
    return request<{
      ok: true
      group_id: number
      before_at: number
      stats: ChatHistoryStats
    }>(`/api/webui/groups/${groupID}/chat-history${suffix}`, options)
  },
  deleteChatHistory(
    groupID: number,
    payload: { before_at?: number; all?: boolean },
  ) {
    return request<{
      ok: true
      group_id: number
      deleted: number
      remaining: number
    }>(`/api/webui/groups/${groupID}/chat-history`, {
      method: 'DELETE',
      body: JSON.stringify(payload),
    })
  },
  chatOverview(period: string, options?: Pick<RequestInit, 'signal'>) {
    return request<{
      ok: true
      groups: {
        group_id: number
        total: number
        text_total: number
        participants: number
      }[]
    }>(`/api/webui/chat-overview?period=${encodeURIComponent(period)}`, options)
  },
}
