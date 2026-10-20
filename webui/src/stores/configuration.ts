import { defineStore } from 'pinia'
import { shallowRef } from 'vue'
import {
  configurationApi,
  type RuntimeConfiguration,
} from '../api/configuration.ts'

export const useConfigurationStore = defineStore('configuration', () => {
  const snapshot = shallowRef<RuntimeConfiguration | null>(null)
  let pending: Promise<RuntimeConfiguration> | undefined
  let generation = 0
  let fetchedAt = 0
  function reset() {
    generation++
    pending = undefined
    snapshot.value = null
    fetchedAt = 0
  }
  async function load(force = false) {
    if (!force && snapshot.value && Date.now() - fetchedAt < 15_000)
      return snapshot.value
    if (pending) return pending
    const current = generation
    const task = configurationApi
      .configuration()
      .then((data) => {
        if (current !== generation)
          throw new DOMException('会话已结束', 'AbortError')
        if (
          current === generation &&
          (!snapshot.value || data.revision >= snapshot.value.revision)
        ) {
          snapshot.value = data
          fetchedAt = Date.now()
        }
        return snapshot.value || data
      })
      .finally(() => {
        if (current === generation) pending = undefined
      })
    pending = task
    return task
  }
  async function save(
    ...args: Parameters<typeof configurationApi.saveConfiguration>
  ) {
    const current = generation
    const result = await configurationApi.saveConfiguration(...args)
    if (current !== generation)
      throw new DOMException('会话已结束', 'AbortError')
    if (
      current === generation &&
      (!snapshot.value || result.config.revision >= snapshot.value.revision)
    ) {
      snapshot.value = result.config
      fetchedAt = Date.now()
    }
    return result
  }
  return { snapshot, load, save, reset }
})
