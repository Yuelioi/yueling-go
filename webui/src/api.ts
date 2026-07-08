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

export interface GroupMessagePayload {
  text?: string
  at_user_ids?: number[]
  images?: string[]
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
