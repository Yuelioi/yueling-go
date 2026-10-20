import { request } from './http.ts'

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

export const memoryApi = {
  memories(q: string, options?: Pick<RequestInit, 'signal'>) {
    const params = new URLSearchParams()
    if (q) params.set('q', q)
    return request<{ ok: true; memories: MemoryRow[] }>(
      `/api/webui/memories?${params}`,
      options,
    )
  },
  deleteMemory(id: number) {
    return request<{ ok: true }>(`/api/webui/memories/${id}`, {
      method: 'DELETE',
    })
  },
  clearUserMemories(userID: number) {
    return request<{ ok: true; deleted: number }>(
      `/api/webui/memories/users/${userID}`,
      { method: 'DELETE' },
    )
  },
}
