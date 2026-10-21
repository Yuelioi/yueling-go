<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  aiApi,
  type AIModelProfile,
  type AIConnection,
  type AIFeature,
} from '../api'
import {
  accountDestination,
  modelCapabilities,
  modelRouteIssue,
  profileOptionLabel,
} from '../ai-models'
const bindings = defineModel<Record<string, string>>({ required: true })
const props = defineProps<{
  profiles: AIModelProfile[]
  connections: AIConnection[]
  disabled?: boolean
}>()
const features = ref<AIFeature[]>([])
const error = ref('')
const primaryIDs = [
  'conversation',
  'vision',
  'image_generation',
  'image_edit',
  'search',
]
const primary = computed(() =>
  primaryIDs.flatMap((id) => features.value.filter((f) => f.id === id)),
)
const secondary = computed(() =>
  features.value.filter((f) => !primaryIDs.includes(f.id)),
)
const descriptions: Record<string, string> = {
  conversation: '机器人对话与操作，使用此处明确选择的模型。',
  vision: '识别附件和引用图片里的画面、文字。',
  image_generation: '根据纯文字描述画图。',
  image_edit: '将原图与描述一起提交，用于重绘、换风格和改图。',
  search: '独立调用 DeepSeek Search / Fetch。',
}
const icons: Record<string, string> = {
  conversation: 'i-tabler-message-circle',
  vision: 'i-tabler-eye',
  image_generation: 'i-tabler-palette',
  image_edit: 'i-tabler-photo-edit',
  search: 'i-tabler-world-search',
}
function selectedID(f: AIFeature) {
  return bindings.value?.[f.id] || 'off'
}
function profile(f: AIFeature) {
  return props.profiles.find((p) => p.id === selectedID(f))
}
function issue(f: AIFeature) {
  const id = selectedID(f)
  if (!id || id === 'off') return ''
  const p = profile(f)
  if (
    !p ||
    p.disabled ||
    !f.capabilities.every((c) => modelCapabilities(p).includes(c))
  )
    return '当前模型已停用、被删除或缺少所需能力，请重新选择。'
  return modelRouteIssue(p, props.connections)
}
function destination(f: AIFeature) {
  const p = profile(f)
  if (!p) return '未启用'
  const c = props.connections.find((c) => c.id === p.connection_id)
  return `${p.model || 'Search / Fetch'} → ${c?.name || '未选择账号'} · ${accountDestination(c)}`
}
function options(f: AIFeature) {
  return [
    { label: '未启用', value: 'off' },
    ...props.profiles
      .filter(
        (p) =>
          !p.disabled &&
          f.capabilities.every((c) => modelCapabilities(p).includes(c)),
      )
      .map((p) => ({
        label: profileOptionLabel(p, props.connections),
        value: p.id,
        disabled: !!modelRouteIssue(p, props.connections),
      })),
  ]
}
function select(f: AIFeature, value: string) {
  bindings.value = { ...bindings.value, [f.id]: value }
}
async function load() {
  try {
    features.value = (await aiApi.aiFeatures()).features
    error.value = ''
  } catch {
    error.value = '功能列表加载失败，请重试。'
  }
}
onMounted(load)
</script>
<template>
  <div class="space-y-5 p-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="font-semibold">功能分配</h3>
        <p class="mt-1 text-sm text-muted">
          每项功能都能看到实际使用的模型、账号和请求目标。
        </p>
      </div>
      <UButton
        to="/ai/models"
        color="neutral"
        variant="soft"
        icon="i-tabler-plug"
        >管理服务与模型</UButton
      >
    </div>
    <UAlert
      v-if="!profiles.length"
      color="neutral"
      variant="soft"
      description="先连接 AI 服务并添加模型，再为下列功能选择模型。"
      :actions="[{ label: '连接 AI 服务', to: '/ai/models' }]"
    />
    <UAlert
      icon="i-tabler-alert-circle"
      v-if="error"
      color="error"
      variant="soft"
      :description="error"
      :actions="[{ label: '重试', onClick: load }]"
    />
    <div class="divide-y divide-default rounded-xl border border-default">
      <div
        v-for="f in primary"
        :key="f.id"
        class="min-w-0 space-y-3 p-4 sm:p-5"
      >
        <div
          class="grid min-w-0 gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(14rem,22rem)] xl:items-center"
        >
          <div class="flex min-w-0 items-start gap-3">
            <UIcon
              :name="icons[f.id]"
              class="mt-0.5 size-5 shrink-0 text-primary"
            />
            <div>
              <h4 class="text-sm font-medium">{{ f.name }}</h4>
              <p class="mt-1 text-xs leading-5 text-muted">
                {{ descriptions[f.id] }}
              </p>
            </div>
          </div>
          <USelect
            :aria-label="`${f.name}使用的模型`"
            :model-value="selectedID(f) || 'off'"
            :items="options(f)"
            class="w-full min-w-0"
            :disabled="disabled"
            @update:model-value="select(f, String($event))"
          />
        </div>
        <div
          v-if="profile(f) && selectedID(f) !== 'off'"
          class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-muted/40 px-3 py-2"
        >
          <p class="min-w-0 flex-1 break-all text-xs leading-5 text-muted">
            {{ destination(f) }}
          </p>
          <ULink
            :to="`/ai/models?model=${encodeURIComponent(profile(f)!.id)}`"
            class="shrink-0 text-xs text-primary"
            >配置模型</ULink
          >
        </div>
        <p v-if="issue(f)" class="text-sm leading-6 text-error">
          {{ issue(f) }}
        </p>
      </div>
    </div>
    <UCollapsible class="space-y-4"
      ><UButton
        color="neutral"
        variant="link"
        trailing-icon="i-tabler-chevron-down"
        >其他文字功能的模型分配</UButton
      ><template #content
        ><div class="space-y-4">
          <p class="text-sm text-muted">
            每项功能单独选择模型或关闭，保存后按此处绑定执行。
          </p>
          <div class="grid gap-5 sm:grid-cols-2">
            <UFormField
              v-for="f in secondary"
              :key="f.id"
              :label="f.name"
              :description="issue(f) || destination(f)"
              ><USelect
                :aria-label="`${f.name}使用的模型`"
                :model-value="selectedID(f)"
                :items="options(f)"
                class="w-full"
                :disabled="disabled"
                @update:model-value="select(f, String($event))"
            /></UFormField>
          </div></div></template
    ></UCollapsible>
  </div>
</template>
