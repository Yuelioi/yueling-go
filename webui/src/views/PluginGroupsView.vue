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

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)

const groupedPlugins = computed(() => {
  const map = new Map<string, PluginEntry[]>()
  for (const plugin of plugins.value) {
    if (!map.has(plugin.group)) {
      map.set(plugin.group, [])
    }
    map.get(plugin.group)!.push(plugin)
  }
  return Array.from(map.entries())
})

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
    disabled.value = res.disabled
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
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold">群插件</h1>
        <p class="text-sm text-neutral-400">禁用后该群静默跳过对应插件</p>
      </div>
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">
        刷新
      </UButton>
    </div>

    <UAlert
      v-if="error"
      color="error"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
      <aside class="rounded-lg border border-neutral-800 bg-neutral-900 p-3">
        <div class="mb-3 flex items-center justify-between text-sm text-neutral-400">
          <span>群列表</span>
          <UBadge color="neutral" variant="subtle">{{ groups.length }}</UBadge>
        </div>
        <div v-if="groups.length" class="space-y-1">
          <button
            v-for="group in groups"
            :key="group.group_id"
            class="w-full rounded-md px-3 py-2 text-left text-sm text-neutral-300 hover:bg-neutral-800 hover:text-white"
            :class="{ 'bg-neutral-800 text-white': selectedGroupID === group.group_id }"
            @click="selectedGroupID = group.group_id"
          >
            <div class="truncate">{{ group.group_name || group.group_id }}</div>
            <div class="text-xs text-neutral-500">{{ group.group_id }}</div>
          </button>
        </div>
        <UEmpty
          v-else
          icon="i-tabler-users-off"
          title="暂无群"
          description="Bot 连接后才能读取群列表"
        />
      </aside>

      <div class="space-y-4">
        <div
          class="flex min-h-10 items-center justify-between rounded-lg border border-neutral-800 bg-neutral-900 px-4 py-2"
        >
          <div class="min-w-0">
            <div class="truncate text-sm font-medium">
              {{ selectedGroup?.group_name || '未选择群' }}
            </div>
            <div class="text-xs text-neutral-500">
              {{ selectedGroupID || '选择一个群后可管理插件' }}
            </div>
          </div>
          <UBadge v-if="loadingDisabled" color="neutral" variant="subtle">
            读取中
          </UBadge>
        </div>

        <section
          v-for="[groupName, items] in groupedPlugins"
          :key="groupName"
          class="rounded-lg border border-neutral-800 bg-neutral-900 p-4"
        >
          <h2 class="mb-3 text-sm font-semibold text-neutral-300">{{ groupName }}</h2>
          <div class="divide-y divide-neutral-800">
            <div
              v-for="plugin in items"
              :key="plugin.id"
              class="grid gap-3 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
            >
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-medium">{{ plugin.name }}</span>
                  <UBadge v-if="disabled[String(plugin.id)]" color="warning" variant="subtle">
                    已禁用
                  </UBadge>
                  <span v-if="saved[plugin.id]" class="text-xs text-emerald-400">已保存</span>
                </div>
                <div class="mt-1 text-sm text-neutral-400">{{ plugin.desc }}</div>
              </div>
              <div class="flex flex-wrap items-center gap-2 sm:justify-end">
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-tabler-ban"
                  :loading="saving[plugin.id]"
                  @click="applyAll(plugin.id, true)"
                >
                  全部禁用
                </UButton>
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-tabler-check"
                  :loading="saving[plugin.id]"
                  @click="applyAll(plugin.id, false)"
                >
                  全部启用
                </UButton>
                <USwitch
                  :model-value="!disabled[String(plugin.id)]"
                  :disabled="!selectedGroupID || saving[plugin.id]"
                  @update:model-value="setPlugin(plugin.id, Boolean($event))"
                />
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
