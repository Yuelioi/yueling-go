import { computed, ref } from 'vue'
import type { GroupInfo } from '../../api/groups.ts'
import type { FeedSubscription } from '../../api/feeds.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
import { feedClient, type FeedClient } from './client.ts'
import { useFeedSettings } from './settings.ts'
import { useFeedCreation } from './creation.ts'
import { useFeedActions } from './actions.ts'
export function useFeed(client: FeedClient = feedClient) {
  const requests = useRequestScope()
  const groups = ref<GroupInfo[]>([])

  const feeds = ref<FeedSubscription[]>([])

  const selectedGroupID = ref<number | null>(null)

  const loading = ref(false)

  const error = ref('')

  const notice = ref('')

  const selectedGroup = computed(() =>
    groups.value.find((group) => group.group_id === selectedGroupID.value),
  )

  const groupFeeds = computed(() =>
    feeds.value.filter(
      (row) =>
        selectedGroupID.value === null ||
        row.group_id === selectedGroupID.value,
    ),
  )

  const coveredGroups = computed(
    () => new Set(feeds.value.map((row) => row.group_id)).size,
  )

  const activeFeeds = computed(
    () => feeds.value.filter((row) => row.enabled).length,
  )

  const failingFeeds = computed(
    () =>
      feeds.value.filter((row) => row.enabled && row.consecutive_failures > 0)
        .length,
  )

  function sourceHost(url: string) {
    try {
      return new URL(url).hostname
    } catch {
      return url
    }
  }

  function formatDate(timestamp: number) {
    if (!timestamp) return '-'
    return new Intl.DateTimeFormat('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(timestamp * 1000))
  }

  async function load() {
    const alive = requests.begin('load')
    const current = () => alive()

    loading.value = true
    error.value = ''
    try {
      const [groupRes, feedRes] = await Promise.all([
        client.groups(),
        client.feeds({ signal: alive.signal }),
      ])
      if (!current()) return

      groups.value = groupRes.groups
      feeds.value = feedRes.feeds
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '订阅数据加载失败'
    } finally {
      if (!alive()) return

      loading.value = false
    }
  }
  const context = { selectedGroupID, feeds, error, notice }
  const settings = useFeedSettings(client, context)
  const creation = useFeedCreation(client, context)
  const actions = useFeedActions(client, context, settings)
  return {
    catalog: {
      groups,
      feeds,
      selectedGroupID,
      loading,
      error,
      notice,
      selectedGroup,
      groupFeeds,
      coveredGroups,
      activeFeeds,
      failingFeeds,
      sourceHost,
      formatDate,
      load,
    },
    settings,
    creation,
    actions,
  }
}
