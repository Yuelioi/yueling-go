<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type AffinityRow, type AffinitySettings, type AffinityTier, type GroupInfo } from '../api'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

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

const lowScoreCount = computed(() =>
  rows.value.filter((row) => row.Score < blockBelow.value).length,
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
const savedTiersJSON = computed(() => JSON.stringify(tierSettings.value?.tiers ?? []))
const draftTiersJSON = computed(() => JSON.stringify(tierDraft.value))
const tiersDirty = computed(() => draftTiersJSON.value !== savedTiersJSON.value)
const tierValidationError = computed(() => {
  const settings = tierSettings.value
  if (!settings) return ''
  if (tierDraft.value.length < 2) return '至少保留两个回复阶梯'
  if (tierDraft.value.length > settings.max_tiers) return `最多可设置 ${settings.max_tiers} 个阶梯`
  const thresholds = tierDraft.value.map((tier) => Number(tier.min_score)).sort((a, b) => a - b)
  if (thresholds.some((value) => !Number.isInteger(value))) return '起始分数必须是整数'
  if (thresholds[0] !== settings.min_score) return `最低阶梯必须从 ${settings.min_score} 分开始`
  if (thresholds.some((value) => value < settings.min_score || value > settings.max_score)) {
    return `起始分数必须在 ${settings.min_score}–${settings.max_score} 之间`
  }
  if (new Set(thresholds).size !== thresholds.length) return '每个阶梯的起始分数不能重复'
  if (tierDraft.value.some((tier) => !tier.name.trim())) return '请填写每个阶梯的名称'
  if (tierDraft.value.some((tier) => !tier.prompt.trim())) return '请填写每个阶梯的独立回复提示词'
  if (tierDraft.value.some((tier) => Array.from(tier.name.trim()).length > settings.max_name_chars)) {
    return `阶梯名称不能超过 ${settings.max_name_chars} 个字符`
  }
  if (tierDraft.value.some((tier) => Array.from(tier.prompt.trim()).length > settings.max_prompt_chars)) {
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
  try {
    const res = await api.groups()
    groups.value = res.groups
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群列表加载失败'
  }
}

async function loadRows() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.affinity(groupID.value, q.value)
    rows.value = res.affinity
    blockBelow.value = res.block_below
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function applyTierSettings(settings: AffinitySettings) {
  tierSettings.value = settings
  tierDraft.value = settings.tiers.map((tier) => ({ ...tier }))
  blockBelow.value = settings.block_below
}

async function loadTierSettings() {
  tiersLoading.value = true
  tiersError.value = ''
  try {
    applyTierSettings(await api.affinitySettings())
  } catch (err) {
    tiersError.value = err instanceof Error ? err.message : '回复阶梯加载失败'
  } finally {
    tiersLoading.value = false
  }
}

function addTier() {
  const settings = tierSettings.value
  if (!settings || tierDraft.value.length >= settings.max_tiers) return
  const thresholds = tierDraft.value.map((tier) => Number(tier.min_score)).sort((a, b) => a - b)
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
  if (tierValidationError.value) {
    tiersError.value = tierValidationError.value
    return
  }
  tiersSaving.value = true
  tiersError.value = ''
  tiersSuccess.value = ''
  try {
    const tiers = tierDraft.value
      .map((tier) => ({ min_score: Number(tier.min_score), name: tier.name.trim(), prompt: tier.prompt.trim() }))
      .sort((a, b) => a.min_score - b.min_score)
    applyTierSettings(await api.setAffinitySettings(tiers))
    tiersSuccess.value = '回复阶梯已保存，新的提示词会立即生效'
  } catch (err) {
    tiersError.value = err instanceof Error ? err.message : '回复阶梯保存失败'
  } finally {
    tiersSaving.value = false
  }
}

async function resetTiers() {
  tiersResetting.value = true
  tiersError.value = ''
  tiersSuccess.value = ''
  try {
    applyTierSettings(await api.resetAffinitySettings())
    tiersSuccess.value = '已恢复内置回复阶梯'
  } catch (err) {
    tiersError.value = err instanceof Error ? err.message : '恢复默认失败'
  } finally {
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
    const res = await api.setAffinityScore(row.ID, score)
    Object.assign(row, res.affinity)
    scoreModalOpen.value = false
    markSaved(row.ID)
  } catch (err) {
    scoreError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value[row.ID] = false
  }
}

async function adjust(row: AffinityRow, delta: number) {
  saving.value[row.ID] = true
  error.value = ''
  try {
    const res = await api.adjustAffinity(row.ID, delta)
    Object.assign(row, res.affinity)
    markSaved(row.ID)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value[row.ID] = false
  }
}

async function reset(row: AffinityRow) {
  saving.value[row.ID] = true
  error.value = ''
  try {
    const res = await api.resetAffinity(row.ID)
    Object.assign(row, res.affinity)
    markSaved(row.ID)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '重置失败'
  } finally {
    saving.value[row.ID] = false
  }
}

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
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :loading="loading" @click="loadRows">
        刷新
      </UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="当前范围" :value="selectedGroupLabel" detail="筛选作用域" icon="i-tabler-filter" tone="violet" />
      <MetricCard label="关系记录" :value="rows.length" detail="当前查询结果" icon="i-tabler-database-heart" tone="cyan" />
      <MetricCard label="静默阈值" :value="blockBelow" :detail="`${lowScoreCount} 人低于阈值`" icon="i-tabler-heart-off" tone="rose" />
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
      <div class="panel-header affinity-tier-header">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <div class="section-title">回复阶梯</div>
            <UBadge :color="tierSettings?.custom ? 'primary' : 'neutral'" variant="subtle">
              {{ tierSettings?.custom ? '自定义生效中' : '内置默认' }}
            </UBadge>
          </div>
          <div class="section-caption">达到起始分数后，AI 会使用该级独立提示词；更高阶梯优先。</div>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <UButton
            color="neutral"
            variant="soft"
            icon="i-tabler-plus"
            :disabled="tiersLoading || !tierSettings || tierDraft.length >= (tierSettings?.max_tiers ?? 0)"
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
              { label: '恢复内置阶梯', description: '移除全部自定义阈值和提示词', icon: 'i-tabler-restore', color: 'warning', loading: tiersResetting, onSelect: resetTiers },
            ]"
          />
          <UButton
            icon="i-tabler-device-floppy"
            :loading="tiersSaving"
            :disabled="tiersLoading || !tiersDirty || Boolean(tierValidationError)"
            @click="saveTiers"
          >
            保存阶梯
          </UButton>
        </div>
      </div>

      <div v-if="tiersLoading" class="flex items-center gap-2 p-5 text-sm text-zinc-400">
        <UIcon name="i-tabler-loader-2" class="size-4 animate-spin" />
        正在读取回复阶梯…
      </div>
      <div v-else class="affinity-tier-list">
        <div v-for="(tier, index) in tierDraft" :key="index" class="affinity-tier-row">
          <div class="affinity-tier-order" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</div>
          <UFormField label="起始分数" :description="index === 0 ? '覆盖最低分' : '达到后切换'">
            <UInput
              v-model.number="tier.min_score"
              type="number"
              :min="scoreMin"
              :max="scoreMax"
              :disabled="index === 0"
            />
          </UFormField>
          <UFormField label="关系名称">
            <UInput v-model="tier.name" class="w-full" :maxlength="tierSettings?.max_name_chars" placeholder="例如：信赖" />
          </UFormField>
          <UFormField label="独立回复提示词" :description="`${Array.from(tier.prompt).length}/${tierSettings?.max_prompt_chars ?? 2000}`">
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
            color="neutral"
            variant="ghost"
            icon="i-tabler-trash"
            aria-label="删除阶梯"
            :disabled="index === 0 || tierDraft.length <= 2"
            @click="removeTier(index)"
          />
        </div>
      </div>

      <div v-if="tiersError || tierValidationError || tiersSuccess" class="affinity-tier-feedback">
        <span v-if="tiersError || tierValidationError" class="text-rose-300">
          <UIcon name="i-tabler-alert-circle" class="size-4" />
          {{ tiersError || tierValidationError }}
        </span>
        <span v-else class="text-emerald-300">
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
          <thead class="border-b border-white/5 bg-black/10 text-xs uppercase text-zinc-500">
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
              <td class="px-4 py-3 font-mono text-xs text-zinc-300">{{ row.UserID }}</td>
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
                      { label: '重置好感度', description: '恢复系统默认分数', icon: 'i-tabler-rotate', color: 'warning', onSelect: () => reset(row) },
                    ]"
                  />
                  <span v-if="saved[row.ID]" class="inline-status self-center text-xs">
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
        content: 'affinity-score-dialog z-50 text-zinc-100 divide-zinc-800',
        header: 'border-b border-zinc-800',
        body: 'bg-zinc-900',
        footer: 'border-t border-zinc-800 bg-zinc-900',
        title: 'text-white',
        description: 'text-zinc-400'
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
              :ui="{ base: 'affinity-score-input' }"
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
