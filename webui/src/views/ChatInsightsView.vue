<script setup lang="ts">
import PeriodSelect from '../components/PeriodSelect.vue'
import DataTable from '../components/DataTable.vue'
import EmptyState from '../components/EmptyState.vue'
import { useRequestScope } from '../composables/useRequestScope'
import { useGroupsStore } from '../stores/groups.ts'
import { computed, onMounted, ref, watch } from 'vue'
import {
  analyticsApi,
  type ChatInsightUser,
  type ChatInsightsResponse,
  type GroupInfo,
} from '../api'
import ChatHistoryManager from '../components/ChatHistoryManager.vue'
import ChatWordCloud from '../components/ChatWordCloud.vue'
import { useGroupFilter } from '../composables/useGroupFilter'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

type InsightPeriod = 'today' | 'yesterday' | '7days' | '30days'

const groups = ref<GroupInfo[]>([])
const selectedGroupID = useGroupFilter()
const overview = ref<
  Awaited<ReturnType<typeof analyticsApi.chatOverview>>['groups']
>([])
const overviewRows = computed(() => {
  const byGroup = new Map(overview.value.map((row) => [row.group_id, row]))
  for (const group of groups.value) {
    if (!byGroup.has(group.group_id))
      byGroup.set(group.group_id, {
        group_id: group.group_id,
        total: 0,
        text_total: 0,
        participants: 0,
      })
  }
  return [...byGroup.values()].sort(
    (a, b) => b.total - a.total || a.group_id - b.group_id,
  )
})
const period = ref<InsightPeriod>('today')
const insights = ref<ChatInsightsResponse | null>(null)
const loading = ref(false)
const groupsLoading = ref(false)
const error = ref('')
const notice = ref('')
const historyOpen = ref(false)
let initialized = false
const requests = useRequestScope()

const periodOptions: Array<{ value: InsightPeriod; label: string }> = [
  { value: 'today', label: '今日' },
  { value: 'yesterday', label: '昨日' },
  { value: '7days', label: '近 7 天' },
  { value: '30days', label: '近 30 天' },
]

const personAvatarClass =
  'grid size-[34px] shrink-0 place-items-center rounded-[10px] border border-[var(--border)] bg-[var(--violet-soft)] text-xs font-semibold text-[var(--violet-bright)]'

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)
const textRatio = computed(() => {
  const summary = insights.value?.summary
  if (!summary?.total) return '0%'
  return `${Math.round((summary.text_total / summary.total) * 100)}%`
})
const maxUserCount = computed(() =>
  Math.max(1, ...(insights.value?.users.map((user) => user.count) || [1])),
)

function displayName(user: ChatInsightUser) {
  return user.nickname?.trim() || `群友 ${user.user_id}`
}

function displayPhrases(user: ChatInsightUser) {
  return user.phrases.length ? user.phrases : user.words
}

async function loadInsights() {
  const currentRequest = requests.begin('load')
  insights.value = null
  loading.value = true
  error.value = ''
  try {
    if (selectedGroupID.value === null) {
      const res = await analyticsApi.chatOverview(period.value, {
        signal: currentRequest.signal,
      })
      if (currentRequest()) overview.value = res.groups
    } else {
      const res = await analyticsApi.chatInsights(
        selectedGroupID.value,
        period.value,
        { signal: currentRequest.signal },
      )
      if (currentRequest()) insights.value = res
    }
  } catch (err) {
    if (currentRequest()) {
      insights.value = null
      error.value = err instanceof Error ? err.message : '聊天洞察加载失败'
    }
  } finally {
    if (currentRequest()) loading.value = false
  }
}

async function loadGroups() {
  groupsLoading.value = true
  error.value = ''
  try {
    const res = await useGroupsStore().loadManaged()
    groups.value = res.groups
  } catch (err) {
    groups.value = []
    selectedGroupID.value = null
    error.value = err instanceof Error ? err.message : '群聊列表加载失败'
  } finally {
    groupsLoading.value = false
  }
}

function openHistoryManager() {
  if (!selectedGroupID.value) return
  historyOpen.value = true
}

