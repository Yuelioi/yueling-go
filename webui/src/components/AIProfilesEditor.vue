<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type AIModelProfile, type AIConnection, type AIFeature } from '../api'
import AIConnectionFields from './AIConnectionFields.vue'
const profiles = defineModel<AIModelProfile[]>({ required: true })
const connections = defineModel<AIConnection[]>('connections', { required: true })
const props = defineProps<{ saveError?: string; disabled?: boolean; unsaved?: boolean; saving?: boolean; bindings: Record<string, string>; conversation: string; vision: string }>()
const emit = defineEmits<{ save: [] }>()
const features = ref<AIFeature[]>([])
const error = ref('')
const selectedID = ref('')
const selectedAccountID = ref('')
const modelOpen = ref(false)
const accountOpen = ref(false)
const testOpen = ref(false)
const testing = ref(false)
const testCapability = ref('chat')
const testResult = ref('')
const testImage = ref('')
const auth = ref<Record<string, string>>({})
const replacing = ref(false)
const authFile = ref<File | null>(null)
const busyAuth = ref(false)
const pendingDelete = ref<{ kind: 'model' | 'account'; id: string; name: string } | null>(null)
const selected = computed(() => profiles.value.find(p => p.id === selectedID.value))
const account = computed(() => connections.value.find(c => c.id === selectedAccountID.value))
const modelAccount = computed(() => connections.value.find(c => c.id === selected.value?.connection_id))
const caps = [
  { value: 'chat', label: '聊天', description: '文本对话、总结和翻译' },
  { value: 'tools', label: '工具调用', description: 'Bot 对话与操作需要此能力' },
  { value: 'vision', label: '读图', description: '识别图片内容与文字' },
  { value: 'image_generation', label: '生图', description: '根据文字生成图片' },
  { value: 'image_edit', label: '改图', description: '参考原图编辑或换风格' },
  { value: 'structured_output', label: '结构化输出', description: '返回 JSON 等固定格式' },
]
const connectionOptions = computed(() => connections.value.map(c => ({ label: c.name || '未命名账号', value: c.id })))
const testOptions = computed(() => caps.filter(c => selected.value && capabilities(selected.value).includes(c.value)))
function capabilities(p: AIModelProfile) { return p.capabilities ?? ['chat', 'tools', ...(p.vision ? ['vision'] : [])] }
function setCapability(p: AIModelProfile, capability: string, checked: boolean) {
  p.capabilities = capabilities(p).filter(c => c !== capability)
  if (checked) p.capabilities.push(capability)
  p.vision = p.capabilities.includes('vision')
}
function connectionName(p: AIModelProfile) { return connections.value.find(c => c.id === p.connection_id)?.name || '未选择账号' }
function isCodex(p: AIModelProfile) { return connections.value.find(c => c.id === p.connection_id)?.kind === 'codex' }
// HTTP 管理入口也需支持新增；randomUUID 仅在安全上下文可用。
function newID() {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), b => b.toString(16).padStart(2, '0')).join('')
}
function addConnection(forModel = false) {
  const c: AIConnection = { id: newID(), name: '新账号', kind: 'openai', base_url: '', api_key: '', key_configured: false }
  connections.value.push(c)
  if (forModel && selected.value) selected.value.connection_id = c.id
  else { selectedAccountID.value = c.id; accountOpen.value = true }
}
function addModel() {
  const p: AIModelProfile = { id: newID(), name: '', model: '', connection_id: connections.value[0]?.id ?? '', capabilities: ['chat', 'tools'], disabled: false, max_tokens: 0, timeout_seconds: 0, reasoning_effort: '', base_url: '', api_key: '', vision: false, key_configured: false }
  profiles.value.push(p); selectedID.value = p.id
  if (!connections.value.length) addConnection(true)
  modelOpen.value = true
}
function edit(p: AIModelProfile) { selectedID.value = p.id; modelOpen.value = true }
function editAccount(c: AIConnection) { selectedAccountID.value = c.id; authFile.value = null; replacing.value = false; accountOpen.value = true }
function openTest(p: AIModelProfile) {
  selectedID.value = p.id; testCapability.value = capabilities(p)[0] ?? 'chat'; testResult.value = ''; testImage.value = ''; testOpen.value = true
}
async function test() {
  if (!selected.value) return
  testing.value = true; testResult.value = ''; testImage.value = ''
  try { const r = await api.testAIProfile(selected.value.id, testCapability.value); testImage.value = r.image_url || ''; testResult.value = `${caps.find(c => c.value === testCapability.value)?.label || testCapability.value}${r.status === 'verified' ? '测试通过' : '测试失败'}：${r.message}（${r.duration_ms}ms）` }
  catch (e) { testResult.value = e instanceof Error ? e.message : '测试失败' }
  finally { testing.value = false }
}
function deleteItem() {
  const target = pendingDelete.value
  if (!target) return
  const used = target.kind === 'account' ? profiles.value.filter(p => p.connection_id === target.id).map(p => p.name || p.model) : [
    ...features.value.filter(f => props.bindings?.[f.id] === target.id).map(f => f.name),
    ...(props.conversation === target.id ? ['默认对话'] : []), ...(props.vision === target.id ? ['默认读图'] : []),
  ]
  if (used.length) error.value = `仍被 ${used.join('、')} 使用。请先修改对应选择，再删除。`
  else if (target.kind === 'account') connections.value = connections.value.filter(c => c.id !== target.id)
  else profiles.value = profiles.value.filter(p => p.id !== target.id)
  pendingDelete.value = null
}
async function status(c: AIConnection) {
  try { const r = await api.codexStatus(c.id); auth.value[c.id] = r.imported ? '登录态已导入，可以配置模型并分配功能；测试为可选诊断。' : '尚未导入登录态。' }
  catch (e) { auth.value[c.id] = e instanceof Error ? e.message : '读取失败' }
}
async function importAuth() {
  if (!account.value || !authFile.value) return
  const c = account.value
  busyAuth.value = true
  try {
    if (authFile.value.size > 65536) throw new Error('登录文件不能超过64KB')
    await api.importCodex(c.id, await authFile.value.text(), replacing.value)
    auth.value[c.id] = '登录态已导入。服务器将维护后续刷新。'; authFile.value = null
  } catch (e) { auth.value[c.id] = e instanceof Error ? e.message : '导入失败' }
  finally { busyAuth.value = false }
}
watch([profiles, connections], () => { testResult.value = '' }, { deep: true })
onMounted(async () => { try { features.value = (await api.aiFeatures()).features } catch { error.value = '功能引用加载失败，删除操作仍会由服务器校验。' } })
defineExpose({ addModel, saved: () => { modelOpen.value = false } })
</script>
<template>
  <div class="space-y-5">
    <UAlert variant="soft" v-if="error" color="error" :description="error" closable @update:open="error = ''" />
    <UCard v-if="!profiles.length" :ui="{ body: 'flex flex-col items-center gap-4 py-12 text-center' }">
      <UIcon name="i-tabler-cpu" class="size-10 text-muted" />
      <div><h3 class="font-semibold">添加第一个 AI 模型</h3><p class="mt-2 text-sm text-muted">填写账号和模型名称，选择能力，再到 AI 配置中分配给各功能。</p></div>
      <UButton icon="i-tabler-plus" :disabled="disabled" @click="addModel">添加模型</UButton>
    </UCard>
    <div v-else class="grid gap-4 md:grid-cols-2">
      <UCard v-for="p in profiles" :key="p.id" :ui="{ body: 'space-y-4', footer: 'flex items-center justify-between gap-2' }">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0"><h3 class="truncate font-semibold text-highlighted">{{ p.name || '未命名模型' }}</h3><p class="mt-1 break-all text-sm text-muted">{{ p.model || '尚未填写模型 ID' }}</p></div>
          <UBadge :color="p.disabled ? 'neutral' : 'success'" variant="subtle">{{ p.disabled ? '已停用' : '已启用' }}</UBadge>
        </div>
        <p class="text-sm text-muted"><UIcon name="i-tabler-key" class="mr-1 inline size-4" />{{ connectionName(p) }} <span v-if="isCodex(p)">· Codex 订阅</span></p>
        <div class="flex flex-wrap gap-2"><UBadge v-for="cap in caps.filter(c => capabilities(p).includes(c.value))" :key="cap.value" color="neutral" variant="soft">{{ cap.label }}</UBadge></div>
        <template #footer>
          <div class="flex gap-2"><UButton color="neutral" variant="soft" icon="i-tabler-pencil" :disabled="disabled" @click="edit(p)">编辑</UButton><UButton variant="ghost" icon="i-tabler-flask" :disabled="disabled || unsaved || p.disabled" @click="openTest(p)">测试能力</UButton></div>
          <UButton color="error" variant="ghost" icon="i-tabler-trash" :aria-label="`删除模型 ${p.name}`" :disabled="disabled" @click="pendingDelete = { kind: 'model', id: p.id, name: p.name || p.model }" />
        </template>
      </UCard>
    </div>
    <p v-if="profiles.length" class="text-sm text-muted">保存模型后，前往 <ULink to="/ai" class="font-medium text-primary">AI 配置</ULink> 选择各功能使用的模型。能力由你直接勾选并保存，测试是可选诊断，不影响保存和功能分配。</p>
    <UCollapsible class="rounded-xl border border-default">
      <UButton block color="neutral" variant="ghost" icon="i-tabler-key" trailing-icon="i-tabler-chevron-down" class="justify-between p-4">账号与凭据 · {{ connections.length }} 个</UButton>
      <template #content>
        <div class="space-y-3 border-t border-default p-4">
          <p class="text-sm text-muted">多个模型可以共用一个账号。修改凭据会影响使用它的全部模型。</p>
          <div v-for="c in connections" :key="c.id" class="flex items-center justify-between gap-3 rounded-lg bg-elevated/50 p-3">
            <div class="min-w-0"><p class="truncate font-medium">{{ c.name }}</p><p class="text-xs text-muted">{{ c.kind === 'codex' ? 'Codex 订阅' : 'OpenAI 兼容 API' }} · {{ profiles.filter(p => p.connection_id === c.id).length }} 个模型</p></div>
            <div class="flex shrink-0 gap-1"><UButton color="neutral" variant="soft" :disabled="disabled" @click="editAccount(c)">{{ c.kind === 'codex' ? '管理登录态' : '编辑账号' }}</UButton><UButton color="error" variant="ghost" icon="i-tabler-trash" :aria-label="`删除账号 ${c.name}`" :disabled="disabled" @click="pendingDelete = { kind: 'account', id: c.id, name: c.name }" /></div>
          </div>
          <UButton variant="soft" icon="i-tabler-plus" :disabled="disabled || connections.length >= 32" @click="addConnection()">添加账号</UButton>
        </div>
      </template>
    </UCollapsible>

    <USlideover v-model:open="modelOpen" title="编辑模型" description="填写模型信息和能力；保存后即可分配给 AI 功能。" :ui="{ content: 'sm:max-w-2xl' }" :dismissible="!disabled">
      <template #body>
        <div v-if="selected" class="space-y-6">
          <UAlert variant="soft" v-if="saveError" color="error" :description="saveError" />
          <UAlert v-if="unsaved" color="warning" variant="soft" description="修改尚未保存。关闭面板会保留草稿，也可在页面顶部保存。" />
          <div class="grid gap-4 sm:grid-cols-2">
            <UFormField label="模型显示名称" required><UInput v-model="selected.name" class="w-full" placeholder="例如：日常对话" :disabled="disabled" /></UFormField>
            <UFormField label="模型 ID" required description="使用服务商提供的准确模型 ID。"><UInput v-model="selected.model" class="w-full" placeholder="例如：服务商提供的模型名称" :disabled="disabled" /></UFormField>
          </div>
          <UFormField label="使用账号" required>
            <div class="flex gap-2"><USelect v-model="selected.connection_id" :items="connectionOptions" placeholder="选择账号" class="min-w-0 flex-1" :disabled="disabled" /><UButton color="neutral" variant="soft" icon="i-tabler-plus" :disabled="disabled || connections.length >= 32" @click="addConnection(true)">新账号</UButton></div>
          </UFormField>
          <UCollapsible v-if="modelAccount" :key="modelAccount.id" :default-open="!modelAccount.base_url && !modelAccount.key_configured" class="space-y-3">
            <UButton color="neutral" variant="link" trailing-icon="i-tabler-chevron-down">账号地址与凭据</UButton>
            <template #content><AIConnectionFields v-model="connections[connections.findIndex(c => c.id === selected!.connection_id)]!" :disabled="disabled" /></template>
          </UCollapsible>
          <section class="space-y-3">
            <h3 class="font-semibold">模型能力</h3><p class="text-sm text-muted">直接勾选所需能力，无需先测试。Bot 对话需同时支持聊天和工具调用。</p>
            <p v-if="isCodex(selected)" class="text-sm text-muted">Codex 订阅通过内置图片工具生图和改图；暂不支持结构化输出约束。</p>
            <div class="grid gap-4 sm:grid-cols-2"><UCheckbox v-for="cap in caps" :key="cap.value" :model-value="capabilities(selected).includes(cap.value)" :label="cap.label" :description="cap.description" :disabled="disabled || (isCodex(selected) && !capabilities(selected).includes(cap.value) && cap.value === 'structured_output')" @update:model-value="setCapability(selected!, cap.value, $event === true)" /></div>
          </section>
          <UCollapsible class="space-y-3">
            <UButton color="neutral" variant="link" trailing-icon="i-tabler-chevron-down">高级参数（可选）</UButton>
            <template #content><div class="grid gap-4 sm:grid-cols-2">
              <UFormField v-if="capabilities(selected).includes('chat')" label="推理强度"><USelect :model-value="selected.reasoning_effort || '__default__'" @update:model-value="selected.reasoning_effort = $event === '__default__' ? '' : String($event)" :items="[{ label: '使用默认', value: '__default__' }, ...['none','minimal','low','medium','high','xhigh'].map(value => ({ label: value, value }))]" class="w-full" :disabled="disabled" /></UFormField>
              <UFormField v-if="!isCodex(selected) && capabilities(selected).includes('chat')" label="最大输出 Token" description="0 表示使用 AI 配置中的默认值。"><UInput v-model.number="selected.max_tokens" type="number" :min="0" :max="200000" class="w-full" :disabled="disabled" /></UFormField>
              <UFormField label="超时（秒）" description="0 使用默认超时。"><UInput v-model.number="selected.timeout_seconds" type="number" :min="0" :max="600" class="w-full" :disabled="disabled" /></UFormField>
              <UFormField label="停用模型"><USwitch v-model="selected.disabled" :disabled="disabled" /></UFormField>
            </div></template>
          </UCollapsible>
        </div>
      </template>
      <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="soft" :disabled="disabled" @click="modelOpen = false">返回列表</UButton><UButton icon="i-tabler-device-floppy" :loading="saving" :disabled="!unsaved || disabled" @click="emit('save')">保存修改</UButton></div></template>
    </USlideover>

    <UModal v-model:open="accountOpen" title="账号与凭据" description="同一账号可供多个模型使用。" :ui="{ content: 'sm:max-w-2xl' }" :dismissible="!disabled && !busyAuth">
      <template #body><div v-if="account" class="space-y-5">
        <UAlert variant="soft" v-if="saveError" color="error" :description="saveError" />
        <UBadge :color="unsaved ? 'warning' : 'success'">{{ unsaved ? '未保存' : '已保存' }}</UBadge>
        <AIConnectionFields v-model="connections[connections.findIndex(c => c.id === selectedAccountID)]!" :disabled="disabled || busyAuth" />
        <template v-if="account.kind === 'codex'">
          <UAlert variant="soft" v-if="unsaved" color="warning" description="请先保存账号，再导入登录态。" />
          <UFileUpload v-model="authFile" label="选择 Codex 登录文件" description="auth.json，最大 64 KB" accept=".json,application/json" :disabled="disabled || unsaved || busyAuth" />
          <UCheckbox v-model="replacing" label="替换已有登录态" description="重新授权时使用；请先暂停该账号的请求。" :disabled="disabled || busyAuth" />
          <div class="flex gap-2"><UButton :loading="busyAuth" :disabled="disabled || unsaved || !authFile" @click="importAuth">导入登录态</UButton><UButton variant="soft" :disabled="disabled || unsaved || busyAuth" @click="status(account)">检查状态</UButton></div>
          <UAlert variant="soft" v-if="auth[account.id]" color="neutral" :description="auth[account.id]" />
        </template>
      </div></template>
      <template #footer><div class="flex w-full justify-end gap-2"><UButton color="neutral" variant="soft" :disabled="disabled || busyAuth" @click="accountOpen = false">返回列表</UButton><UButton :loading="saving" :disabled="!unsaved || disabled || busyAuth" @click="emit('save')">保存修改</UButton></div></template>
    </UModal>
    <UModal v-model:open="testOpen" title="测试模型能力" :description="selected?.name || '验证当前模型'" :dismissible="!testing">
      <template #body><div class="space-y-4"><UFormField label="要测试的能力"><USelect v-model="testCapability" :items="testOptions" class="w-full" :disabled="testing" /></UFormField><UAlert color="warning" variant="soft" description="测试仅检查当前所选能力，不影响模型保存和功能分配。会消耗账号额度；生图和改图可能需要几分钟。" /><UAlert variant="soft" v-if="testResult" color="neutral" :description="testResult" /><img v-if="testImage" :src="testImage" alt="本次能力测试生成的图片" class="max-h-80 w-full rounded-lg object-contain" /></div></template>
      <template #footer><UButton :loading="testing" :disabled="unsaved || disabled || !testOptions.length" @click="test">开始测试</UButton></template>
    </UModal>
    <UModal :open="!!pendingDelete" :title="`删除${pendingDelete?.kind === 'model' ? '模型' : '账号'}？`" :description="pendingDelete?.name" @update:open="!$event && (pendingDelete = null)">
      <template #body><p class="text-sm text-muted">删除后需要保存。正在使用的模型或账号需先解除关联。</p></template>
      <template #footer><UButton color="neutral" variant="soft" @click="pendingDelete = null">取消</UButton><UButton color="error" :disabled="disabled" @click="deleteItem">删除</UButton></template>
    </UModal>
  </div>
</template>
