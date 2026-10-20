import { computed, ref } from 'vue'
import {
  configurationApi,
  pluginsApi,
  type RuntimeConfiguration,
  type PluginEntry,
} from '../api.ts'
import { useConfigurationStore } from '../stores/configuration.ts'
import {
  emptyDraft,
  normalizeField,
  fieldChanged,
  encodeFields,
  type ConfigurationDraft,
} from '../features/configuration/draft.ts'
import { useRequestScope } from './useRequestScope.ts'

const defaultClient = { ...configurationApi, ...pluginsApi }

export function useRuntimeConfiguration(
  sections: readonly string[],
  client: Pick<
    typeof defaultClient,
    'configuration' | 'plugins' | 'saveConfiguration'
  > = {
    ...defaultClient,
    configuration: () => useConfigurationStore().load(),
    saveConfiguration: (...args) => useConfigurationStore().save(...args),
  },
  acceptsField: (path: string) => boolean = () => true,
) {
  const requests = useRequestScope()
  const revisions = new Map<string, number>()
  const configuration = ref<RuntimeConfiguration | null>(null)
  const selected = ref(sections[0] ?? '')
  const values = ref<ConfigurationDraft>(emptyDraft())
  const baseline = ref<ConfigurationDraft>(emptyDraft())
  const cleared = ref<Record<string, boolean>>({})
  const plugins = ref<PluginEntry[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  const success = ref('')
  const active = computed(() =>
    configuration.value?.sections.find(
      (s) => sections.includes(s.id) && s.id === selected.value,
    ),
  )
  const editableFields = computed(
    () => active.value?.fields.filter((f) => acceptsField(f.path)) ?? [],
  )
  const dirty = computed(() =>
    editableFields.value.some((f) =>
      fieldChanged(f.path, values.value, baseline.value, cleared.value),
    ),
  )

  function apply(c: RuntimeConfiguration, resetSection?: string) {
    configuration.value = c
    for (const section of c.sections) {
      if (
        resetSection &&
        section.id !== resetSection &&
        section.fields.some((f) =>
          fieldChanged(f.path, values.value, baseline.value, cleared.value),
        )
      )
        continue
      revisions.set(section.id, c.revision)
      for (const f of section.fields) {
        const value = normalizeField(f)
        values.value[f.path] = structuredClone(value)
        baseline.value[f.path] = structuredClone(value)
        cleared.value[f.path] = false
      }
    }
  }
  async function load() {
    if (saving.value) return
    const current = requests.begin('load')
    loading.value = true
    error.value = ''
    try {
      const [c, p] = await Promise.all([
        client.configuration(),
        sections.includes('plugins')
          ? client.plugins()
          : Promise.resolve({ plugins: [] }),
      ])
      if (!current()) return
      plugins.value = p.plugins
      apply(c)
    } catch (err) {
      if (current())
        error.value = err instanceof Error ? err.message : '加载失败'
    } finally {
      if (current()) loading.value = false
    }
  }
  function pluginEnabled(id: number) {
    return !String(values.value['plugins.disabled'] ?? '')
      .split(/[\s,，]+/)
      .includes(String(id))
  }
  function setPlugin(id: number, enabled: boolean) {
    const ids = String(values.value['plugins.disabled'] ?? '')
      .split(/[\s,，]+/)
      .filter(Boolean)
      .map(Number)
      .filter((n) => n !== id)
    if (!enabled) ids.push(id)
    values.value['plugins.disabled'] = ids.sort((a, b) => a - b).join(', ')
  }
  async function save() {
    if (!configuration.value || !active.value || saving.value || loading.value)
      return
    const current = requests.begin('save')
    const sectionID = active.value.id
    error.value = ''
    success.value = ''
    saving.value = true
    try {
      const { payload, clear } = encodeFields(
        editableFields.value,
        values.value,
        cleared.value,
      )
      const res = await client.saveConfiguration(
        revisions.get(sectionID) ?? configuration.value.revision,
        sectionID,
        payload,
        clear,
      )
      if (!current()) return
      apply(res.config, sectionID)
      success.value = res.reload
        ? '已保存，部分设置待重启生效。服务将退出并由进程管理器重启；直接运行时请手动启动。'
        : '已保存并立即生效。'
      return true
    } catch (err) {
      if (current())
        error.value = err instanceof Error ? err.message : '保存失败'
      return false
    } finally {
      if (current()) saving.value = false
    }
  }

  function reset() {
    if (saving.value || loading.value) return
    for (const f of editableFields.value) {
      values.value[f.path] = JSON.parse(JSON.stringify(baseline.value[f.path]))
      cleared.value[f.path] = false
    }
    error.value = ''
    success.value = ''
  }

  return {
    reset,
    configuration,
    selected,
    values,
    cleared,
    plugins,
    loading,
    saving,
    error,
    success,
    active,
    dirty,
    load,
    save,
    pluginEnabled,
    setPlugin,
  }
}
