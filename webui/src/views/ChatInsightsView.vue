<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type ChatInsightUser, type ChatInsightsResponse, type GroupInfo } from '../api'
import ChatHistoryManager from '../components/ChatHistoryManager.vue'
import ChatWordCloud from '../components/ChatWordCloud.vue'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

type InsightPeriod = 'today' | 'yesterday' | '7days' | '30days'

const groups = ref<GroupInfo[]>([])
const selectedGroupID = ref<number | null>(null)
const period = ref<InsightPeriod>('today')
const insights = ref<ChatInsightsResponse | null>(null)
const loading = ref(false)
const groupsLoading = ref(false)
const error = ref('')
const notice = ref('')
const historyOpen = ref(false)
let initialized = false
let requestVersion = 0

const periodOptions: Array<{ value: InsightPeriod; label: string }> = [
  { value: 'today', label: '今日' },
  { value: 'yesterday', label: '昨日' },
  { value: '7days', label: '近 7 天' },
  { value: '30days', label: '近 30 天' },
]

const personAvatarClass = 'grid size-[34px] shrink-0 place-items-center rounded-[10px] border border-[var(--border)] bg-[var(--violet-soft)] text-[0.72rem] font-[720] text-[var(--violet-bright)]'

const selectedGroup = computed(() => groups.value.find((group) => group.group_id === selectedGroupID.value))
const textRatio = computed(() => {
  const summary = insights.value?.summary
  if (!summary?.total) return '0%'
  return `${Math.round(summary.text_total / summary.total * 100)}%`
})
const maxUserCount = computed(() => Math.max(1, ...(insights.value?.users.map((user) => user.count) || [1])))

function displayName(user: ChatInsightUser) {
  return user.nickname?.trim() || `群友 ${user.user_id}`
}

function displayPhrases(user: ChatInsightUser) {
  return user.phrases.length ? user.phrases : user.words
}

