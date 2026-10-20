import { computed, ref } from 'vue'
import type { FeedSettings } from '../../api/feeds.ts'
import type { FeedClient } from './client.ts'
import type { FeedContext } from './context.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
export function useFeedSettings(
  client: Pick<FeedClient, 'feedSettings' | 'setFeedSettings'>,
  context: Pick<FeedContext, 'selectedGroupID' | 'error' | 'notice'>,
) {
  const { selectedGroupID, error, notice } = context
  const requests = useRequestScope()
  const settingsLoading = ref(false)

  const settingsSaving = ref(false)

  const pendingCount = ref(0)

  const strategyEditorOpen = ref(false)

  const feedSettings = ref<FeedSettings>({
    poll_interval_minutes: 5,
    group_id: 0,
    quiet_enabled: false,
    quiet_start: '23:00',
    quiet_end: '08:00',
    item_max_chars: 0,
    updated_at: 0,
  })

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

  function itemLengthLabel(value: number) {
    return value === 0 ? '完整内容' : `最多 ${value} 字`
  }

  async function loadSettings(groupID: number) {
    const alive = requests.begin('loadSettings')
    const scope = selectedGroupID.value
    const current = () => alive() && scope === selectedGroupID.value

    settingsLoading.value = true
    try {
      const res = await client.feedSettings(groupID, { signal: alive.signal })
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
  return {
    settingsLoading,
    settingsSaving,
    pendingCount,
    strategyEditorOpen,
    feedSettings,
    itemLengthModeOptions,
    itemLengthMode,
    customItemMaxChars,
    itemLengthError,
    itemLengthLabel,
    loadSettings,
    saveSettings,
  }
}
