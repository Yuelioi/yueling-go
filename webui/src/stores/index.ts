import { createPinia } from 'pinia'
import { useConfigurationStore } from './configuration.ts'
import { useGroupsStore } from './groups.ts'

export const pinia = createPinia()
export function resetPrivateData() {
  useConfigurationStore(pinia).reset()
  useGroupsStore(pinia).reset()
}
