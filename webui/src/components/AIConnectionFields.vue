<script setup lang="ts">
import type { AIConnection } from '../api'
const connection = defineModel<AIConnection>({ required: true })
defineProps<{ disabled?: boolean }>()
function setKind(value: string) {
  connection.value.kind = value as AIConnection['kind']
  if (value === 'codex') { connection.value.base_url = ''; connection.value.api_key = ''; connection.value.clear_key = true }
  else connection.value.clear_key = false
}
</script>
<template>
  <div class="grid gap-4 sm:grid-cols-2">
    <UFormField label="账号名称" description="方便区分不同服务商或订阅。">
      <UInput v-model="connection.name" class="w-full" placeholder="例如：我的 OpenAI" :disabled="disabled" />
    </UFormField>
    <UFormField label="接入方式">
      <USelect :model-value="connection.kind" :items="[{ label: 'OpenAI 兼容 API', value: 'openai' }, { label: 'Codex 订阅', value: 'codex' }]" class="w-full" :disabled="disabled" @update:model-value="setKind(String($event))" />
    </UFormField>
    <template v-if="connection.kind === 'openai'">
      <UFormField label="API 地址" description="填写服务商提供的完整 API 地址。">
        <UInput v-model="connection.base_url" placeholder="https://api.example.com/v1" class="w-full" :disabled="disabled" />
      </UFormField>
      <UFormField label="API Key" :description="connection.key_configured ? '已设置。留空保留原密钥。' : '本地无认证服务可以留空。'">
        <UInput v-model="connection.api_key" type="password" autocomplete="new-password" class="w-full" :disabled="disabled || connection.clear_key" />
      </UFormField>
      <UCheckbox v-if="connection.key_configured" v-model="connection.clear_key" label="清除已保存的密钥" :disabled="disabled" />
    </template>
    <UAlert v-else class="sm:col-span-2" color="neutral" variant="soft" icon="i-tabler-info-circle" description="保存账号后，在「账号与凭据」中导入 Codex 登录文件。订阅支持聊天、读图和工具调用。" />
  </div>
</template>
