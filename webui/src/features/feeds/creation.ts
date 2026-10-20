import { computed, ref } from 'vue'
import type { FeedClient } from './client.ts'
import type { FeedContext } from './context.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
export function useFeedCreation(
  client: Pick<FeedClient, 'addFeed' | 'addPlatformFeed'>,
  context: FeedContext,
) {
  const { selectedGroupID, feeds, error, notice } = context
  const requests = useRequestScope()
  const feedURL = ref('')

  const feedName = ref('')

  const platformName = ref('')

  const platform = ref('bilibili_video')

  const platformTarget = ref('')

  const adding = ref(false)

  const platformAdding = ref(false)

  const platformEditorOpen = ref(false)

  const rawEditorOpen = ref(false)

  const platformOptions = [
    { label: 'B站 · UP主投稿', value: 'bilibili_video' },
    { label: 'B站 · UP主动态', value: 'bilibili_dynamic' },
    { label: 'B站 · 直播开播', value: 'bilibili_live' },
    { label: 'X · 用户发推', value: 'x_user' },
    { label: '小红书 · 用户笔记', value: 'xiaohongshu_user' },
    { label: 'GitHub · 版本发布', value: 'github_releases' },
    { label: 'GitHub · Issues', value: 'github_issues' },
  ]

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
  return {
    feedURL,
    feedName,
    platformName,
    platform,
    platformTarget,
    adding,
    platformAdding,
    platformEditorOpen,
    rawEditorOpen,
    platformOptions,
    platformPlaceholder,
    add,
    addPlatform,
  }
}
