import test from 'node:test'
import assert from 'node:assert/strict'
import { useFeed } from '../src/composables/useFeed.ts'
import { useAffinity } from '../src/composables/useAffinity.ts'
const deferred = () => {
  let resolve, reject
  const promise = new Promise((a, b) => {
    resolve = a
    reject = b
  })
  return { promise, resolve, reject }
}

test('feed old group errors cannot replace new settings or clear its loading state', async () => {
  const first = deferred(),
    second = deferred()
  const state = useFeed({
    feedSettings: (id) => (id === 1 ? first.promise : second.promise),
  })
  state.selectedGroupID.value = 1
  const old = state.loadSettings(1)
  state.selectedGroupID.value = 2
  const recent = state.loadSettings(2)
  first.reject(new Error('old failure'))
  await old
  assert.equal(state.error.value, '')
  assert.equal(state.settingsLoading.value, true)
  second.resolve({ settings: { group_id: 2 }, pending_count: 3 })
  await recent
  assert.equal(state.feedSettings.value.group_id, 2)
  assert.equal(state.pendingCount.value, 3)
})

test('feed saving previous group cannot overwrite new group draft', async () => {
  const pending = deferred()
  const state = useFeed({ setFeedSettings: () => pending.promise })
  state.selectedGroupID.value = 1
  const saving = state.saveSettings()
  state.selectedGroupID.value = 2
  state.feedSettings.value.group_id = 2
  pending.resolve({ settings: { group_id: 1 }, pending_count: 99 })
  await saving
  assert.equal(state.feedSettings.value.group_id, 2)
  assert.equal(state.pendingCount.value, 0)
})

test('affinity ignores out-of-order search results', async () => {
  const first = deferred(),
    second = deferred()
  let n = 0
  const state = useAffinity({
    affinity: () => (++n === 1 ? first.promise : second.promise),
  })
  const old = state.loadRows()
  state.q.value = 'new'
  const recent = state.loadRows()
  second.resolve({ affinity: [{ ID: 2 }], block_below: 20 })
  await recent
  first.resolve({ affinity: [{ ID: 1 }], block_below: 10 })
  await old
  assert.equal(state.rows.value[0].ID, 2)
  assert.equal(state.blockBelow.value, 20)
})
