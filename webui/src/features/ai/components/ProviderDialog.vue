<script setup lang="ts">
import type { Provider } from '../accountDraft.ts'
const providerOpen = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ choose: [provider: Provider] }>()
</script>
<template>
  <UModal
    v-model:open="providerOpen"
    title="连接 AI 服务"
    description="先选择接入来源。不同来源使用独立账号和凭据。"
  >
    <template #body
      ><div class="space-y-3">
        <button
          v-for="provider in [
            {
              id: 'codex',
              title: 'Codex 订阅',
              text: '导入 auth.json，使用服务器上的 Codex 登录态。',
              icon: 'i-tabler-brand-openai',
            },
            {
              id: 'deepseek',
              title: 'DeepSeek API',
              text: '填写 DeepSeek API Key，用于对话、读图或联网搜索。',
              icon: 'i-tabler-world-search',
            },
            {
              id: 'api',
              title: '其他 OpenAI 兼容 API',
              text: '填写服务商地址与 API Key，可接入图片生成服务。',
              icon: 'i-tabler-api',
            },
          ] as const"
          :key="provider.id"
          type="button"
          class="flex w-full items-center gap-4 rounded-xl border border-default p-4 text-left transition-colors hover:border-primary hover:bg-primary/5"
          @click="emit('choose', provider.id)"
        >
          <UIcon
            :name="provider.icon"
            class="size-6 shrink-0 text-primary"
          /><span
            ><span class="block font-semibold">{{ provider.title }}</span
            ><span class="mt-1 block text-sm leading-5 text-muted">{{
              provider.text
            }}</span></span
          ><UIcon
            name="i-tabler-chevron-right"
            class="ml-auto size-4 shrink-0 text-muted"
          />
        </button></div
    ></template>
  </UModal>
</template>
