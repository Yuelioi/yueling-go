<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupCommandUsageStats, type GroupInfo } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const selectedGroupID = ref<number | null>(null)
const days = ref(7)
const stats = ref<GroupCommandUsageStats | null>(null)
const pluginNames = ref<Record<string, string>>({})
const groupsLoading = ref(false)
const statsLoading = ref(false)
const error = ref('')
let requestVersion = 0

const selectedGroup = computed(() => groups.value.find((group) => group.group_id === selectedGroupID.value))
const isAllGroups = computed(() => selectedGroupID.value === 0)
const peakCalls = computed(() => Math.max(1, ...(stats.value?.daily.map((day) => day.calls) || [0])))
const peakCommandCalls = computed(() => Math.max(1, ...(stats.value?.top_commands.map((row) => row.calls) || [0])))
const averageDaily = computed(() => stats.value ? Math.round(stats.value.total_calls / stats.value.days * 10) / 10 : 0)

function shortDate(value: string) {
  const [, month, day] = value.split('-')
  return `${month}/${day}`
}

function formatLastUsed(timestamp: number) {
  if (!timestamp) return '暂无记录'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

function pluginName(pluginID: number) {
  return pluginNames.value[String(pluginID)] || (pluginID ? `插件 #${pluginID}` : '未分类')
}

async function loadGroups() {
  groupsLoading.value = true
  error.value = ''
  try {
    const res = await api.groups()
    groups.value = [{ group_id: 0, group_name: '所有群聊' }, ...res.groups]
    if (selectedGroupID.value === null || !groups.value.some((group) => group.group_id === selectedGroupID.value)) {
      selectedGroupID.value = 0
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群列表加载失败'
  } finally {
    groupsLoading.value = false
  }
}

async function loadStats() {
  if (selectedGroupID.value === null) {
    stats.value = null
    return
  }
  const version = ++requestVersion
  statsLoading.value = true
  error.value = ''
  try {
    const res = await api.commandUsage(selectedGroupID.value, days.value)
    if (version !== requestVersion) return
    stats.value = res.stats
    pluginNames.value = res.plugin_names
  } catch (err) {
    if (version !== requestVersion) return
    stats.value = null
    error.value = err instanceof Error ? err.message : '调用统计加载失败'
  } finally {
    if (version === requestVersion) statsLoading.value = false
  }
}

watch([selectedGroupID, days], loadStats)
onMounted(loadGroups)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Command analytics"
      title="调用统计"
      description="查看全部群汇总，或观察单个群里哪些命令真的有人使用。"
      icon="i-tabler-chart-bar"
    >
      <div class="usage-period-switch" aria-label="统计时间范围">
        <button
          v-for="period in [7, 30, 90]"
          :key="period"
          type="button"
          :class="{ 'usage-period-active': days === period }"
          @click="days = period"
        >
          {{ period }} 天
        </button>
      </div>
      <UButton color="neutral" icon="i-tabler-refresh" variant="soft" :loading="groupsLoading || statsLoading" @click="loadStats">刷新</UButton>
    </PageHeader>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />

    <div class="space-y-4">
      <GroupScopeSelect
        v-model="selectedGroupID"
        :groups="groups"
        title="统计群聊"
        description="全部汇总或指定群聊"
        zero-label="跨群汇总"
      />

      <div class="min-w-0 space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择群' }}</div>
              <div class="text-xs text-zinc-500">{{ selectedGroupID === null ? '选择范围后查看数据' : isAllGroups ? `${groups.length - 1} 个群聊汇总 · 最近 ${days} 天` : `群号 ${selectedGroupID} · 最近 ${days} 天` }}</div>
            </div>
            <UBadge color="primary" variant="subtle">实时累计</UBadge>
          </div>
          <div class="usage-metrics">
            <MetricCard label="命令调用" :value="stats?.total_calls ?? 0" :detail="`日均 ${averageDaily} 次`" icon="i-tabler-terminal-2" tone="violet" />
            <MetricCard label="使用群友" :value="stats?.unique_users ?? 0" :detail="isAllGroups ? '跨群按 QQ 去重' : '本群按 QQ 去重'" icon="i-tabler-users" tone="cyan" />
            <MetricCard label="活跃命令" :value="stats?.active_commands ?? 0" detail="统计期内至少调用一次" icon="i-tabler-command" tone="amber" />
          </div>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div>
              <div class="section-title">每日趋势</div>
              <div class="section-caption">调用量与每天的独立使用人数</div>
            </div>
            <span class="count-pill">峰值 {{ peakCalls }}</span>
          </div>
          <div v-if="stats?.daily.length" class="usage-chart-scroll">
            <div class="usage-chart" :class="{ 'usage-chart-dense': days > 30 }">
              <div v-for="(day, index) in stats.daily" :key="day.date" class="usage-bar-column" :title="`${day.date} · ${day.calls} 次 · ${day.unique_users} 人`">
                <div class="usage-bar-value">{{ day.calls || '' }}</div>
                <div class="usage-bar-track">
                  <span :style="{ height: `${Math.max(day.calls ? 7 : 2, day.calls / peakCalls * 100)}%` }" />
                </div>
                <div class="usage-bar-date">{{ days <= 7 || index % (days === 30 ? 5 : 15) === 0 || index === stats.daily.length - 1 ? shortDate(day.date) : '' }}</div>
              </div>
            </div>
          </div>
          <div v-else-if="!statsLoading" class="empty-state overview-empty">
            <UIcon name="i-tabler-chart-bar-off" class="size-6" />
            <span>这个群在所选时间内还没有命令调用</span>
          </div>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div>
              <div class="section-title">常用命令</div>
              <div class="section-caption">按实际调用次数排序，只统计明确命令</div>
            </div>
            <span class="count-pill">TOP {{ stats?.top_commands.length || 0 }}</span>
          </div>
          <div v-if="stats?.top_commands.length" class="usage-command-list">
            <div v-for="(row, index) in stats.top_commands" :key="`${row.plugin_id}-${row.command}`" class="usage-command-row">
              <span class="usage-rank" :class="{ 'usage-rank-top': index < 3 }">{{ index + 1 }}</span>
              <div class="usage-command-main">
                <div class="usage-command-title">
                  <code>{{ row.command }}</code>
                  <UBadge color="neutral" variant="subtle">{{ pluginName(row.plugin_id) }}</UBadge>
                </div>
                <div class="usage-command-progress"><span :style="{ width: `${row.calls / peakCommandCalls * 100}%` }" /></div>
                <div class="usage-command-meta">{{ row.unique_users }} 人使用 · 最近 {{ formatLastUsed(row.last_used_at) }}</div>
              </div>
              <div class="usage-command-count"><strong>{{ row.calls }}</strong><span>次</span></div>
            </div>
          </div>
          <div v-else-if="!statsLoading" class="empty-state overview-empty">
            <UIcon name="i-tabler-command-off" class="size-6" />
            <span>暂无常用命令排行</span>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
