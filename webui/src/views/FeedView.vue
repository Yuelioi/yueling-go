<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type FeedSettings, type FeedSubscription, type GroupInfo } from '../api'
import GroupPicker from '../components/GroupPicker.vue'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

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
const pendingCount = ref(0)
const feedSettings = ref<FeedSettings>({
  group_id: 0,
  quiet_enabled: false,
  quiet_start: '23:00',
  quiet_end: '08:00',
  updated_at: 0,
})

const selectedGroup = computed(() => groups.value.find((group) => group.group_id === selectedGroupID.value))
const groupFeeds = computed(() => feeds.value.filter((row) => row.group_id === selectedGroupID.value))
const groupActiveFeeds = computed(() => groupFeeds.value.filter((row) => row.enabled))
const coveredGroups = computed(() => new Set(feeds.value.map((row) => row.group_id)).size)
const activeFeeds = computed(() => feeds.value.filter((row) => row.enabled).length)
const failingFeeds = computed(() => feeds.value.filter((row) => row.enabled && row.consecutive_failures > 0).length)
const platformOptions = [
  { label: 'B站 · UP主投稿', value: 'bilibili_video' },
  { label: 'B站 · UP主动态', value: 'bilibili_dynamic' },
  { label: 'B站 · 直播开播', value: 'bilibili_live' },
  { label: 'X · 用户发推', value: 'x_user' },
]
const platformPlaceholder = computed(() => {
  if (platform.value === 'bilibili_live') return '直播间号或 live.bilibili.com 链接'
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
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    .format(new Date(timestamp * 1000))
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [groupRes, feedRes] = await Promise.all([api.groups(), api.feeds()])
    groups.value = groupRes.groups
    feeds.value = feedRes.feeds
    if (!selectedGroupID.value) {
      selectedGroupID.value = feeds.value[0]?.group_id || groups.value[0]?.group_id || null
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '订阅数据加载失败'
  } finally {
    loading.value = false
  }
}

async function loadSettings(groupID: number) {
  settingsLoading.value = true
  try {
    const res = await api.feedSettings(groupID)
    if (selectedGroupID.value !== groupID) return
    feedSettings.value = res.settings
    pendingCount.value = res.pending_count
  } catch (err) {
    error.value = err instanceof Error ? err.message : '推送策略加载失败'
  } finally {
    settingsLoading.value = false
  }
}

async function saveSettings() {
  if (!selectedGroupID.value) return
  settingsSaving.value = true
  error.value = ''
  notice.value = ''
  try {
    const res = await api.setFeedSettings(
      selectedGroupID.value,
      feedSettings.value.quiet_enabled,
      feedSettings.value.quiet_start,
      feedSettings.value.quiet_end,
    )
    feedSettings.value = res.settings
    pendingCount.value = res.pending_count
    notice.value = res.settings.quiet_enabled
      ? `静默时段已设置为 ${res.settings.quiet_start}–${res.settings.quiet_end}`
      : '静默时段已关闭'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '推送策略保存失败'
  } finally {
    settingsSaving.value = false
  }
}

async function add() {
  if (!selectedGroupID.value) {
    error.value = '请选择群聊'
    return
  }
  try {
    const parsed = new URL(feedURL.value.trim())
    if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error()
  } catch {
    error.value = '请输入有效的 HTTP/HTTPS 订阅地址'
    return
  }
  adding.value = true
  error.value = ''
  notice.value = ''
  try {
    const res = await api.addFeed(selectedGroupID.value, feedURL.value.trim(), feedName.value.trim())
    feeds.value.push(res.feed)
    feedURL.value = ''
    feedName.value = ''
    notice.value = res.latest_title ? `已订阅，从“${res.latest_title}”之后开始推送` : '订阅已添加，等待新内容'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '添加订阅失败'
  } finally {
    adding.value = false
  }
}

async function addPlatform() {
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
    const res = await api.addPlatformFeed(selectedGroupID.value, platform.value, platformTarget.value.trim(), platformName.value.trim())
    feeds.value.push(res.feed)
    platformTarget.value = ''
    platformName.value = ''
    notice.value = res.latest_title ? `平台订阅已添加，从“${res.latest_title}”之后开始推送` : '平台订阅已添加，等待新内容'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '添加平台订阅失败'
  } finally {
    platformAdding.value = false
  }
}

async function check() {
  if (!selectedGroupID.value) return
  checking.value = true
  error.value = ''
  notice.value = ''
  try {
    const { result } = await api.checkFeeds(selectedGroupID.value)
    pendingCount.value = result.queued
    notice.value = `检查 ${result.checked} 个订阅，发现 ${result.items} 条，推送 ${result.delivered} 条，队列剩余 ${result.queued} 条，失败 ${result.failed} 个`
    const feedRes = await api.feeds()
    feeds.value = feedRes.feeds
  } catch (err) {
    error.value = err instanceof Error ? err.message : '检查订阅失败'
  } finally {
    checking.value = false
  }
}

