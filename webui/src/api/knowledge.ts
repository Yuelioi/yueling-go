import { request } from './http.ts'

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

export const knowledgeApi = {
  knowledge(groupID: number | null, options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; knowledge: KnowledgeEntry[] }>(
      `/api/webui/knowledge${groupID === null ? '' : `?group_id=${groupID}`}`,
      options,
    )
  },
  addKnowledge(
    groupID: number,
    payload: {
      title: string
      content?: string
      url?: string
      shortcuts?: string[]
    },
  ) {
    const path =
      groupID === 0
        ? '/api/webui/knowledge/shared'
        : `/api/webui/groups/${groupID}/knowledge`
    return request<{ ok: true; knowledge: KnowledgeEntry }>(path, {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  },
  deleteKnowledge(groupID: number, knowledgeID: number) {
    const path =
      groupID === 0
        ? `/api/webui/knowledge/shared/${knowledgeID}`
        : `/api/webui/groups/${groupID}/knowledge/${knowledgeID}`
    return request<{ ok: true }>(path, { method: 'DELETE' })
  },
  setKnowledgeShortcuts(
    groupID: number,
    knowledgeID: number,
    shortcuts: string[],
  ) {
    const path =
      groupID === 0
        ? `/api/webui/knowledge/shared/${knowledgeID}/shortcuts`
        : `/api/webui/groups/${groupID}/knowledge/${knowledgeID}/shortcuts`
    return request<{ ok: true; shortcuts: KnowledgeShortcut[] }>(path, {
      method: 'PUT',
      body: JSON.stringify({ shortcuts }),
    })
  },
  searchKnowledge(
    groupID: number,
    q: string,
    options?: Pick<RequestInit, 'signal'>,
  ) {
    const params = new URLSearchParams({ q })
    const path =
      groupID === 0
        ? '/api/webui/knowledge/shared/search'
        : `/api/webui/groups/${groupID}/knowledge/search`
    return request<{ ok: true; knowledge: KnowledgeEntry[] }>(
      `${path}?${params}`,
      options,
    )
  },
}
