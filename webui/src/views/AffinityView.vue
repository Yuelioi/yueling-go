<script setup lang="ts">
import FilterBar from '../components/FilterBar.vue'
import DataTable from '../components/DataTable.vue'
import EmptyState from '../components/EmptyState.vue'
import AffinityTiersPanel from '../features/affinity/components/AffinityTiersPanel.vue'
import AffinityScoreDialog from '../features/affinity/components/AffinityScoreDialog.vue'
import { onMounted } from 'vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'
import { useAffinity } from '../features/affinity/useAffinity'

const state = useAffinity()
const {
  q,
  rows,
  blockBelow,
  loading,
  error,
  groupKey,
  groupOptions,
  lowScoreCount,
  selectedGroupLabel,
  loadGroups,
  loadRows,
} = state.records
const { loadTierSettings } = state.tiers
const { saving, saved, scoreColor, openScoreModal, adjust, reset } =
  state.scores

onMounted(async () => {
  await Promise.all([loadGroups(), loadRows(), loadTierSettings()])
})
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="关系引擎"
      description="查看月灵与群成员的关系状态。低于静默阈值后，AI 会主动减少回应。"
      icon="i-tabler-heart-handshake"
    >
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="loadRows"
      >
        刷新
      </UButton>
    </PageHeader>

    <div class="grid grid-cols-3 gap-3 sm:gap-4">
      <MetricCard
        label="当前范围"
        :value="selectedGroupLabel"
        detail="筛选作用域"
        icon="i-tabler-filter"
        tone="violet"
      />
      <MetricCard
        label="关系记录"
        :value="rows.length"
        detail="当前查询结果"
        icon="i-tabler-database-heart"
        tone="cyan"
      />
      <MetricCard
        label="静默阈值"
        :value="blockBelow"
        :detail="`${lowScoreCount} 人低于阈值`"
        icon="i-tabler-heart-off"
        tone="rose"
      />
    </div>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <AffinityTiersPanel :tiers="state.tiers" />

    <FilterBar title="筛选关系记录" description="按群聊、QQ 或昵称查找">
      <USelect
        v-model="groupKey"
        class="w-64"
        :items="groupOptions"
        value-key="value"
        aria-label="按群筛选关系记录"
        placeholder="全部群"
      />
      <UInput
        v-model="q"
        class="w-64"
        icon="i-tabler-search"
        aria-label="按 QQ 或昵称筛选"
        placeholder="QQ 或昵称"
        @keyup.enter="loadRows"
      />
      <UButton icon="i-tabler-search" :loading="loading" @click="loadRows">
        查询
      </UButton>
    </FilterBar>

    <div class="rounded-2xl border border-default bg-default overflow-hidden">
      <div class="overflow-x-auto">
        <DataTable
          :loading="loading"
          label="关系记录"
          table-class="min-w-[760px]"
        >
          <thead>
            <tr>
              <th scope="col">群</th>
              <th scope="col">QQ</th>
              <th scope="col">昵称</th>
              <th scope="col">分数</th>
              <th scope="col">最近原因</th>
              <th scope="col">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.ID">
              <td class="text-muted">{{ row.GroupID }}</td>
              <td class="font-mono text-xs text-toned">
                {{ row.UserID }}
              </td>
              <td class="text-highlighted">{{ row.Nickname || '-' }}</td>
              <td>
                <UBadge :color="scoreColor(row.Score)" variant="subtle">
                  {{ row.Score }}
                </UBadge>
              </td>
              <td class="max-w-[360px] text-muted">
                <span class="line-clamp-2">{{ row.LastReason || '-' }}</span>
              </td>
              <td>
                <div class="flex flex-wrap gap-1">
                  <UButton
                    size="xs"
                    color="warning"
                    variant="soft"
                    icon="i-tabler-minus"
                    :loading="saving[row.ID]"
                    @click="adjust(row, -5)"
                  >
                    -5
                  </UButton>
                  <UButton
                    size="xs"
                    color="success"
                    variant="soft"
                    icon="i-tabler-plus"
                    :loading="saving[row.ID]"
                    @click="adjust(row, 5)"
                  >
                    +5
                  </UButton>
                  <UButton
                    size="xs"
                    color="primary"
                    variant="soft"
                    icon="i-tabler-pencil"
                    :loading="saving[row.ID]"
                    @click="openScoreModal(row)"
                  >
                    设置
                  </UButton>
                  <MoreActions
                    label="关系记录更多操作"
                    :disabled="saving[row.ID]"
                    :items="[
                      {
                        label: '重置好感度',
                        description: '恢复系统默认分数',
                        icon: 'i-tabler-rotate',
                        color: 'warning',
                        onSelect: () => reset(row),
                      },
                    ]"
                  />
                  <span
                    v-if="saved[row.ID]"
                    class="inline-flex items-center gap-1.5 text-success self-center text-xs"
                  >
                    <UIcon name="i-tabler-check" class="size-3.5" />
                    已保存
                  </span>
                </div>
              </td>
            </tr>
          </tbody>
        </DataTable>
      </div>
      <EmptyState
        v-if="!loading && rows.length === 0"
        title="没有记录"
        description="换个群或关键词再查"
        icon="i-tabler-database-off"
      />
    </div>

    <AffinityScoreDialog :tiers="state.tiers" :scores="state.scores" />
  </section>
</template>
