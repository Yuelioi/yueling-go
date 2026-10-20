import { request } from './http.ts'

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

export interface GroupInfo {
  group_id: number
  group_name: string
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

export interface GroupMessagePayload {
  text?: string
  at_user_ids?: number[]
  images?: string[]
}

export const groupsApi = {
  groups(options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; groups: GroupInfo[] }>(
      '/api/webui/groups',
      options,
    )
  },
  async managedGroups() {
    const res = await groupsApi.groupAdministration()
    return {
      ok: true as const,
      groups: res.groups.filter((row) => row.group_id > 0),
    }
  },
  groupAdministration(options?: Pick<RequestInit, 'signal'>) {
    return request<{
      ok: true
      groups: GroupAdministration[]
      warning: string
    }>('/api/webui/group-administration', options)
  },
  joinReview(groupID: number, options?: Pick<RequestInit, 'signal'>) {
    return request<JoinReviewState>(
      `/api/webui/join-review/${groupID === 0 ? 'default' : groupID}`,
      options,
    )
  },
  setJoinReview(groupID: number, config: JoinReviewConfig) {
    return request<JoinReviewState>(
      `/api/webui/join-review/${groupID === 0 ? 'default' : groupID}`,
      {
        method: 'PUT',
        body: JSON.stringify(config),
      },
    )
  },
  sendGroupMessage(groupID: number, payload: string | GroupMessagePayload) {
    const body = typeof payload === 'string' ? { text: payload } : payload
    return request<{ ok: true; message_id: number }>(
      `/api/webui/groups/${groupID}/messages`,
      {
        method: 'POST',
        body: JSON.stringify(body),
      },
    )
  },
}
