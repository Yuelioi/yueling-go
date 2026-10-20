<script setup lang="ts">
import EmptyState from '../components/EmptyState.vue'
import { useTimeoutFn } from '@vueuse/core'
import { useRequestScope } from '../composables/useRequestScope'
import { useGroupsStore } from '../stores/groups.ts'
import { computed, onMounted, ref, watch } from 'vue'
import { pluginsApi, type GroupInfo, type PluginEntry } from '../api'
import { useGroupFilter } from '../composables/useGroupFilter'
import GroupAdministrationTable from '../components/GroupAdministrationTable.vue'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const plugins = ref<PluginEntry[]>([])
const selectedGroupID = useGroupFilter()
const disabled = ref<Record<string, boolean>>({})
const loading = ref(false)
const loadingDisabled = ref(false)
const saving = ref<Record<number, boolean>>({})
const saved = ref<Record<number, boolean>>({})
const error = ref('')
const requests = useRequestScope()
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
    ]
      .join(' ')
      .toLowerCase()
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

const disabledCount = computed(
  () => Object.values(disabled.value ?? {}).filter(Boolean).length,
)

async function load() {
  error.value = ''
  loading.value = true
  try {
    const [groupRes, pluginRes] = await Promise.all([
      useGroupsStore().loadManaged(),
      pluginsApi.plugins(),
    ])
    groups.value = groupRes.groups
    plugins.value = pluginRes.plugins
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
  const currentRequest = requests.begin('load')
  const groupID = selectedGroupID.value
  disabled.value = {}
  loadingDisabled.value = false
  if (!groupID) {
    disabled.value = {}
    return
  }
  error.value = ''
  loadingDisabled.value = true
  try {
    const res = await pluginsApi.groupPlugins(groupID, {
      signal: currentRequest.signal,
    })
    if (!currentRequest()) return
    disabled.value = res.disabled ?? {}
  } catch (err) {
    if (currentRequest())
      error.value = err instanceof Error ? err.message : '加载插件状态失败'
  } finally {
    if (currentRequest()) loadingDisabled.value = false
  }
}

const { start: clearSaved } = useTimeoutFn(
  () => {
    saved.value = {}
  },
  1200,
  { immediate: false },
)
function markSaved(pluginID: number) {
  saved.value[pluginID] = true
  clearSaved()
}

async function setPlugin(pluginID: number, enabled: boolean) {
  if (!selectedGroupID.value) {
    return
  }
  error.value = ''
  saving.value[pluginID] = true
  saved.value[pluginID] = false
  const groupID = selectedGroupID.value
  const nextDisabled = !enabled
  try {
    await pluginsApi.setGroupPlugin(groupID, pluginID, nextDisabled)
    if (selectedGroupID.value !== groupID) return
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
    await pluginsApi.applyPluginAll(
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
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="插件策略"
      description="设置各群启用的插件，修改立即生效。"
      icon="i-tabler-components"
    >
      <div class="flex flex-wrap gap-2">
        <UInput
          v-if="selectedGroupID !== null"
          v-model="pluginQuery"
          class="w-64"
          icon="i-tabler-search"
          placeholder="搜索插件 / 命令"
        />
        <UButton
          color="neutral"
          variant="soft"
          icon="i-tabler-refresh"
          :loading="loading"
          @click="load"
        >
          刷新
        </UButton>
      </div>
    </PageHeader>

    <div
      v-if="selectedGroupID !== null"
      class="grid grid-cols-3 gap-3 sm:gap-4"
    >
      <MetricCard
        label="群聊范围"
        :value="groups.length"
        detail="已连接或有保存配置"
        icon="i-tabler-users-group"
        tone="cyan"
      />
      <MetricCard
        label="能力模块"
        :value="plugins.length"
        detail="已注册插件"
        icon="i-tabler-box-multiple"
        tone="violet"
      />
      <MetricCard
        label="当前群已关闭"
        :value="disabledCount"
        detail="仅影响所选群聊"
        icon="i-tabler-plug-off"
        tone="amber"
      />
    </div>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        allow-all
        :disabled="Object.values(saving).some(Boolean)"
        v-model="selectedGroupID"
        :groups="groups"
        title="策略作用域"
        description="选择要配置的群聊"
      />

      <GroupAdministrationTable
        v-if="selectedGroupID === null"
        feature="plugins"
        @select="selectedGroupID = $event"
      />

      <div v-else class="space-y-4">
        <div
          class="rounded-2xl border border-default bg-default flex min-h-16 items-center justify-between gap-3 px-5 py-3.5"
        >
          <div class="min-w-0">
            <div class="truncate text-sm font-semibold text-highlighted">
              {{
                selectedGroup?.group_name ||
                (selectedGroupID === null ? '全部群' : `群 ${selectedGroupID}`)
              }}
            </div>
            <div class="mt-1 text-xs text-muted">
              {{
                selectedGroupID
                  ? `群号 ${selectedGroupID}`
                  : '选择一个群后可管理插件'
              }}
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
          class="rounded-2xl border border-default bg-default overflow-hidden"
        >
          <div
            class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
          >
            <div class="flex min-w-0 items-center gap-2">
              <UIcon name="i-tabler-category" class="size-4 text-violet-300" />
              <h2 class="truncate text-sm font-semibold text-highlighted">
                {{ groupName }}
              </h2>
            </div>
            <UBadge color="neutral" variant="subtle">{{ items.length }}</UBadge>
          </div>
          <div>
            <div
              v-for="plugin in items"
              :key="plugin.id"
              class="border-t border-default transition-colors first:border-t-0 hover:bg-muted/30 grid min-h-24 gap-4 px-[18px] py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
            >
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-medium text-highlighted">{{
                    plugin.name
                  }}</span>
                  <UBadge
                    v-if="disabled[String(plugin.id)]"
                    color="warning"
                    variant="soft"
                  >
                    已禁用
                  </UBadge>
                  <UBadge v-else color="success" variant="subtle">可用</UBadge>
                  <span
                    v-if="saved[plugin.id]"
                    class="inline-flex items-center gap-1.5 text-success text-xs"
                    ><UIcon
                      name="i-tabler-check"
                      class="size-3.5"
                    />已保存</span
                  >
                </div>
                <div class="mt-1.5 text-xs leading-5 text-muted">
                  {{ plugin.desc }}
                </div>
                <div
                  v-if="pluginCommands(plugin).length"
                  class="mt-2 flex flex-wrap gap-1"
                >
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
                <MoreActions
                  label="插件批量操作"
                  :disabled="saving[plugin.id]"
                  :items="[
                    {
                      label: '所有群启用',
                      description: '批量开启全部群聊',
                      icon: 'i-tabler-check',
                      color: 'primary',
                      onSelect: () => requestApplyAll(plugin, false),
                    },
                    {
                      label: '所有群禁用',
                      description: '批量关闭全部群聊',
                      icon: 'i-tabler-ban',
                      color: 'warning',
                      onSelect: () => requestApplyAll(plugin, true),
                    },
                  ]"
                />
                <USwitch
                  :model-value="!disabled[String(plugin.id)]"
                  color="primary"
                  :disabled="
                    !selectedGroupID || loadingDisabled || saving[plugin.id]
                  "
                  @update:model-value="setPlugin(plugin.id, Boolean($event))"
                />
              </div>
            </div>
          </div>
        </section>

        <EmptyState
          v-if="!loading && groupedPlugins.length === 0"
          title="没有匹配的插件"
          description="换个关键词或清空搜索"
          icon="i-tabler-plug-off"
          class="rounded-2xl border border-default bg-default"
        />
      </div>
    </div>

    <UModal
      v-model:open="batchConfirmOpen"
      :title="
        batchAction?.disabled
          ? '在所有群禁用这个插件？'
          : '在所有群启用这个插件？'
      "
      :description="`该操作将同时修改 ${groups.length} 个群聊的插件策略。`"
    >
      <template #body>
        <div
          class="rounded-lg border border-default bg-muted p-4 text-sm text-toned"
        >
          <strong class="text-highlighted">{{
            batchAction?.plugin.name
          }}</strong>
          将在全部群聊中{{ batchAction?.disabled ? '禁用' : '启用' }}。
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton
            color="neutral"
            variant="ghost"
            @click="batchConfirmOpen = false"
            >取消</UButton
          >
          <UButton
            :color="batchAction?.disabled ? 'warning' : 'primary'"
            :loading="batchAction ? saving[batchAction.plugin.id] : false"
            @click="confirmApplyAll"
            >确认应用</UButton
          >
        </div>
      </template>
    </UModal>
  </section>
</template>
