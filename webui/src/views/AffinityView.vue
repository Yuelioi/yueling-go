<script setup lang="ts">
import { onMounted } from 'vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'
import { useAffinity } from '../composables/useAffinity'

const {
  q,
  rows,
  blockBelow,
  loading,
  saving,
  saved,
  error,
  scoreModalOpen,
  editingRow,
  scoreInput,
  scoreError,
  tierSettings,
  tierDraft,
  tiersLoading,
  tiersSaving,
  tiersResetting,
  tiersError,
  tiersSuccess,
  groupKey,
  groupOptions,
  lowScoreCount,
  selectedGroupLabel,
  scoreMin,
  scoreMax,
  tiersDirty,
  tierValidationError,
  loadGroups,
  loadRows,
  loadTierSettings,
  addTier,
  removeTier,
  saveTiers,
  resetTiers,
  scoreColor,
  openScoreModal,
  closeScoreModal,
  saveScore,
  adjust,
  reset,
} = useAffinity()

onMounted(async () => {
  await Promise.all([loadGroups(), loadRows(), loadTierSettings()])
})
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Relationship engine"
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

    <div class="grid grid-cols-3 gap-3 max-[860px]:grid-cols-1">
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
      class="error-banner"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <section class="surface-panel overflow-hidden">
      <div
        class="panel-header !items-start !gap-[18px] max-[860px]:!flex-col max-[860px]:!items-stretch"
      >
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <div class="section-title">回复阶梯</div>
            <UBadge
              :color="tierSettings?.custom ? 'primary' : 'neutral'"
              variant="subtle"
            >
              {{ tierSettings?.custom ? '自定义生效中' : '内置默认' }}
            </UBadge>
          </div>
          <div class="section-caption">
            达到起始分数后，AI 会使用该级独立提示词；更高阶梯优先。
          </div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UButton
            color="neutral"
            variant="soft"
            icon="i-tabler-plus"
            :disabled="
              tiersLoading ||
              !tierSettings ||
              tierDraft.length >= (tierSettings?.max_tiers ?? 0)
            "
            @click="addTier"
          >
            添加阶梯
          </UButton>
          <MoreActions
            v-if="tierSettings?.custom"
            label="回复阶梯更多操作"
            show-label
            :disabled="tiersResetting"
            :items="[
              {
                label: '恢复内置阶梯',
                description: '移除全部自定义阈值和提示词',
                icon: 'i-tabler-restore',
                color: 'warning',
                loading: tiersResetting,
                onSelect: resetTiers,
              },
            ]"
          />
          <UButton
            icon="i-tabler-device-floppy"
            :loading="tiersSaving"
            :disabled="
              tiersLoading || !tiersDirty || Boolean(tierValidationError)
            "
            @click="saveTiers"
          >
            保存阶梯
          </UButton>
        </div>
      </div>

      <div
        v-if="tiersLoading"
        class="flex items-center gap-2 p-5 text-sm text-zinc-400"
      >
        <UIcon name="i-tabler-loader-2" class="size-4 animate-spin" />
        正在读取回复阶梯…
      </div>
      <div v-else class="px-5 max-[520px]:px-4">
        <div
          v-for="(tier, index) in tierDraft"
          :key="index"
          class="grid grid-cols-[32px_minmax(112px,0.35fr)_minmax(150px,0.55fr)_minmax(320px,2fr)_34px] items-start gap-3.5 border-b border-white/[0.055] py-[17px] last:border-b-0 max-[1060px]:grid-cols-[28px_minmax(108px,0.4fr)_minmax(140px,0.55fr)_minmax(240px,1.5fr)_34px] max-[1060px]:gap-2.5 max-[860px]:grid-cols-[24px_minmax(0,0.7fr)_minmax(0,1fr)_34px] max-[520px]:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)_34px] max-[520px]:gap-x-2.5 max-[520px]:gap-y-3"
        >
          <div
            class="pt-[27px] text-[0.65rem] tabular-nums tracking-[0.08em] text-[var(--dim)] max-[520px]:hidden"
            aria-hidden="true"
          >
            {{ String(index + 1).padStart(2, '0') }}
          </div>
          <UFormField
            label="起始分数"
            :description="index === 0 ? '覆盖最低分' : '达到后切换'"
          >
            <UInput
              v-model.number="tier.min_score"
              type="number"
              :min="scoreMin"
              :max="scoreMax"
              :disabled="index === 0"
            />
          </UFormField>
          <UFormField label="关系名称">
            <UInput
              v-model="tier.name"
              class="w-full"
              :maxlength="tierSettings?.max_name_chars"
              placeholder="例如：信赖"
            />
          </UFormField>
          <UFormField
            class="max-[860px]:col-[2/-1] max-[520px]:col-[1/-1]"
            label="独立回复提示词"
            :description="`${Array.from(tier.prompt).length}/${tierSettings?.max_prompt_chars ?? 2000}`"
          >
            <UTextarea
              v-model="tier.prompt"
              class="w-full"
              :ui="{ root: 'w-full' }"
              :rows="2"
              autoresize
              :maxlength="tierSettings?.max_prompt_chars"
              placeholder="描述这个关系阶段下的语气、称呼和互动边界"
            />
          </UFormField>
          <UButton
            class="mt-[23px] max-[860px]:col-start-4 max-[860px]:row-start-1 max-[520px]:col-start-3"
            color="neutral"
            variant="ghost"
            icon="i-tabler-trash"
            aria-label="删除阶梯"
            :disabled="index === 0 || tierDraft.length <= 2"
            @click="removeTier(index)"
          />
        </div>
      </div>

      <div
        v-if="tiersError || tierValidationError || tiersSuccess"
        class="flex min-h-[42px] items-center border-t border-white/[0.055] bg-black/[0.08] px-5 py-2.5 text-[0.72rem]"
      >
        <span
          v-if="tiersError || tierValidationError"
          class="inline-flex items-center gap-1.5 text-rose-300"
        >
          <UIcon name="i-tabler-alert-circle" class="size-4" />
          {{ tiersError || tierValidationError }}
        </span>
        <span v-else class="inline-flex items-center gap-1.5 text-emerald-300">
          <UIcon name="i-tabler-circle-check" class="size-4" />
          {{ tiersSuccess }}
        </span>
      </div>
    </section>

    <div class="surface-panel flex flex-wrap items-end gap-3 p-4">
      <div class="mr-auto min-w-[180px]">
        <div class="section-title">筛选关系记录</div>
        <div class="section-caption">按群聊、QQ 或昵称查找</div>
      </div>
      <USelect
        v-model="groupKey"
        class="w-64"
        :items="groupOptions"
        value-key="value"
        placeholder="全部群"
      />
      <UInput
        v-model="q"
        class="w-64"
        icon="i-tabler-search"
        placeholder="QQ 或昵称"
        @keyup.enter="loadRows"
      />
      <UButton icon="i-tabler-search" :loading="loading" @click="loadRows">
        查询
      </UButton>
    </div>

    <div class="surface-panel overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm">
          <thead
            class="border-b border-white/5 bg-black/10 text-xs uppercase text-zinc-500"
          >
            <tr>
              <th class="px-4 py-3 font-medium">群</th>
              <th class="px-4 py-3 font-medium">QQ</th>
              <th class="px-4 py-3 font-medium">昵称</th>
              <th class="px-4 py-3 font-medium">分数</th>
              <th class="px-4 py-3 font-medium">最近原因</th>
              <th class="px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.ID" class="data-row">
              <td class="px-4 py-3 text-zinc-400">{{ row.GroupID }}</td>
              <td class="px-4 py-3 font-mono text-xs text-zinc-300">
                {{ row.UserID }}
              </td>
              <td class="px-4 py-3 text-white">{{ row.Nickname || '-' }}</td>
              <td class="px-4 py-3">
                <UBadge :color="scoreColor(row.Score)" variant="subtle">
                  {{ row.Score }}
                </UBadge>
              </td>
              <td class="max-w-[360px] px-4 py-3 text-zinc-400">
                <span class="line-clamp-2">{{ row.LastReason || '-' }}</span>
              </td>
              <td class="px-4 py-3">
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
                    class="inline-status self-center text-xs"
                  >
                    <UIcon name="i-tabler-check" class="size-3.5" />
                    已保存
                  </span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-if="!loading && rows.length === 0"
        class="empty-state border-t border-zinc-800/80 py-12"
      >
        <UIcon name="i-tabler-database-off" class="size-7 text-zinc-500" />
        <div class="font-medium text-zinc-200">没有记录</div>
        <div class="text-sm text-zinc-500">换个群或关键词再查</div>
      </div>
    </div>

    <UModal
      v-model:open="scoreModalOpen"
      title="设置好感度"
      description="保存后会立即影响该用户在对应群里的 AI 回复态度。"
      :ui="{
        overlay: 'z-40 bg-black/70 backdrop-blur-sm',
        content:
          'z-50 divide-zinc-800 text-zinc-100 [&&]:!border [&&]:!border-[var(--border-strong)] [&&]:!bg-[#19182c] [&&]:!shadow-[0_28px_80px_rgba(0,0,0,0.52),inset_0_1px_rgba(255,255,255,0.035)]',
        header: 'border-b border-zinc-800',
        body: 'bg-zinc-900',
        footer: 'border-t border-zinc-800 bg-zinc-900',
        title: 'text-white',
        description: 'text-zinc-400',
      }"
    >
      <template #body>
        <div class="space-y-4">
          <div v-if="editingRow" class="surface-inset p-3 text-sm">
            <div class="flex items-center justify-between gap-3">
              <div class="min-w-0">
                <div class="truncate font-medium text-white">
                  {{ editingRow.Nickname || editingRow.UserID }}
                </div>
                <div class="mt-1 text-xs text-zinc-500">
                  QQ {{ editingRow.UserID }} · 群 {{ editingRow.GroupID }}
                </div>
              </div>
              <UBadge :color="scoreColor(editingRow.Score)" variant="subtle">
                当前 {{ editingRow.Score }}
              </UBadge>
            </div>
          </div>

          <UFormField label="新分数" :error="scoreError || undefined">
            <UInput
              v-model="scoreInput"
              type="number"
              autofocus
              icon="i-tabler-heart-cog"
              class="w-full"
              :min="scoreMin"
              :max="scoreMax"
              :placeholder="`${scoreMin} - ${scoreMax}`"
              :ui="{
                base: '[&&]:!border-0 [&&]:!bg-[rgba(13,12,25,0.82)] [&&]:!text-[var(--ink)] [&&]:!shadow-[inset_0_0_0_1px_var(--border-strong)] [&&]:caret-[var(--violet-bright)] [&&]:[color-scheme:dark] [&&]:hover:!shadow-[inset_0_0_0_1px_rgba(185,175,230,0.34)] [&&]:focus:!shadow-[inset_0_0_0_1px_var(--violet),0_0_0_3px_var(--violet-soft)]',
              }"
              @keyup.enter="saveScore"
            />
          </UFormField>
        </div>
      </template>

      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            class="!text-zinc-300 hover:!bg-zinc-800/80"
            @click="closeScoreModal"
          >
            取消
          </UButton>
          <UButton
            icon="i-tabler-device-floppy"
            :loading="editingRow ? saving[editingRow.ID] : false"
            @click="saveScore"
          >
            保存
          </UButton>
        </div>
      </template>
    </UModal>
  </section>
</template>
