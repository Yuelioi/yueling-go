import test from 'node:test'
import assert from 'node:assert/strict'
import { useFeed } from '../src/features/feeds/useFeed.ts'
import { useAffinity } from '../src/features/affinity/useAffinity.ts'
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
  state.catalog.selectedGroupID.value = 1
  const old = state.settings.loadSettings(1)
  state.catalog.selectedGroupID.value = 2
  const recent = state.settings.loadSettings(2)
  first.reject(new Error('old failure'))
  await old
  assert.equal(state.catalog.error.value, '')
  assert.equal(state.settings.settingsLoading.value, true)
  second.resolve({ settings: { group_id: 2 }, pending_count: 3 })
  await recent
  assert.equal(state.settings.feedSettings.value.group_id, 2)
  assert.equal(state.settings.pendingCount.value, 3)
})

test('feed saving previous group cannot overwrite new group draft', async () => {
  const pending = deferred()
  const state = useFeed({ setFeedSettings: () => pending.promise })
  state.catalog.selectedGroupID.value = 1
  const saving = state.settings.saveSettings()
  state.catalog.selectedGroupID.value = 2
  state.settings.feedSettings.value.group_id = 2
  pending.resolve({ settings: { group_id: 1 }, pending_count: 99 })
  await saving
  assert.equal(state.settings.feedSettings.value.group_id, 2)
  assert.equal(state.settings.pendingCount.value, 0)
})

test('affinity ignores out-of-order search results', async () => {
  const first = deferred(),
    second = deferred()
  let n = 0
  const state = useAffinity({
    affinity: () => (++n === 1 ? first.promise : second.promise),
  })
  const old = state.records.loadRows()
  state.records.q.value = 'new'
  const recent = state.records.loadRows()
  second.resolve({ affinity: [{ ID: 2 }], block_below: 20 })
  await recent
  first.resolve({ affinity: [{ ID: 1 }], block_below: 10 })
  await old
  assert.equal(state.records.rows.value[0].ID, 2)
  assert.equal(state.records.blockBelow.value, 20)
})

test('feed defaults to all groups, filters and returns to all without reloading', async () => {
  const state = useFeed({
    groups: async () => ({ groups: [{ group_id: 1 }, { group_id: 2 }] }),
    feeds: async () => ({
      feeds: [
        { id: 1, group_id: 1 },
        { id: 2, group_id: 2 },
      ],
    }),
  })
  await state.catalog.load()
  assert.equal(state.catalog.selectedGroupID.value, null)
  assert.equal(state.catalog.groupFeeds.value.length, 2)
  state.catalog.selectedGroupID.value = 2
  assert.deepEqual(
    state.catalog.groupFeeds.value.map((row) => row.id),
    [2],
  )
  await state.catalog.load()
  assert.equal(state.catalog.selectedGroupID.value, 2)
  state.catalog.selectedGroupID.value = null
  assert.equal(state.catalog.groupFeeds.value.length, 2)
})

test('feed diagnosis targets row ownership independently of filter and never polls', async () => {
  const calls = []
  const state = useFeed({
    diagnoseFeeds: async (group, id) => {
      calls.push([group, id])
      return {
        results: [{ id: id || 1, group_id: group || 10, status: 'readable' }],
      }
    },
    checkFeeds: () => {
      throw new Error('must not poll')
    },
  })
  await state.actions.diagnose()
  state.catalog.selectedGroupID.value = 20
  await state.actions.diagnose({ id: 7, group_id: 10 })
  assert.deepEqual(calls, [
    [null, 0],
    [10, 7],
  ])
  assert.equal(state.actions.health.value[7].status, 'readable')
})
