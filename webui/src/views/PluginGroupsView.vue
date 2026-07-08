<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo, type PluginEntry } from '../api'

const groups = ref<GroupInfo[]>([])
const plugins = ref<PluginEntry[]>([])
const selectedGroupID = ref<number | null>(null)
const disabled = ref<Record<string, boolean>>({})
const loading = ref(false)
const loadingDisabled = ref(false)
const saving = ref<Record<number, boolean>>({})
const saved = ref<Record<number, boolean>>({})
const error = ref('')
const pluginQuery = ref('')

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)

function pluginCommands(plugin: PluginEntry) {
  return Array.isArray(plugin.commands) ? plugin.commands : []
}

const filteredPlugins = computed(() => {
  const query = pluginQuery.value.trim().toLowerCase()
  if (!query) {
    return plugins.value
  }
  return plugins.value.filter((plugin) => {
    const text = [
      plugin.name,
      plugin.group,
      plugin.desc,
      plugin.usage,
      ...pluginCommands(plugin),
    ].join(' ').toLowerCase()
    return text.includes(query)
  })
})

const groupedPlugins = computed(() => {
  const map = new Map<string, PluginEntry[]>()
  for (const plugin of filteredPlugins.value) {
    if (!map.has(plugin.group)) {
      map.set(plugin.group, [])
    }
    map.get(plugin.group)!.push(plugin)
  }
  return Array.from(map.entries())
})

const disabledCount = computed(() =>
  Object.values(disabled.value ?? {}).filter(Boolean).length,
)

