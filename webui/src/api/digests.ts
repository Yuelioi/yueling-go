import { request } from './http.ts'

export interface DailyDigest {
  ID: number
  GroupID: number
  CreatedBy: number
  SendTime: string
  CronExpr: string
  MessageCount: number
  Enabled: boolean
}

export const digestsApi = {
  digests(options?: Pick<RequestInit, 'signal'>) {
    return request<{ ok: true; digests: DailyDigest[] }>(
      '/api/webui/digests',
      options,
    )
  },
  setDigest(groupID: number, sendTime: string, messageCount: number) {
    return request<{ ok: true; digest: DailyDigest }>(
      `/api/webui/groups/${groupID}/digest`,
      {
        method: 'PUT',
        body: JSON.stringify({
          send_time: sendTime,
          message_count: messageCount,
        }),
      },
    )
  },
  deleteDigest(groupID: number) {
    return request<{ ok: true }>(`/api/webui/groups/${groupID}/digest`, {
      method: 'DELETE',
    })
  },
}
