<script setup lang="ts">
import FilterBar from '../components/FilterBar.vue'
import DataTable from '../components/DataTable.vue'
import EmptyState from '../components/EmptyState.vue'
import { useRequestScope } from '../composables/useRequestScope'
const requests = useRequestScope()
import { computed, onMounted, ref } from 'vue'
import { memoryApi, type MemoryRow } from '../api'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

const rows = ref<MemoryRow[]>([])
const q = ref('')
const loading = ref(false)
const deleting = ref<Record<number, boolean>>({})
const error = ref('')
const confirmOpen = ref(false)
const clearTarget = ref<number | null>(null)
const clearing = ref(false)
const deleteConfirmOpen = ref(false)
const deleteTarget = ref<MemoryRow | null>(null)

const userCount = computed(
  () => new Set(rows.value.map((row) => row.UserID)).size,
)
const categoryCount = computed(
  () => new Set(rows.value.map((row) => row.Category || 'general')).size,
)
const clearTargetCount = computed(
  () => rows.value.filter((row) => row.UserID === clearTarget.value).length,
)

function categoryLabel(category: string) {
  const labels: Record<string, string> = {
    general: '通用',
    food: '饮食',
    location: '位置',
    hobby: '爱好',
    work: '工作',
    preference: '偏好',
    identity: '身份',
  }
  return labels[category] || category || '通用'
}

function sourceLabel(source: string) {
  return source === 'explicit' ? '用户确认' : 'AI 提取'
}

