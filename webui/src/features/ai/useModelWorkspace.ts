import { computed, onMounted, ref, watch, type Ref } from 'vue'
import {
  aiApi,
  type AIConnection,
  type AIModelProfile,
  type AIFeature,
} from '../../api/ai.ts'
import { modelRouteIssue } from '../../ai-models.ts'
import { useRequestScope } from '../../composables/useRequestScope.ts'
import { useModelDraft } from './modelDraft.ts'
import { useAccountDraft, type Provider } from './accountDraft.ts'
import { useCodexAuth } from './codexAuth.ts'
import { useCapabilityTest } from './capabilityTest.ts'
import { copy } from './shared.ts'

interface References {
  bindings: Record<string, string>
  conversation: string
  vision: string
}
export function useModelWorkspace(
  profiles: Ref<AIModelProfile[]>,
  connections: Ref<AIConnection[]>,
  references: () => References,
  requestSave: () => void,
  initialModelID?: string,
) {
  const requests = useRequestScope()
  const model = useModelDraft(connections)
  const account = useAccountDraft()
  const auth = useCodexAuth()
  const test = useCapabilityTest()
  const features = ref<AIFeature[]>([])
  const error = ref('')
  const sourceID = ref('')
  const providerOpen = ref(false)
  const pendingDelete = ref<{
    kind: 'model' | 'account'
    id: string
    name: string
  } | null>(null)
  const persistedAccounts = ref(new Set(connections.value.map((c) => c.id)))
  let afterSave: 'model' | 'account' | 'delete' | '' = ''
  let importAfterSave = false
  const source = computed(() =>
    connections.value.find((c) => c.id === sourceID.value),
  )
  const sourceProfiles = computed(() =>
    profiles.value.filter((p) =>
      source.value
        ? p.connection_id === source.value.id
        : !connections.value.some((c) => c.id === p.connection_id),
    ),
  )
  const unassigned = computed(() =>
    profiles.value.some(
      (p) => !connections.value.some((c) => c.id === p.connection_id),
    ),
  )
  const conflicts = computed(() =>
    profiles.value.filter((p) => modelRouteIssue(p, connections.value)),
  )
  const accountExists = computed(
    () =>
      !!account.selected.value &&
      persistedAccounts.value.has(account.selected.value.id),
  )
  const editing = computed(() => model.dirty.value || account.dirty.value)
  function usages(profile: Pick<AIModelProfile, 'id'>) {
    const config = references()
    return [
      ...new Set([
        ...features.value
          .filter((f) => config.bindings[f.id] === profile.id)
          .map((f) => f.name),
        ...(config.conversation === profile.id ? ['默认对话'] : []),
        ...(config.vision === profile.id ? ['默认读图'] : []),
      ]),
    ]
  }
  function chooseProvider(provider: Provider) {
    account.add(provider)
    providerOpen.value = false
  }
  function addModel(connection: AIConnection) {
    account.close()
    model.add(connection)
  }
  function editAccount(connection: AIConnection) {
    account.edit(connection)
    if (connection.kind === 'codex') void auth.read(connection)
  }
  function upsert<T extends { id: string }>(rows: T[], row: T) {
    return rows.some((p) => p.id === row.id)
      ? rows.map((p) => (p.id === row.id ? row : p))
      : [...rows, row]
  }
  function saveModel() {
    const profile = model.submit()
    if (!profile) return
    profiles.value = upsert(profiles.value, profile)
    afterSave = 'model'
    requestSave()
  }
  function saveAccount() {
    if (!account.selected.value) return
    const connection = copy(account.selected.value)
    connections.value = upsert(connections.value, connection)
    sourceID.value = connection.id
    afterSave = 'account'
    importAfterSave = !!account.file.value
    requestSave()
  }
  async function saved() {
    persistedAccounts.value = new Set(connections.value.map((c) => c.id))
    if (afterSave === 'model') {
      sourceID.value = model.selected.value?.connection_id || ''
      model.close()
    }
    if (afterSave === 'account' && account.selected.value) {
      const fresh = connections.value.find(
        (c) => c.id === account.selected.value?.id,
      )
      if (fresh) account.saved(fresh)
      const selected = account.selected.value
      if (importAfterSave && account.file.value) {
        if (
          await auth.upload(selected, account.file.value, account.replace.value)
        )
          account.file.value = null
      } else if (selected.kind === 'codex') await auth.read(selected)
    }
    afterSave = ''
    importAfterSave = false
  }
  function askDelete(
    kind: 'model' | 'account',
    row: { id: string; name: string },
  ) {
    pendingDelete.value = { kind, id: row.id, name: row.name }
  }
  function deleteItem() {
    const target = pendingDelete.value
    if (!target) return
    const used =
      target.kind === 'account'
        ? profiles.value
            .filter((p) => p.connection_id === target.id)
            .map((p) => p.name || p.model)
        : usages(target)
    if (used.length) {
      error.value = `仍被 ${used.join('、')} 使用。请先修改对应选择，再删除。`
      pendingDelete.value = null
      return
    }
    if (target.kind === 'account')
      connections.value = connections.value.filter((c) => c.id !== target.id)
    else profiles.value = profiles.value.filter((p) => p.id !== target.id)
    pendingDelete.value = null
    model.close()
    account.close()
    afterSave = 'delete'
    requestSave()
  }
  watch(
    connections,
    () => {
      if (
        !connections.value.some((c) => c.id === sourceID.value) &&
        !(sourceID.value === '__unassigned__' && unassigned.value)
      )
        sourceID.value = connections.value[0]?.id || '__unassigned__'
    },
    { immediate: true },
  )
  onMounted(async () => {
    const current = requests.begin('features')
    try {
      const result = await aiApi.aiFeatures({ signal: current.signal })
      if (current()) features.value = result.features
    } catch {
      if (current())
        error.value = '功能引用加载失败，删除操作仍会由服务器校验。'
    }
    for (const connection of connections.value.filter(
      (c) => c.kind === 'codex',
    ))
      void auth.read(connection)
    const requested = profiles.value.find((p) => p.id === initialModelID)
    if (requested) {
      sourceID.value = requested.connection_id || '__unassigned__'
      model.edit(requested)
    }
  })
  function discard() {
    model.close()
    account.close()
    pendingDelete.value = null
  }
  return {
    model,
    account,
    auth,
    test,
    error,
    sourceID,
    providerOpen,
    pendingDelete,
    source,
    sourceProfiles,
    unassigned,
    conflicts,
    accountExists,
    editing,
    usages,
    chooseProvider,
    addModel,
    editAccount,
    saveModel,
    saveAccount,
    saved,
    askDelete,
    deleteItem,
    discard,
  }
}
