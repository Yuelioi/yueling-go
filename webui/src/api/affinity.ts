import { request } from './http.ts'

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

export const affinityApi = {
  affinity(
    groupID: number | null,
    q: string,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    const params = new URLSearchParams()
    if (groupID) params.set('group_id', String(groupID))
    if (q) params.set('q', q)
    return request<{ ok: true; affinity: AffinityRow[]; block_below: number }>(
      `/api/webui/affinity?${params}`,
      options,
    )
  },
  affinitySettings(options?: Pick<RequestInit, 'signal'>) {
    return request<AffinitySettings>('/api/webui/affinity/settings', options)
  },
  setAffinitySettings(tiers: AffinityTier[]) {
    return request<AffinitySettings>('/api/webui/affinity/settings', {
      method: 'PUT',
      body: JSON.stringify({ tiers }),
    })
  },
  resetAffinitySettings() {
    return request<AffinitySettings>('/api/webui/affinity/settings', {
      method: 'DELETE',
    })
  },
  setAffinityScore(id: number, score: number) {
    return request<{ ok: true; affinity: AffinityRow }>(
      `/api/webui/affinity/${id}/score`,
      {
        method: 'PUT',
        body: JSON.stringify({ score }),
      },
    )
  },
  adjustAffinity(id: number, delta: number) {
    return request<{ ok: true; affinity: AffinityRow }>(
      `/api/webui/affinity/${id}/adjust`,
      {
        method: 'POST',
        body: JSON.stringify({ delta }),
      },
    )
  },
  resetAffinity(id: number) {
    return request<{ ok: true; affinity: AffinityRow }>(
      `/api/webui/affinity/${id}/reset`,
      { method: 'POST' },
    )
  },
}
