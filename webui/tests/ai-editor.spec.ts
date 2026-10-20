import { describe, expect, it, vi } from 'vitest'
import { defineComponent, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { aiApi, type AIConnection, type AIModelProfile } from '../src/api/ai.ts'
import ModelDialog from '../src/features/ai/components/ModelDialog.vue'
import { useModelWorkspace } from '../src/features/ai/useModelWorkspace.ts'
import { uiStubs } from './ui-stubs.ts'
const accounts: AIConnection[] = [
  {
    id: 'deepseek',
    name: 'DeepSeek',
    kind: 'openai',
    base_url: 'https://api.deepseek.com',
    api_key: '',
    key_configured: true,
  },
  {
    id: 'codex',
    name: 'Codex',
    kind: 'codex',
    base_url: '',
    api_key: '',
    key_configured: false,
  },
]
const image: AIModelProfile = {
  id: 'image',
  name: '图片',
  model: 'gpt-6.1-sol',
  connection_id: 'deepseek',
  capabilities: ['image_generation', 'image_edit'],
  api_key: '',
  base_url: '',
  vision: false,
  key_configured: false,
}
function fixture() {
  vi.spyOn(aiApi, 'aiFeatures').mockResolvedValue({ features: [] })
  vi.spyOn(aiApi, 'codexStatus').mockResolvedValue({ imported: true })
  const profiles = ref([structuredClone(image)]),
    connections = ref(structuredClone(accounts)),
    save = vi.fn()
  let state!: ReturnType<typeof useModelWorkspace>
  const host = mount(
    defineComponent({
      setup() {
        state = useModelWorkspace(
          profiles,
          connections,
          () => ({
            bindings: { image_edit: 'image' },
            conversation: '',
            vision: '',
          }),
          save,
        )
        return () => null
      },
    }),
  )
  return { profiles, connections, save, state, host }
}
describe('model editing workflow', () => {
  it('shows a misroute, blocks saving, then permits switching to an independent Codex account', async () => {
    const f = fixture()
    f.state.model.edit(f.profiles.value[0]!)
    const dialog = mount(ModelDialog, {
      props: { editor: f.state.model },
      global: { stubs: uiStubs },
    })
    const save = () =>
      dialog.findAll('button').find((button) => button.text() === '保存模型')!
    expect(save().attributes('disabled')).toBeDefined()
    await dialog.find('select').setValue('codex')
    expect(save().attributes('disabled')).toBeUndefined()
    await save().trigger('click')
    expect(dialog.emitted('save')).toHaveLength(1)
    f.state.saveModel()
    expect(f.save).toHaveBeenCalledOnce()
    expect(f.profiles.value[0]?.connection_id).toBe('codex')
    expect(f.connections.value[0]?.base_url).toBe('https://api.deepseek.com')
    await f.state.saved()
    expect(f.state.model.open.value).toBe(false)
    dialog.unmount()
    f.host.unmount()
  })
  it('cancels local edits without changing the saved profile', async () => {
    const f = fixture()
    f.state.model.edit(f.profiles.value[0]!)
    const dialog = mount(ModelDialog, {
      props: { editor: f.state.model },
      global: { stubs: uiStubs },
    })
    await dialog.findAll('input')[1]!.setValue('未保存的名称')
    await dialog
      .findAll('button')
      .find((button) => button.text() === '取消')!
      .trigger('click')
    expect(f.profiles.value[0]?.name).toBe('图片')
    expect(f.save).not.toHaveBeenCalled()
    expect(f.state.model.open.value).toBe(false)
    dialog.unmount()
    f.host.unmount()
  })
  it('saves an account before importing its file and keeps files out of configuration payloads', async () => {
    const upload = vi
      .spyOn(aiApi, 'importCodex')
      .mockResolvedValue({ imported: true })
    const f = fixture()
    f.state.chooseProvider('codex')
    const selected = f.state.account.selected.value!
    f.state.account.file.value = new File(['{"synthetic":true}'], 'auth.json', {
      type: 'application/json',
    })
    f.state.saveAccount()
    expect(f.connections.value.some((row) => row.id === selected.id)).toBe(true)
    expect(upload).not.toHaveBeenCalled()
    await f.state.saved()
    await flushPromises()
    expect(upload).toHaveBeenCalledWith(
      selected.id,
      '{"synthetic":true}',
      false,
    )
    expect(f.state.account.file.value).toBe(null)
    expect(JSON.stringify(f.connections.value)).not.toContain('synthetic')
    f.host.unmount()
  })
})
