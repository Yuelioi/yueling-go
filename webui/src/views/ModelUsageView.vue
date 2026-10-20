<script setup lang="ts">
import PeriodSelect from '../components/PeriodSelect.vue'
import DataTable from '../components/DataTable.vue'
import { useRequestScope } from '../composables/useRequestScope'
import { useGroupsStore } from '../stores/groups.ts'
import { computed, onMounted, ref, watch } from 'vue'
import { aiApi, type GroupInfo, type ModelUsageStats } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import { useGroupFilter } from '../composables/useGroupFilter'

const group = useGroupFilter()
const groups = ref<GroupInfo[]>([])
const rows = ref<ModelUsageStats[]>([])
const days = ref(7)
const loading = ref(false)
const error = ref('')
const requests = useRequestScope()
const totals = computed(() =>
  rows.value.reduce(
    (sum, row) => ({
      requests: sum.requests + row.requests,
      prompt: sum.prompt + row.prompt_tokens,
      cached: sum.cached + row.cached_tokens,
      known: sum.known + row.cache_reported_prompt_tokens,
      reported: sum.reported + row.cache_reported_requests,
    }),
    { requests: 0, prompt: 0, cached: 0, known: 0, reported: 0 },
  ),
)
const percent = (value: number | null) =>
  value === null ? '未提供' : `${(value * 100).toFixed(1)}%`
const count = (value: number) => value.toLocaleString('zh-CN')
const groupName = (id: number) =>
  id === 0
    ? '私聊 / 后台任务'
    : groups.value.find((g) => g.group_id === id)?.group_name || `群 ${id}`
async function load() {
  const current = requests.begin('load')
  loading.value = true
  error.value = ''
  rows.value = []
  try {
    const result = await aiApi.modelUsage(group.value, days.value, {
      signal: current.signal,
    })
    if (current()) rows.value = result.stats
  } catch (err) {
    if (current())
      error.value = err instanceof Error ? err.message : '统计加载失败'
  } finally {
    if (current()) loading.value = false
  }
}
watch([group, days], load, { immediate: true })
onMounted(async () => {
  try {
    groups.value = [
      { group_id: 0, group_name: '私聊 / 后台任务' },
      ...(await useGroupsStore().loadConnected()).groups,
    ]
  } catch {
    /* Usage remains available when group names cannot be loaded. */
  }
})
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <div class="text-sm font-semibold text-highlighted">模型缓存与用量</div>
        <p class="mt-1 text-xs leading-5 text-muted">
          按功能和配置统计成功返回的模型请求；后台任务与能力测试归入无群范围。
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <PeriodSelect
          v-model="days"
          :options="
            [1, 7, 30, 90].map((value) => ({ label: `${value} 天`, value }))
          "
          label="用量时间范围"
        />
        <UButton
          icon="i-tabler-refresh"
          color="neutral"
          variant="soft"
          :loading="loading"
          @click="load"
          >刷新</UButton
        >
      </div>
    </div>
    <GroupScopeSelect
      v-model="group"
      :groups="groups"
      allow-all
      title="统计范围"
      description="默认列出全部群，可筛选单个群"
      zero-label="私聊与后台任务"
    />
    <UAlert
      icon="i-tabler-alert-circle"
      v-if="error"
      color="error"
      variant="soft"
      :description="error"
    />
    <div class="grid grid-cols-3 gap-3 sm:gap-4">
      <MetricCard
        label="模型请求"
        :value="count(totals.requests)"
        detail="按请求计数，不等于用户消息数"
        icon="i-tabler-messages"
      />
      <MetricCard
        label="输入 tokens"
        :value="count(totals.prompt)"
        detail="包含缓存命中与未命中输入"
        icon="i-tabler-coin"
        tone="cyan"
      />
      <MetricCard
        label="缓存命中率"
        :value="percent(totals.known ? totals.cached / totals.known : null)"
        :detail="`${totals.reported} / ${totals.requests} 次请求返回缓存数据`"
        icon="i-tabler-bolt"
        tone="amber"
      />
    </div>
    <p class="text-xs text-dimmed">
      命中率按已提供缓存数据的输入 tokens
      加权计算；未提供数据不按零命中计算。用量从本功能启用后累计，不推算账单金额。
    </p>
    <div
      class="rounded-2xl border border-default bg-default overflow-x-auto"
      :aria-busy="loading"
    >
      <DataTable
        :loading="loading"
        label="模型用量"
        table-class="min-w-[850px]"
      >
        <thead>
          <tr>
            <th scope="col">群聊 / 模型</th>
            <th scope="col">请求</th>
            <th scope="col">输入 / 输出 tokens</th>
            <th scope="col">命中 / 未命中 tokens</th>
            <th scope="col">命中率</th>
            <th scope="col">平均耗时</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="`${row.group_id}:${row.model}:${row.profile_id}:${row.feature}`"
            class="border-[var(--border)]"
          >
            <td>
              <div>{{ groupName(row.group_id) }}</div>
              <div class="text-xs text-dimmed">{{ row.model }}</div>
              <div class="text-xs text-muted">
                {{ row.feature || '历史记录' }} ·
                {{ row.profile_id || '旧配置' }}
              </div>
            </td>
            <td>{{ count(row.requests) }}</td>
            <td>
              {{ count(row.prompt_tokens) }} /
              {{ count(row.completion_tokens) }}
            </td>
            <td>
              {{
                row.cache_reported_requests
                  ? `${count(row.cached_tokens)} / ${count(row.uncached_tokens)}`
                  : '未提供'
              }}
              <div class="text-xs text-dimmed">
                覆盖 {{ row.cache_reported_requests }} / {{ row.requests }} 次
              </div>
            </td>
            <td>{{ percent(row.cache_hit_rate) }}</td>
            <td>
              {{
                row.requests
                  ? (row.duration_ms / row.requests / 1000).toFixed(2)
                  : '0'
              }}
              秒
            </td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="6" class="text-center text-dimmed">
              {{
                loading
                  ? '正在加载…'
                  : error
                    ? '统计暂不可用'
                    : '此范围暂无模型用量记录'
              }}
            </td>
          </tr>
        </tbody>
      </DataTable>
    </div>
  </section>
</template>