function askDelete(row: FeedSubscription) {
  pendingDelete.value = row
  confirmOpen.value = true
}

async function remove() {
  const row = pendingDelete.value
  if (!row) return
  deleting.value = true
  error.value = ''
  try {
    await api.deleteFeed(row.group_id, row.id)
    feeds.value = feeds.value.filter((item) => item.id !== row.id)
    confirmOpen.value = false
    pendingDelete.value = null
    notice.value = `已删除“${row.name}”`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除订阅失败'
  } finally {
    deleting.value = false
  }
}

async function setEnabled(row: FeedSubscription, enabled: boolean) {
  toggling.value[row.id] = true
  error.value = ''
  notice.value = ''
  try {
    const res = await api.setFeedEnabled(row.group_id, row.id, enabled)
    const index = feeds.value.findIndex((item) => item.id === row.id)
    if (index >= 0) feeds.value[index] = res.feed
    if (!enabled) {
      const settings = await api.feedSettings(row.group_id)
      if (selectedGroupID.value === row.group_id) pendingCount.value = settings.pending_count
    }
    notice.value = enabled ? `已恢复“${row.name}”，下一轮会重新检查` : `已暂停“${row.name}”并清理待推送内容`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '修改订阅状态失败'
  } finally {
    toggling.value[row.id] = false
  }
}

