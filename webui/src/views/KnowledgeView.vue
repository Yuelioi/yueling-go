<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo, type KnowledgeEntry } from '../api'
import GroupPicker from '../components/GroupPicker.vue'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const entries = ref<KnowledgeEntry[]>([])
const selectedGroupID = ref<number | null>(null)
const mode = ref<'text' | 'url'>('text')
const title = ref('')
const content = ref('')
const sourceURL = ref('')
const query = ref('')
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const confirmOpen = ref(false)
const pendingDelete = ref<KnowledgeEntry | null>(null)
const error = ref('')
const notice = ref('')

const selectedGroup = computed(() => groups.value.find((group) => group.group_id === selectedGroupID.value))
const groupEntries = computed(() => entries.value)
const filteredEntries = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return groupEntries.value
  return groupEntries.value.filter((row) => `${row.title} ${row.content} ${row.source_url}`.toLowerCase().includes(keyword))
})

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
    if (!selectedGroupID.value) {
      selectedGroupID.value = groups.value[0]?.group_id || null
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
	if (!selectedGroupID.value) {
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
  if (!selectedGroupID.value) {
    error.value = '请选择群聊'
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
    const payload = mode.value === 'url'
      ? { title: title.value.trim(), url: sourceURL.value.trim() }
      : { title: title.value.trim(), content: content.value.trim() }
    const res = await api.addKnowledge(selectedGroupID.value, payload)
    entries.value.unshift(res.knowledge)
    title.value = ''
    content.value = ''
    sourceURL.value = ''
    notice.value = `“${res.knowledge.title}”已加入当前群知识库`
  } catch (err) {
    error.value = err instanceof Error ? err.message : '知识添加失败'
  } finally {
    saving.value = false
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
      description="把群规则、项目资料和常见问题变成有来源、按群隔离的 AI 问答。"
      icon="i-tabler-books"
    >
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">刷新资料</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="当前群知识" :value="entries.length" detail="所选群聊资料" icon="i-tabler-library" tone="violet" />
      <MetricCard label="可用群聊" :value="groups.length" detail="资料严格按群隔离" icon="i-tabler-users-group" tone="cyan" />
      <MetricCard label="单群容量" value="100" detail="单条最多 12,000 字" icon="i-tabler-database" tone="amber" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />
    <UAlert v-if="notice" color="success" variant="subtle" icon="i-tabler-circle-check" :description="notice" />

    <div class="grid gap-4 lg:grid-cols-[292px_minmax(0,1fr)]">
      <GroupPicker v-model="selectedGroupID" :groups="groups" title="知识所属群" description="资料严格按群隔离" />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0"><div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择群' }}</div><div class="text-xs text-zinc-500">{{ selectedGroupID ? `群号 ${selectedGroupID}` : '选择群聊后可录入资料' }}</div></div>
            <UBadge color="primary" variant="subtle">{{ groupEntries.length }} / 100</UBadge>
          </div>
          <form class="space-y-4 p-4" @submit.prevent="save">
            <div class="flex gap-2">
              <UButton type="button" size="sm" :color="mode === 'text' ? 'primary' : 'neutral'" :variant="mode === 'text' ? 'soft' : 'ghost'" icon="i-tabler-align-left" @click="mode = 'text'">录入文本</UButton>
              <UButton type="button" size="sm" :color="mode === 'url' ? 'primary' : 'neutral'" :variant="mode === 'url' ? 'soft' : 'ghost'" icon="i-tabler-world-download" @click="mode = 'url'">导入网页</UButton>
            </div>
            <UFormField label="标题" description="留空时根据正文或网页自动生成">
              <UInput v-model="title" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-heading" placeholder="例如：入群规则" :disabled="!selectedGroupID" />
            </UFormField>
            <UFormField v-if="mode === 'text'" label="知识内容" description="只保存明确、长期有效的群资料">
              <UTextarea v-model="content" class="w-full" :ui="{ root: 'w-full' }" :rows="7" autoresize placeholder="新成员加入后，需要把群名片修改为……" :disabled="!selectedGroupID" />
            </UFormField>
            <UFormField v-else label="公网网页地址" description="提取 HTML 或纯文本正文，拒绝私网地址和超大页面">
              <UInput v-model="sourceURL" class="w-full" :ui="{ root: 'w-full' }" icon="i-tabler-link" placeholder="https://example.com/docs/rules" :disabled="!selectedGroupID" />
            </UFormField>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-zinc-500">群员只有显式发送“知识问 …”或要求查知识库时才会调用 AI。</p>
              <UButton type="submit" icon="i-tabler-database-plus" :loading="saving" :disabled="!selectedGroupID || groupEntries.length >= 100">保存到知识库</UButton>
            </div>
          </form>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div><div class="section-title">当前群资料</div><div class="section-caption">回答会引用知识 ID，不允许脱离资料猜测</div></div>
            <UInput v-model="query" size="sm" icon="i-tabler-search" placeholder="筛选标题或内容" />
          </div>
          <div v-if="filteredEntries.length">
            <article v-for="row in filteredEntries" :key="row.id" class="data-row flex items-start gap-3 p-4">
              <span class="activity-icon mt-0.5 shrink-0"><UIcon :name="row.source_url ? 'i-tabler-world' : 'i-tabler-file-text'" class="size-4" /></span>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2"><strong class="text-sm text-white">{{ row.title }}</strong><UBadge color="neutral" variant="subtle">知识 #{{ row.id }}</UBadge></div>
                <p class="mt-2 text-xs leading-5 text-zinc-400">{{ preview(row.content) }}</p>
                <div class="mt-2 flex flex-wrap items-center gap-3 text-[11px] text-zinc-600"><span>{{ formatDate(row.updated_at) }}</span><a v-if="row.source_url" :href="row.source_url" target="_blank" rel="noreferrer" class="max-w-80 truncate text-violet-300 hover:text-violet-200">{{ row.source_url }}</a></div>
              </div>
              <UButton color="error" variant="ghost" icon="i-tabler-trash" aria-label="删除知识" @click="askDelete(row)" />
            </article>
          </div>
          <div v-else class="empty-state overview-empty">
            <span class="empty-icon"><UIcon name="i-tabler-books-off" class="size-6" /></span>
            <div class="empty-title">当前没有匹配的群资料</div>
            <div class="empty-description">录入规则或导入项目文档后即可开始问答</div>
          </div>
        </section>
      </div>
    </div>

    <UModal v-model:open="confirmOpen" title="删除这条知识？" description="删除后，AI 将不再把它作为当前群的问答来源。" :ui="{ overlay: 'z-40 bg-black/70 backdrop-blur-sm', content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl', header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900', title: 'text-white', description: 'text-zinc-400' }">
      <template #body><div class="surface-inset p-4 text-sm text-zinc-300">知识 #{{ pendingDelete?.id }} · {{ pendingDelete?.title }}</div></template>
      <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="ghost" @click="confirmOpen = false">取消</UButton><UButton color="error" icon="i-tabler-trash" :loading="deleting" @click="remove">确认删除</UButton></div></template>
    </UModal>
  </section>
</template>
