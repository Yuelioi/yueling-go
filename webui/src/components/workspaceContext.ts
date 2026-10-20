import type { InjectionKey } from 'vue'

export const nestedWorkspaceKey: InjectionKey<boolean> =
  Symbol('nested-workspace')