watch(selectedGroupID, (groupID) => {
  notice.value = ''
  if (groupID) loadSettings(groupID)
})
onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Signal subscriptions"
      title="订阅中心"
      description="把站点、博客、GitHub Releases 与 RSSHub 信息源聚合到群聊。"
      icon="i-tabler-rss"
    >
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">刷新数据</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="活跃订阅" :value="activeFeeds" :detail="`${feeds.length - activeFeeds} 个已暂停`" icon="i-tabler-rss" tone="violet" />
      <MetricCard label="覆盖群聊" :value="coveredGroups" :detail="`${groups.length} 个可用群聊`" icon="i-tabler-users-group" tone="cyan" />
      <MetricCard label="异常源" :value="failingFeeds" :detail="`${pendingCount} 条等待推送`" icon="i-tabler-heart-rate-monitor" tone="amber" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />
    <UAlert v-if="notice" color="success" variant="subtle" icon="i-tabler-circle-check" :description="notice" />

    <div class="grid gap-4 lg:grid-cols-[292px_minmax(0,1fr)]">
      <GroupPicker v-model="selectedGroupID" :groups="groups" title="订阅群聊" description="选择要接收更新的群" />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div><div class="section-title">可靠推送策略</div><div class="section-caption">静默期间继续抓取，结束后按群合并推送</div></div>
            <UBadge :color="pendingCount ? 'warning' : 'success'" variant="subtle">{{ pendingCount }} 条待推送</UBadge>
          </div>
          <div class="grid items-end gap-4 p-4 md:grid-cols-[minmax(180px,0.6fr)_150px_150px_auto]">
            <div class="surface-inset flex min-h-16 items-center justify-between gap-3 px-4 py-3">
              <div><div class="text-sm font-medium text-white">夜间静默</div><div class="mt-1 text-xs text-zinc-500">只延迟群消息，不停止检查</div></div>
              <USwitch v-model="feedSettings.quiet_enabled" color="primary" :disabled="!selectedGroupID || settingsLoading" />
            </div>
            <UFormField label="开始时间">
              <UInput v-model="feedSettings.quiet_start" type="time" icon="i-tabler-moon" :disabled="!selectedGroupID || !feedSettings.quiet_enabled" />
            </UFormField>
            <UFormField label="结束时间">
              <UInput v-model="feedSettings.quiet_end" type="time" icon="i-tabler-sun" :disabled="!selectedGroupID || !feedSettings.quiet_enabled" />
            </UFormField>
            <UButton icon="i-tabler-device-floppy" :loading="settingsSaving" :disabled="!selectedGroupID || settingsLoading" @click="saveSettings">保存策略</UButton>
          </div>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div><div class="section-title">平台快捷订阅</div><div class="section-caption">输入主页、UID 或用户名，不需要手写 RSSHub 路由</div></div>
            <UBadge color="primary" variant="subtle">按群生效</UBadge>
          </div>
          <form class="space-y-4 p-4" @submit.prevent="addPlatform">
            <div class="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)_minmax(160px,0.55fr)]">
              <UFormField label="订阅类型" description="更新只推送到当前群">
                <USelect v-model="platform" class="w-full" :items="platformOptions" value-key="value" :disabled="!selectedGroupID" />
              </UFormField>
              <UFormField label="账号或直播间" description="支持直接粘贴平台主页链接">
                <UInput v-model="platformTarget" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-brand-bilibili" :placeholder="platformPlaceholder" :disabled="!selectedGroupID" />
              </UFormField>
              <UFormField label="显示名称" description="可选">
                <UInput v-model="platformName" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-tag" placeholder="关注对象" :disabled="!selectedGroupID" />
              </UFormField>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-zinc-500">X 路由受平台限制，稳定运行建议配置自己的 RSSHub 实例。</p>
              <UButton type="submit" icon="i-tabler-bell-plus" :loading="platformAdding" :disabled="!selectedGroupID || groupFeeds.length >= 10">添加平台订阅</UButton>
            </div>
          </form>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择群' }}</div>
              <div class="text-xs text-zinc-500">{{ selectedGroupID ? `群号 ${selectedGroupID}` : '选择群聊后可添加订阅' }}</div>
            </div>
            <UBadge color="primary" variant="subtle">{{ groupFeeds.length }} / 10</UBadge>
          </div>

          <form class="space-y-4 p-4" @submit.prevent="add">
            <div class="grid gap-3 md:grid-cols-[minmax(0,1.6fr)_minmax(180px,0.7fr)]">
              <UFormField label="RSS / Atom 地址" description="仅允许公网 HTTP/HTTPS 地址">
                <UInput v-model="feedURL" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-link" placeholder="https://example.com/feed.xml" :disabled="!selectedGroupID" />
              </UFormField>
              <UFormField label="显示名称" description="留空时读取信息源标题">
                <UInput v-model="feedName" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-tag" placeholder="项目动态" :disabled="!selectedGroupID" />
              </UFormField>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-zinc-500">添加时以当前最新内容为基线，不会把历史文章一次性刷进群。</p>
              <UButton type="submit" icon="i-tabler-plus" :loading="adding" :disabled="!selectedGroupID || groupFeeds.length >= 10">添加订阅</UButton>
            </div>
          </form>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div><div class="section-title">当前信息源</div><div class="section-caption">支持 RSS 2.0、Atom 与 RSS 1.0</div></div>
            <UButton size="sm" color="neutral" variant="soft" icon="i-tabler-refresh-dot" :loading="checking" :disabled="!selectedGroupID || groupActiveFeeds.length === 0" @click="check">立即检查</UButton>
          </div>

          <div v-if="groupFeeds.length">
            <div v-for="row in groupFeeds" :key="row.id" class="data-row flex items-center gap-3 p-4">
              <span class="activity-icon shrink-0"><UIcon name="i-tabler-rss" class="size-4" /></span>
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium text-white">{{ row.name }}</div>
                <a :href="row.url" target="_blank" rel="noreferrer" class="mt-1 block truncate text-xs text-violet-300 hover:text-violet-200">{{ sourceHost(row.url) }}</a>
                <div class="mt-1 text-[11px] text-zinc-600">
                  {{ row.last_checked_at ? `上次检查 ${formatDate(row.last_checked_at)}` : `添加于 ${formatDate(row.created_at)}` }}
                  <span v-if="row.next_check_at"> · 下次 {{ formatDate(row.next_check_at) }}</span>
                </div>
                <div v-if="row.enabled && row.last_error" class="mt-1 truncate text-[11px] text-rose-400" :title="row.last_error">{{ row.last_error }}</div>
              </div>
              <UBadge :color="!row.enabled ? 'neutral' : row.consecutive_failures ? 'error' : 'success'" variant="subtle">
                {{ !row.enabled ? '已暂停' : row.consecutive_failures ? `异常 ${row.consecutive_failures} 次` : '运行正常' }}
              </UBadge>
              <USwitch
                :model-value="row.enabled"
                color="primary"
                :disabled="toggling[row.id]"
                :aria-label="row.enabled ? '暂停订阅' : '恢复订阅'"
                @update:model-value="setEnabled(row, Boolean($event))"
              />
              <UButton color="error" variant="ghost" icon="i-tabler-trash" aria-label="删除订阅" @click="askDelete(row)" />
            </div>
          </div>
          <div v-else class="empty-state overview-empty">
            <span class="empty-icon"><UIcon name="i-tabler-rss-off" class="size-6" /></span>
            <div class="empty-title">这个群还没有订阅</div>
            <div class="empty-description">添加信息源后，新内容会自动推送到群聊</div>
          </div>
        </section>
      </div>
    </div>

    <UModal v-model:open="confirmOpen" title="删除这个订阅？" description="删除后将停止检查和推送，操作无法撤销。" :ui="{ overlay: 'z-40 bg-black/70 backdrop-blur-sm', content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl', header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900', title: 'text-white', description: 'text-zinc-400' }">
      <template #body><div class="surface-inset p-4 text-sm text-zinc-300">{{ pendingDelete?.name }}<div class="mt-1 truncate text-xs text-zinc-500">{{ pendingDelete?.url }}</div></div></template>
      <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="ghost" @click="confirmOpen = false">取消</UButton><UButton color="error" icon="i-tabler-trash" :loading="deleting" @click="remove">确认删除</UButton></div></template>
    </UModal>
  </section>
</template>
