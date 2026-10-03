<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { AIModelProfile } from '../api'

const profiles = defineModel<AIModelProfile[]>({ required: true })
const conversation = defineModel<string>('conversation', { required: true })
const vision = defineModel<string>('vision', { required: true })
const expanded = ref<Record<string, boolean>>({})
watch(
  profiles,
  (list) => {
    for (const p of list)
      if (!(p.id in expanded.value)) expanded.value[p.id] = !p.key_configured
  },
  { immediate: true, deep: true },
)
const props = defineProps<{ disabled?: boolean }>()
const none = '~none~'
const conversationSelection = computed({
  get: () => conversation.value || none,
  set: (value: string) => {
    conversation.value = value === none ? '' : value
  },
})
const visionSelection = computed({
  get: () => vision.value || none,
  set: (value: string) => {
    vision.value = value === none ? '' : value
  },
})
const options = computed(() => [
  { label: '不启用', value: none },
  ...profiles.value.map((p) => ({
    label: p.name || '未命名模型',
    value: p.id,
  })),
])
const visionOptions = computed(() => [
  { label: '不启用', value: none },
  ...profiles.value
    .filter((p) => p.vision)
    .map((p) => ({ label: p.name || '未命名模型', value: p.id })),
])
function setVision(p: AIModelProfile, enabled: boolean) {
  p.vision = enabled
  if (!enabled && vision.value === p.id) vision.value = ''
}
function remove(id: string) {
  profiles.value = profiles.value.filter((p) => p.id !== id)
  if (conversation.value === id) conversation.value = ''
  if (vision.value === id) vision.value = ''
}
</script>

<template>
  <section class="min-w-0 space-y-5 p-5">
    <div class="grid min-w-0 gap-5 @min-[36rem]:grid-cols-2">
      <UFormField label="对话模型">
        <USelect
          v-model="conversationSelection"
          :items="options"
          :disabled="props.disabled"
          class="w-full"
          aria-label="选择对话模型"
        />
      </UFormField>
      <UFormField label="视觉模型">
        <USelect
          v-model="visionSelection"
          :items="visionOptions"
          :disabled="props.disabled"
          class="w-full"
          aria-label="选择视觉模型"
        />
      </UFormField>
    </div>
    <p class="text-xs leading-5 text-muted">
      支持视觉的 Profile 可以同时用于对话与图片识别，也可以分别选择两套配置。
    </p>
    <UAlert
      v-if="!profiles.length"
      color="neutral"
      variant="soft"
      icon="i-tabler-sparkles"
      description="还没有模型配置，点击右上角加号添加 Profile。"
    />
    <UCard
      v-for="p in profiles"
      :key="p.id"
      :ui="{ header: 'p-2 sm:p-2', body: 'p-0 sm:p-0' }"
    >
      <template #header>
        <div
          class="flex min-w-0 items-center gap-2 rounded-lg bg-elevated/50 p-2"
        >
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            :trailing-icon="
              expanded[p.id] ? 'i-tabler-chevron-up' : 'i-tabler-chevron-down'
            "
            @click="expanded[p.id] = !expanded[p.id]"
            :aria-expanded="expanded[p.id]"
            :aria-controls="`profile-${p.id}`"
            class="min-w-0 flex-1 justify-between"
            :ui="{ label: 'truncate' }"
            :label="p.name || '未命名模型'"
            :aria-label="`展开或收起 ${p.name || '模型'} 配置`"
          />
          <UBadge
            v-if="conversation === p.id"
            color="primary"
            variant="soft"
            size="xs"
            >对话</UBadge
          >
          <UBadge
            v-if="vision === p.id"
            color="primary"
            variant="soft"
            size="xs"
            >视觉</UBadge
          >
          <UButton
            type="button"
            color="error"
            variant="ghost"
            icon="i-tabler-trash"
            :disabled="props.disabled"
            :aria-label="`删除 ${p.name || '模型'} 配置`"
            @click="remove(p.id)"
          />
        </div>
      </template>
      <UCollapsible :open="expanded[p.id]">
        <template #content>
          <div :id="`profile-${p.id}`">
            <div class="grid min-w-0 gap-5 p-4 @min-[36rem]:grid-cols-2">
              <UFormField label="配置名称"
                ><UInput
                  v-model="p.name"
                  :disabled="props.disabled"
                  maxlength="100"
                  placeholder="例如 DeepSeek、图片识别"
                  class="w-full"
                  :aria-label="`${p.name}配置名称`"
              /></UFormField>
              <UFormField label="模型名称"
                ><UInput
                  v-model="p.model"
                  :disabled="props.disabled"
                  maxlength="256"
                  placeholder="服务商提供的模型 ID"
                  class="w-full"
                  :aria-label="`${p.name}模型名称`"
              /></UFormField>
              <UFormField label="API 地址"
                ><UInput
                  v-model="p.base_url"
                  :disabled="props.disabled"
                  placeholder="https://api.example.com/v1"
                  class="w-full"
                  :aria-label="`${p.name}API 地址`"
              /></UFormField>
              <UFormField label="API Key">
                <UInput
                  v-model="p.api_key"
                  type="password"
                  autocomplete="new-password"
                  :disabled="props.disabled || p.clear_key"
                  :placeholder="
                    p.key_configured ? '已设置，留空保留' : '尚未设置'
                  "
                  class="w-full"
                  :aria-label="`${p.name}API Key`"
                />
                <UCheckbox
                  v-if="p.key_configured"
                  v-model="p.clear_key"
                  :disabled="props.disabled"
                  class="mt-2"
                  label="清除已保存的 Key"
                />
              </UFormField>
              <UCheckbox
                :model-value="p.vision"
                :disabled="props.disabled"
                label="支持视觉"
                description="勾选后可在上方选为视觉模型。"
                @update:model-value="setVision(p, $event === true)"
              />
            </div>
          </div>
        </template>
      </UCollapsible>
    </UCard>
  </section>
</template>
