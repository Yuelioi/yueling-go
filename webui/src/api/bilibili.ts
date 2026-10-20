import { request } from './http.ts'

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
  contacts: {
    number: number
    uid: number
    name: string
    preview: string
    last_message_at: number
  }[]
}

export const bilibiliApi = {
  bilibili(options?: Pick<RequestInit, 'signal'>) {
    return request<BilibiliResponse>('/api/webui/bilibili', options)
  },
  saveBilibili(settings: BilibiliSettings) {
    return request<BilibiliResponse>('/api/webui/bilibili/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },
}
