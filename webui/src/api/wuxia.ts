import { request } from './http'

export interface WuxiaSettings {
  daily_runs: number
  sign_silver: number
  practice_base: number
}
export interface WuxiaState {
  ok: true
  config: { settings: WuxiaSettings; global: WuxiaSettings; custom: boolean }
  catalog: {
    items: { id: string; name: string; kind: string; min_level: number }[]
    skills: {
      kind?: string
      tier?: number
      id: string
      name: string
      description: string
      weapon_kind: string
    }[]
    pools: {
      id: string
      name: string
      cost_item: string
      cost: number
      entries: {
        reward: { item_id: string; quantity: number }
        weight: number
      }[]
    }[]
    max_level: number
    version: string
    professions: {
      id: string
      name: string
      description: string
      weapon_kind: string
      per_level: { hp: number; attack: number; defense: number; speed: number }
    }[]
    weapons: { id: string; name: string; kind: string; min_level: number }[]
    events: { id: string; title: string; kind: string; weight: number }[]
  }
  players: {
    character: {
      user_id: number
      name: string
      silver: number
      mastery: number
    }
    level: number
    power: number
    profession: { name: string }
    weapon: { name: string }
  }[]
  ledger: {
    id: number
    user_id: number
    delta: number
    balance: number
    reason: string
    created_at: string
  }[]
}
export const wuxiaApi = {
  read(scope: number, signal?: AbortSignal) {
    return request<WuxiaState>(`/api/webui/wuxia/${scope}`, { signal })
  },
  save(scope: number, settings: WuxiaSettings) {
    return request<WuxiaState>(`/api/webui/wuxia/${scope}`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    })
  },
  inherit(scope: number) {
    return request<WuxiaState>(`/api/webui/wuxia/${scope}`, {
      method: 'DELETE',
    })
  },
}