function formatDate(timestamp: number) {
  if (!timestamp) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

async function loadRows() {
  const currentRequest = requests.begin('load')
  loading.value = true
  error.value = ''
  try {
    const res = await memoryApi.memories(q.value.trim(), {
      signal: currentRequest.signal,
    })
    if (!currentRequest()) return
    rows.value = res.memories
  } catch (err) {
    if (!currentRequest()) return
    error.value = err instanceof Error ? err.message : '记忆加载失败'
  } finally {
    if (!currentRequest()) return
    loading.value = false
  }
}

async function remove(row: MemoryRow) {
  deleting.value[row.ID] = true
  error.value = ''
  try {
    await memoryApi.deleteMemory(row.ID)
    rows.value = rows.value.filter((item) => item.ID !== row.ID)
    deleteConfirmOpen.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除失败'
  } finally {
    deleting.value[row.ID] = false
  }
}

function openDelete(row: MemoryRow) {
  deleteTarget.value = row
  deleteConfirmOpen.value = true
}

function openClear(userID: number) {
  clearTarget.value = userID
  confirmOpen.value = true
}

async function clearUser() {
  if (!clearTarget.value) return
  clearing.value = true
  error.value = ''
  try {
    await memoryApi.clearUserMemories(clearTarget.value)
    rows.value = rows.value.filter((row) => row.UserID !== clearTarget.value)
    confirmOpen.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : '清空失败'
  } finally {
    clearing.value = false
  }
}

onMounted(loadRows)
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="长期记忆"
      description="审阅月灵自动提取的用户偏好，及时删除误判或不应保留的信息。"
      icon="i-tabler-brain"
    >
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="loadRows"
        >刷新</UButton
      >
    </PageHeader>

    <div class="grid grid-cols-3 gap-3 sm:gap-4">
      <MetricCard
        label="记忆条目"
        :value="rows.length"
        detail="当前查询结果"
        icon="i-tabler-brain"
        tone="violet"
      />
      <MetricCard
        label="关联用户"
        :value="userCount"
        detail="按 QQ 去重"
        icon="i-tabler-users"
        tone="cyan"
      />
      <MetricCard
        label="内容分类"
        :value="categoryCount"
        detail="用户确认或 AI 提取"
        icon="i-tabler-category"
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

    <FilterBar title="筛选长期记忆" description="按用户 QQ 或记忆内容查找">
      <UInput
        v-model="q"
        class="w-72"
        icon="i-tabler-search"
        aria-label="按 QQ、内容或分类筛选记忆"
        placeholder="QQ、偏好内容或分类"
        @keyup.enter="loadRows"
      />
      <UButton icon="i-tabler-search" :loading="loading" @click="loadRows"
        >查询</UButton
      >
    </FilterBar>

    <div class="rounded-2xl border border-default bg-default overflow-hidden">
      <div class="overflow-x-auto">
        <DataTable
          :loading="loading"
          label="长期记忆"
          table-class="min-w-[800px]"
        >
          <thead>
            <tr>
              <th scope="col">用户 QQ</th>
              <th scope="col">分类</th>
              <th scope="col">来源</th>
              <th scope="col">记忆内容</th>
              <th scope="col">写入时间</th>
              <th scope="col">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.ID">
              <td class="font-mono text-xs text-toned">
                {{ row.UserID }}
              </td>
              <td>
                <UBadge color="primary" variant="subtle">{{
                  categoryLabel(row.Category)
                }}</UBadge>
              </td>
              <td>
                <UBadge
                  :color="row.Source === 'explicit' ? 'success' : 'neutral'"
                  variant="soft"
                >
                  {{ sourceLabel(row.Source) }}
                </UBadge>
              </td>
              <td class="max-w-[480px] text-toned">
                <span class="line-clamp-2">{{ row.Content }}</span>
              </td>
              <td class="whitespace-nowrap text-xs text-muted">
                {{ formatDate(row.CreatedAt) }}
              </td>
              <td>
                <div class="flex items-center gap-1">
                  <UButton
                    size="xs"
                    color="error"
                    variant="soft"
                    icon="i-tabler-trash"
                    :loading="deleting[row.ID]"
                    @click="openDelete(row)"
                    >删除</UButton
                  >
                  <MoreActions
                    label="记忆批量操作"
                    :items="[
                      {
                        label: '清空该用户记忆',
                        description: `QQ ${row.UserID} 的全部长期记忆`,
                        icon: 'i-tabler-eraser',
                        color: 'error',
                        onSelect: () => openClear(row.UserID),
                      },
                    ]"
                  />
                </div>
              </td>
            </tr>
          </tbody>
        </DataTable>
      </div>
      <EmptyState
        v-if="!loading && rows.length === 0"
        title="没有长期记忆"
        description="当前筛选条件下没有保存的用户偏好"
        icon="i-tabler-brain-off"
      />
    </div>

    <UModal
      v-model:open="deleteConfirmOpen"
      title="删除这条长期记忆？"
      description="删除后，月灵不会再把这条内容加入该用户的 AI 上下文。"
    >
      <template #body>
        <div
          class="rounded-lg border border-default bg-muted p-4 text-sm leading-6 text-toned"
        >
          {{ deleteTarget?.Content }}
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            @click="deleteConfirmOpen = false"
            >取消</UButton
          >
          <UButton
            color="error"
            icon="i-tabler-trash"
            :loading="deleteTarget ? deleting[deleteTarget.ID] : false"
            @click="deleteTarget && remove(deleteTarget)"
            >确认删除</UButton
          >
        </div>
      </template>
    </UModal>

    <UModal
      v-model:open="confirmOpen"
      title="清空该用户的长期记忆？"
      description="此操作不可撤销，但不会删除用户主动设置的资料、待办、积分或好感度。"
    >
      <template #body>
        <div
          class="rounded-lg border border-default bg-muted p-4 text-sm text-toned"
        >
          QQ
          <span class="font-mono text-highlighted">{{ clearTarget }}</span>
          当前在列表中有
          <strong class="text-rose-300">{{ clearTargetCount }}</strong> 条记忆。
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="confirmOpen = false"
            >取消</UButton
          >
          <UButton
            color="error"
            icon="i-tabler-eraser"
            :loading="clearing"
            @click="clearUser"
            >确认清空</UButton
          >
        </div>
      </template>
    </UModal>
  </section>
</template>
