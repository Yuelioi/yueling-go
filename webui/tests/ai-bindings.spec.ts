import { describe, expect, it, vi } from 'vitest'
import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import {
  aiApi,
  configurationApi,
  type AIModelProfile,
  type ConfigurationField,
  type RuntimeConfiguration,
} from '../src/api.ts'
import RuntimeSettings from '../src/components/RuntimeSettings.vue'
import { useConfigurationStore } from '../src/stores/configuration.ts'
import { uiStubs } from './ui-stubs.ts'

function profile(id: string, capabilities: string[]): AIModelProfile {
  return {
    id,
    name: id,
    model: 'fixture-model',
    connection_id: 'account',
    base_url: '',
    api_key: '',
    key_configured: false,
    vision: capabilities.includes('vision'),
    capabilities,
  }
}
describe('primary feature model save', () => {
  it('submits both default selections, removes old off overrides and keeps them selected after save and refresh', async () => {
    const fields: Pick<ConfigurationField, 'path' | 'kind' | 'value'>[] = [
      {
        path: 'ai.profiles',
        kind: 'ai_profiles',
        value: [
          profile('chat', ['chat', 'tools']),
          profile('eyes', ['chat', 'vision']),
          profile('image', ['image_generation']),
        ],
      },
      {
        path: 'ai.connections',
        kind: 'ai_connections',
        value: [
          {
            id: 'account',
            name: '测试账号',
            kind: 'openai',
            base_url: 'https://fixture.invalid/v1',
            api_key: '',
            key_configured: true,
          },
        ],
      },
      {
        path: 'ai.bindings',
        kind: 'ai_bindings',
        value: {
          conversation: 'off',
          vision: 'off',
          image_generation: 'image',
        },
      },
      { path: 'ai.conversation_profile', kind: 'text', value: '' },
      { path: 'ai.vision_profile', kind: 'text', value: '' },
    ]
    let view: RuntimeConfiguration = {
      revision: 1,
      applied_revision: 1,
      restart_pending: false,
      bootstrap: {
        data_dir: 'fixture',
        note: 'fixture',
        database_dsn_configured: true,
      },
      sections: [
        {
          id: 'ai',
          title: 'AI',
          fields: fields.map((field) => ({
            ...field,
            label: field.path,
            secret: false,
            configured: false,
            reload: false,
          })),
        },
      ],
    }
    vi.spyOn(configurationApi, 'configuration').mockImplementation(async () =>
      structuredClone(view),
    )
    const save = vi
      .spyOn(configurationApi, 'saveConfiguration')
      .mockImplementation(async (revision, section, values) => {
        const payload: Record<string, unknown> = JSON.parse(
          JSON.stringify(values),
        )
        view = {
          ...view,
          revision: revision + 1,
          applied_revision: revision + 1,
          sections: view.sections.map((row) =>
            row.id !== section
              ? row
              : {
                  ...row,
                  fields: row.fields.map((field) => ({
                    ...field,
                    value:
                      field.path in payload
                        ? (payload[field.path] as typeof field.value)
                        : field.value,
                  })),
                },
          ),
        }
        return { ok: true, reload: false, config: structuredClone(view) }
      })
    vi.spyOn(aiApi, 'aiFeatures').mockResolvedValue({
      features: [
        {
          id: 'conversation',
          name: 'Bot 对话与操作',
          capabilities: ['chat', 'tools'],
        },
        { id: 'vision', name: '图片理解', capabilities: ['chat', 'vision'] },
        {
          id: 'image_generation',
          name: '生成图片',
          capabilities: ['image_generation'],
        },
      ],
    })
    const pinia = createPinia()
    const render = () =>
      mount(RuntimeSettings, {
        props: { sections: ['ai'] },
        global: {
          plugins: [pinia],
          stubs: {
            ...uiStubs,
            ConfigurationGuard: true,
            UCard: { template: '<div><slot /></div>' },
            ULink: { template: '<a><slot /></a>' },
          },
        },
      })
    let wrapper = render()
    await flushPromises()
    const choice = (name: string) =>
      wrapper.get(`select[aria-label="${name}使用的模型"]`)
    expect((choice('Bot 对话与操作').element as HTMLSelectElement).value).toBe(
      'off',
    )
    expect((choice('图片理解').element as HTMLSelectElement).value).toBe('off')
    await choice('Bot 对话与操作').setValue('chat')
    await choice('图片理解').setValue('eyes')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '保存修改')!
      .trigger('click')
    await flushPromises()
    expect(save).toHaveBeenCalledTimes(1)
    expect(save.mock.calls[0]![2]).toEqual({
      'ai.bindings': { image_generation: 'image' },
      'ai.conversation_profile': 'chat',
      'ai.vision_profile': 'eyes',
    })
    const assertSelected = () => {
      expect(
        (choice('Bot 对话与操作').element as HTMLSelectElement).value,
      ).toBe('chat')
      expect((choice('图片理解').element as HTMLSelectElement).value).toBe(
        'eyes',
      )
      expect((choice('生成图片').element as HTMLSelectElement).value).toBe(
        'image',
      )
    }
    assertSelected()
    wrapper.unmount()
    await useConfigurationStore(pinia).load(true)
    wrapper = render()
    await flushPromises()
    assertSelected()
    wrapper.unmount()
  })
})
