import { computed, ref } from 'vue'
import type { GroupInfo } from '../../api/groups.ts'
import type { AffinityRow } from '../../api/affinity.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
import { affinityClient, type AffinityClient } from './client.ts'
import { useAffinityTiers } from './tiers.ts'
import { useAffinityScores } from './scores.ts'
export function useAffinity(client: AffinityClient = affinityClient) {
  const requests = useRequestScope()
  const groups = ref<GroupInfo[]>([])

  const groupID = ref<number | null>(null)

  const q = ref('')

  const rows = ref<AffinityRow[]>([])

  const blockBelow = ref(10)

  const loading = ref(false)

  const error = ref('')

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
      const res = await client.affinity(groupID.value, q.value, {
        signal: alive.signal,
      })
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
  const tiers = useAffinityTiers(client, blockBelow)
  const scores = useAffinityScores(client, {
    groupID,
    q,
    blockBelow,
    error,
    scoreMin: tiers.scoreMin,
    scoreMax: tiers.scoreMax,
  })
  return {
    records: {
      groups,
      groupID,
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
    },
    tiers,
    scores,
  }
}
