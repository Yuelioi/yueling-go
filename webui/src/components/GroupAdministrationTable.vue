<script setup lang="ts">
import EmptyState from './EmptyState.vue'
import DataTable from './DataTable.vue'
import { useGroupsStore } from '../stores/groups.ts'
import { computed, onMounted, ref } from 'vue'
import { pluginsApi, type GroupAdministration } from '../api'

const props = withDefaults(
  defineProps<{
    feature?: 'all' | 'plugins' | 'style' | 'review' | 'digest' | 'message'
    includeDefault?: boolean
    groupId?: number | null
  }>(),
  { feature: 'all', includeDefault: false, groupId: null },
)
const emit = defineEmits<{ select: [id: number] }>()
const rows = ref<GroupAdministration[]>([])
const query = ref('')
const loading = ref(false)
const error = ref('')
const warning = ref('')
const pluginNames = ref<Record<number, string>>({})
const filtered = computed(() =>
  rows.value.filter(
    (row) =>
      (props.includeDefault || row.group_id !== 0) &&
      (props.groupId === null || row.group_id === props.groupId) &&
      `${row.group_name} ${row.group_id}`
        .toLowerCase()
        .includes(query.value.trim().toLowerCase()),
  ),
)
const show = (feature: string) =>
  props.feature === 'all' || props.feature === feature
function name(row: GroupAdministration) {
  return row.group_id === 0
    ? '全局默认 / 共享空间'
    : row.group_name || `群 ${row.group_id}`
}
function reviewLabel(mode: string) {
  return mode === 'disabled'
    ? '全部人工'
    : mode === 'override'
      ? '独立规则'
      : '继承全局'
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [res, plugins] = await Promise.all([
      useGroupsStore().loadAdministration(),
      pluginsApi.plugins(),
    ])
    rows.value = res.groups
    warning.value = res.warning
    pluginNames.value = Object.fromEntries(
      plugins.plugins.map((p) => [p.id, p.name]),
    )
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群概览加载失败'
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <section class="rounded-2xl border border-default bg-default overflow-hidden">
    <div class="flex flex-wrap items-center justify-between gap-3 p-4">
      <div>
        <h2 class="text-sm font-semibold text-highlighted">全部群情况</h2>
        <p class="mt-1 text-xs leading-5 text-muted">
          {{ filtered.length }} 个范围 · 点击管理进入对应群
        </p>
      </div>
      <div class="flex gap-2">
        <UInput
          v-model="query"
          icon="i-tabler-search"
          placeholder="筛选群名 / 群号"
          aria-label="筛选群名或群号"
        />
        <UButton variant="soft" color="neutral" :loading="loading" @click="load"
          >刷新</UButton
        >
      </div>
    </div>
    <UAlert
      icon="i-tabler-alert-circle"
      v-if="error"
      variant="soft"
      color="error"
      :description="error"
    />
    <UAlert
      icon="i-tabler-alert-triangle"
      v-if="warning"
      variant="soft"
      color="warning"
      :description="warning"
    />
    <div class="overflow-x-auto">
      <DataTable
        :loading="loading"
        label="群管理"
        :table-class="feature === 'all' ? 'min-w-[1000px]' : 'min-w-[520px]'"
      >
        <thead>
          <tr>
            <th scope="col">群聊</th>
            <th scope="col" v-if="show('plugins')">群内禁用插件</th>
            <th scope="col" v-if="show('style')">AI 风格</th>
            <th scope="col" v-if="show('review')">入群审核</th>
            <th scope="col" v-if="feature === 'all'">RSS 订阅</th>
            <th scope="col" v-if="show('digest')">日报</th>
            <th scope="col" v-if="feature === 'all'">知识库</th>
            <th scope="col">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in filtered" :key="row.group_id">
            <td>
              <div class="font-medium text-highlighted">{{ name(row) }}</div>
              <div class="text-xs text-muted">
                {{ row.group_id || '默认配置'
                }}<span v-if="row.group_id && !row.connected">
                  · 已保存配置</span
                >
              </div>
            </td>
            <td v-if="show('plugins')" class="max-w-72 text-xs">
              {{
                row.disabled_plugins
                  .map((id) => pluginNames[id] || `插件 ${id}`)
                  .join('、') || '无群级禁用'
              }}
            </td>
            <td v-if="show('style')" class="max-w-80">
              <UBadge
                variant="subtle"
                :color="row.custom_style ? 'primary' : 'neutral'"
                >{{
                  row.custom_style
                    ? '自定义'
                    : row.group_id
                      ? '继承全局'
                      : '内置默认'
                }}</UBadge
              >
              <p
                v-if="row.custom_style"
                class="mt-2 line-clamp-2 text-xs text-muted"
              >
                {{ row.style_prompt }}
              </p>
            </td>
            <td v-if="show('review')" class="whitespace-nowrap">
              {{ reviewLabel(row.join_mode) }}
            </td>
            <td v-if="feature === 'all'" class="whitespace-nowrap">
              {{ row.feed_count }} 个<span
                v-if="row.failing_feeds"
                class="ml-2 text-rose-400"
                >{{ row.failing_feeds }} 异常</span
              >
            </td>
            <td v-if="show('digest')" class="whitespace-nowrap">
              {{ row.digest_time || '未开启' }}
            </td>
            <td v-if="feature === 'all'">
              {{ row.knowledge_count }}
            </td>
            <td>
              <slot name="actions" :row="row"
                ><UButton
                  variant="soft"
                  size="xs"
                  @click="emit('select', row.group_id)"
                  >管理</UButton
                ></slot
              >
            </td>
          </tr>
        </tbody>
      </DataTable>
    </div>
    <EmptyState
      v-if="!loading && !filtered.length"
      title="没有匹配的群聊"
      description="试试其他群名或群号。"
      icon="i-tabler-users-group"
    />
    <p
      v-if="show('plugins')"
      class="border-t border-default px-4 py-3 text-xs text-muted"
    >
      插件最终可用性还受全局开关影响。
    </p>
  </section>
</template>
