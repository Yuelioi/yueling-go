import { affinityApi } from '../../api/affinity.ts'
import { useGroupsStore } from '../../stores/groups.ts'
export const affinityClient = {
  ...affinityApi,
  groups: () => useGroupsStore().loadConnected(),
}
export type AffinityClient = typeof affinityClient
