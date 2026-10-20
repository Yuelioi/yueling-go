<script setup lang="ts">
import type { useAffinity } from '../useAffinity.ts'
const props = defineProps<{
  tiers: Pick<ReturnType<typeof useAffinity>['tiers'], 'scoreMin' | 'scoreMax'>
  scores: Pick<
    ReturnType<typeof useAffinity>['scores'],
    | 'saving'
    | 'scoreModalOpen'
    | 'editingRow'
    | 'scoreInput'
    | 'scoreError'
    | 'scoreColor'
    | 'closeScoreModal'
    | 'saveScore'
  >
}>()
const { scoreMin, scoreMax } = props.tiers
const {
  saving,
  scoreModalOpen,
  editingRow,
  scoreInput,
  scoreError,
  scoreColor,
  closeScoreModal,
  saveScore,
} = props.scores
</script>
<template>
  <UModal
    v-model:open="scoreModalOpen"
    title="设置好感度"
    description="保存后会立即影响该用户在对应群里的 AI 回复态度。"
  >
    <template #body>
      <div class="space-y-4">
        <div
          v-if="editingRow"
          class="rounded-lg border border-default bg-muted p-3 text-sm"
        >
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="truncate font-medium text-highlighted">
                {{ editingRow.Nickname || editingRow.UserID }}
              </div>
              <div class="mt-1 text-xs text-muted">
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
            @keyup.enter="saveScore"
          />
        </UFormField>
      </div>
    </template>

    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="closeScoreModal">
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
</template>
