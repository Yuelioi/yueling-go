import { computed, ref, type Ref } from 'vue'
import type { AIConnection, AIModelProfile } from '../../api/ai.ts'
import {
  modelCapabilities,
  modelPurposes,
  modelRouteIssue,
} from '../../ai-models.ts'
import { copy, newID } from './shared.ts'

export function useModelDraft(connections: Ref<AIConnection[]>) {
  const selected = ref<AIModelProfile | null>(null)
  const open = ref(false)
  const baseline = ref('')
  const custom = ref(false)
  const account = computed(() =>
    connections.value.find((c) => c.id === selected.value?.connection_id),
  )
  const issue = computed(() =>
    selected.value ? modelRouteIssue(selected.value, connections.value) : '',
  )
  const dirty = computed(
    () => open.value && JSON.stringify(selected.value) !== baseline.value,
  )
  const purpose = computed(
    () =>
      modelPurposes.find(
        (p) =>
          selected.value &&
          [...p.capabilities].sort().join() ===
            [...modelCapabilities(selected.value)].sort().join(),
      )?.value ?? 'custom',
  )
  const purposeOptions = computed(() => [
    ...modelPurposes
      .filter((p) => account.value?.kind !== 'codex' || p.value !== 'search')
      .map((p) => ({ label: p.label, value: p.value })),
    { label: '自定义能力', value: 'custom' },
  ])
  const accountOptions = computed(() =>
    connections.value.map((c) => ({
      label: `${c.name} · ${c.kind === 'codex' ? 'Codex' : 'API'}`,
      value: c.id,
    })),
  )
  function edit(profile: AIModelProfile) {
    selected.value = copy(profile)
    baseline.value = JSON.stringify(selected.value)
    custom.value = purpose.value === 'custom'
    open.value = true
  }
  function add(account: AIConnection) {
    selected.value = {
      id: newID(),
      name: '',
      model: '',
      connection_id: account.id,
      capabilities: ['chat', 'tools'],
      disabled: false,
      max_tokens: 0,
      timeout_seconds: 0,
      reasoning_effort: '',
      base_url: '',
      api_key: '',
      vision: false,
      key_configured: false,
    }
    baseline.value = ''
    custom.value = false
    open.value = true
  }
  function setPurpose(value: string) {
    const preset = modelPurposes.find((p) => p.value === value)
    if (selected.value && preset) {
      selected.value.capabilities = [...preset.capabilities]
      selected.value.vision = preset.capabilities.includes('vision')
    }
    custom.value = value === 'custom'
  }
  function setCapability(capability: string, checked: boolean) {
    if (!selected.value) return
    selected.value.capabilities = modelCapabilities(selected.value).filter(
      (c) => c !== capability,
    )
    if (checked) selected.value.capabilities.push(capability)
    selected.value.vision = selected.value.capabilities.includes('vision')
  }
  function submit() {
    if (!selected.value || issue.value) return
    const profile = copy(selected.value)
    if (
      profile.connection_id &&
      (profile.base_url || profile.api_key || profile.key_configured)
    ) {
      profile.base_url = ''
      profile.api_key = ''
      profile.clear_key = true
    }
    if (!profile.name.trim())
      profile.name =
        modelCapabilities(profile).join() === 'web_search'
          ? '联网搜索'
          : profile.model
    return profile
  }
  function close() {
    open.value = false
  }
  return {
    selected,
    open,
    baseline,
    custom,
    account,
    issue,
    dirty,
    purpose,
    purposeOptions,
    accountOptions,
    add,
    edit,
    setPurpose,
    setCapability,
    submit,
    close,
  }
}
export type ModelDraft = ReturnType<typeof useModelDraft>
