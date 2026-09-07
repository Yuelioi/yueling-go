<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo, type JoinReviewConfig, type JoinReviewState } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const scope = ref<number | null>(0)
const state = ref<JoinReviewState | null>(null)
const mode = ref<JoinReviewConfig['mode']>('override')
const allow = ref('')
const deny = ref('')
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const groupError = ref('')
const saved = ref(false)
let generation = 0
const scopes = computed(() => [{ group_id: 0, group_name: '全局默认配置' }, ...groups.value])
const modes = computed(() => [
  ...(scope.value === 0 ? [] : [{ label: '继承全局', value: 'inherit' }]),
  { label: scope.value === 0 ? '启用全局规则' : '独立配置', value: 'override' },
  { label: '全部留人工', value: 'disabled' },
])
const parse = (text: string) => [...new Set(text.split(/[,，\n]/).map(s => s.trim().toLowerCase()).filter(Boolean))]
const draft = computed<JoinReviewConfig>(() => ({ mode: mode.value, allow: parse(allow.value), deny: parse(deny.value) }))
const effective = computed(() => mode.value === 'inherit' ? state.value?.global : draft.value)
const editable = computed(() => mode.value === 'override')
const dirty = computed(() => state.value && JSON.stringify(draft.value) !== JSON.stringify(state.value.config))
const preview = ref('')
const verdict = computed(() => {
  const cfg = effective.value
  const comment = preview.value.toLowerCase()
  if (!cfg || cfg.mode === 'disabled' || !comment) return '留管理员人工处理'
  if (cfg.deny.some(word => word && comment.includes(word))) return '自动拒绝：命中黑名单'
  if (cfg.allow.some(word => word === '*' || (word && comment.includes(word)))) return '自动通过：命中白名单'
  return '留管理员人工处理：未命中规则'
})
function apply(value: JoinReviewState) {
  state.value = value
  mode.value = value.config.mode
  allow.value = value.config.allow.join('\n')
  deny.value = value.config.deny.join('\n')
}
async function load() {
  const id = scope.value
  const token = ++generation
  state.value = null
  saved.value = false
  error.value = ''
  loading.value = false
  if (id === null) return
  loading.value = true
  try {
    const value = await api.joinReview(id)
    if (token === generation) apply(value)
  } catch (err) {
    if (token === generation) error.value = err instanceof Error ? err.message : '读取失败'
  } finally {
    if (token === generation) loading.value = false
  }
}
async function save() {
  const id = scope.value
  if (id === null || !state.value) return
  const token = generation
  saving.value = true
  error.value = ''
  saved.value = false
  try {
    const value = await api.setJoinReview(id, draft.value)
    if (token === generation) { apply(value); saved.value = true }
  } catch (err) {
    if (token === generation) error.value = err instanceof Error ? err.message : '保存失败'
  } finally { saving.value = false }
}
watch(scope, load, { immediate: true })
onMounted(async () => {
  try { groups.value = (await api.groups()).groups }
  catch { groupError.value = '群列表暂不可用，请确认 bot 在线后刷新；仍可管理全局配置。' }
})
</script>

<template>
  <section class="space-y-5">
    <PageHeader title="入群审核" eyebrow="Join review" description="统一管理入群理由的关键词名单，也可以为单个群单独设置。保存后立即生效。" icon="i-tabler-user-check" />
    <UAlert v-if="groupError" color="warning" variant="subtle" :description="groupError" />
    <GroupScopeSelect v-model="scope" :groups="scopes" title="配置范围" description="全局默认，或选择单个群" zero-label="未独立配置的群使用此规则" />
    <UAlert v-if="error" color="error" variant="subtle" :description="error" />
    <div v-if="loading" class="surface-panel p-6 text-sm text-zinc-400">正在读取规则…</div>
    <UButton v-if="!loading && !state" @click="load">重新读取</UButton>
    <template v-if="state && !loading">
      <section class="surface-panel space-y-5 p-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="font-semibold text-white">{{ scope === 0 ? '全局默认规则' : '本群审核方式' }}</h2>
          <USelect v-model="mode" :items="modes" value-key="value" class="w-52" :disabled="saving" aria-label="审核方式" />
        </div>
        <p class="text-sm leading-6 text-zinc-400">
          {{ mode === 'inherit' ? '本群跟随全局配置更新，下方显示当前全局名单。' : mode === 'disabled' ? '本范围内的申请全部留给管理员处理。已保存的名单保留，重新启用后可继续使用。' : scope === 0 ? '应用于所有选择继承全局的群。已有独立配置的群不受影响。' : '仅使用本群名单，完整覆盖全局的黑白名单。' }}
        </p>
        <div v-if="editable" class="grid gap-5 md:grid-cols-2">
          <UFormField label="白名单 · 通过词" description="每行一词，也支持中英文逗号。* 匹配任意非空理由。">
            <UTextarea v-model="allow" :rows="8" class="w-full" placeholder="交流&#10;学习" :disabled="saving" aria-label="白名单关键词" />
          </UFormField>
          <UFormField label="黑名单 · 拒绝词" description="每行一词，同时命中时优先拒绝。* 在黑名单中按普通字符匹配。">
            <UTextarea v-model="deny" :rows="8" class="w-full" placeholder="广告&#10;招商" :disabled="saving" aria-label="黑名单关键词" />
          </UFormField>
        </div>
        <div v-else-if="mode === 'inherit'" class="surface-inset space-y-2 p-4 text-sm text-zinc-300">
          <p v-if="state.global.mode === 'disabled'">全局当前为全部留人工。</p>
          <p>全局白名单：{{ state.global.allow.join('、') || '（空）' }}</p>
          <p>全局黑名单：{{ state.global.deny.join('、') || '（空）' }}</p>
        </div>
        <p class="text-xs leading-6 text-zinc-500">空理由、未命中规则均留人工。机器人需在线且拥有群管理权限，该群的加群审核插件需启用。每份名单最多 200 个词，每词最多 128 字。</p>
        <div class="flex flex-wrap items-center gap-3">
          <UButton :loading="saving" :disabled="!dirty" @click="save">保存配置</UButton>
          <UButton color="neutral" variant="ghost" :disabled="saving" @click="load">重新读取</UButton>
          <span v-if="dirty" class="text-sm text-amber-300">有未保存的修改，切换群前请先保存</span>
          <span v-else-if="saved" class="text-sm text-emerald-300">已保存，立即生效</span>
        </div>
      </section>
      <section class="surface-panel space-y-3 p-5">
        <h2 class="font-semibold text-white">试一下审核结果</h2>
        <p class="text-xs text-zinc-400">按当前表单预览，不会处理真实申请。</p>
        <UInput v-model="preview" class="w-full" placeholder="输入申请人的入群理由" aria-label="测试入群理由" />
        <p class="text-sm text-violet-300">{{ verdict }}</p>
      </section>
    </template>
  </section>
</template>
