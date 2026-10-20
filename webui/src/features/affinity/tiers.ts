import { computed, ref, type Ref } from 'vue'
import type { AffinitySettings, AffinityTier } from '../../api/affinity.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
import type { AffinityClient } from './client.ts'
export function useAffinityTiers(
  client: Pick<
    AffinityClient,
    'affinitySettings' | 'setAffinitySettings' | 'resetAffinitySettings'
  >,
  blockBelow: Ref<number>,
) {
  const requests = useRequestScope()
  const tierSettings = ref<AffinitySettings | null>(null)

  const tierDraft = ref<AffinityTier[]>([])

  const tiersLoading = ref(false)

  const tiersSaving = ref(false)

  const tiersResetting = ref(false)

  const tiersError = ref('')

  const tiersSuccess = ref('')

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
      const response = await client.affinitySettings({ signal: alive.signal })
      if (!current()) return
      applyTierSettings(response)
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
      const response = await client.setAffinitySettings(tiers)
      if (!current()) return
      applyTierSettings(response)

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
      const response = await client.resetAffinitySettings()
      if (!current()) return
      applyTierSettings(response)

      tiersSuccess.value = '已恢复内置回复阶梯'
    } catch (err) {
      if (!current()) return

      tiersError.value = err instanceof Error ? err.message : '恢复默认失败'
    } finally {
      if (!alive()) return

      tiersResetting.value = false
    }
  }
  return {
    tierSettings,
    tierDraft,
    tiersLoading,
    tiersSaving,
    tiersResetting,
    tiersError,
    tiersSuccess,
    scoreMin,
    scoreMax,
    savedTiersJSON,
    draftTiersJSON,
    tiersDirty,
    tierValidationError,
    applyTierSettings,
    loadTierSettings,
    addTier,
    removeTier,
    saveTiers,
    resetTiers,
  }
}
