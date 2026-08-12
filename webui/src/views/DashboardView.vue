<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api, type OverviewData } from '../api'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

const overview = ref<OverviewData | null>(null)
const loading = ref(false)
const error = ref('')

const todayLabel = computed(() => new Intl.DateTimeFormat('zh-CN', {
  month: 'long', day: 'numeric', weekday: 'long',
}).format(new Date()))

const connectionLabel = computed(() => overview.value?.bot_connected ? 'NapCat 已连接' : 'NapCat 未连接')
const connectionDetail = computed(() => overview.value?.bot_connected
  ? `正在服务 ${overview.value.group_count} 个群聊`
  : '后台可访问，但机器人连接暂不可用')

const quickActions = [
  { to: '/group-actions', title: '发送群消息', desc: '组合文本、艾特和图片', icon: 'i-tabler-send', tone: 'violet' },
  { to: '/knowledge', title: '维护知识库', desc: '录入群规则与项目资料', icon: 'i-tabler-books', tone: 'cyan' },
  { to: '/digests', title: '配置日报', desc: '设置每日 AI 群聊摘要', icon: 'i-tabler-notes', tone: 'rose' },
  { to: '/feeds', title: '管理订阅', desc: '聚合站点与项目更新', icon: 'i-tabler-rss', tone: 'amber' },
]

function formatDate(timestamp: number) {
  if (!timestamp) return '-'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    .format(new Date(timestamp * 1000))
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
  <section class="space-y-5">
    <PageHeader
      eyebrow="Command overview"
      title="运行总览"
      description="快速确认机器人连接、AI 状态和需要处理的运营事项。"
      icon="i-tabler-layout-dashboard"
    >
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">刷新状态</UButton>
    </PageHeader>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />

    <div class="overview-hero surface-panel">
      <div class="overview-orbit" />
      <div class="overview-hero-copy">
        <div class="overview-date">{{ todayLabel }}</div>
        <h2>月灵的控制面板已经就绪。</h2>
        <p>从这里掌握群聊能力、AI 关系和长期记忆，并处理每日自动化任务。</p>
        <div class="connection-chip" :class="{ 'connection-chip-offline': overview && !overview.bot_connected }">
          <span class="connection-chip-dot" />
          <span>{{ connectionLabel }}</span>
          <small>{{ connectionDetail }}</small>
        </div>
      </div>
      <div class="overview-sigil">
        <div class="overview-sigil-ring"><UIcon name="i-tabler-moon-stars" class="size-12" /></div>
        <span>YUELING</span>
      </div>
    </div>

    <div class="metrics-grid">
      <MetricCard label="已连接群聊" :value="overview?.group_count ?? '—'" detail="NapCat 当前群列表" icon="i-tabler-users-group" tone="cyan" />
      <MetricCard label="能力模块" :value="overview?.plugin_count ?? '—'" detail="可配置插件总数" icon="i-tabler-box-multiple" tone="violet" />
      <MetricCard label="自动日报" :value="overview?.digest_count ?? '—'" detail="每日执行的群聊摘要" icon="i-tabler-calendar-stats" tone="amber" />
      <MetricCard label="信息订阅" :value="overview?.feed_count ?? '—'" detail="RSS / Atom 自动推送" icon="i-tabler-rss" tone="cyan" />
      <MetricCard label="群知识资料" :value="overview?.knowledge_count ?? '—'" detail="用于群内可信问答" icon="i-tabler-books" tone="violet" />
      <MetricCard label="长期记忆" :value="overview?.memory_count ?? '—'" :detail="`${overview?.memory_user_count ?? 0} 位用户`" icon="i-tabler-brain" tone="rose" />
    </div>

    <div class="overview-grid">
      <section class="surface-panel overflow-hidden">
        <div class="panel-header">
          <div>
            <div class="section-title">需要关注</div>
            <div class="section-caption">最近更新的 AI 关系记录</div>
          </div>
          <RouterLink class="panel-link" to="/affinity">查看全部 <UIcon name="i-tabler-arrow-right" class="size-3.5" /></RouterLink>
        </div>
        <div v-if="overview?.recent_affinity?.length">
          <div v-for="row in overview.recent_affinity" :key="row.ID" class="activity-row">
            <div class="activity-avatar">{{ (row.Nickname || String(row.UserID)).slice(0, 1) }}</div>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium text-white">{{ row.Nickname || row.UserID }}</div>
              <div class="truncate text-xs text-zinc-500">群 {{ row.GroupID }} · {{ row.LastReason || '普通交流' }}</div>
            </div>
            <UBadge :color="row.Score < 20 ? 'error' : row.Score >= 80 ? 'success' : 'primary'" variant="subtle">{{ row.Score }}</UBadge>
          </div>
        </div>
        <div v-else class="empty-state overview-empty">
          <UIcon name="i-tabler-heart-check" class="size-6" />
          <span>暂无关系记录</span>
        </div>
        <div v-if="overview?.low_affinity_count" class="attention-strip">
          <UIcon name="i-tabler-alert-triangle" class="size-4" />
          {{ overview.low_affinity_count }} 位用户当前低于 AI 静默阈值
        </div>
      </section>

      <section class="surface-panel overflow-hidden">
        <div class="panel-header">
          <div>
            <div class="section-title">新近记忆</div>
            <div class="section-caption">AI 最近提取的用户偏好</div>
          </div>
          <RouterLink class="panel-link" to="/memories">审阅记忆 <UIcon name="i-tabler-arrow-right" class="size-3.5" /></RouterLink>
        </div>
        <div v-if="overview?.recent_memories?.length">
          <div v-for="row in overview.recent_memories" :key="row.ID" class="activity-row">
            <div class="activity-icon"><UIcon name="i-tabler-sparkles" class="size-4" /></div>
            <div class="min-w-0 flex-1">
              <div class="line-clamp-1 text-sm text-zinc-200">{{ row.Content }}</div>
              <div class="mt-1 text-xs text-zinc-500">QQ {{ row.UserID }} · {{ formatDate(row.CreatedAt) }}</div>
            </div>
          </div>
        </div>
        <div v-else class="empty-state overview-empty">
          <UIcon name="i-tabler-brain-off" class="size-6" />
          <span>暂无长期记忆</span>
        </div>
      </section>
    </div>

    <section>
      <div class="section-heading-row">
        <div>
          <div class="section-title">快速操作</div>
          <div class="section-caption">常用管理入口</div>
        </div>
      </div>
      <div class="quick-action-grid">
        <RouterLink v-for="action in quickActions" :key="action.to" :to="action.to" class="quick-action-card" :class="`quick-action-${action.tone}`">
          <span class="quick-action-icon"><UIcon :name="action.icon" class="size-5" /></span>
          <span class="min-w-0 flex-1"><strong>{{ action.title }}</strong><small>{{ action.desc }}</small></span>
          <UIcon name="i-tabler-arrow-up-right" class="size-4 quick-action-arrow" />
        </RouterLink>
      </div>
    </section>
  </section>
</template>
