import { request } from './http.ts'

export interface GroupAIStyle {
  ok: true
  group_id: number
  style_prompt: string
  custom: boolean
  default_scope: boolean
  default_style_prompt: string
  max_chars: number
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

export interface AIConnection {
  id: string
  name: string
  kind: 'openai' | 'codex'
  base_url: string
  api_key: string
  key_configured: boolean
  clear_key?: boolean
}

export interface AIFeature {
  id: string
  name: string
  capabilities: string[]
}

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

export const aiApi = {
  modelUsage(
    groupID: number | null,
    days: number,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    const query = new URLSearchParams({ days: String(days) })
    if (groupID !== null) query.set('group_id', String(groupID))
    return request<{ ok: true; stats: ModelUsageStats[] }>(
      `/api/webui/ai/model-usage?${query}`,
      options,
    )
  },
  aiFeatures(options?: Pick<RequestInit, 'signal'>) {
    return request<{ features: AIFeature[] }>('/api/webui/ai/features', options)
  },
  testAIProfile(id: string, capability: string) {
    return request<{
      status: string
      message: string
      duration_ms: number
      image_url?: string
    }>(`/api/webui/ai/profiles/${encodeURIComponent(id)}/test`, {
      method: 'POST',
      body: JSON.stringify({ capability }),
    })
  },
  codexStatus(id: string, options?: Pick<RequestInit, 'signal'>) {
    return request<{ imported: boolean; account_id?: string }>(
      `/api/webui/ai/connections/${encodeURIComponent(id)}/auth`,
      options,
    )
  },
  importCodex(id: string, contents: string, replace: boolean) {
    return request<{ imported: boolean; account_id?: string }>(
      `/api/webui/ai/connections/${encodeURIComponent(id)}/auth?replace=${replace}`,
      { method: 'PUT', body: contents },
    )
  },
  groupAIStyle(groupID: number, options?: Pick<RequestInit, 'signal'>) {
    const path =
      groupID === 0
        ? '/api/webui/ai-style/default'
        : `/api/webui/groups/${groupID}/ai-style`
    return request<GroupAIStyle>(path, options)
  },
  setGroupAIStyle(groupID: number, stylePrompt: string) {
    const path =
      groupID === 0
        ? '/api/webui/ai-style/default'
        : `/api/webui/groups/${groupID}/ai-style`
    return request<GroupAIStyle>(path, {
      method: 'PUT',
      body: JSON.stringify({ style_prompt: stylePrompt }),
    })
  },
  resetGroupAIStyle(groupID: number) {
    const path =
      groupID === 0
        ? '/api/webui/ai-style/default'
        : `/api/webui/groups/${groupID}/ai-style`
    return request<GroupAIStyle>(path, { method: 'DELETE' })
  },
}
