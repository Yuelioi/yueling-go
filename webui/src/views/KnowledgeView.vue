<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo, type KnowledgeEntry } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const entries = ref<KnowledgeEntry[]>([])
const selectedGroupID = ref<number | null>(null)
const mode = ref<'text' | 'url'>('text')
const title = ref('')
const content = ref('')
const sourceURL = ref('')
const shortcutText = ref('')
const query = ref('')
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const confirmOpen = ref(false)
const pendingDelete = ref<KnowledgeEntry | null>(null)
const error = ref('')
const notice = ref('')
const shortcutEditingID = ref<number | null>(null)
const shortcutDraft = ref('')
const shortcutSaving = ref(false)
const editorOpen = ref(false)

const knowledgeScopes = computed<GroupInfo[]>(() => [
  { group_id: 0, group_name: '所有群共享' },
  ...groups.value,
])
const selectedGroup = computed(() => knowledgeScopes.value.find((group) => group.group_id === selectedGroupID.value))
const isShared = computed(() => selectedGroupID.value === 0)
const groupEntries = computed(() => entries.value)
const shortcutCount = computed(() => entries.value.reduce((total, row) => total + (row.shortcuts?.length || 0), 0))
const filteredEntries = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return groupEntries.value
  return groupEntries.value.filter((row) => `${row.title} ${row.content} ${row.source_url} ${(row.shortcuts || []).map((item) => item.trigger).join(' ')}`.toLowerCase().includes(keyword))
})

function parseShortcuts(value: string) {
  return [...new Set(value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean))]
}

function preview(value: string) {
  const cleaned = value.replace(/\s+/g, ' ').trim()
  return cleaned.length > 150 ? `${cleaned.slice(0, 150)}…` : cleaned
}

function formatDate(timestamp: number) {
  if (!timestamp) return '-'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    .format(new Date(timestamp * 1000))
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const groupRes = await api.groups()
    groups.value = groupRes.groups
    if (selectedGroupID.value === null) {
      selectedGroupID.value = 0
    } else {
      await loadEntries()
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '知识库加载失败'
  } finally {
    loading.value = false
  }
}

async function loadEntries() {
	if (selectedGroupID.value === null) {
    entries.value = []
    return
	}
	const groupID = selectedGroupID.value
	notice.value = ''
	query.value = ''
	error.value = ''
	try {
		const res = await api.knowledge(groupID)
		if (selectedGroupID.value === groupID) entries.value = res.knowledge
	} catch (err) {
		if (selectedGroupID.value === groupID) {
			entries.value = []
			error.value = err instanceof Error ? err.message : '群知识加载失败'
		}
	}
}

async function save() {
  if (selectedGroupID.value === null) {
    error.value = '请选择知识作用域'
    return
  }
  if (mode.value === 'text' && !content.value.trim()) {
    error.value = '请输入知识内容'
    return
  }
  if (mode.value === 'url') {
    try {
      const parsed = new URL(sourceURL.value.trim())
      if (!['http:', 'https:'].includes(parsed.protocol)) throw new Error()
    } catch {
      error.value = '请输入有效的公网网页地址'
      return
    }
  }
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const shortcuts = parseShortcuts(shortcutText.value)
    const payload = mode.value === 'url'
      ? { title: title.value.trim(), url: sourceURL.value.trim(), shortcuts }
      : { title: title.value.trim(), content: content.value.trim(), shortcuts }
    const res = await api.addKnowledge(selectedGroupID.value, payload)
    entries.value.unshift(res.knowledge)
    title.value = ''
    content.value = ''
    sourceURL.value = ''
    shortcutText.value = ''
    editorOpen.value = false
    notice.value = `“${res.knowledge.title}”已加入${isShared.value ? '所有群共享知识库' : '当前群知识库'}`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '知识添加失败'
  } finally {
    saving.value = false
  }
}

function beginShortcutEdit(row: KnowledgeEntry) {
  shortcutEditingID.value = row.id
  shortcutDraft.value = (row.shortcuts || []).map((item) => item.trigger).join(', ')
}

function cancelShortcutEdit() {
  shortcutEditingID.value = null
  shortcutDraft.value = ''
}

