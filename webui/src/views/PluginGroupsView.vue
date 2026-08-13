<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo, type PluginEntry } from '../api'
import GroupPicker from '../components/GroupPicker.vue'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

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
const batchConfirmOpen = ref(false)
const batchAction = ref<{ plugin: PluginEntry; disabled: boolean } | null>(null)

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

function requestApplyAll(plugin: PluginEntry, disabled: boolean) {
  batchAction.value = { plugin, disabled }
  batchConfirmOpen.value = true
}

async function confirmApplyAll() {
  if (!batchAction.value) return
  await applyAll(batchAction.value.plugin.id, batchAction.value.disabled)
  batchConfirmOpen.value = false
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
    <PageHeader
      eyebrow="Capability policy"
      title="插件策略"
      description="为每个群定制月灵的能力边界。修改会即时生效，不会改写配置文件。"
      icon="i-tabler-components"
    >
      <div class="flex flex-wrap gap-2">
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
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="已连接群聊" :value="groups.length" detail="Bot 当前可访问" icon="i-tabler-users-group" tone="cyan" />
      <MetricCard label="能力模块" :value="plugins.length" detail="已注册插件" icon="i-tabler-box-multiple" tone="violet" />
      <MetricCard label="当前群已关闭" :value="disabledCount" detail="仅影响所选群聊" icon="i-tabler-plug-off" tone="amber" />
    </div>

    <UAlert
      v-if="error"
      class="error-banner"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="grid gap-4 lg:grid-cols-[292px_minmax(0,1fr)]">
      <GroupPicker
        v-model="selectedGroupID"
        :groups="groups"
        title="策略作用域"
        description="选择要配置的群聊"
      />

      <div class="space-y-4">
        <div class="surface-panel flex min-h-16 items-center justify-between gap-3 px-5 py-3.5">
          <div class="min-w-0">
            <div class="truncate text-sm font-semibold text-white">
              {{ selectedGroup?.group_name || '未选择群' }}
            </div>
            <div class="mt-1 text-xs text-zinc-500">
              {{ selectedGroupID ? `群号 ${selectedGroupID}` : '选择一个群后可管理插件' }}
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
              <UIcon name="i-tabler-category" class="size-4 text-violet-300" />
              <h2 class="truncate text-sm font-semibold text-white">{{ groupName }}</h2>
            </div>
            <UBadge color="neutral" variant="subtle">{{ items.length }}</UBadge>
          </div>
          <div>
            <div
              v-for="plugin in items"
              :key="plugin.id"
              class="plugin-policy-row data-row grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
            >
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-medium text-white">{{ plugin.name }}</span>
                  <UBadge v-if="disabled[String(plugin.id)]" color="warning" variant="soft">
                    已禁用
                  </UBadge>
                  <UBadge v-else color="success" variant="subtle">可用</UBadge>
                  <span v-if="saved[plugin.id]" class="inline-status text-xs"><UIcon name="i-tabler-check" class="size-3.5" />已保存</span>
                </div>
                <div class="mt-1.5 text-xs leading-5 text-zinc-400">{{ plugin.desc }}</div>
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
                  class="plugin-batch-button"
                  color="warning"
                  variant="soft"
                  icon="i-tabler-ban"
                  title="在所有群禁用这个插件"
                  :loading="saving[plugin.id]"
                  @click="requestApplyAll(plugin, true)"
                >
                  所有群禁用
                </UButton>
                <UButton
                  size="xs"
                  class="plugin-batch-button"
                  color="primary"
                  variant="soft"
                  icon="i-tabler-check"
                  title="在所有群启用这个插件"
                  :loading="saving[plugin.id]"
                  @click="requestApplyAll(plugin, false)"
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

    <UModal
      v-model:open="batchConfirmOpen"
      :title="batchAction?.disabled ? '在所有群禁用这个插件？' : '在所有群启用这个插件？'"
      :description="`该操作将同时修改 ${groups.length} 个群聊的插件策略。`"
      :ui="{ overlay: 'z-40 bg-black/70 backdrop-blur-sm', content: 'z-50 bg-zinc-900 text-zinc-100 ring ring-violet-500/30 divide-zinc-800 shadow-2xl', header: 'border-b border-zinc-800', body: 'bg-zinc-900', footer: 'border-t border-zinc-800 bg-zinc-900', title: 'text-white', description: 'text-zinc-400' }"
    >
      <template #body>
        <div class="surface-inset p-4 text-sm text-zinc-300">
          <strong class="text-white">{{ batchAction?.plugin.name }}</strong>
          将在全部群聊中{{ batchAction?.disabled ? '禁用' : '启用' }}。
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="batchConfirmOpen = false">取消</UButton>
          <UButton :color="batchAction?.disabled ? 'warning' : 'primary'" :loading="batchAction ? saving[batchAction.plugin.id] : false" @click="confirmApplyAll">确认应用</UButton>
        </div>
      </template>
    </UModal>
  </section>
</template>
