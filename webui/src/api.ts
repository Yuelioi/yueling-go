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
  commands: string[]
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

async function request<T>(url: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(init.headers || {}) },
    ...init,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
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
  plugins() {
    return request<{ ok: true; plugins: PluginEntry[] }>('/api/webui/plugins')
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
  affinity(groupID: number | null, q: string) {
    const params = new URLSearchParams()
    if (groupID) params.set('group_id', String(groupID))
    if (q) params.set('q', q)
    return request<{ ok: true; affinity: AffinityRow[]; block_below: number }>(`/api/webui/affinity?${params}`)
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
}
