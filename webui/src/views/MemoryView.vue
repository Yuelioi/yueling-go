<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type MemoryRow } from '../api'
import MetricCard from '../components/MetricCard.vue'
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

const userCount = computed(() => new Set(rows.value.map((row) => row.UserID)).size)
const categoryCount = computed(() => new Set(rows.value.map((row) => row.Category || 'general')).size)
const clearTargetCount = computed(() => rows.value.filter((row) => row.UserID === clearTarget.value).length)

function categoryLabel(category: string) {
  const labels: Record<string, string> = {
    general: '通用', food: '饮食', location: '位置', hobby: '爱好',
    work: '工作', preference: '偏好', identity: '身份',
  }
  return labels[category] || category || '通用'
}

function sourceLabel(source: string) {
  return source === 'explicit' ? '用户确认' : 'AI 提取'
}

function formatDate(timestamp: number) {
  if (!timestamp) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

async function loadRows() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.memories(q.value.trim())
    rows.value = res.memories
  } catch (err) {
    error.value = err instanceof Error ? err.message : '记忆加载失败'
  } finally {
    loading.value = false
  }
}

async function remove(row: MemoryRow) {
  deleting.value[row.ID] = true
  error.value = ''
  try {
    await api.deleteMemory(row.ID)
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
    await api.clearUserMemories(clearTarget.value)
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
  <section class="space-y-5">
    <PageHeader
      eyebrow="Memory observatory"
      title="长期记忆"
      description="审阅月灵自动提取的用户偏好，及时删除误判或不应保留的信息。"
      icon="i-tabler-brain"
    >
      <UButton icon="i-tabler-refresh" :loading="loading" @click="loadRows">刷新</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="记忆条目" :value="rows.length" detail="当前查询结果" icon="i-tabler-brain" tone="violet" />
      <MetricCard label="关联用户" :value="userCount" detail="按 QQ 去重" icon="i-tabler-users" tone="cyan" />
      <MetricCard label="内容分类" :value="categoryCount" detail="用户确认或 AI 提取" icon="i-tabler-category" tone="rose" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />

    <div class="surface-panel flex flex-wrap items-end gap-3 p-4">
      <div class="mr-auto min-w-[220px]">
        <div class="section-title">检索记忆</div>
        <div class="section-caption">可按 QQ、内容或分类查找，最多显示 200 条</div>
      </div>
      <UInput v-model="q" class="w-72" icon="i-tabler-search" placeholder="QQ、偏好内容或分类" @keyup.enter="loadRows" />
      <UButton icon="i-tabler-search" :loading="loading" @click="loadRows">查询</UButton>
    </div>

    <div class="surface-panel overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[800px] text-left text-sm">
          <thead class="border-b border-white/5 bg-black/10 text-xs uppercase text-zinc-500">
            <tr>
              <th class="px-4 py-3 font-medium">用户 QQ</th>
              <th class="px-4 py-3 font-medium">分类</th>
	          <th class="px-4 py-3 font-medium">来源</th>
	          <th class="px-4 py-3 font-medium">记忆内容</th>
              <th class="px-4 py-3 font-medium">写入时间</th>
              <th class="px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.ID" class="data-row">
              <td class="px-4 py-3 font-mono text-xs text-zinc-300">{{ row.UserID }}</td>
              <td class="px-4 py-3"><UBadge color="primary" variant="subtle">{{ categoryLabel(row.Category) }}</UBadge></td>
	          <td class="px-4 py-3"><UBadge :color="row.Source === 'explicit' ? 'success' : 'neutral'" variant="soft">{{ sourceLabel(row.Source) }}</UBadge></td>
              <td class="max-w-[480px] px-4 py-3 text-zinc-200"><span class="line-clamp-2">{{ row.Content }}</span></td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-zinc-500">{{ formatDate(row.CreatedAt) }}</td>
              <td class="px-4 py-3">
                <div class="flex gap-1">
                  <UButton size="xs" color="error" variant="soft" icon="i-tabler-trash" :loading="deleting[row.ID]" @click="openDelete(row)">删除</UButton>
                  <UButton size="xs" color="neutral" variant="ghost" icon="i-tabler-eraser" @click="openClear(row.UserID)">清空用户</UButton>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!loading && rows.length === 0" class="empty-state border-t border-zinc-800/80 py-12">
        <UIcon name="i-tabler-brain-off" class="size-7 text-zinc-500" />
        <div class="font-medium text-zinc-200">没有长期记忆</div>
        <div class="text-sm text-zinc-500">当前筛选条件下没有保存的用户偏好</div>
      </div>
    </div>

    <UModal
      v-model:open="deleteConfirmOpen"
      title="删除这条长期记忆？"
      description="删除后，月灵不会再把这条内容加入该用户的 AI 上下文。"
      :ui="{ overlay: 'z-40 bg-black/70 backdrop-blur-sm', content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl', header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900', title: 'text-white', description: 'text-zinc-400' }"
    >
      <template #body>
        <div class="surface-inset p-4 text-sm leading-6 text-zinc-300">{{ deleteTarget?.Content }}</div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="deleteConfirmOpen = false">取消</UButton>
          <UButton color="error" icon="i-tabler-trash" :loading="deleteTarget ? deleting[deleteTarget.ID] : false" @click="deleteTarget && remove(deleteTarget)">确认删除</UButton>
        </div>
      </template>
    </UModal>

    <UModal
      v-model:open="confirmOpen"
      title="清空该用户的长期记忆？"
      description="此操作不可撤销，但不会删除用户主动设置的资料、待办、积分或好感度。"
      :ui="{
        overlay: 'z-40 bg-black/70 backdrop-blur-sm',
        content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl',
        header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900',
        title: 'text-white', description: 'text-zinc-400'
      }"
    >
      <template #body>
        <div class="surface-inset p-4 text-sm text-zinc-300">
          QQ <span class="font-mono text-white">{{ clearTarget }}</span> 当前在列表中有
          <strong class="text-rose-300">{{ clearTargetCount }}</strong> 条记忆。
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="confirmOpen = false">取消</UButton>
          <UButton color="error" icon="i-tabler-eraser" :loading="clearing" @click="clearUser">确认清空</UButton>
        </div>
      </template>
    </UModal>
  </section>
</template>
