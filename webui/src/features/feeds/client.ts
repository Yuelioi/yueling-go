import { feedsApi } from '../../api/feeds.ts'
import { useGroupsStore } from '../../stores/groups.ts'
export const feedClient = {
  ...feedsApi,
  groups: () => useGroupsStore().loadManaged(),
}
export type FeedClient = typeof feedClient
