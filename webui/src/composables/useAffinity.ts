import { computed, ref } from 'vue'
import {
  api,
  type AffinityRow,
  type AffinitySettings,
  type AffinityTier,
  type GroupInfo,
} from '../api.ts'
import { useRequestScope } from './useRequestScope.ts'

export function useAffinity(
  client: Pick<
    typeof api,
    | 'adjustAffinity'
    | 'affinity'
    | 'affinitySettings'
    | 'groups'
    | 'resetAffinity'
    | 'resetAffinitySettings'
    | 'setAffinityScore'
    | 'setAffinitySettings'
  > = api,
) {
  const requests = useRequestScope()
  const groups = ref<GroupInfo[]>([])
  const groupID = ref<number | null>(null)
  const q = ref('')
  const rows = ref<AffinityRow[]>([])
  const blockBelow = ref(10)
  const loading = ref(false)
  const saving = ref<Record<number, boolean>>({})
  const saved = ref<Record<number, boolean>>({})
  const error = ref('')
  const scoreModalOpen = ref(false)
  const editingRow = ref<AffinityRow | null>(null)
  const scoreInput = ref('')
  const scoreError = ref('')
  const tierSettings = ref<AffinitySettings | null>(null)
  const tierDraft = ref<AffinityTier[]>([])
  const tiersLoading = ref(false)
  const tiersSaving = ref(false)
  const tiersResetting = ref(false)
  const tiersError = ref('')
  const tiersSuccess = ref('')

  const groupKey = computed({
    get: () => (groupID.value ? String(groupID.value) : 'all'),
    set: (value: string) => {
      groupID.value = value === 'all' ? null : Number(value)
    },
  })

  const groupOptions = computed(() => [
    { label: '全部群', value: 'all' },
    ...groups.value.map((group) => ({
      label: group.group_name || String(group.group_id),
      value: String(group.group_id),
    })),
  ])

  const lowScoreCount = computed(
    () => rows.value.filter((row) => row.Score < blockBelow.value).length,
  )

  const selectedGroupLabel = computed(() => {
    if (!groupID.value) {
      return '全部群'
    }
    const group = groups.value.find((item) => item.group_id === groupID.value)
    return group?.group_name || String(groupID.value)
  })

  const scoreMin = computed(() => tierSettings.value?.min_score ?? 0)
  const scoreMax = computed(() => tierSettings.value?.max_score ?? 100)
  const savedTiersJSON = computed(() =>
    JSON.stringify(tierSettings.value?.tiers ?? []),
  )
  const draftTiersJSON = computed(() => JSON.stringify(tierDraft.value))
  const tiersDirty = computed(
    () => draftTiersJSON.value !== savedTiersJSON.value,
  )
  const tierValidationError = computed(() => {
    const settings = tierSettings.value
    if (!settings) return ''
    if (tierDraft.value.length < 2) return '至少保留两个回复阶梯'
    if (tierDraft.value.length > settings.max_tiers)
      return `最多可设置 ${settings.max_tiers} 个阶梯`
    const thresholds = tierDraft.value
      .map((tier) => Number(tier.min_score))
      .sort((a, b) => a - b)
    if (thresholds.some((value) => !Number.isInteger(value)))
      return '起始分数必须是整数'
    if (thresholds[0] !== settings.min_score)
      return `最低阶梯必须从 ${settings.min_score} 分开始`
    if (
      thresholds.some(
        (value) => value < settings.min_score || value > settings.max_score,
      )
    ) {
      return `起始分数必须在 ${settings.min_score}–${settings.max_score} 之间`
    }
    if (new Set(thresholds).size !== thresholds.length)
      return '每个阶梯的起始分数不能重复'
    if (tierDraft.value.some((tier) => !tier.name.trim()))
      return '请填写每个阶梯的名称'
    if (tierDraft.value.some((tier) => !tier.prompt.trim()))
      return '请填写每个阶梯的独立回复提示词'
    if (
      tierDraft.value.some(
        (tier) => Array.from(tier.name.trim()).length > settings.max_name_chars,
      )
    ) {
      return `阶梯名称不能超过 ${settings.max_name_chars} 个字符`
    }
    if (
      tierDraft.value.some(
        (tier) =>
          Array.from(tier.prompt.trim()).length > settings.max_prompt_chars,
      )
    ) {
      return `单条提示词不能超过 ${settings.max_prompt_chars} 个字符`
    }
    return ''
  })

  function markSaved(id: number) {
    saved.value[id] = true
    window.setTimeout(() => {
      saved.value[id] = false
    }, 1200)
  }

  async function loadGroups() {
    const alive = requests.begin('loadGroups')
    const current = () => alive()

    try {
      const res = await client.groups()
      if (!current()) return

      groups.value = res.groups
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '群列表加载失败'
    }
  }

  async function loadRows() {
    const alive = requests.begin('loadRows')
    const scope = String(groupID.value) + '/' + q.value
    const current = () =>
      alive() && scope === String(groupID.value) + '/' + q.value

    loading.value = true
    error.value = ''
    try {
      const res = await client.affinity(groupID.value, q.value)
      if (!current()) return

      rows.value = res.affinity
      blockBelow.value = res.block_below
    } catch (err) {
      if (!current()) return

      error.value = err instanceof Error ? err.message : '加载失败'
    } finally {
      if (!alive()) return

      loading.value = false
    }
  }

  function applyTierSettings(settings: AffinitySettings) {
    tierSettings.value = settings
    tierDraft.value = settings.tiers.map((tier) => ({ ...tier }))
    blockBelow.value = settings.block_below
  }

  async function loadTierSettings() {
    const alive = requests.begin('loadTierSettings')
    const current = () => alive()

    tiersLoading.value = true
    tiersError.value = ''
    try {
      applyTierSettings(await client.affinitySettings())
      if (!current()) return
    } catch (err) {
      if (!current()) return

      tiersError.value = err instanceof Error ? err.message : '回复阶梯加载失败'
    } finally {
      if (!alive()) return

      tiersLoading.value = false
    }
  }

  function addTier() {
    const settings = tierSettings.value
    if (!settings || tierDraft.value.length >= settings.max_tiers) return
    const thresholds = tierDraft.value
      .map((tier) => Number(tier.min_score))
      .sort((a, b) => a - b)
    let bestStart = settings.min_score
    let bestEnd = settings.max_score
    let bestGap = -1
    for (let i = 0; i < thresholds.length - 1; i += 1) {
      const gap = thresholds[i + 1] - thresholds[i]
      if (gap > bestGap) {
        bestGap = gap
        bestStart = thresholds[i]
        bestEnd = thresholds[i + 1]
      }
    }
    if (bestGap <= 1) {
      const last = thresholds[thresholds.length - 1]
      if (last < settings.max_score) {
        bestStart = last
        bestEnd = settings.max_score + 1
      } else {
        tiersError.value = '当前分数范围内没有可插入的新阈值'
        return
      }
    }
    tierDraft.value.push({
      min_score: Math.floor((bestStart + bestEnd) / 2),
      name: '新关系',
      prompt: '',
    })
    tierDraft.value.sort((a, b) => a.min_score - b.min_score)
    tiersError.value = ''
    tiersSuccess.value = ''
  }

  function removeTier(index: number) {
    if (index === 0 || tierDraft.value.length <= 2) return
    tierDraft.value.splice(index, 1)
    tiersError.value = ''
    tiersSuccess.value = ''
  }

  async function saveTiers() {
    const alive = requests.begin('saveTiers')
    const current = () => alive()

    if (tierValidationError.value) {
      tiersError.value = tierValidationError.value
      return
    }
    tiersSaving.value = true
    tiersError.value = ''
    tiersSuccess.value = ''
    try {
      const tiers = tierDraft.value
        .map((tier) => ({
          min_score: Number(tier.min_score),
          name: tier.name.trim(),
          prompt: tier.prompt.trim(),
        }))
        .sort((a, b) => a.min_score - b.min_score)
      applyTierSettings(await client.setAffinitySettings(tiers))
      if (!current()) return

      tiersSuccess.value = '回复阶梯已保存，新的提示词会立即生效'
    } catch (err) {
      if (!current()) return

      tiersError.value = err instanceof Error ? err.message : '回复阶梯保存失败'
    } finally {
      if (!alive()) return

      tiersSaving.value = false
    }
  }

  async function resetTiers() {
    const alive = requests.begin('resetTiers')
    const current = () => alive()

    tiersResetting.value = true
    tiersError.value = ''
    tiersSuccess.value = ''
    try {
      applyTierSettings(await client.resetAffinitySettings())
      if (!current()) return

      tiersSuccess.value = '已恢复内置回复阶梯'
    } catch (err) {
      if (!current()) return

      tiersError.value = err instanceof Error ? err.message : '恢复默认失败'
    } finally {
      if (!alive()) return

      tiersResetting.value = false
    }
  }

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
    addTier,
    adjust,
    blockBelow,
    closeScoreModal,
    editingRow,
    error,
    groupKey,
    groupOptions,
    loadGroups,
    loadRows,
    loadTierSettings,
    loading,
    lowScoreCount,
    openScoreModal,
    q,
    removeTier,
    reset,
    resetTiers,
    rows,
    saveScore,
    saveTiers,
    saved,
    saving,
    scoreColor,
    scoreError,
    scoreInput,
    scoreMax,
    scoreMin,
    scoreModalOpen,
    selectedGroupLabel,
    tierDraft,
    tierSettings,
    tierValidationError,
    tiersDirty,
    tiersError,
    tiersLoading,
    tiersResetting,
    tiersSaving,
    tiersSuccess,
  }
}
