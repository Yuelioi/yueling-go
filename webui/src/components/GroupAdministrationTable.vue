<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type GroupAdministration } from '../api'

const props = withDefaults(defineProps<{
  feature?: 'all' | 'plugins' | 'style' | 'review' | 'digest' | 'message'
  includeDefault?: boolean
  groupId?: number | null
}>(), { feature: 'all', includeDefault: false, groupId: null })
const emit = defineEmits<{ select: [id: number] }>()
const rows = ref<GroupAdministration[]>([])
const query = ref('')
const loading = ref(false)
const error = ref('')
const warning = ref('')
const pluginNames = ref<Record<number, string>>({})
const filtered = computed(() => rows.value.filter(row =>
  (props.includeDefault || row.group_id !== 0) &&
  (props.groupId === null || row.group_id === props.groupId) &&
  `${row.group_name} ${row.group_id}`.toLowerCase().includes(query.value.trim().toLowerCase()),
))
const show = (feature: string) => props.feature === 'all' || props.feature === feature
function name(row: GroupAdministration) {
  return row.group_id === 0 ? '全局默认 / 共享空间' : row.group_name || `群 ${row.group_id}`
}
function reviewLabel(mode: string) {
  return mode === 'disabled' ? '全部人工' : mode === 'override' ? '独立规则' : '继承全局'
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [res, plugins] = await Promise.all([api.groupAdministration(), api.plugins()])
    rows.value = res.groups
    warning.value = res.warning
    pluginNames.value = Object.fromEntries(plugins.plugins.map(p => [p.id, p.name]))
  } catch (err) { error.value = err instanceof Error ? err.message : '群概览加载失败' }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <section class="surface-panel overflow-hidden">
    <div class="flex flex-wrap items-center justify-between gap-3 p-4">
      <div><h2 class="section-title">全部群情况</h2><p class="section-caption">{{ filtered.length }} 个范围 · 点击管理进入对应群</p></div>
      <div class="flex gap-2">
        <UInput v-model="query" icon="i-tabler-search" placeholder="筛选群名 / 群号" aria-label="筛选群名或群号" />
        <UButton variant="soft" color="neutral" :loading="loading" @click="load">刷新</UButton>
      </div>
    </div>
    <UAlert v-if="error" variant="subtle" color="error" :description="error" />
    <UAlert v-if="warning" variant="subtle" color="warning" :description="warning" />
    <div class="overflow-x-auto">
      <table class="w-full text-left text-sm" :class="feature === 'all' ? 'min-w-[1000px]' : 'min-w-[520px]'">
        <thead class="border-y border-white/5 text-xs text-zinc-400"><tr>
          <th class="px-4 py-3">群聊</th>
          <th v-if="show('plugins')" class="px-4 py-3">群内禁用插件</th>
          <th v-if="show('style')" class="px-4 py-3">AI 风格</th>
          <th v-if="show('review')" class="px-4 py-3">入群审核</th>
          <th v-if="feature === 'all'" class="px-4 py-3">RSS 订阅</th>
          <th v-if="show('digest')" class="px-4 py-3">日报</th>
          <th v-if="feature === 'all'" class="px-4 py-3">知识库</th>
          <th class="px-4 py-3">操作</th>
        </tr></thead>
        <tbody><tr v-for="row in filtered" :key="row.group_id" class="data-row">
          <td class="px-4 py-3"><div class="font-medium text-white">{{ name(row) }}</div><div class="text-xs text-zinc-500">{{ row.group_id || '默认配置' }}<span v-if="row.group_id && !row.connected"> · 已保存配置</span></div></td>
          <td v-if="show('plugins')" class="max-w-72 px-4 py-3 text-xs">{{ row.disabled_plugins.map(id => pluginNames[id] || `插件 ${id}`).join('、') || '无群级禁用' }}</td>
          <td v-if="show('style')" class="max-w-80 px-4 py-3"><UBadge variant="subtle" :color="row.custom_style ? 'primary' : 'neutral'">{{ row.custom_style ? '自定义' : row.group_id ? '继承全局' : '内置默认' }}</UBadge><p v-if="row.custom_style" class="mt-2 line-clamp-2 text-xs text-zinc-400">{{ row.style_prompt }}</p></td>
          <td v-if="show('review')" class="whitespace-nowrap px-4 py-3">{{ reviewLabel(row.join_mode) }}</td>
          <td v-if="feature === 'all'" class="whitespace-nowrap px-4 py-3">{{ row.feed_count }} 个<span v-if="row.failing_feeds" class="ml-2 text-rose-400">{{ row.failing_feeds }} 异常</span></td>
          <td v-if="show('digest')" class="whitespace-nowrap px-4 py-3">{{ row.digest_time || '未开启' }}</td>
          <td v-if="feature === 'all'" class="px-4 py-3">{{ row.knowledge_count }}</td>
          <td class="px-4 py-3"><slot name="actions" :row="row"><UButton variant="soft" size="xs" @click="emit('select', row.group_id)">管理</UButton></slot></td>
        </tr></tbody>
      </table>
    </div>
    <p v-if="!loading && !filtered.length" class="p-8 text-center text-sm text-zinc-500">没有符合筛选条件的群</p>
    <p v-if="show('plugins')" class="border-t border-white/5 px-4 py-3 text-xs text-zinc-500">插件最终可用性还受全局开关影响。</p>
  </section>
</template>
