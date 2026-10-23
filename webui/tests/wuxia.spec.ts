import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import WuxiaView from '../src/views/WuxiaView.vue'
import { wuxiaApi, type WuxiaSettings, type WuxiaState } from '../src/api/wuxia'
import { uiStubs } from './ui-stubs'
vi.mock('../src/stores/groups', () => ({
  useGroupsStore: () => ({
    loadManaged: async () => ({
      groups: [{ group_id: 100, group_name: '测试群' }],
    }),
  }),
}))
const numberInput = defineComponent({
  props: ['modelValue', 'type'],
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.modelValue,
        type: props.type,
        onInput: (e: Event) =>
          emit(
            'update:modelValue',
            Number((e.target as HTMLInputElement).value),
          ),
      })
  },
})
function state(settings: WuxiaSettings, custom = false): WuxiaState {
  return {
    ok: true,
    config: {
      settings: { ...settings },
      global: { daily_runs: 3, sign_silver: 30, practice_base: 50 },
      custom,
    },
    catalog: {
      items: [],
      skills: [],
      pools: [],
      version: 'fixture',
      max_level: 10,
      professions: [],
      weapons: [],
      events: [],
    },
    players: [],
    ledger: [],
  }
}
afterEach(() => vi.restoreAllMocks())
describe('wuxia economy settings', () => {
  it('saves numeric drafts to the selected scope and restores inherited values', async () => {
    const defaults = { daily_runs: 3, sign_silver: 30, practice_base: 50 }
    vi.spyOn(wuxiaApi, 'read').mockImplementation(async () => state(defaults))
    const save = vi
      .spyOn(wuxiaApi, 'save')
      .mockImplementation(async (_id, value) => state(value, true))
    const inherit = vi
      .spyOn(wuxiaApi, 'inherit')
      .mockResolvedValue(state(defaults))
    const wrapper = mount(WuxiaView, {
      global: {
        stubs: {
          WuxiaEquipmentPreview: true,
          ...uiStubs,
          UInput: numberInput,
          GroupScopeSelect: true,
          UBadge: true,
        },
      },
    })
    await flushPromises()
    const scope = wrapper.findComponent({ name: 'GroupScopeSelect' })
    scope.vm.$emit('update:modelValue', 100)
    await flushPromises()
    await wrapper.findAll('input')[0]!.setValue('6')
    expect(wrapper.find('pre').text()).toContain('每日 3 场')
    const button = () =>
      wrapper.findAll('button').find((b) => b.text().includes('保存数值'))!
    expect(button().attributes('disabled')).toBeUndefined()
    await button().trigger('click')
    await flushPromises()
    expect(save).toHaveBeenCalledWith(100, { ...defaults, daily_runs: 6 })
    expect(wrapper.find('pre').text()).toContain('每日 6 场')
    expect(button().attributes('disabled')).toBeDefined()
    await wrapper
      .findAll('button')
      .find((b) => b.text().includes('恢复全局默认'))!
      .trigger('click')
    await flushPromises()
    expect(inherit).toHaveBeenCalledWith(100)
    expect(wrapper.find('pre').text()).toContain('每日 3 场')
    expect(
      (wrapper.findAll('input')[0]!.element as HTMLInputElement).value,
    ).toBe('3')
    wrapper.unmount()
  })
})

describe('wuxia draw pools', () => {
  it('shows configured costs, reward names and raw weights and player eligibility explanation', async () => {
    const value = state({ daily_runs: 3, sign_silver: 30, practice_base: 50 })
    value.catalog.items = [
      { id: 'armor', name: '青布劲装', kind: 'armor', min_level: 1 },
      { id: 'manual', name: '骤雨剑诀秘籍', kind: 'manual', min_level: 1 },
    ]
    value.catalog.skills = [
      {
        id: 'sword',
        name: '骤雨剑诀',
        description: '提升暴击机会',
        weapon_kind: '剑',
      },
    ]
    value.catalog.pools = [
      {
        id: 'test',
        name: '测试宝匣',
        cost_item: 'silver',
        cost: 60,
        entries: [
          { reward: { item_id: 'armor', quantity: 1 }, weight: 1 },
          { reward: { item_id: 'manual', quantity: 2 }, weight: 3 },
        ],
      },
    ]
    vi.spyOn(wuxiaApi, 'read').mockResolvedValue(value)
    const wrapper = mount(WuxiaView, {
      global: {
        stubs: {
          WuxiaEquipmentPreview: true,
          ...uiStubs,
          UInput: numberInput,
          GroupScopeSelect: true,
          UBadge: true,
        },
      },
    })
    await flushPromises()
    for (const text of [
      '测试宝匣',
      '每抽 60 银两',
      '青布劲装 ×1',
      '骤雨剑诀秘籍 ×2',
      '权重 1',
      '权重 3',
      '提升暴击机会',
    ])
      expect(wrapper.text()).toContain(text)
  })
})
