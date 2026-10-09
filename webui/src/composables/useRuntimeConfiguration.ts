import { computed, ref } from 'vue'
import {
  api,
  type RuntimeConfiguration,
  type ImageEntry,
  type AIModelProfile,
  type PluginEntry,
} from '../api.ts'
import { useRequestScope } from './useRequestScope.ts'

export function useRuntimeConfiguration(
  sections: readonly string[],
  client: Pick<
    typeof api,
    'configuration' | 'plugins' | 'saveConfiguration'
  > = api,
) {
  const requests = useRequestScope()
  const revisions = new Map<string, number>()
  const configuration = ref<RuntimeConfiguration | null>(null)
  const selected = ref(sections[0] ?? '')
  const values = ref<Record<string, any>>({})
  const baseline = ref<Record<string, any>>({})
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
  const dirty = computed(() =>
    active.value?.fields.some(
      (f) =>
        JSON.stringify(values.value[f.path]) !==
          JSON.stringify(baseline.value[f.path]) || cleared.value[f.path],
    ),
  )

  function apply(c: RuntimeConfiguration, resetSection?: string) {
    configuration.value = c
    for (const section of c.sections) {
      if (
        resetSection &&
        section.id !== resetSection &&
        section.fields.some(
          (f) =>
            JSON.stringify(values.value[f.path]) !==
              JSON.stringify(baseline.value[f.path]) || cleared.value[f.path],
        )
      )
        continue
      revisions.set(section.id, c.revision)
      for (const f of section.fields) {
        const value =
          f.kind === 'numbers'
            ? ((f.value as number[] | null)?.join(', ') ?? '')
            : f.kind === 'ai_profiles'
              ? ((f.value as AIModelProfile[] | null) ?? []).map((p) => ({
                  ...p,
                  clear_key: false,
                }))
              : f.kind === 'image_entries'
                ? ((f.value as ImageEntry[] | null) ?? []).map((entry) => ({
                    ...entry,
                    kind: entry.kind || 'single',
                    call: entry.call || [],
                    arg: entry.arg ?? null,
                  }))
                : (f.value ?? '')
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
  function addProfile() {
    const bytes = crypto.getRandomValues(new Uint8Array(16))
    const id = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join(
      '',
    )
    const list = (values.value['ai.profiles'] ??= []) as AIModelProfile[]
    list.push({
      id,
      name: '新模型',
      base_url: '',
      model: '',
      api_key: '',
      vision: false,
      key_configured: false,
      clear_key: false,
    })
    if (!values.value['ai.conversation_profile'])
      values.value['ai.conversation_profile'] = id
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
      const payload: Record<string, unknown> = {}
      const clear: string[] = []
      for (const f of active.value.fields) {
        if (cleared.value[f.path]) clear.push(f.path)
        let v = values.value[f.path]
        if (f.kind === 'number') {
          if (String(v).trim() === '' || !Number.isSafeInteger(Number(v)))
            throw new Error(`${f.label} 请填写整数`)
          v = Number(v)
        }
        if (f.kind === 'numbers') {
          const parts = String(v)
            .trim()
            .split(/[\s,，]+/)
            .filter(Boolean)
          if (
            parts.some(
              (n) =>
                !/^\d+$/.test(n) ||
                !Number.isSafeInteger(Number(n)) ||
                Number(n) <= 0,
            )
          )
            throw new Error(`${f.label} 请填写正整数，多个号码用逗号分隔`)
          v = [...new Set(parts.map(Number))]
        }
        if (f.kind === 'ai_profiles') {
          v = (v as AIModelProfile[]).map(
            ({ key_configured, clear_key, ...p }) => ({
              ...p,
              api_key: clear_key ? '' : p.api_key,
              clear_key: !!clear_key,
            }),
          )
        }
        payload[f.path] = v
      }
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
    } catch (err) {
      if (current())
        error.value = err instanceof Error ? err.message : '保存失败'
    } finally {
      if (current()) saving.value = false
    }
  }

  return {
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
    addProfile,
  }
}
