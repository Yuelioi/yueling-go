import { request } from './http'

export interface EquipmentStats {
  hp: number
  attack: number
  defense: number
  speed: number
}
export interface EquipmentTraits {
  critical: number
  dodge: number
  guard: number
  pierce: number
}
export interface PreviewItem {
  image?: string
  description?: string
  max_weapon_level?: number
  weapon_level?: number
  id: string
  name: string
  kind: string
  tier: number
  min_level: number
  attack?: number
  stats: EquipmentStats
  traits: EquipmentTraits
  professions?: string[]
}
export interface PreviewBuild {
  mind?: string
  weapon_level?: number
  profession: string
  level: number
  mastery: number
  weapon: string
  armor: string
  charm: string
  skill: string
}
export interface PreviewView {
  stats: EquipmentStats
  power: number
  profession: { name: string }
  weapon: { name: string }
}
export interface PreviewResult {
  samples?: number
  version: string
  actor: PreviewView
  opponent: PreviewView
  win_probability: number
  events: [EquipmentTraits, EquipmentTraits]
}
export interface PreviewData {
  weapon_chances?: { tier: number; name: string; weight: number }[]
  catalog: {
    version: string
    max_level: number
    professions: { id: string; name: string; weapon_kind: string }[]
    weapons: PreviewItem[]
    items: PreviewItem[]
    skills: {
      kind?: string
      id: string
      name: string
      weapon_kind: string
      min_level: number
    }[]
  }
  balance: {
    scope: string
    builds: { name: string; build: PreviewBuild }[]
    matches: {
      left: number
      right: number
      expected: number
      observed: number
      samples: number
    }[]
  }
}
export const wuxiaPreviewApi = {
  draw: (profession: string, count: number) =>
    request<{ items: PreviewItem[] }>('/api/webui/wuxia-preview/draw', {
      method: 'POST',
      body: JSON.stringify({ profession, count }),
    }),
  catalog: () => request<PreviewData>('/api/webui/wuxia-preview/catalog'),
  build: (left: PreviewBuild, right: PreviewBuild, signal?: AbortSignal) =>
    request<PreviewResult>('/api/webui/wuxia-preview/build', {
      method: 'POST',
      body: JSON.stringify({ left, right }),
      signal,
    }),
  card: (build: PreviewBuild) =>
    `/api/webui/wuxia-preview/card?${new URLSearchParams({ build: JSON.stringify(build) })}`,
}
