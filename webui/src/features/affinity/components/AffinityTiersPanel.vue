<script setup lang="ts">
import { ref, useId } from 'vue'
import type { useAffinity } from '../useAffinity.ts'
import MoreActions from '../../../components/MoreActions.vue'
const expanded = ref(false)
const panelId = useId()
const props = defineProps<{
  tiers: Pick<
    ReturnType<typeof useAffinity>['tiers'],
    | 'tierSettings'
    | 'tierDraft'
    | 'tiersLoading'
    | 'tiersSaving'
    | 'tiersResetting'
    | 'tiersError'
    | 'tiersSuccess'
    | 'scoreMin'
    | 'scoreMax'
    | 'tiersDirty'
    | 'tierValidationError'
    | 'addTier'
    | 'removeTier'
    | 'saveTiers'
    | 'resetTiers'
  >
}>()
const {
  tierSettings,
  tierDraft,
  tiersLoading,
  tiersSaving,
  tiersResetting,
  tiersError,
  tiersSuccess,
  scoreMin,
  scoreMax,
  tiersDirty,
  tierValidationError,
  addTier,
  removeTier,
  saveTiers,
  resetTiers,
} = props.tiers
</script>
<template>
  <section
    class="@container rounded-2xl border border-default bg-default overflow-hidden"
  >
    <div
      class="flex flex-col items-stretch justify-between gap-4 px-5 py-4 sm:flex-row sm:items-start"
    >
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-sm font-semibold text-highlighted">回复阶梯</h2>
          <UBadge
            :color="tierSettings?.custom ? 'primary' : 'neutral'"
            variant="subtle"
          >
            {{ tierSettings?.custom ? '自定义生效中' : '内置默认' }}
          </UBadge>
        </div>
        <UBadge v-if="tiersDirty" class="mt-2" color="warning" variant="soft"
          >未保存</UBadge
        >
        <div class="mt-1 text-xs leading-5 text-muted">
          达到起始分数后，AI 会使用该级独立提示词；更高阶梯优先。
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2 sm:shrink-0">
        <UButton
          color="neutral"
          variant="soft"
          :icon="expanded ? 'i-tabler-chevron-up' : 'i-tabler-adjustments'"
          :aria-expanded="expanded"
          :aria-controls="panelId"
          @click="expanded = !expanded"
          >{{ expanded ? '收起' : '编辑阶梯' }}</UButton
        >
        <template v-if="expanded">
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
        </template>
      </div>
    </div>

    <div :id="panelId" v-show="expanded" class="border-t border-default">
      <div
        v-if="tiersLoading"
        class="flex items-center gap-2 p-5 text-sm text-muted"
      >
        <UIcon name="i-tabler-loader-2" class="size-4 animate-spin" />
        正在读取回复阶梯…
      </div>
      <div v-else class="px-5 max-[520px]:px-4">
        <div
          v-for="(tier, index) in tierDraft"
          :key="index"
          class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-start gap-4 border-b border-default py-5 last:border-b-0 @min-[52rem]:grid-cols-[1.5rem_7rem_9rem_minmax(0,1fr)_2.5rem]"
        >
          <div
            class="hidden pt-8 text-xs tabular-nums text-dimmed @min-[52rem]:block"
            aria-hidden="true"
          >
            {{ String(index + 1).padStart(2, '0') }}
          </div>
          <UFormField
            label="起始分数"
            :description="index === 0 ? '覆盖最低分' : '达到后切换'"
          >
            <UInput
              class="w-full"
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
            class="col-span-full min-w-0 @min-[52rem]:col-auto"
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
            class="col-start-3 row-start-1 mt-7 @min-[52rem]:col-auto @min-[52rem]:row-auto"
            color="neutral"
            variant="ghost"
            icon="i-tabler-trash"
            aria-label="删除阶梯"
            :disabled="index === 0 || tierDraft.length <= 2"
            @click="removeTier(index)"
          />
        </div>
      </div>
    </div>

    <div
      v-if="tiersError || tierValidationError || tiersSuccess"
      class="flex min-h-[42px] items-center border-t border-default bg-muted/40 px-5 py-2.5 text-xs"
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
</template>
