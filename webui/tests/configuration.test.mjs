import test from 'node:test'
import assert from 'node:assert/strict'
import { effectScope } from 'vue'
import { useRuntimeConfiguration } from '../src/composables/useRuntimeConfiguration.ts'

const config = (revision = 1) => ({
  revision,
  applied_revision: revision,
  restart_pending: false,
  bootstrap: {},
  sections: [
    {
      id: 'bot',
      title: 'Bot',
      fields: [
        { path: 'bot.name', kind: 'text', value: 'saved', label: '名称' },
      ],
    },
    {
      id: 'ai',
      title: 'AI',
      fields: [
        { path: 'ai.model', kind: 'text', value: 'model', label: '模型' },
        {
          path: 'ai.key',
          kind: 'text',
          value: '',
          label: 'Key',
          secret: true,
          configured: true,
        },
      ],
    },
  ],
})
const deferred = () => {
  let resolve, reject
  const promise = new Promise((a, b) => {
    resolve = a
    reject = b
  })
  return { promise, resolve, reject }
}

test('failed and conflicting saves preserve drafts and credential intent', async () => {
  let sent
  const client = {
    configuration: async () => config(),
    plugins: async () => ({ plugins: [] }),
    saveConfiguration: async (...args) => {
      sent = args
      throw new Error('409 conflict')
    },
  }
  const state = useRuntimeConfiguration(['bot', 'ai'], client)
  await state.load()
  state.selected.value = 'ai'
  state.values.value['ai.model'] = 'draft'
  state.cleared.value['ai.key'] = true
  await state.save()
  assert.equal(state.values.value['ai.model'], 'draft')
  assert.equal(state.cleared.value['ai.key'], true)
  assert.equal(state.configuration.value.revision, 1)
  assert.equal(state.error.value, '409 conflict')
  assert.equal(sent[2]['ai.key'], '')
  assert.deepEqual(sent[3], ['ai.key'])
})

test('saving one section does not advance a dirty section revision', async () => {
  const revisions = []
  const client = {
    configuration: async () => config(),
    plugins: async () => ({ plugins: [] }),
    saveConfiguration: async (revision) => {
      revisions.push(revision)
      return { config: config(2), reload: false }
    },
  }
  const state = useRuntimeConfiguration(['bot', 'ai'], client)
  await state.load()
  state.values.value['ai.model'] = 'unsaved AI'
  state.values.value['bot.name'] = 'new bot'
  await state.save()
  assert.equal(state.values.value['ai.model'], 'unsaved AI')
  state.selected.value = 'ai'
  await state.save()
  assert.deepEqual(revisions, [1, 1])
})

test('out-of-order loads and disposed owners cannot publish responses', async () => {
  const first = deferred(),
    second = deferred()
  let calls = 0
  const client = {
    configuration: () => (++calls === 1 ? first.promise : second.promise),
    plugins: async () => ({ plugins: [] }),
  }
  const scope = effectScope()
  const state = scope.run(() => useRuntimeConfiguration(['bot'], client))
  const old = state.load(),
    recent = state.load()
  second.resolve(config(2))
  await recent
  first.resolve(config(1))
  await old
  assert.equal(state.configuration.value.revision, 2)
  const pending = deferred()
  client.configuration = () => pending.promise
  const last = state.load()
  scope.stop()
  pending.resolve(config(3))
  await last
  assert.equal(state.configuration.value.revision, 2)
})

test('profile updates preserve blank keys and encode explicit clearing without echoing stored keys', async () => {
  const view = config()
  view.sections[1].fields.push({
    path: 'ai.profiles', kind: 'ai_profiles', label: '模型配置', value: [
      { id: 'keep', name: 'Keep', api_key: '', key_configured: true },
      { id: 'clear', name: 'Clear', api_key: '', key_configured: true },
    ],
  })
  let sent
  const state = useRuntimeConfiguration(['ai'], {
    configuration: async () => view,
    plugins: async () => ({ plugins: [] }),
    saveConfiguration: async (...args) => { sent = args; return { config: view, reload: false } },
  })
  await state.load()
  state.values.value['ai.profiles'][1].clear_key = true
  await state.save()
  const profiles = sent[2]['ai.profiles']
  assert.equal(profiles[0].api_key, '')
  assert.equal(profiles[0].clear_key, false)
  assert.equal(profiles[1].api_key, '')
  assert.equal(profiles[1].clear_key, true)
  assert.equal('key_configured' in profiles[0], false)
})
