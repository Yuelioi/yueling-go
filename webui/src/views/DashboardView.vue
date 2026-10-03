<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type OverviewData } from '../api'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

const overview = ref<OverviewData | null>(null)
const loading = ref(false)
const error = ref('')

const todayLabel = computed(() =>
  new Intl.DateTimeFormat('zh-CN', {
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  }).format(new Date()),
)

const connectionLabel = computed(() =>
  overview.value?.bot_connected ? 'NapCat 已连接' : 'NapCat 未连接',
)
const connectionDetail = computed(() =>
  overview.value?.bot_connected
    ? `正在服务 ${overview.value.group_count} 个群聊`
    : '后台可访问，但机器人连接暂不可用',
)

const quickActions = [
  {
    to: '/group-actions',
    title: '发送群消息',
    desc: '组合文本、艾特和图片',
    icon: 'i-tabler-send',
    tone: 'violet',
  },
  {
    to: '/knowledge',
    title: '维护知识库',
    desc: '录入群规则与项目资料',
    icon: 'i-tabler-books',
    tone: 'cyan',
  },
  {
    to: '/digests',
    title: '配置日报',
    desc: '设置每日 AI 群聊摘要',
    icon: 'i-tabler-notes',
    tone: 'rose',
  },
  {
    to: '/feeds',
    title: '管理订阅',
    desc: '聚合站点与项目更新',
    icon: 'i-tabler-rss',
    tone: 'amber',
  },
]

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
  loading.value = true
  error.value = ''
  try {
    overview.value = await api.overview()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '总览加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="space-y-6">
    <PageHeader
      eyebrow="Overview"
      title="运行总览"
      description="查看连接状态、群聊能力和最近的 AI 记录。"
      icon="i-tabler-layout-dashboard"
      ><UButton
        color="neutral"
        variant="outline"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="load"
        >刷新状态</UButton
      ></PageHeader
    >
    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />
    <UCard
      :ui="{
        body: 'flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between',
      }"
    >
      <div>
        <p class="text-xs text-muted">{{ todayLabel }}</p>
        <h2 class="mt-1 text-lg font-semibold text-highlighted">
          {{ loading && !overview ? '正在读取运行状态' : connectionLabel }}
        </h2>
        <p class="mt-1 text-sm text-muted">
          {{ overview ? connectionDetail : '获取机器人连接与群聊数据。' }}
        </p>
      </div>
      <UBadge
        :color="
          !overview ? 'neutral' : overview.bot_connected ? 'success' : 'warning'
        "
        variant="soft"
        :icon="
          overview?.bot_connected
            ? 'i-tabler-plug-connected'
            : 'i-tabler-plug-off'
        "
        >{{
          !overview
            ? '等待状态'
            : overview.bot_connected
              ? '运行中'
              : '等待连接'
        }}</UBadge
      >
    </UCard>
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <MetricCard
        label="已连接群聊"
        :value="overview?.group_count ?? '—'"
        detail="NapCat 当前群列表"
        icon="i-tabler-users-group"
        tone="cyan"
      />
      <MetricCard
        label="能力模块"
        :value="overview?.plugin_count ?? '—'"
        detail="可配置插件总数"
        icon="i-tabler-components"
      />
      <MetricCard
        label="自动日报"
        :value="overview?.digest_count ?? '—'"
        detail="每日执行的群聊摘要"
        icon="i-tabler-calendar-stats"
        tone="amber"
      />
      <MetricCard
        label="信息订阅"
        :value="overview?.feed_count ?? '—'"
        detail="RSS / Atom 自动推送"
        icon="i-tabler-rss"
        tone="cyan"
      />
      <MetricCard
        label="群知识资料"
        :value="overview?.knowledge_count ?? '—'"
        detail="用于群内可信问答"
        icon="i-tabler-books"
      />
      <MetricCard
        label="长期记忆"
        :value="overview?.memory_count ?? '—'"
        :detail="`${overview?.memory_user_count ?? 0} 位用户`"
        icon="i-tabler-brain"
        tone="rose"
      />
    </div>
    <div class="grid min-w-0 gap-5 xl:grid-cols-2">
      <UCard :ui="{ body: 'p-0 sm:p-0' }">
        <template #header
          ><div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-highlighted">最近关系</h2>
              <p class="mt-1 text-xs text-muted">最近更新的 AI 关系记录</p>
            </div>
            <UButton
              to="/affinity"
              color="neutral"
              variant="ghost"
              trailing-icon="i-tabler-arrow-right"
              >查看全部</UButton
            >
          </div></template
        >
        <div
          v-if="overview?.recent_affinity?.length"
          class="divide-y divide-default"
        >
          <div
            v-for="row in overview.recent_affinity"
            :key="row.ID"
            class="flex items-center gap-3 px-5 py-4"
          >
            <UAvatar :alt="row.Nickname || String(row.UserID)" size="sm" />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium text-highlighted">
                {{ row.Nickname || row.UserID }}
              </p>
              <p class="mt-1 truncate text-xs text-muted">
                群 {{ row.GroupID }} · {{ row.LastReason || '普通交流' }}
              </p>
            </div>
            <UBadge
              :color="
                row.Score < 20
                  ? 'error'
                  : row.Score >= 80
                    ? 'success'
                    : 'primary'
              "
              variant="soft"
              >{{ row.Score }}</UBadge
            >
          </div>
        </div>
        <div
          v-else
          class="flex min-h-40 flex-col items-center justify-center gap-2 p-5 text-sm text-muted"
        >
          <UIcon
            name="i-tabler-heart-check"
            class="size-6 text-dimmed"
          />暂无关系记录
        </div>
        <template v-if="overview?.low_affinity_count" #footer
          ><p class="flex items-center gap-2 text-xs text-warning">
            <UIcon name="i-tabler-alert-triangle" class="size-4 shrink-0" />{{
              overview.low_affinity_count
            }}
            位用户当前低于 AI 静默阈值
          </p></template
        >
      </UCard>
      <UCard :ui="{ body: 'p-0 sm:p-0' }">
        <template #header
          ><div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-sm font-semibold text-highlighted">新近记忆</h2>
              <p class="mt-1 text-xs text-muted">AI 最近提取的用户偏好</p>
            </div>
            <UButton
              to="/memories"
              color="neutral"
              variant="ghost"
              trailing-icon="i-tabler-arrow-right"
              >审阅记忆</UButton
            >
          </div></template
        >
        <div
          v-if="overview?.recent_memories?.length"
          class="divide-y divide-default"
        >
          <div
            v-for="row in overview.recent_memories"
            :key="row.ID"
            class="flex items-start gap-3 px-5 py-4"
          >
            <UIcon
              name="i-tabler-sparkles"
              class="mt-0.5 size-5 shrink-0 text-primary"
            />
            <div class="min-w-0">
              <p class="line-clamp-2 text-sm leading-6">{{ row.Content }}</p>
              <p class="mt-1 text-xs text-muted">
                QQ {{ row.UserID }} · {{ formatDate(row.CreatedAt) }}
              </p>
            </div>
          </div>
        </div>
        <div
          v-else
          class="flex min-h-40 flex-col items-center justify-center gap-2 p-5 text-sm text-muted"
        >
          <UIcon
            name="i-tabler-brain-off"
            class="size-6 text-dimmed"
          />暂无长期记忆
        </div>
      </UCard>
    </div>
    <section class="space-y-3">
      <h2 class="text-sm font-semibold text-highlighted">快速操作</h2>
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <UButton
          v-for="action in quickActions"
          :key="action.to"
          :to="action.to"
          color="neutral"
          variant="outline"
          class="justify-start gap-3 p-4"
          ><UIcon
            :name="action.icon"
            class="size-5 shrink-0 text-primary"
          /><span class="min-w-0 text-left"
            ><span class="block text-sm font-medium">{{ action.title }}</span
            ><span class="mt-1 block text-xs text-muted">{{
              action.desc
            }}</span></span
          ></UButton
        >
      </div>
    </section>
  </section>
</template>
