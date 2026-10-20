import { ref, type Ref, type ComputedRef } from 'vue'
import { useTimeoutFn } from '@vueuse/core'
import type { AffinityRow } from '../../api/affinity.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
import type { AffinityClient } from './client.ts'
export function useAffinityScores(
  client: Pick<
    AffinityClient,
    'setAffinityScore' | 'adjustAffinity' | 'resetAffinity'
  >,
  context: {
    groupID: Ref<number | null>
    q: Ref<string>
    blockBelow: Ref<number>
    error: Ref<string>
    scoreMin: ComputedRef<number>
    scoreMax: ComputedRef<number>
  },
) {
  const { groupID, q, blockBelow, error, scoreMin, scoreMax } = context
  const requests = useRequestScope()
  const { start: clearSaved } = useTimeoutFn(
    () => {
      saved.value = {}
    },
    1200,
    { immediate: false },
  )
  const saving = ref<Record<number, boolean>>({})

  const saved = ref<Record<number, boolean>>({})

  const scoreModalOpen = ref(false)

  const editingRow = ref<AffinityRow | null>(null)

  const scoreInput = ref('')

  const scoreError = ref('')

  type BadgeColor = 'primary' | 'neutral' | 'success' | 'warning' | 'error'

  function scoreColor(score: number): BadgeColor {
    if (score < blockBelow.value) {
      return 'error'
    }
    if (score < 40) {
      return 'warning'
    }
    if (score >= 80) {
      return 'success'
    }
    return 'primary'
  }

  function markSaved(id: number) {
    saved.value[id] = true
    clearSaved()
  }

  function openScoreModal(row: AffinityRow) {
    editingRow.value = row
    scoreInput.value = String(row.Score)
    scoreError.value = ''
    scoreModalOpen.value = true
  }

  function closeScoreModal() {
    scoreModalOpen.value = false
    scoreError.value = ''
  }

  async function saveScore() {
    const alive = requests.begin('saveScore')
    const scope = String(groupID.value) + '/' + q.value
    const current = () =>
      alive() && scope === String(groupID.value) + '/' + q.value

    const row = editingRow.value
    if (!row) {
      return
    }
    const score = Number(scoreInput.value)
    if (!Number.isFinite(score)) {
      scoreError.value = '分数必须是数字'
      return
    }
    if (score < scoreMin.value || score > scoreMax.value) {
      scoreError.value = `分数必须在 ${scoreMin.value}–${scoreMax.value} 之间`
      return
    }
    saving.value[row.ID] = true
    scoreError.value = ''
    error.value = ''
    try {
      const res = await client.setAffinityScore(row.ID, score)
      if (!current()) return

      Object.assign(row, res.affinity)
      scoreModalOpen.value = false
      markSaved(row.ID)
    } catch (err) {
      if (!current()) return

      scoreError.value = err instanceof Error ? err.message : '保存失败'
    } finally {
      if (!alive()) return

      saving.value[row.ID] = false
    }
  }

  async function adjust(row: AffinityRow, delta: number) {
    const alive = requests.begin('adjust' + String(row.ID))
    const scope = String(groupID.value) + '/' + q.value
    const current = () =>
      alive() && scope === String(groupID.value) + '/' + q.value

    saving.value[row.ID] = true
    error.value = ''
    try {
      const res = await client.adjustAffinity(row.ID, delta)
      if (!current()) return

      Object.assign(row, res.affinity)
      markSaved(row.ID)
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '保存失败'
    } finally {
      if (!alive()) return

      saving.value[row.ID] = false
    }
  }

  async function reset(row: AffinityRow) {
    const alive = requests.begin('reset' + String(row.ID))
    const scope = String(groupID.value) + '/' + q.value
    const current = () =>
      alive() && scope === String(groupID.value) + '/' + q.value

    saving.value[row.ID] = true
    error.value = ''
    try {
      const res = await client.resetAffinity(row.ID)
      if (!current()) return

      Object.assign(row, res.affinity)
      markSaved(row.ID)
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '重置失败'
    } finally {
      if (!alive()) return

      saving.value[row.ID] = false
    }
  }
  return {
    saving,
    saved,
    scoreModalOpen,
    editingRow,
    scoreInput,
    scoreError,
    scoreColor,
    markSaved,
    openScoreModal,
    closeScoreModal,
    saveScore,
    adjust,
    reset,
  }
}
