import { computed, ref } from 'vue'
import type { AIConnection } from '../../api/ai.ts'
import { copy, newID } from './shared.ts'
export type Provider = 'codex' | 'deepseek' | 'api'
export function useAccountDraft() {
  const selected = ref<AIConnection | null>(null)
  const baseline = ref('')
  const open = ref(false)
  const file = ref<File | null>(null)
  const replace = ref(false)
  const dirty = computed(
    () =>
      open.value &&
      (JSON.stringify(selected.value) !== baseline.value || !!file.value),
  )
  function edit(account: AIConnection) {
    selected.value = copy(account)
    baseline.value = JSON.stringify(selected.value)
    file.value = null
    replace.value = false
    open.value = true
  }
  function add(provider: Provider) {
    selected.value = {
      id: newID(),
      name:
        provider === 'codex'
          ? 'Codex'
          : provider === 'deepseek'
            ? 'DeepSeek'
            : '',
      kind: provider === 'codex' ? 'codex' : 'openai',
      base_url: provider === 'deepseek' ? 'https://api.deepseek.com' : '',
      api_key: '',
      key_configured: false,
    }
    baseline.value = ''
    file.value = null
    replace.value = false
    open.value = true
  }
  function saved(account: AIConnection) {
    selected.value = copy(account)
    baseline.value = JSON.stringify(selected.value)
  }
  function close() {
    open.value = false
    file.value = null
  }
  return {
    selected,
    baseline,
    open,
    file,
    replace,
    dirty,
    edit,
    add,
    saved,
    close,
  }
}
export type AccountDraft = ReturnType<typeof useAccountDraft>