async function loadInsights() {
  if (!selectedGroupID.value) {
    insights.value = null
    return
  }
  const version = ++requestVersion
  loading.value = true
  error.value = ''
  try {
    const res = await api.chatInsights(selectedGroupID.value, period.value)
    if (version === requestVersion) insights.value = res
  } catch (err) {
    if (version === requestVersion) {
      insights.value = null
      error.value = err instanceof Error ? err.message : '聊天洞察加载失败'
    }
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

async function loadGroups() {
  groupsLoading.value = true
  error.value = ''
  try {
    const res = await api.groups()
    groups.value = res.groups
    if (!selectedGroupID.value || !groups.value.some((group) => group.group_id === selectedGroupID.value)) {
      selectedGroupID.value = groups.value[0]?.group_id ?? null
    }
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

async function handleHistoryDeleted(result: { deleted: number; remaining: number }) {
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
  <section class="space-y-5">
    <PageHeader
      eyebrow="Everyday conversation signals"
      title="聊天洞察"
      description="看看群里最近在聊什么、谁最活跃，以及每位群友经常挂在嘴边的话。"
      icon="i-tabler-message-circle-star"
    >
      <div class="inline-flex items-center rounded-[10px] border border-[var(--border)] bg-[var(--field)] p-[3px] max-[520px]:w-full" aria-label="聊天统计时间范围">
        <button
          v-for="option in periodOptions"
          :key="option.value"
          type="button"
          class="h-[29px] rounded-[7px] border-0 bg-transparent px-2.5 text-[0.68rem] font-[650] text-[var(--dim)] transition duration-150 hover:text-[var(--ink-soft)] max-[520px]:flex-1"
          :class="{ 'bg-[var(--violet-soft)] text-[var(--violet-bright)] shadow-[inset_0_0_0_1px_rgba(125,108,200,0.16)]': period === option.value }"
          @click="period = option.value"
        >
          {{ option.label }}
        </button>
      </div>
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :loading="loading || groupsLoading" @click="loadInsights">刷新</UButton>
      <MoreActions
        label="聊天洞察更多操作"
        :disabled="!selectedGroupID"
        :items="[
          { label: '管理历史消息', description: '按时间清理当前群记录', icon: 'i-tabler-database-cog', onSelect: openHistoryManager },
        ]"
      />
    </PageHeader>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />
    <UAlert v-if="notice" color="success" variant="subtle" icon="i-tabler-check" :description="notice" :close="{ onClick: () => { notice = '' } }" />

    <GroupScopeSelect
      v-model="selectedGroupID"
      :groups="groups"
      title="洞察群聊"
      description="聊天数据严格按群隔离"
    />

    <div class="grid grid-cols-3 gap-3 max-[860px]:grid-cols-1">
      <MetricCard label="群消息" :value="insights?.summary.total ?? 0" :detail="`${insights?.period_label || '当前范围'}记录`" icon="i-tabler-messages" tone="violet" />
      <MetricCard label="参与群友" :value="insights?.summary.participants ?? 0" detail="按 QQ 去重" icon="i-tabler-users-group" tone="cyan" />
      <MetricCard label="文字消息" :value="textRatio" :detail="`${insights?.summary.text_total ?? 0} 条可分词消息`" icon="i-tabler-text-recognition" tone="amber" />
    </div>

    <div v-if="insights?.summary.total" class="grid grid-cols-[minmax(0,1.45fr)_minmax(300px,0.72fr)] gap-3.5 max-[860px]:grid-cols-1">
      <ChatWordCloud :words="insights.words" :period-label="insights.period_label" />

      <section class="surface-panel overflow-hidden">
        <div class="panel-header">
          <div>
            <div class="section-title">活跃群友</div>
            <div class="section-caption">按消息数量排列</div>
          </div>
          <span class="count-pill">TOP {{ insights.users.length }}</span>
        </div>
        <div class="grid">
          <div
            v-for="(user, index) in insights.users"
            :key="user.user_id"
            class="flex min-h-[66px] items-center gap-2.5 border-t border-[var(--border)] px-3.5 py-2.5 first:border-t-0"
          >
            <span
              class="w-[21px] shrink-0 text-center font-mono text-[0.66rem] text-[var(--dim)]"
              :class="{ 'font-[750] text-[var(--violet-bright)]': index < 3 }"
            >{{ index + 1 }}</span>
            <span :class="personAvatarClass">{{ displayName(user).slice(0, 1) }}</span>
            <span class="min-w-0 flex-1">
              <strong class="block truncate text-xs text-[var(--ink-soft)]">{{ displayName(user) }}</strong>
              <small class="text-[0.61rem] text-[var(--dim)]">QQ {{ user.user_id }}</small>
              <span class="mt-1.5 block h-[3px] w-full overflow-hidden rounded-full bg-white/[0.035]">
                <i
                  class="block h-full rounded-[inherit] bg-gradient-to-r from-[#7464c7] to-[#67b9c8]"
                  :style="{ width: `${user.count / maxUserCount * 100}%` }"
                />
              </span>
            </span>
            <span class="min-w-[38px] text-right text-[var(--ink)]">
              <strong class="block text-[0.9rem]">{{ user.count }}</strong><small class="block">条</small>
            </span>
          </div>
        </div>
      </section>
    </div>

    <section v-if="insights?.summary.total" class="surface-panel overflow-hidden">
      <div class="panel-header">
        <div>
          <div class="section-title">群友常说的话</div>
          <div class="section-caption">优先展示重复原句；没有稳定原句时展示 zhparser 常用词</div>
        </div>
        <UBadge color="neutral" variant="subtle">{{ selectedGroup?.group_name || selectedGroupID }}</UBadge>
      </div>
      <div class="grid grid-cols-2 gap-2.5 p-3 max-[860px]:grid-cols-1">
        <article
          v-for="user in insights.users"
          :key="user.user_id"
          class="min-w-0 rounded-xl border border-[var(--border)] bg-[rgba(10,9,20,0.42)] p-3"
        >
          <div class="flex items-center gap-[9px]">
            <span :class="personAvatarClass">{{ displayName(user).slice(0, 1) }}</span>
            <span class="min-w-0 flex-1">
              <strong class="block truncate text-xs text-[var(--ink-soft)]">{{ displayName(user) }}</strong>
              <small class="text-[0.61rem] text-[var(--dim)]">{{ user.count }} 条消息</small>
            </span>
            <UBadge :color="user.phrases.length ? 'primary' : 'neutral'" variant="subtle">
              {{ user.phrases.length ? '常说原句' : '常用词' }}
            </UBadge>
          </div>
          <div v-if="displayPhrases(user).length" class="mt-[11px] flex flex-wrap gap-1.5">
            <span
              v-for="phrase in displayPhrases(user)"
              :key="phrase.text"
              class="inline-flex max-w-full items-center gap-1.5 rounded-lg border border-[rgba(184,172,224,0.11)] bg-[rgba(125,108,200,0.075)] px-[7px] py-[5px] text-[0.68rem] text-[#c8c2d7]"
            >
              <span class="truncate">{{ user.phrases.length ? `“${phrase.text}”` : phrase.text }}</span>
              <small class="shrink-0 text-[0.58rem] text-[var(--violet-bright)]">×{{ phrase.count }}</small>
            </span>
          </div>
          <div v-else class="text-xs text-zinc-600">还没有形成稳定的常用表达</div>
        </article>
      </div>
    </section>

    <div v-if="!loading && insights && insights.summary.total === 0" class="surface-panel empty-state py-16">
      <UIcon name="i-tabler-messages-off" class="size-8 text-zinc-500" />
      <div class="font-medium text-zinc-200">这个时间范围还没有聊天记录</div>
      <div class="text-sm text-zinc-500">Bot 运行后会持续记录群文字消息，首次在群内使用词云命令也会补取最近消息</div>
    </div>

    <div class="flex items-start gap-2 rounded-[11px] border border-[rgba(100,215,165,0.12)] bg-[rgba(100,215,165,0.045)] px-3 py-2.5 text-[0.68rem] leading-[1.55] text-[#88b8a5]">
      <UIcon name="i-tabler-shield-lock" class="size-4" />
      <span>聊天记录持续保留在本地 PostgreSQL，并严格按群隔离；统计不上传聊天内容，也不调用 AI。</span>
    </div>

    <ChatHistoryManager
      v-model:open="historyOpen"
      :group-id="selectedGroupID"
      :group-name="selectedGroup?.group_name"
      @deleted="handleHistoryDeleted"
    />
  </section>
</template>
