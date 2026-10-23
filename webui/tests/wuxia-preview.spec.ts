import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import WuxiaEquipmentPreview from '../src/components/WuxiaEquipmentPreview.vue'
import {
  wuxiaPreviewApi,
  type PreviewData,
  type PreviewResult,
} from '../src/api/wuxiaPreview'
import { uiStubs } from './ui-stubs'

const fixtures = vi.hoisted(() => ({
  catalog: vi.fn(),
  build: vi.fn(),
  draw: vi.fn(),
  card: vi.fn(() => '/preview-card'),
}))
vi.mock('../src/api/wuxiaPreview', () => ({ wuxiaPreviewApi: fixtures }))
afterEach(() => {
  vi.clearAllMocks()
  vi.useRealTimers()
})
const stats = { hp: 200, attack: 30, defense: 15, speed: 20 }
const traits = { critical: 0.1, guard: 0.1, dodge: 0.05, pierce: 0.05 }
const input = {
  profession: 'swordsman',
  level: 10,
  mastery: 10,
  weapon: 'sword',
  armor: '',
  charm: '',
  skill: '',
}
const data: PreviewData = {
  catalog: {
    version: 'equipment-preview-1',
    max_level: 10,
    professions: [{ id: 'swordsman', name: '剑客', weapon_kind: '剑' }],
    weapons: [
      {
        id: 'sword',
        name: '松纹剑',
        kind: '剑',
        tier: 1,
        min_level: 1,
        stats,
        traits,
        professions: ['swordsman'],
      },
      {
        id: 'elite',
        name: '惊鸿剑',
        kind: '剑',
        tier: 4,
        min_level: 1,
        max_weapon_level: 10,
        stats,
        traits,
        professions: ['swordsman'],
      },
      {
        id: 'blade',
        name: '朴刀',
        kind: '刀',
        tier: 1,
        min_level: 1,
        stats,
        traits,
        professions: ['blade'],
      },
    ],
    items: [],
    skills: [],
  },
  balance: {
    scope: '10级样本',
    builds: [
      { name: '剑客搭配', build: input },
      { name: '对手二', build: input },
      { name: '对手三', build: input },
    ],
    matches: [
      { left: 0, right: 2, expected: 0.5, observed: 0.49, samples: 600 },
    ],
  },
}
const result: PreviewResult = {
  version: 'equipment-preview-1',
  actor: {
    stats,
    power: 100,
    profession: { name: '剑客' },
    weapon: { name: '松纹剑' },
  },
  opponent: {
    stats,
    power: 100,
    profession: { name: '刀客' },
    weapon: { name: '朴刀' },
  },
  win_probability: 0.5,
  events: [traits, traits],
}
it('previews without mutating live players and filters by level/profession', async () => {
  vi.useFakeTimers()
  fixtures.catalog.mockResolvedValue(data)
  fixtures.build.mockResolvedValue(result)
  const wrapper = mount(WuxiaEquipmentPreview, { global: { stubs: uiStubs } })
  await flushPromises()
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  expect(wrapper.text()).toContain('待发布设计')
  expect(wrapper.text()).toContain('名器')
  expect(wrapper.text()).toContain('理论胜率 50.0%')
  expect(wrapper.text()).not.toContain('朴刀 ·')
  await wrapper.findAll('select')[1]!.setValue('1')
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  expect(wrapper.text()).toContain('惊鸿剑')
  expect(wuxiaPreviewApi.build).toHaveBeenLastCalledWith(
    expect.objectContaining({ level: 1, mastery: 1 }),
    expect.anything(),
    expect.any(AbortSignal),
  )
  wrapper.unmount()
})
it('shows API failures instead of retaining a previous estimate', async () => {
  fixtures.catalog.mockResolvedValue(data)
  fixtures.build.mockRejectedValue(new Error('装备不符合等级'))
  const wrapper = mount(WuxiaEquipmentPreview, { global: { stubs: uiStubs } })
  await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toBe('装备不符合等级')
  expect(wrapper.text()).not.toContain('理论胜率')
  wrapper.unmount()
})
