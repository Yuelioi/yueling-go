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
  { to: '/chat-insights', title: '查看聊天洞察', desc: '词云、活跃榜与口头禅', icon: 'i-tabler-message-circle-star', tone: 'rose' },
  { to: '/knowledge', title: '维护知识库', desc: '录入群规则与项目资料', icon: 'i-tabler-books', tone: 'cyan' },
  { to: '/digests', title: '配置日报', desc: '设置每日 AI 群聊摘要', icon: 'i-tabler-notes', tone: 'rose' },
  { to: '/feeds', title: '管理订阅', desc: '聚合站点与项目更新', icon: 'i-tabler-rss', tone: 'amber' },
] as const

const quickActionToneClasses = {
  violet: 'text-[var(--violet-bright)]',
  cyan: 'text-[var(--cyan)]',
  rose: 'text-[var(--rose)]',
  amber: 'text-[var(--amber)]',
} as const

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
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :loading="loading" @click="load">刷新状态</UButton>
    </PageHeader>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />

    <div class="surface-panel relative grid min-h-[238px] grid-cols-[minmax(0,1fr)_210px] items-center overflow-hidden !bg-[linear-gradient(115deg,rgba(33,27,66,0.96),rgba(19,18,36,0.9)_56%,rgba(19,34,45,0.76))] p-[clamp(28px,4vw,48px)] before:pointer-events-none before:absolute before:inset-0 before:bg-[radial-gradient(rgba(255,255,255,0.11)_0.6px,transparent_0.6px)] before:opacity-[0.28] before:content-[''] before:[background-size:25px_25px] before:[mask-image:linear-gradient(90deg,black,transparent_76%)] max-[860px]:grid-cols-[minmax(0,1fr)_130px] max-[520px]:block max-[520px]:min-h-0 max-[520px]:px-[22px] max-[520px]:py-[26px]">
      <div class="absolute -right-28 size-[28rem] rounded-full border border-[rgba(103,216,236,0.11)] shadow-[inset_0_0_70px_rgba(103,216,236,0.045),0_0_0_3rem_rgba(125,108,200,0.018)] after:absolute after:left-[4%] after:top-[21%] after:size-[9px] after:rounded-full after:bg-[var(--cyan)] after:shadow-[0_0_24px_rgba(103,216,236,0.8)] after:content-['']" />
      <div class="relative z-[1]">
        <div class="text-[0.68rem] font-[720] tracking-[0.08em] text-[var(--violet-bright)]">{{ todayLabel }}</div>
        <h2 class="m-0 mt-2.5 text-[clamp(1.75rem,3.4vw,3rem)] font-[730] leading-[1.08] tracking-[-0.055em]">月灵的控制面板已经就绪。</h2>
        <p class="m-0 mt-3.5 max-w-[630px] text-[0.84rem] leading-[1.7] text-[var(--muted)]">从这里掌握群聊能力、AI 关系和长期记忆，并处理每日自动化任务。</p>
        <div
          class="mt-[25px] inline-flex items-center gap-2 rounded-full border px-3 py-[7px] text-[0.72rem] font-[650] max-[520px]:items-start max-[520px]:rounded-[13px]"
          :class="overview && !overview.bot_connected
            ? 'border-[rgba(255,140,168,0.2)] bg-[rgba(255,140,168,0.07)] text-[var(--rose)]'
            : 'border-[rgba(100,215,165,0.2)] bg-[rgba(100,215,165,0.07)] text-[var(--success)]'"
        >
          <span class="size-[7px] rounded-full bg-current shadow-[0_0_12px_currentColor]" />
          <span>{{ connectionLabel }}</span>
          <small class="border-l border-white/10 pl-2 text-[0.65rem] font-medium text-[var(--muted)] max-[520px]:hidden">{{ connectionDetail }}</small>
        </div>
      </div>
      <div class="relative z-[1] grid justify-items-center gap-3.5 text-[var(--violet-bright)] max-[520px]:hidden">
        <div class="grid size-[126px] place-items-center rounded-full border border-[rgba(185,175,230,0.22)] bg-[radial-gradient(circle,rgba(125,108,200,0.16),transparent_68%)] shadow-[0_0_0_14px_rgba(125,108,200,0.025),inset_0_0_30px_rgba(125,108,200,0.08)] max-[860px]:size-24"><UIcon name="i-tabler-moon-stars" class="size-12" /></div>
        <span class="text-[0.58rem] font-[760] tracking-[0.34em] text-[var(--dim)]">YUELING</span>
      </div>
    </div>

    <div class="grid grid-cols-3 gap-3 max-[860px]:grid-cols-1">
      <MetricCard label="已连接群聊" :value="overview?.group_count ?? '—'" detail="NapCat 当前群列表" icon="i-tabler-users-group" tone="cyan" />
      <MetricCard label="能力模块" :value="overview?.plugin_count ?? '—'" detail="可配置插件总数" icon="i-tabler-box-multiple" tone="violet" />
      <MetricCard label="自动日报" :value="overview?.digest_count ?? '—'" detail="每日执行的群聊摘要" icon="i-tabler-calendar-stats" tone="amber" />
      <MetricCard label="信息订阅" :value="overview?.feed_count ?? '—'" detail="RSS / Atom 自动推送" icon="i-tabler-rss" tone="cyan" />
      <MetricCard label="群知识资料" :value="overview?.knowledge_count ?? '—'" detail="用于群内可信问答" icon="i-tabler-books" tone="violet" />
      <MetricCard label="长期记忆" :value="overview?.memory_count ?? '—'" :detail="`${overview?.memory_user_count ?? 0} 位用户`" icon="i-tabler-brain" tone="rose" />
    </div>

    <div class="grid grid-cols-2 gap-3.5 max-[860px]:grid-cols-1">
      <section class="surface-panel overflow-hidden">
        <div class="panel-header">
          <div>
            <div class="section-title">需要关注</div>
            <div class="section-caption">最近更新的 AI 关系记录</div>
          </div>
          <RouterLink class="inline-flex items-center gap-[5px] text-[0.68rem] text-[var(--violet-bright)]" to="/affinity">查看全部 <UIcon name="i-tabler-arrow-right" class="size-3.5" /></RouterLink>
        </div>
        <div v-if="overview?.recent_affinity?.length">
          <div v-for="row in overview.recent_affinity" :key="row.ID" class="flex min-h-[62px] items-center gap-[11px] border-t border-[var(--border)] px-[15px] py-2.5 first:border-t-0">
            <div class="grid size-[34px] shrink-0 place-items-center rounded-[10px] border border-[var(--border)] bg-[var(--violet-soft)] text-xs font-bold text-[var(--violet-bright)]">{{ (row.Nickname || String(row.UserID)).slice(0, 1) }}</div>
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
        <div v-if="overview?.low_affinity_count" class="flex items-center gap-[7px] border-t border-[rgba(255,140,168,0.14)] bg-[rgba(255,140,168,0.055)] px-[15px] py-[9px] text-[0.68rem] text-[var(--rose)]">
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
          <RouterLink class="inline-flex items-center gap-[5px] text-[0.68rem] text-[var(--violet-bright)]" to="/memories">审阅记忆 <UIcon name="i-tabler-arrow-right" class="size-3.5" /></RouterLink>
        </div>
        <div v-if="overview?.recent_memories?.length">
          <div v-for="row in overview.recent_memories" :key="row.ID" class="flex min-h-[62px] items-center gap-[11px] border-t border-[var(--border)] px-[15px] py-2.5 first:border-t-0">
            <div class="grid size-[34px] shrink-0 place-items-center rounded-[10px] border border-[var(--border)] bg-[rgba(103,216,236,0.08)] text-[var(--cyan)]"><UIcon name="i-tabler-sparkles" class="size-4" /></div>
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
      <div class="mx-0.5 mb-2.5 flex items-end justify-between">
        <div>
          <div class="section-title">快速操作</div>
          <div class="section-caption">常用管理入口</div>
        </div>
      </div>
      <div class="grid grid-cols-4 gap-[11px] max-[1060px]:grid-cols-2 max-[860px]:grid-cols-1">
        <RouterLink
          v-for="action in quickActions"
          :key="action.to"
          :to="action.to"
          class="flex items-center gap-[11px] rounded-[13px] border border-[var(--border)] bg-[rgba(20,19,36,0.75)] p-[13px] text-[var(--muted)] transition duration-160 hover:-translate-y-0.5 hover:border-[var(--border-strong)] hover:bg-[rgba(27,25,47,0.9)]"
          :class="quickActionToneClasses[action.tone]"
        >
          <span class="grid size-[38px] shrink-0 place-items-center rounded-[11px] bg-[color-mix(in_srgb,currentColor_12%,transparent)]"><UIcon :name="action.icon" class="size-5" /></span>
          <span class="min-w-0 flex-1">
            <strong class="block text-[0.76rem] text-[var(--ink)]">{{ action.title }}</strong>
            <small class="mt-0.5 block truncate text-[0.63rem] text-[var(--dim)]">{{ action.desc }}</small>
          </span>
          <UIcon name="i-tabler-arrow-up-right" class="size-4 text-[var(--dim)]" />
        </RouterLink>
      </div>
    </section>
  </section>
</template>