async function saveShortcuts(row: KnowledgeEntry) {
  if (selectedGroupID.value === null) return
  shortcutSaving.value = true
  error.value = ''
  notice.value = ''
  try {
    const res = await api.setKnowledgeShortcuts(selectedGroupID.value, row.id, parseShortcuts(shortcutDraft.value))
    entries.value = entries.value.map((item) => item.id === row.id ? { ...item, shortcuts: res.shortcuts } : item)
    shortcutEditingID.value = null
    shortcutDraft.value = ''
    notice.value = `“${row.title}”的快捷触发词已更新`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '快捷触发词保存失败'
  } finally {
    shortcutSaving.value = false
  }
}

function askDelete(row: KnowledgeEntry) {
  pendingDelete.value = row
  confirmOpen.value = true
}

async function remove() {
  const row = pendingDelete.value
  if (!row) return
  deleting.value = true
  error.value = ''
  try {
    await api.deleteKnowledge(row.group_id, row.id)
    entries.value = entries.value.filter((item) => item.id !== row.id)
    confirmOpen.value = false
    pendingDelete.value = null
    notice.value = `已删除“${row.title}”`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除知识失败'
  } finally {
    deleting.value = false
  }
}

watch(selectedGroupID, loadEntries)
onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Grounded group intelligence"
      title="群知识库"
      description="群专属资料严格隔离，共享资料供所有群共同检索和快捷触发。"
      icon="i-tabler-books"
    >
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :loading="loading" @click="load">刷新资料</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard :label="isShared ? '共享知识' : '当前群知识'" :value="entries.length" :detail="isShared ? '所有群共同使用' : '仅所选群聊使用'" icon="i-tabler-library" tone="violet" />
      <MetricCard label="快捷触发词" :value="shortcutCount" detail="精确命中，不消耗 AI" icon="i-tabler-bolt" tone="cyan" />
      <MetricCard label="单空间容量" value="100" detail="每条最多 10 个快捷词" icon="i-tabler-database" tone="amber" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />
    <UAlert v-if="notice" color="success" variant="subtle" icon="i-tabler-circle-check" :description="notice" />

    <div class="space-y-4">
      <GroupScopeSelect v-model="selectedGroupID" :groups="knowledgeScopes" title="知识作用域" description="共享空间或指定群聊" zero-label="共享知识空间" />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0"><div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择作用域' }}</div><div class="text-xs text-zinc-500">{{ isShared ? '所有群都能检索和快捷触发' : selectedGroupID !== null ? `群号 ${selectedGroupID} · 仅本群可用` : '选择作用域后可录入资料' }}</div></div>
            <div class="flex items-center gap-2">
              <UBadge color="primary" variant="subtle">{{ groupEntries.length }} / 100</UBadge>
              <UButton color="primary" variant="soft" :icon="editorOpen ? 'i-tabler-chevron-up' : 'i-tabler-plus'" @click="editorOpen = !editorOpen">
                {{ editorOpen ? '收起' : '新增知识' }}
              </UButton>
            </div>
          </div>
          <form v-if="editorOpen" class="space-y-4 p-4" @submit.prevent="save">
            <div class="flex gap-2">
              <UButton type="button" size="sm" :color="mode === 'text' ? 'primary' : 'neutral'" :variant="mode === 'text' ? 'soft' : 'ghost'" icon="i-tabler-align-left" @click="mode = 'text'">录入文本</UButton>
              <UButton type="button" size="sm" :color="mode === 'url' ? 'primary' : 'neutral'" :variant="mode === 'url' ? 'soft' : 'ghost'" icon="i-tabler-world-download" @click="mode = 'url'">导入网页</UButton>
            </div>
            <UFormField label="标题" description="留空时根据正文或网页自动生成">
              <UInput v-model="title" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-heading" :placeholder="isShared ? '例如：Bot 使用说明' : '例如：入群规则'" :disabled="selectedGroupID === null" />
            </UFormField>
            <UFormField v-if="mode === 'text'" label="知识内容" description="只保存明确、长期有效的群资料">
              <UTextarea v-model="content" class="w-full" :ui="{ root: 'w-full' }" :rows="6" autoresize :placeholder="isShared ? '月灵的通用能力、公共项目资料或所有群都适用的说明……' : '新成员加入后，需要把群名片修改为……'" :disabled="selectedGroupID === null" />
            </UFormField>
            <UFormField v-else label="公网网页地址" description="提取 HTML 或纯文本正文，拒绝私网地址和超大页面">
              <UInput v-model="sourceURL" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-link" placeholder="https://example.com/docs/rules" :disabled="selectedGroupID === null" />
            </UFormField>
            <UFormField label="快捷触发词（可选）" description="逗号或换行分隔；群友发送完全相同的文字时直接回复，不调用 AI">
              <UInput v-model="shortcutText" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-bolt" placeholder="例如：ae下载, AE安装包" :disabled="selectedGroupID === null" />
            </UFormField>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-zinc-500">{{ isShared ? '共享快捷词在所有群生效；同名群专属快捷词会优先。' : '普通问答会同时检索本群资料与共享资料。' }}</p>
              <UButton type="submit" icon="i-tabler-database-plus" :loading="saving" :disabled="selectedGroupID === null || groupEntries.length >= 100">保存到知识库</UButton>
            </div>
          </form>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div><div class="section-title">{{ isShared ? '所有群共享资料' : '当前群专属资料' }}</div><div class="section-caption">回答会引用知识 ID，不允许脱离资料猜测</div></div>
            <UInput v-model="query" size="sm" icon="i-tabler-search" placeholder="筛选标题或内容" />
          </div>
          <div v-if="filteredEntries.length">
            <article v-for="row in filteredEntries" :key="row.id" class="data-row flex items-start gap-3 p-4">
              <span class="activity-icon mt-0.5 shrink-0"><UIcon :name="row.source_url ? 'i-tabler-world' : 'i-tabler-file-text'" class="size-4" /></span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2"><strong class="text-sm text-white">{{ row.title }}</strong><UBadge color="neutral" variant="subtle">知识 #{{ row.id }}</UBadge></div>
                <div v-if="row.shortcuts?.length" class="mt-2 flex flex-wrap gap-1.5"><UBadge v-for="shortcut in row.shortcuts" :key="shortcut.id" color="primary" variant="subtle"><UIcon name="i-tabler-bolt" class="mr-1 size-3" />{{ shortcut.trigger }}</UBadge></div>
                <p class="mt-2 text-xs leading-5 text-zinc-400">{{ preview(row.content) }}</p>
                <div class="mt-2 flex flex-wrap items-center gap-3 text-[11px] text-zinc-600"><span>{{ formatDate(row.updated_at) }}</span><a v-if="row.source_url" :href="row.source_url" target="_blank" rel="noreferrer" class="max-w-80 truncate text-violet-300 hover:text-violet-200">{{ row.source_url }}</a></div>
                <div v-if="shortcutEditingID === row.id" class="mt-3 flex flex-col gap-2 rounded-xl border border-violet-400/20 bg-violet-400/5 p-3 sm:flex-row">
                  <UInput v-model="shortcutDraft" class="min-w-0 flex-1" :ui="{ root: 'w-full' }" icon="i-tabler-bolt" placeholder="逗号分隔；留空并保存可清除" />
                  <div class="flex gap-2"><UButton size="sm" color="neutral" variant="ghost" @click="cancelShortcutEdit">取消</UButton><UButton size="sm" icon="i-tabler-check" :loading="shortcutSaving" @click="saveShortcuts(row)">保存</UButton></div>
                </div>
              </div>
              <div class="flex shrink-0 items-center gap-1">
                <UButton color="neutral" variant="ghost" icon="i-tabler-bolt" aria-label="编辑快捷触发词" @click="beginShortcutEdit(row)" />
                <MoreActions
                  label="知识更多操作"
                  :items="[
                    { label: '删除知识', description: '停止检索和快捷触发', icon: 'i-tabler-trash', color: 'error', onSelect: () => askDelete(row) },
                  ]"
                />
              </div>
            </article>
          </div>
          <div v-else class="empty-state overview-empty">
            <span class="empty-icon"><UIcon name="i-tabler-books-off" class="size-6" /></span>
            <div class="empty-title">当前空间没有匹配的资料</div>
            <div class="empty-description">录入说明或导入项目文档后即可开始问答</div>
          </div>
        </section>
      </div>
    </div>

    <UModal v-model:open="confirmOpen" title="删除这条知识？" :description="pendingDelete?.group_id === 0 ? '删除后，所有群都无法再检索或快捷触发这条共享知识。' : '删除后，AI 将不再把它作为当前群的问答来源。'" :ui="{ overlay: 'z-40 bg-black/70 backdrop-blur-sm', content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl', header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900', title: 'text-white', description: 'text-zinc-400' }">
      <template #body><div class="surface-inset p-4 text-sm text-zinc-300">知识 #{{ pendingDelete?.id }} · {{ pendingDelete?.title }}</div></template>
      <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="ghost" @click="confirmOpen = false">取消</UButton><UButton color="error" icon="i-tabler-trash" :loading="deleting" @click="remove">确认删除</UButton></div></template>
    </UModal>
  </section>
</template>