async function load() {
  error.value = ''
  loading.value = true
  try {
    const [groupRes, pluginRes] = await Promise.all([api.groups(), api.plugins()])
    groups.value = groupRes.groups
    plugins.value = pluginRes.plugins
    selectedGroupID.value = groups.value[0]?.group_id ?? null
    if (selectedGroupID.value) {
      await loadDisabled()
    } else {
      disabled.value = {}
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function loadDisabled() {
  if (!selectedGroupID.value) {
    disabled.value = {}
    return
  }
  error.value = ''
  loadingDisabled.value = true
  try {
    const res = await api.groupPlugins(selectedGroupID.value)
    disabled.value = res.disabled ?? {}
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载插件状态失败'
  } finally {
    loadingDisabled.value = false
  }
}

function markSaved(pluginID: number) {
  saved.value[pluginID] = true
  window.setTimeout(() => {
    saved.value[pluginID] = false
  }, 1200)
}

async function setPlugin(pluginID: number, enabled: boolean) {
  if (!selectedGroupID.value) {
    return
  }
  error.value = ''
  saving.value[pluginID] = true
  saved.value[pluginID] = false
  const nextDisabled = !enabled
  try {
    await api.setGroupPlugin(selectedGroupID.value, pluginID, nextDisabled)
    disabled.value = { ...disabled.value, [String(pluginID)]: nextDisabled }
    markSaved(pluginID)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value[pluginID] = false
  }
}

async function applyAll(pluginID: number, value: boolean) {
  if (groups.value.length === 0) {
    return
  }
  error.value = ''
  saving.value[pluginID] = true
  saved.value[pluginID] = false
  try {
    await api.applyPluginAll(
      pluginID,
      groups.value.map((group) => group.group_id),
      value,
    )
    if (selectedGroupID.value) {
      disabled.value = { ...disabled.value, [String(pluginID)]: value }
    }
    markSaved(pluginID)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '批量操作失败'
  } finally {
    saving.value[pluginID] = false
  }
}

watch(selectedGroupID, () => {
  void loadDisabled()
})

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="space-y-5">
    <div class="page-head">
      <div>
        <div class="eyebrow">Plugin Policy</div>
        <h1 class="page-title mt-1">群插件管理</h1>
        <p class="page-subtitle mt-1 text-sm">按群关闭指定插件；未禁用的插件保持正常响应。</p>
      </div>
      <div class="mt-4 flex flex-wrap gap-2 sm:mt-0">
        <UInput
          v-model="pluginQuery"
          class="w-64"
          icon="i-tabler-search"
          placeholder="搜索插件 / 命令"
        />
        <UButton icon="i-tabler-refresh" :loading="loading" @click="load">
          刷新
        </UButton>
      </div>
    </div>

    <div class="grid gap-3 md:grid-cols-3">
      <div class="surface-panel px-4 py-3">
        <div class="text-xs text-zinc-500">群数量</div>
        <div class="mt-1 flex items-end gap-2">
          <span class="text-2xl font-semibold text-white">{{ groups.length }}</span>
          <span class="pb-1 text-xs text-zinc-500">已连接群</span>
        </div>
      </div>
      <div class="surface-panel px-4 py-3">
        <div class="text-xs text-zinc-500">插件数量</div>
        <div class="mt-1 flex items-end gap-2">
          <span class="text-2xl font-semibold text-white">{{ plugins.length }}</span>
          <span class="pb-1 text-xs text-zinc-500">已注册插件</span>
        </div>
      </div>
      <div class="surface-panel px-4 py-3">
        <div class="text-xs text-zinc-500">当前群禁用</div>
        <div class="mt-1 flex items-end gap-2">
          <span class="text-2xl font-semibold text-amber-300">{{ disabledCount }}</span>
          <span class="pb-1 text-xs text-zinc-500">仅当前群</span>
        </div>
      </div>
    </div>

    <UAlert
      v-if="error"
      class="error-banner"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="grid gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
      <aside class="surface-panel overflow-hidden">
        <div class="panel-header text-sm">
          <div>
            <div class="font-medium text-white">群列表</div>
            <div class="text-xs text-zinc-500">选择一个群后编辑插件策略</div>
          </div>
          <UBadge color="neutral" variant="subtle">{{ groups.length }}</UBadge>
        </div>
        <div v-if="groups.length" class="max-h-[650px] overflow-y-auto p-2">
          <button
            v-for="group in groups"
            :key="group.group_id"
            class="mb-1 w-full rounded-md border px-3 py-2.5 text-left text-sm transition"
            :class="selectedGroupID === group.group_id
              ? 'border-teal-500/40 bg-teal-500/10 text-white'
              : 'border-transparent text-zinc-400 hover:bg-zinc-800/70 hover:text-white'"
            @click="selectedGroupID = group.group_id"
          >
            <div class="flex min-w-0 items-center gap-2">
              <UIcon name="i-tabler-users-group" class="size-4 shrink-0 text-teal-300" />
              <div class="min-w-0">
                <div class="truncate font-medium">{{ group.group_name || group.group_id }}</div>
                <div class="text-xs text-zinc-500">{{ group.group_id }}</div>
              </div>
            </div>
          </button>
        </div>
        <div
          v-else
          class="empty-state m-3"
        >
          <UIcon name="i-tabler-users-off" class="size-6 text-zinc-500" />
          <div class="font-medium text-zinc-200">暂无群</div>
          <div class="text-xs text-zinc-500">Bot 连接后才能读取群列表</div>
        </div>
      </aside>

      <div class="space-y-4">
        <div class="surface-panel flex min-h-14 items-center justify-between gap-3 px-4 py-3">
          <div class="min-w-0">
            <div class="truncate font-medium text-white">
              {{ selectedGroup?.group_name || '未选择群' }}
            </div>
            <div class="text-xs text-zinc-500">
              {{ selectedGroupID || '选择一个群后可管理插件' }}
            </div>
          </div>
          <div class="flex items-center gap-2">
            <UBadge v-if="loadingDisabled" color="neutral" variant="subtle">
              读取中
            </UBadge>
            <UBadge v-else color="primary" variant="subtle">
              {{ filteredPlugins.length }} 个插件
            </UBadge>
          </div>
        </div>

        <section
          v-for="[groupName, items] in groupedPlugins"
          :key="groupName"
          class="surface-panel overflow-hidden"
        >
          <div class="panel-header">
            <div class="flex min-w-0 items-center gap-2">
              <UIcon name="i-tabler-category" class="size-4 text-teal-300" />
              <h2 class="truncate text-sm font-semibold text-white">{{ groupName }}</h2>
            </div>
            <UBadge color="neutral" variant="subtle">{{ items.length }}</UBadge>
          </div>
          <div>
            <div
              v-for="plugin in items"
              :key="plugin.id"
              class="data-row grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
            >
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-medium text-white">{{ plugin.name }}</span>
                  <UBadge v-if="disabled[String(plugin.id)]" color="warning" variant="soft">
                    已禁用
                  </UBadge>
                  <UBadge v-else color="success" variant="subtle">可用</UBadge>
                  <span v-if="saved[plugin.id]" class="text-xs text-teal-300">已保存</span>
                </div>
                <div class="mt-1 text-sm text-zinc-400">{{ plugin.desc }}</div>
                <div v-if="pluginCommands(plugin).length" class="mt-2 flex flex-wrap gap-1">
                  <UBadge
                    v-for="command in pluginCommands(plugin).slice(0, 4)"
                    :key="command"
                    color="neutral"
                    variant="subtle"
                    size="xs"
                  >
                    {{ command }}
                  </UBadge>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-2 sm:justify-end">
                <UButton
                  size="xs"
                  color="warning"
                  variant="soft"
                  icon="i-tabler-ban"
                  title="在所有群禁用这个插件"
                  :loading="saving[plugin.id]"
                  @click="applyAll(plugin.id, true)"
                >
                  所有群禁用
                </UButton>
                <UButton
                  size="xs"
                  color="primary"
                  variant="soft"
                  icon="i-tabler-check"
                  title="在所有群启用这个插件"
                  :loading="saving[plugin.id]"
                  @click="applyAll(plugin.id, false)"
                >
                  所有群启用
                </UButton>
                <USwitch
                  :model-value="!disabled[String(plugin.id)]"
                  color="primary"
                  :disabled="!selectedGroupID || saving[plugin.id]"
                  @update:model-value="setPlugin(plugin.id, Boolean($event))"
                />
              </div>
            </div>
          </div>
        </section>

        <div
          v-if="!loading && groupedPlugins.length === 0"
          class="surface-panel empty-state py-12"
        >
          <UIcon name="i-tabler-plug-off" class="size-7 text-zinc-500" />
          <div class="font-medium text-zinc-200">没有匹配的插件</div>
          <div class="text-sm text-zinc-500">换个关键词或清空搜索</div>
        </div>
      </div>
    </div>
  </section>
</template>
