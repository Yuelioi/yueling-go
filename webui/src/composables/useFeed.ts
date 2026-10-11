import { computed, ref } from 'vue'
import {
  api,
  type FeedHealth,
  type FeedSettings,
  type FeedSubscription,
  type GroupInfo,
} from '../api.ts'
import { useRequestScope } from './useRequestScope.ts'

export function useFeed(
  client: Pick<
    typeof api,
    | 'diagnoseFeeds'
    | 'addFeed'
    | 'addPlatformFeed'
    | 'checkFeeds'
    | 'deleteFeed'
    | 'feedSettings'
    | 'feeds'
    | 'groups'
    | 'setFeedEnabled'
    | 'setFeedSettings'
    | 'setFeedTranslation'
  > = { ...api, groups: api.managedGroups },
) {
  const requests = useRequestScope()
  const diagnosing = ref(false)
  const health = ref<Record<number, FeedHealth>>({})
  const groups = ref<GroupInfo[]>([])
  const feeds = ref<FeedSubscription[]>([])
  const selectedGroupID = ref<number | null>(null)
  const feedURL = ref('')
  const feedName = ref('')
  const platformName = ref('')
  const platform = ref('bilibili_video')
  const platformTarget = ref('')
  const loading = ref(false)
  const adding = ref(false)
  const platformAdding = ref(false)
  const checking = ref(false)
  const deleting = ref(false)
  const confirmOpen = ref(false)
  const pendingDelete = ref<FeedSubscription | null>(null)
  const error = ref('')
  const notice = ref('')
  const settingsLoading = ref(false)
  const settingsSaving = ref(false)
  const toggling = ref<Record<number, boolean>>({})
  const translationSaving = ref<Record<number, boolean>>({})
  const pendingCount = ref(0)
  const strategyEditorOpen = ref(false)
  const platformEditorOpen = ref(false)
  const rawEditorOpen = ref(false)
  const feedSettings = ref<FeedSettings>({
    poll_interval_minutes: 5,
    group_id: 0,
    quiet_enabled: false,
    quiet_start: '23:00',
    quiet_end: '08:00',
    item_max_chars: 0,
    updated_at: 0,
  })

  const selectedGroup = computed(() =>
    groups.value.find((group) => group.group_id === selectedGroupID.value),
  )
  const feedSwitchClass =
    '[&_[role=switch][data-state=unchecked]]:!bg-[rgba(147,141,168,0.18)] [&_[role=switch][data-state=unchecked]_[data-slot=thumb]]:!bg-[#777185] [&_[role=switch][data-state=unchecked]_[data-slot=thumb]]:!shadow-[inset_0_0_0_1px_rgba(212,208,228,0.08),0_1px_3px_rgba(0,0,0,0.3)]'
  const groupFeeds = computed(() =>
    feeds.value.filter((row) => selectedGroupID.value === null || row.group_id === selectedGroupID.value),
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
  const platformOptions = [
    { label: 'B站 · UP主投稿', value: 'bilibili_video' },
    { label: 'B站 · UP主动态', value: 'bilibili_dynamic' },
    { label: 'B站 · 直播开播', value: 'bilibili_live' },
    { label: 'X · 用户发推', value: 'x_user' },
    { label: '小红书 · 用户笔记', value: 'xiaohongshu_user' },
    { label: 'GitHub · 版本发布', value: 'github_releases' },
    { label: 'GitHub · Issues', value: 'github_issues' },
  ]
  const itemLengthModeOptions = [
    { label: '完整内容', value: 'full' },
    { label: '限制字符数', value: 'limited' },
  ]
  const itemLengthMode = computed({
    get: () => (feedSettings.value.item_max_chars === 0 ? 'full' : 'limited'),
    set: (value: string) => {
      feedSettings.value.item_max_chars =
        value === 'full' ? 0 : feedSettings.value.item_max_chars || 320
    },
  })
  const customItemMaxChars = computed<number | null>({
    get: () => feedSettings.value.item_max_chars || 320,
    set: (value) => {
      feedSettings.value.item_max_chars = value ?? 320
    },
  })
  const itemLengthError = computed(() => {
    const value = feedSettings.value.item_max_chars
    if (value === 0 || (value >= 80 && value <= 4000)) return ''
    return '字符数需在 80–4000 之间'
  })
  const platformPlaceholder = computed(() => {
    if (platform.value === 'bilibili_live')
      return '直播间号或 live.bilibili.com 链接'
    if (platform.value === 'xiaohongshu_user')
      return '24 位用户 ID 或小红书完整主页链接'
    if (platform.value.startsWith('github_'))
      return 'owner/repo 或 GitHub 仓库链接'
    if (platform.value === 'x_user') return '@username 或 X 主页链接'
    return 'UP 主 UID 或 space.bilibili.com 链接'
  })

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

  function itemLengthLabel(value: number) {
    return value === 0 ? '完整内容' : `最多 ${value} 字`
  }

  async function load() {
    const alive = requests.begin('load')
    const current = () => alive()

    loading.value = true
    error.value = ''
    try {
      const [groupRes, feedRes] = await Promise.all([
        client.groups(),
        client.feeds(),
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

  async function loadSettings(groupID: number) {
    const alive = requests.begin('loadSettings')
    const scope = selectedGroupID.value
    const current = () => alive() && scope === selectedGroupID.value

    settingsLoading.value = true
    try {
      const res = await client.feedSettings(groupID)
      if (!current()) return

      if (selectedGroupID.value !== groupID) return
      feedSettings.value = res.settings
      pendingCount.value = res.pending_count
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '推送策略加载失败'
    } finally {
      if (!alive()) return

      settingsLoading.value = false
    }
  }

  async function saveSettings() {
    const alive = requests.begin('saveSettings')
    const scope = selectedGroupID.value
    const current = () => alive() && scope === selectedGroupID.value

    if (!selectedGroupID.value) return
    settingsSaving.value = true
    error.value = ''
    notice.value = ''
    try {
      const res = await client.setFeedSettings(selectedGroupID.value, {
        poll_interval_minutes: feedSettings.value.poll_interval_minutes,
        quiet_enabled: feedSettings.value.quiet_enabled,
        quiet_start: feedSettings.value.quiet_start,
        quiet_end: feedSettings.value.quiet_end,
        item_max_chars: feedSettings.value.item_max_chars,
      })
      if (!current()) return

      feedSettings.value = res.settings
      pendingCount.value = res.pending_count
      const quietSummary = res.settings.quiet_enabled
        ? `静默 ${res.settings.quiet_start}–${res.settings.quiet_end}`
        : '不启用静默'
      notice.value = `推送策略已保存 · ${itemLengthLabel(res.settings.item_max_chars)} · ${quietSummary}`
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '推送策略保存失败'
    } finally {
      if (!alive()) return

      settingsSaving.value = false
    }
  }

  async function add() {
    const alive = requests.begin('add')
    const scope = selectedGroupID.value
    const current = () => alive() && scope === selectedGroupID.value

    if (!selectedGroupID.value) {
      error.value = '请选择群聊'
      return
    }
    try {
      const parsed = new URL(feedURL.value.trim())
      if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error()
    } catch {
      if (!current()) return

      error.value = '请输入有效的 HTTP/HTTPS 订阅地址'
      return
    }
    adding.value = true
    error.value = ''
    notice.value = ''
    try {
      const res = await client.addFeed(
        selectedGroupID.value,
        feedURL.value.trim(),
        feedName.value.trim(),
      )
      if (!current()) return

      feeds.value.push(res.feed)
      feedURL.value = ''
      feedName.value = ''
      notice.value = res.latest_title
        ? `已订阅，从“${res.latest_title}”之后开始推送`
        : '订阅已添加，等待新内容'
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '添加订阅失败'
    } finally {
      if (!alive()) return

      adding.value = false
    }
  }

  async function addPlatform() {
    const alive = requests.begin('addPlatform')
    const scope = selectedGroupID.value
    const current = () => alive() && scope === selectedGroupID.value

    if (!selectedGroupID.value) {
      error.value = '请选择群聊'
      return
    }
    if (!platformTarget.value.trim()) {
      error.value = '请输入 UID、用户名或主页/直播间链接'
      return
    }
    platformAdding.value = true
    error.value = ''
    notice.value = ''
    try {
      const res = await client.addPlatformFeed(
        selectedGroupID.value,
        platform.value,
        platformTarget.value.trim(),
        platformName.value.trim(),
      )
      if (!current()) return

      feeds.value.push(res.feed)
      platformTarget.value = ''
      platformName.value = ''
      notice.value = res.latest_title
        ? `平台订阅已添加，从“${res.latest_title}”之后开始推送`
        : '平台订阅已添加，等待新内容'
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '添加平台订阅失败'
    } finally {
      if (!alive()) return

      platformAdding.value = false
    }
  }

  async function diagnose(row?: FeedSubscription) {
    const alive = requests.begin('diagnose')
    diagnosing.value = true
    error.value = ''
    try {
      const res = await client.diagnoseFeeds(row?.group_id ?? selectedGroupID.value, row?.id ?? 0)
      if (!alive()) return
      for (const result of res.results) health.value[result.id] = result
    } catch (err) {
      if (alive()) error.value = err instanceof Error ? err.message : '健康检测失败'
    } finally { if (alive()) diagnosing.value = false }
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
      const feedRes = await client.feeds()
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
        const settings = await client.feedSettings(row.group_id)
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
    diagnose,
    diagnosing,
    health,
    activeFeeds,
    add,
    addPlatform,
    adding,
    askDelete,
    check,
    checking,
    confirmOpen,
    coveredGroups,
    customItemMaxChars,
    deleting,
    error,
    failingFeeds,
    feedName,
    feedSettings,
    feedSwitchClass,
    feedURL,
    feeds,
    formatDate,
    groupFeeds,
    groups,
    itemLengthError,
    itemLengthLabel,
    itemLengthMode,
    itemLengthModeOptions,
    load,
    loadSettings,
    loading,
    notice,
    pendingCount,
    pendingDelete,
    platform,
    platformAdding,
    platformEditorOpen,
    platformName,
    platformOptions,
    platformPlaceholder,
    platformTarget,
    rawEditorOpen,
    remove,
    saveSettings,
    selectedGroup,
    selectedGroupID,
    setEnabled,
    setTranslation,
    settingsLoading,
    settingsSaving,
    sourceHost,
    strategyEditorOpen,
    toggling,
    translationSaving,
  }
}
