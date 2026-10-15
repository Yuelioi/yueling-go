<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type AIModelProfile, type AIFeature } from '../api'
// 组件选项不能使用空字符串；仅在界面层映射默认值。
const defaultChoice = '__default__'
const decodeChoice = (value: string) => value === defaultChoice ? '' : value
const bindings = defineModel<Record<string, string>>({ required: true })
const conversation = defineModel<string>('conversation', { required: true })
const vision = defineModel<string>('vision', { required: true })
const props = defineProps<{ profiles: AIModelProfile[]; disabled?: boolean }>()
const features = ref<AIFeature[]>([])
const error = ref('')
const groups = computed(() => [
  { title: '对话与图片', ids: ['conversation', 'vision', 'image_generation', 'image_edit'] },
  { title: '整理与记忆', ids: ['summary', 'digest', 'memory', 'compression', 'knowledge'] },
  { title: '其他功能', ids: ['proactive', 'translation', 'bilibili', 'zssm'] },
].map(g => ({ ...g, features: features.value.filter(f => g.ids.includes(f.id)) })))
function options(required: string[], feature?: string) {
  const defaults = feature ? [
    { label: feature.startsWith('image_') ? '未选择（不启用）' : feature === 'vision' ? '使用默认读图模型' : '使用默认对话模型', value: defaultChoice },
    { label: '关闭此功能', value: 'off' },
  ] : [{ label: '未选择', value: defaultChoice }]
  return [...defaults, ...props.profiles.filter(p => !p.disabled && required.every(c => (p.capabilities ?? ['chat', 'tools', ...(p.vision ? ['vision'] : [])]).includes(c))).map(p => ({ label: p.name || p.model, value: p.id }))]
}
function effective(f: AIFeature) {
  const binding = bindings.value?.[f.id] ?? ''
  const id = binding || (f.id.startsWith('image_') ? '' : f.id === 'vision' ? vision.value : conversation.value)
  return !id || id === 'off' ? '当前未启用' : `当前使用：${props.profiles.find(p => p.id === id)?.name ?? '模型已不可用'}`
}
async function load() { try { features.value = (await api.aiFeatures()).features; error.value = '' } catch { error.value = '功能列表加载失败，请重试。' } }
onMounted(load)
</script>
<template>
  <div class="space-y-6 p-5">
    <div class="flex flex-wrap items-center justify-between gap-3"><div><h3 class="font-semibold">功能使用哪个模型</h3><p class="mt-1 text-sm text-muted">只列出具备所需能力的模型。先设置默认，再按需为功能单独选择。</p></div><UButton to="/ai/models" color="neutral" variant="soft" icon="i-tabler-cpu">管理模型</UButton></div>
    <UAlert variant="soft" v-if="!profiles.length" color="warning" description="还没有模型。请先到「AI 模型」添加并保存，再回来分配。" />
    <UAlert variant="soft" v-if="error" color="error" :description="error" :actions="[{ label: '重试', onClick: load }]" />
    <UCard :ui="{ body: 'grid gap-5 sm:grid-cols-2' }" variant="subtle">
      <UFormField label="默认对话模型" description="用于没有单独选择模型的文本功能。"><USelect :model-value="conversation || defaultChoice" @update:model-value="conversation = decodeChoice(String($event))" :items="options(['chat','tools'])" class="w-full" :disabled="disabled" /></UFormField>
      <UFormField label="默认读图模型" description="需要同时支持聊天和读图。"><USelect :model-value="vision || defaultChoice" @update:model-value="vision = decodeChoice(String($event))" :items="options(['chat','vision'])" class="w-full" :disabled="disabled" /></UFormField>
    </UCard>
    <section v-for="group in groups" :key="group.title" class="space-y-3">
      <h4 class="text-sm font-semibold">{{ group.title }}</h4>
      <div class="grid gap-5 sm:grid-cols-2"><UFormField v-for="f in group.features" :key="f.id" :label="f.name" :description="effective(f)"><USelect :model-value="bindings?.[f.id] || defaultChoice" :items="options(f.capabilities, f.id)" class="w-full" :disabled="disabled" @update:model-value="bindings = { ...bindings, [f.id]: decodeChoice(String($event)) }" /></UFormField></div>
    </section>
  </div>
</template>
