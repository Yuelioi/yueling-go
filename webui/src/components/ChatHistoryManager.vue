<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type ChatHistoryStats } from '../api'

type HistoryCleanupMode = '30' | '90' | '180' | '365' | 'all'

const props = defineProps<{
  groupId: number | null
  groupName?: string
}>()

const emit = defineEmits<{
  deleted: [result: { deleted: number; remaining: number }]
}>()

const open = defineModel<boolean>('open', { required: true })
const loading = ref(false)
const deleting = ref(false)
const error = ref('')
const stats = ref<ChatHistoryStats | null>(null)
const cleanupMode = ref<HistoryCleanupMode>('90')
let requestVersion = 0

const cleanupOptions: Array<{ value: HistoryCleanupMode; label: string; description: string }> = [
  { value: '30', label: '30 天以前', description: '保留最近一个月' },
  { value: '90', label: '90 天以前', description: '保留最近三个月' },
  { value: '180', label: '180 天以前', description: '保留最近半年' },
  { value: '365', label: '365 天以前', description: '保留最近一年' },
  { value: 'all', label: '全部记录', description: '清空当前群历史' },
]

const beforeAt = computed(() => cleanupMode.value === 'all'
  ? undefined
  : Math.floor(Date.now() / 1000) - Number(cleanupMode.value) * 24 * 60 * 60)

const groupLabel = computed(() => props.groupName?.trim() || props.groupId || '当前群')
const statClass = 'min-w-0 rounded-[11px] border border-[var(--border)] bg-[rgba(8,8,17,0.34)] px-[11px] py-2.5'
const statLabelClass = 'block text-[0.61rem] text-[var(--dim)]'
const statValueClass = 'mt-1 block truncate text-[0.72rem] font-[650] text-[var(--ink-soft)]'
const cleanupButtonBaseClass = 'min-w-0 rounded-[10px] border border-[var(--border)] bg-[rgba(8,8,17,0.32)] px-2.5 py-[9px] text-left text-[var(--muted)] transition-[border-color,background,color] duration-150 hover:border-[var(--border-strong)] hover:bg-[rgba(125,108,200,0.065)] focus-visible:border-[rgba(139,121,220,0.58)] focus-visible:outline-none focus-visible:shadow-[0_0_0_2px_rgba(125,108,200,0.14)] disabled:cursor-wait disabled:opacity-60'
const cleanupButtonActiveClass = 'border-[rgba(139,121,220,0.5)] bg-[rgba(125,108,200,0.14)] shadow-[inset_0_0_0_1px_rgba(139,121,220,0.12)]'
const cleanupButtonDangerClass = 'focus-visible:border-[rgba(224,94,114,0.48)] focus-visible:shadow-[0_0_0_2px_rgba(224,94,114,0.12)]'
const cleanupButtonDangerActiveClass = 'border-[rgba(224,94,114,0.4)] bg-[rgba(224,94,114,0.1)]'

function formatTime(timestamp: number) {
  if (!timestamp) return '暂无'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

function cleanupButtonClasses(value: HistoryCleanupMode) {
  return [
    cleanupButtonBaseClass,
    value === 'all' ? cleanupButtonDangerClass : '',
    cleanupMode.value === value
      ? (value === 'all' ? cleanupButtonDangerActiveClass : cleanupButtonActiveClass)
      : '',
  ]
}

async function loadStats() {
  if (!props.groupId) return
  const version = ++requestVersion
  loading.value = true
  error.value = ''
  try {
    const response = await api.chatHistory(props.groupId, beforeAt.value)
    if (version === requestVersion) stats.value = response.stats
  } catch (err) {
    if (version === requestVersion) {
      stats.value = null
      error.value = err instanceof Error ? err.message : '历史消息统计加载失败'
    }
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

async function deleteHistory() {
  if (!props.groupId || !stats.value?.matched) return
  deleting.value = true
  error.value = ''
  try {
    const payload = cleanupMode.value === 'all'
      ? { all: true }
      : { before_at: beforeAt.value }
    const response = await api.deleteChatHistory(props.groupId, payload)
    emit('deleted', response)
    open.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除历史消息失败'
  } finally {
    deleting.value = false
  }
}

watch(open, (isOpen) => {
  if (!isOpen) {
    requestVersion++
    return
  }
  if (cleanupMode.value === '90') {
    void loadStats()
  } else {
    cleanupMode.value = '90'
  }
})

watch(cleanupMode, () => {
  if (open.value) void loadStats()
})

watch(() => props.groupId, () => {
  if (open.value) open.value = false
})
</script>

<template>
  <UModal
    v-model:open="open"
    title="管理聊天历史"
    :description="`只会清理“${groupLabel}”的记录，不影响其他群。`"
    :dismissible="!deleting"
    :ui="{
      overlay: 'z-40 bg-black/70 backdrop-blur-sm',
      content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/25 divide-zinc-800 shadow-2xl',
      header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900',
      title: 'text-white', description: 'text-zinc-400'
    }"
  >
    <template #body>
      <div class="space-y-4">
        <UAlert v-if="error" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />

        <div class="grid grid-cols-[0.72fr_1fr_1fr] gap-2 max-[860px]:grid-cols-1">
          <div :class="statClass"><small :class="statLabelClass">当前总量</small><strong :class="statValueClass">{{ (stats?.total || 0).toLocaleString('zh-CN') }}</strong></div>
          <div :class="statClass"><small :class="statLabelClass">最早记录</small><strong :class="statValueClass">{{ formatTime(stats?.oldest_at || 0) }}</strong></div>
          <div :class="statClass"><small :class="statLabelClass">最近记录</small><strong :class="statValueClass">{{ formatTime(stats?.newest_at || 0) }}</strong></div>
        </div>

        <div>
          <div class="mb-2 text-xs font-medium text-zinc-300">选择清理范围</div>
          <div class="grid grid-cols-3 gap-[7px] max-[860px]:grid-cols-1">
            <button
              v-for="option in cleanupOptions"
              :key="option.value"
              type="button"
              :class="cleanupButtonClasses(option.value)"
              :disabled="deleting"
              @click="cleanupMode = option.value"
            >
              <strong class="block text-[0.7rem] text-[var(--ink-soft)]">{{ option.label }}</strong>
              <small class="mt-[3px] block text-[0.58rem] text-[var(--dim)]">{{ option.description }}</small>
            </button>
          </div>
        </div>

        <div class="flex items-center gap-2 rounded-[10px] border border-[rgba(224,94,114,0.16)] bg-[rgba(224,94,114,0.055)] px-[11px] py-[9px] text-[0.68rem] text-[#c59aa4]">
          <UIcon :name="loading ? 'i-tabler-loader-2' : 'i-tabler-alert-triangle'" class="size-4" :class="{ 'animate-spin': loading }" />
          <span v-if="loading">正在计算将被删除的消息数量…</span>
          <span v-else>
            此操作将永久删除 <strong class="text-[#f3a8b6]">{{ (stats?.matched || 0).toLocaleString('zh-CN') }}</strong> 条消息，无法恢复。
          </span>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" :disabled="deleting" @click="open = false">取消</UButton>
        <UButton
          color="error"
          icon="i-tabler-trash"
          :loading="deleting"
          :disabled="deleting || loading || !stats?.matched"
          @click="deleteHistory"
        >
          删除 {{ (stats?.matched || 0).toLocaleString('zh-CN') }} 条
        </UButton>
      </div>
    </template>
  </UModal>
</template>
