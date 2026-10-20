import { ref } from 'vue'
import type { FeedSubscription, FeedHealth } from '../../api/feeds.ts'
import type { FeedClient } from './client.ts'
import type { FeedContext } from './context.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
import type { useFeedSettings } from './settings.ts'
export function useFeedActions(
  client: Pick<
    FeedClient,
    | 'diagnoseFeeds'
    | 'checkFeeds'
    | 'feeds'
    | 'deleteFeed'
    | 'setFeedEnabled'
    | 'setFeedTranslation'
    | 'feedSettings'
  >,
  context: FeedContext,
  settings: ReturnType<typeof useFeedSettings>,
) {
  const { selectedGroupID, feeds, error, notice } = context
  const requests = useRequestScope()
  const { pendingCount, loadSettings } = settings
  const diagnosing = ref(false)

  const health = ref<Record<number, FeedHealth>>({})

  const checking = ref(false)

  const deleting = ref(false)

  const confirmOpen = ref(false)

  const pendingDelete = ref<FeedSubscription | null>(null)

  const toggling = ref<Record<number, boolean>>({})

  const translationSaving = ref<Record<number, boolean>>({})

  async function diagnose(row?: FeedSubscription) {
    const alive = requests.begin('diagnose')
    diagnosing.value = true
    error.value = ''
    try {
      const res = await client.diagnoseFeeds(
        row?.group_id ?? selectedGroupID.value,
        row?.id ?? 0,
      )
      if (!alive()) return
      for (const result of res.results) health.value[result.id] = result
    } catch (err) {
      if (alive())
        error.value = err instanceof Error ? err.message : '健康检测失败'
    } finally {
      if (alive()) diagnosing.value = false
    }
  }

  async function check() {
    const alive = requests.begin('check')
    const current = () => alive()

    checking.value = true
    error.value = ''
    notice.value = ''
    try {
      const { result } = await client.checkFeeds()
      if (!current()) return

      notice.value = `全局检查 ${result.checked} 个订阅，发现 ${result.items} 条，推送 ${result.delivered} 条，队列剩余 ${result.queued} 条，失败 ${result.failed} 个`
      const feedRes = await client.feeds({ signal: alive.signal })
      if (!current()) return

      feeds.value = feedRes.feeds
      if (selectedGroupID.value) await loadSettings(selectedGroupID.value)
      if (!current()) return
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '检查订阅失败'
    } finally {
      if (!alive()) return

      checking.value = false
    }
  }

  function askDelete(row: FeedSubscription) {
    pendingDelete.value = row
    confirmOpen.value = true
  }

  async function remove() {
    const alive = requests.begin('remove')
    const current = () => alive()

    const row = pendingDelete.value
    if (!row) return
    deleting.value = true
    error.value = ''
    try {
      await client.deleteFeed(row.group_id, row.id)
      if (!current()) return

      feeds.value = feeds.value.filter((item) => item.id !== row.id)
      confirmOpen.value = false
      pendingDelete.value = null
      notice.value = `已删除“${row.name}”`
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '删除订阅失败'
    } finally {
      if (!alive()) return

      deleting.value = false
    }
  }

  async function setEnabled(row: FeedSubscription, enabled: boolean) {
    const alive = requests.begin('setEnabled' + String(row.id))
    const current = () => alive()

    toggling.value[row.id] = true
    error.value = ''
    notice.value = ''
    try {
      const res = await client.setFeedEnabled(row.group_id, row.id, enabled)
      if (!current()) return

      const index = feeds.value.findIndex((item) => item.id === row.id)
      if (index >= 0) feeds.value[index] = res.feed
      if (!enabled) {
        const settings = await client.feedSettings(row.group_id, {
          signal: alive.signal,
        })
        if (!current()) return

        if (selectedGroupID.value === row.group_id)
          pendingCount.value = settings.pending_count
      }
      if (!current()) return

      notice.value = enabled
        ? `已恢复“${row.name}”，下一轮会重新检查`
        : `已暂停“${row.name}”并清理待推送内容`
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '修改订阅状态失败'
    } finally {
      if (!alive()) return

      toggling.value[row.id] = false
    }
  }

  async function setTranslation(
    row: FeedSubscription,
    translateToChinese: boolean,
  ) {
    const alive = requests.begin('setTranslation' + String(row.id))
    const current = () => alive()

    translationSaving.value[row.id] = true
    error.value = ''
    notice.value = ''
    try {
      const res = await client.setFeedTranslation(
        row.group_id,
        row.id,
        translateToChinese,
      )
      if (!current()) return

      const index = feeds.value.findIndex((item) => item.id === row.id)
      if (index >= 0) feeds.value[index] = res.feed
      notice.value = translateToChinese
        ? `已为“${row.name}”启用中文翻译`
        : `“${row.name}”将保留原文`
    } catch (err) {
      if (!current()) return

      error.value =
        err instanceof Error ? err.message : '修改信息源翻译设置失败'
    } finally {
      if (!alive()) return

      translationSaving.value[row.id] = false
    }
  }
  return {
    diagnosing,
    health,
    checking,
    deleting,
    confirmOpen,
    pendingDelete,
    toggling,
    translationSaving,
    diagnose,
    check,
    askDelete,
    remove,
    setEnabled,
    setTranslation,
  }
}