async function handleHistoryDeleted(result: {
  deleted: number
  remaining: number
}) {
  notice.value = `已删除 ${result.deleted.toLocaleString('zh-CN')} 条历史消息，当前群剩余 ${result.remaining.toLocaleString('zh-CN')} 条。`
  await loadInsights()
}

watch([selectedGroupID, period], () => {
  if (initialized) void loadInsights()
})

onMounted(async () => {
  await loadGroups()
  initialized = true
  await loadInsights()
})
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="聊天洞察"
      description="看看群里最近在聊什么、谁最活跃，以及每位群友经常挂在嘴边的话。"
      icon="i-tabler-message-circle-star"
    >
      <PeriodSelect
        v-model="period"
        :options="periodOptions"
        label="聊天统计时间范围"
      />
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading || groupsLoading"
        @click="loadInsights"
        >刷新</UButton
      >
      <MoreActions
        label="聊天洞察更多操作"
        :disabled="!selectedGroupID"
        :items="[
          {
            label: '管理历史消息',
            description: '按时间清理当前群记录',
            icon: 'i-tabler-database-cog',
            onSelect: openHistoryManager,
          },
        ]"
      />
    </PageHeader>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />
    <UAlert
      v-if="notice"
      color="success"
      variant="soft"
      icon="i-tabler-check"
      :description="notice"
      :close="{
        onClick: () => {
          notice = ''
        },
      }"
    />

    <GroupScopeSelect
      allow-all
      v-model="selectedGroupID"
      :groups="groups"
      title="洞察群聊"
      description="默认查看所有群的消息统计，选择群后查看词云和成员详情"
    />

    <section
      v-if="selectedGroupID === null"
      class="rounded-2xl border border-default bg-default overflow-x-auto"
    >
      <DataTable :loading="loading" label="聊天统计">
        <thead>
          <tr class="text-muted">
            <th scope="col">群聊</th>
            <th scope="col">消息</th>
            <th scope="col">文字消息</th>
            <th scope="col">参与群友</th>
            <th scope="col">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in overviewRows" :key="row.group_id">
            <td>
              {{
                groups.find((g) => g.group_id === row.group_id)?.group_name ||
                '群'
              }}
              · {{ row.group_id }}
            </td>
            <td>{{ row.total }}</td>
            <td>{{ row.text_total }}</td>
            <td>{{ row.participants }}</td>
            <td>
              <UButton
                variant="soft"
                size="xs"
                @click="selectedGroupID = row.group_id"
                >查看详情</UButton
              >
            </td>
          </tr>
        </tbody>
      </DataTable>
      <EmptyState
        v-if="!loading && !overviewRows.length"
        title="暂无聊天记录"
        description="当前时间范围内还没有聊天数据。"
        icon="i-tabler-messages"
      />
    </section>

    <div
      v-if="selectedGroupID !== null"
      class="grid grid-cols-3 gap-3 sm:gap-4"
    >
      <MetricCard
        label="群消息"
        :value="insights?.summary.total ?? 0"
        :detail="`${insights?.period_label || '当前范围'}记录`"
        icon="i-tabler-messages"
        tone="violet"
      />
      <MetricCard
        label="参与群友"
        :value="insights?.summary.participants ?? 0"
        detail="按 QQ 去重"
        icon="i-tabler-users-group"
        tone="cyan"
      />
      <MetricCard
        label="文字消息"
        :value="textRatio"
        :detail="`${insights?.summary.text_total ?? 0} 条可分词消息`"
        icon="i-tabler-text-recognition"
        tone="amber"
      />
    </div>

    <div
      v-if="insights?.summary.total"
      class="grid grid-cols-[minmax(0,1.45fr)_minmax(300px,0.72fr)] gap-3.5 max-[860px]:grid-cols-1"
    >
      <ChatWordCloud
        :words="insights.words"
        :period-label="insights.period_label"
      />

      <section
        class="rounded-2xl border border-default bg-default overflow-hidden"
      >
        <div
          class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
        >
          <div>
            <div class="text-sm font-semibold text-highlighted">活跃群友</div>
            <div class="mt-1 text-xs leading-5 text-muted">按消息数量排列</div>
          </div>
          <span
            class="inline-flex h-5.5 min-w-6 items-center justify-center rounded-full border border-default bg-elevated px-2 text-xs text-muted"
            >TOP {{ insights.users.length }}</span
          >
        </div>
        <div class="grid">
          <div
            v-for="(user, index) in insights.users"
            :key="user.user_id"
            class="flex min-h-[66px] items-center gap-2.5 border-t border-[var(--border)] px-3.5 py-2.5 first:border-t-0"
          >
            <span
              class="w-[21px] shrink-0 text-center font-mono text-xs text-dimmed"
              :class="{ 'font-bold text-[var(--violet-bright)]': index < 3 }"
              >{{ index + 1 }}</span
            >
            <span :class="personAvatarClass">{{
              displayName(user).slice(0, 1)
            }}</span>
            <span class="min-w-0 flex-1">
              <strong class="block truncate text-xs text-toned">{{
                displayName(user)
              }}</strong>
              <small class="text-xs text-dimmed">QQ {{ user.user_id }}</small>
              <span
                class="mt-1.5 block h-[3px] w-full overflow-hidden rounded-full bg-white/[0.035]"
              >
                <i
                  class="block h-full rounded-[inherit] bg-gradient-to-r from-[#7464c7] to-[#67b9c8]"
                  :style="{ width: `${(user.count / maxUserCount) * 100}%` }"
                />
              </span>
            </span>
            <span class="min-w-[38px] text-right text-highlighted">
              <strong class="block text-sm">{{ user.count }}</strong
              ><small class="block">条</small>
            </span>
          </div>
        </div>
      </section>
    </div>

    <section
      v-if="insights?.summary.total"
      class="rounded-2xl border border-default bg-default overflow-hidden"
    >
      <div
        class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
      >
        <div>
          <div class="text-sm font-semibold text-highlighted">群友常说的话</div>
          <div class="mt-1 text-xs leading-5 text-muted">常说的话与高频词</div>
        </div>
        <UBadge color="neutral" variant="subtle">{{
          selectedGroup?.group_name || selectedGroupID
        }}</UBadge>
      </div>
      <div class="grid grid-cols-2 gap-2.5 p-3 max-[860px]:grid-cols-1">
        <article
          v-for="user in insights.users"
          :key="user.user_id"
          class="min-w-0 rounded-xl border border-[var(--border)] bg-[rgba(10,9,20,0.42)] p-3"
        >
          <div class="flex items-center gap-[9px]">
            <span :class="personAvatarClass">{{
              displayName(user).slice(0, 1)
            }}</span>
            <span class="min-w-0 flex-1">
              <strong class="block truncate text-xs text-toned">{{
                displayName(user)
              }}</strong>
              <small class="text-xs text-dimmed">{{ user.count }} 条消息</small>
            </span>
            <UBadge
              :color="user.phrases.length ? 'primary' : 'neutral'"
              variant="subtle"
            >
              {{ user.phrases.length ? '常说原句' : '常用词' }}
            </UBadge>
          </div>
          <div
            v-if="displayPhrases(user).length"
            class="mt-[11px] flex flex-wrap gap-1.5"
          >
            <span
              v-for="phrase in displayPhrases(user)"
              :key="phrase.text"
              class="inline-flex max-w-full items-center gap-1.5 rounded-lg border border-[rgba(184,172,224,0.11)] bg-[rgba(125,108,200,0.075)] px-[7px] py-[5px] text-xs text-[#c8c2d7]"
            >
              <span class="truncate">{{
                user.phrases.length ? `“${phrase.text}”` : phrase.text
              }}</span>
              <small class="shrink-0 text-xs text-[var(--violet-bright)]"
                >×{{ phrase.count }}</small
              >
            </span>
          </div>
          <div v-else class="text-xs text-dimmed">还没有形成稳定的常用表达</div>
        </article>
      </div>
    </section>

    <EmptyState
      v-if="!loading && insights && insights.summary.total === 0"
      title="这个时间范围还没有聊天记录"
      description="Bot 运行后会持续记录群文字消息，首次在群内使用词云命令也会补取最近消息"
      icon="i-tabler-messages-off"
      class="rounded-2xl border border-default bg-default"
    />

    <ChatHistoryManager
      v-model:open="historyOpen"
      :group-id="selectedGroupID"
      :group-name="selectedGroup?.group_name"
      @deleted="handleHistoryDeleted"
    />
  </section>
</template>
