<script setup lang="ts">
import EmptyState from '../components/EmptyState.vue'
import PeriodSelect from '../components/PeriodSelect.vue'
import { useRequestScope } from '../composables/useRequestScope'
import { useGroupsStore } from '../stores/groups.ts'
import { computed, onMounted, ref, watch } from 'vue'
import {
  analyticsApi,
  type GroupCommandUsageStats,
  type GroupInfo,
} from '../api'
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
const requests = useRequestScope()

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)
const isAllGroups = computed(() => selectedGroupID.value === 0)
const peakCalls = computed(() =>
  Math.max(1, ...(stats.value?.daily.map((day) => day.calls) || [0])),
)
const peakCommandCalls = computed(() =>
  Math.max(1, ...(stats.value?.top_commands.map((row) => row.calls) || [0])),
)
const averageDaily = computed(() =>
  stats.value
    ? Math.round((stats.value.total_calls / stats.value.days) * 10) / 10
    : 0,
)

function shortDate(value: string) {
  const [, month, day] = value.split('-')
  return `${month}/${day}`
}

function formatLastUsed(timestamp: number) {
  if (!timestamp) return '暂无记录'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

function pluginName(pluginID: number) {
  return (
    pluginNames.value[String(pluginID)] ||
    (pluginID ? `插件 #${pluginID}` : '未分类')
  )
}

async function loadGroups() {
  groupsLoading.value = true
  error.value = ''
  try {
    const res = await useGroupsStore().loadConnected()
    groups.value = [{ group_id: 0, group_name: '所有群聊' }, ...res.groups]
    if (
      selectedGroupID.value === null ||
      !groups.value.some((group) => group.group_id === selectedGroupID.value)
    ) {
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
  const currentRequest = requests.begin('load')
  statsLoading.value = true
  error.value = ''
  try {
    const res = await analyticsApi.commandUsage(
      selectedGroupID.value,
      days.value,
      { signal: currentRequest.signal },
    )
    if (!currentRequest()) return
    stats.value = res.stats
    pluginNames.value = res.plugin_names
  } catch (err) {
    if (!currentRequest()) return
    stats.value = null
    error.value = err instanceof Error ? err.message : '调用统计加载失败'
  } finally {
    if (currentRequest()) statsLoading.value = false
  }
}

watch([selectedGroupID, days], loadStats)
onMounted(loadGroups)
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="调用统计"
      description="查看全部群汇总，或观察单个群里哪些命令真的有人使用。"
      icon="i-tabler-chart-bar"
    >
      <PeriodSelect
        v-model="days"
        :options="[7, 30, 90].map((value) => ({ label: `${value} 天`, value }))"
        label="统计时间范围"
      />
      <UButton
        color="neutral"
        icon="i-tabler-refresh"
        variant="soft"
        :loading="groupsLoading || statsLoading"
        @click="loadStats"
        >刷新</UButton
      >
    </PageHeader>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        v-model="selectedGroupID"
        :groups="groups"
        title="统计群聊"
        description="全部汇总或指定群聊"
        zero-label="跨群汇总"
      />

      <div class="min-w-0 space-y-4">
        <section
          class="rounded-2xl border border-default bg-default overflow-hidden"
        >
          <div
            class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
          >
            <div class="min-w-0">
              <div class="truncate font-medium text-highlighted">
                {{ selectedGroup?.group_name || '未选择群' }}
              </div>
              <div class="text-xs text-muted">
                {{
                  selectedGroupID === null
                    ? '选择范围后查看数据'
                    : isAllGroups
                      ? `${groups.length - 1} 个群聊汇总 · 最近 ${days} 天`
                      : `群号 ${selectedGroupID} · 最近 ${days} 天`
                }}
              </div>
            </div>
            <UBadge color="primary" variant="subtle">实时累计</UBadge>
          </div>
          <div class="grid grid-cols-3 gap-2.5 p-3.5 max-[860px]:grid-cols-1">
            <MetricCard
              label="命令调用"
              :value="stats?.total_calls ?? 0"
              :detail="`日均 ${averageDaily} 次`"
              icon="i-tabler-terminal-2"
              tone="violet"
            />
            <MetricCard
              label="使用群友"
              :value="stats?.unique_users ?? 0"
              :detail="isAllGroups ? '跨群按 QQ 去重' : '本群按 QQ 去重'"
              icon="i-tabler-users"
              tone="cyan"
            />
            <MetricCard
              label="活跃命令"
              :value="stats?.active_commands ?? 0"
              detail="统计期内至少调用一次"
              icon="i-tabler-command"
              tone="amber"
            />
          </div>
        </section>

        <section
          class="rounded-2xl border border-default bg-default overflow-hidden"
        >
          <div
            class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
          >
            <div>
              <div class="text-sm font-semibold text-highlighted">每日趋势</div>
              <div class="mt-1 text-xs leading-5 text-muted">
                调用量与每天的独立使用人数
              </div>
            </div>
            <span
              class="inline-flex h-5.5 min-w-6 items-center justify-center rounded-full border border-default bg-elevated px-2 text-xs text-muted"
              >峰值 {{ peakCalls }}</span
            >
          </div>
          <div
            v-if="stats?.daily.length"
            class="overflow-x-auto overscroll-x-contain px-[18px] pb-[13px] pt-[18px]"
          >
            <div
              class="flex h-[226px] min-w-[580px] items-stretch gap-2"
              :class="{ 'min-w-[1120px] gap-1': days > 30 }"
            >
              <div
                v-for="(day, index) in stats.daily"
                :key="day.date"
                class="grid min-w-[18px] flex-1 grid-rows-[18px_minmax(0,1fr)_18px] items-end gap-1.5"
                :title="`${day.date} · ${day.calls} 次 · ${day.unique_users} 人`"
              >
                <div
                  class="overflow-hidden whitespace-nowrap text-center font-mono text-xs text-dimmed [text-overflow:clip]"
                >
                  {{ day.calls || '' }}
                </div>
                <div
                  class="relative flex h-full items-end overflow-hidden rounded-[7px] bg-[linear-gradient(to_top,rgba(184,172,224,0.075)_1px,transparent_1px)_0_0/100%_25%,rgba(255,255,255,0.015)]"
                >
                  <span
                    class="min-h-0.5 w-full rounded-[6px_6px_3px_3px] bg-[linear-gradient(180deg,var(--cyan),var(--violet))] shadow-[0_0_16px_rgba(125,108,200,0.24)] transition-[height] duration-240"
                    :style="{
                      height: `${Math.max(day.calls ? 7 : 2, (day.calls / peakCalls) * 100)}%`,
                    }"
                  />
                </div>
                <div
                  class="overflow-hidden whitespace-nowrap text-center font-mono text-xs text-dimmed [text-overflow:clip]"
                >
                  {{
                    days <= 7 ||
                    index % (days === 30 ? 5 : 15) === 0 ||
                    index === stats.daily.length - 1
                      ? shortDate(day.date)
                      : ''
                  }}
                </div>
              </div>
            </div>
          </div>
          <EmptyState
            v-else-if="!statsLoading"
            icon="i-tabler-chart-bar-off"
            title="这个群在所选时间内还没有命令调用"
          />
        </section>

        <section
          class="rounded-2xl border border-default bg-default overflow-hidden"
        >
          <div
            class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
          >
            <div>
              <div class="text-sm font-semibold text-highlighted">常用命令</div>
              <div class="mt-1 text-xs leading-5 text-muted">
                按实际调用次数排序，只统计明确命令
              </div>
            </div>
            <span
              class="inline-flex h-5.5 min-w-6 items-center justify-center rounded-full border border-default bg-elevated px-2 text-xs text-muted"
              >TOP {{ stats?.top_commands.length || 0 }}</span
            >
          </div>
          <div v-if="stats?.top_commands.length" class="py-0.5">
            <div
              v-for="(row, index) in stats.top_commands"
              :key="`${row.plugin_id}-${row.command}`"
              class="grid grid-cols-[30px_minmax(0,1fr)_58px] items-center gap-3 border-t border-[var(--border)] px-4 py-[13px] first:border-t-0 max-[520px]:grid-cols-[28px_minmax(0,1fr)_42px] max-[520px]:gap-[9px] max-[520px]:p-3"
            >
              <span
                class="grid size-7 place-items-center rounded-[9px] border border-[var(--border)] bg-white/[0.025] text-xs font-semibold text-dimmed"
                :class="{
                  'border-[rgba(125,108,200,0.2)] bg-[var(--violet-soft)] text-[var(--violet-bright)]':
                    index < 3,
                }"
                >{{ index + 1 }}</span
              >
              <div class="min-w-0">
                <div class="flex min-w-0 items-center gap-2">
                  <code
                    class="truncate font-mono text-sm font-semibold text-highlighted"
                    >{{ row.command }}</code
                  >
                  <UBadge color="neutral" variant="subtle">{{
                    pluginName(row.plugin_id)
                  }}</UBadge>
                </div>
                <div
                  class="mt-[9px] h-[3px] overflow-hidden rounded-full bg-white/[0.045]"
                >
                  <span
                    class="block h-full rounded-[inherit] bg-[linear-gradient(90deg,var(--violet),var(--cyan))]"
                    :style="{
                      width: `${(row.calls / peakCommandCalls) * 100}%`,
                    }"
                  />
                </div>
                <div class="mt-1.5 text-xs text-dimmed">
                  {{ row.unique_users }} 人使用 · 最近
                  {{ formatLastUsed(row.last_used_at) }}
                </div>
              </div>
              <div class="text-right">
                <strong
                  class="block text-[1.05rem] leading-[1.1] text-highlighted"
                  >{{ row.calls }}</strong
                >
                <span class="mt-0.5 block text-xs text-dimmed">次</span>
              </div>
            </div>
          </div>
          <EmptyState
            v-else-if="!statsLoading"
            icon="i-tabler-command-off"
            title="暂无常用命令排行"
          />
        </section>
      </div>
    </div>
  </section>
</template>
