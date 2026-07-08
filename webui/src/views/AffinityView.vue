<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type AffinityRow, type GroupInfo } from '../api'

const groups = ref<GroupInfo[]>([])
const groupID = ref<number | null>(null)
const q = ref('')
const rows = ref<AffinityRow[]>([])
const blockBelow = ref(10)
const loading = ref(false)
const saving = ref<Record<number, boolean>>({})
const saved = ref<Record<number, boolean>>({})
const error = ref('')

const groupOptions = computed(() => [
  { label: '全部群', value: null },
  ...groups.value.map((group) => ({
    label: group.group_name || String(group.group_id),
    value: group.group_id,
  })),
])

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

async function setScore(row: AffinityRow) {
  const raw = window.prompt('设置分数', String(row.Score))
  if (raw === null) {
    return
  }
  const score = Number(raw)
  if (!Number.isFinite(score)) {
    error.value = '分数必须是数字'
    return
  }
  saving.value[row.ID] = true
  error.value = ''
  try {
    const res = await api.setAffinityScore(row.ID, score)
    Object.assign(row, res.affinity)
    markSaved(row.ID)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
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
  await loadGroups()
  await loadRows()
})
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-lg font-semibold">AI 好感度</h1>
      <p class="text-sm text-neutral-400">查看和修正隐藏好感度分数</p>
    </div>

    <UAlert
      v-if="error"
      color="error"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="flex flex-wrap gap-2">
      <USelect
        v-model="groupID"
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

    <div class="overflow-hidden rounded-lg border border-neutral-800 bg-neutral-900">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-left text-sm">
          <thead class="bg-neutral-900 text-neutral-400">
            <tr>
              <th class="px-3 py-2">群</th>
              <th class="px-3 py-2">QQ</th>
              <th class="px-3 py-2">昵称</th>
              <th class="px-3 py-2">分数</th>
              <th class="px-3 py-2">最近原因</th>
              <th class="px-3 py-2">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-neutral-800">
            <tr v-for="row in rows" :key="row.ID">
              <td class="px-3 py-2">{{ row.GroupID }}</td>
              <td class="px-3 py-2">{{ row.UserID }}</td>
              <td class="px-3 py-2">{{ row.Nickname || '-' }}</td>
              <td class="px-3 py-2">
                <UBadge :color="row.Score < blockBelow ? 'error' : 'neutral'">
                  {{ row.Score }}
                </UBadge>
              </td>
              <td class="px-3 py-2">{{ row.LastReason || '-' }}</td>
              <td class="px-3 py-2">
                <div class="flex flex-wrap gap-1">
                  <UButton
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    :loading="saving[row.ID]"
                    @click="adjust(row, -5)"
                  >
                    -5
                  </UButton>
                  <UButton
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    :loading="saving[row.ID]"
                    @click="adjust(row, 5)"
                  >
                    +5
                  </UButton>
                  <UButton
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    :loading="saving[row.ID]"
                    @click="setScore(row)"
                  >
                    设置
                  </UButton>
                  <UButton
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    :loading="saving[row.ID]"
                    @click="reset(row)"
                  >
                    重置
                  </UButton>
                  <span v-if="saved[row.ID]" class="self-center text-xs text-emerald-400">
                    已保存
                  </span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <UEmpty
        v-if="!loading && rows.length === 0"
        class="border-t border-neutral-800"
        icon="i-tabler-database-off"
        title="没有记录"
        description="换个群或关键词再查"
      />
    </div>
  </section>
</template>
