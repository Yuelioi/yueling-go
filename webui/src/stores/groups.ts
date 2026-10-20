import { defineStore } from 'pinia'
import { shallowRef } from 'vue'
import {
  groupsApi,
  type GroupInfo,
  type GroupAdministration,
} from '../api/groups.ts'

export const useGroupsStore = defineStore('groups', () => {
  const connected = shallowRef<GroupInfo[]>([])
  const managed = shallowRef<GroupAdministration[]>([])
  const warning = shallowRef('')
  const pending = new Map<string, Promise<unknown>>()
  const fetchedAt = new Map<string, number>()
  let generation = 0
  function reset() {
    generation++
    pending.clear()
    fetchedAt.clear()
    connected.value = []
    managed.value = []
    warning.value = ''
  }
  async function read<T>(
    key: string,
    fetcher: () => Promise<T>,
    apply: (data: T) => void,
    snapshot: () => T,
    force: boolean,
  ) {
    if (
      !force &&
      fetchedAt.has(key) &&
      Date.now() - fetchedAt.get(key)! < 30_000
    )
      return snapshot()
    if (pending.has(key)) return pending.get(key) as Promise<T>
    const current = generation
    const task = fetcher()
      .then((data) => {
        if (current !== generation)
          throw new DOMException('会话已结束', 'AbortError')
        apply(data)
        fetchedAt.set(key, Date.now())
        return data
      })
      .finally(() => {
        if (current === generation) pending.delete(key)
      })
    pending.set(key, task)
    return task
  }
  function loadConnected(force = false) {
    return read(
      'connected',
      groupsApi.groups,
      (data) => {
        connected.value = data.groups
      },
      () => ({ ok: true, groups: connected.value }),
      force,
    )
  }
  function loadAdministration(force = false) {
    return read(
      'managed',
      groupsApi.groupAdministration,
      (data) => {
        managed.value = data.groups
        warning.value = data.warning
      },
      () => ({ ok: true, groups: managed.value, warning: warning.value }),
      force,
    )
  }
  async function loadManaged(force = false) {
    const data = await loadAdministration(force)
    return {
      ok: true as const,
      groups: data.groups.filter((group) => group.group_id > 0),
    }
  }
  return {
    connected,
    managed,
    warning,
    loadConnected,
    loadManaged,
    loadAdministration,
    reset,
  }
})
