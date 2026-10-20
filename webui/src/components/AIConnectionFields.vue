<script setup lang="ts">
import type { AIConnection } from '../api'
import { accountDestination } from '../ai-models'
const connection = defineModel<AIConnection>({ required: true })
defineProps<{ disabled?: boolean }>()
</script>
<template>
  <div class="space-y-4">
    <p class="break-all rounded-lg bg-elevated px-3 py-2 text-sm font-medium">
      {{ accountDestination(connection) }}
    </p>
    <UFormField label="账号名称" required>
      <UInput
        v-model="connection.name"
        class="w-full"
        placeholder="给这个账号起一个容易辨认的名字"
        :disabled="disabled"
      />
    </UFormField>
    <template v-if="connection.kind === 'openai'">
      <UFormField label="API 地址" required>
        <UInput
          v-model="connection.base_url"
          placeholder="https://api.example.com/v1"
          class="w-full"
          :disabled="disabled"
        />
      </UFormField>
      <UFormField
        label="API Key"
        :description="
          connection.key_configured
            ? '已保存。留空保留原密钥。'
            : '填写这个服务商的密钥；本地无认证服务可留空。'
        "
      >
        <UInput
          v-model="connection.api_key"
          type="password"
          autocomplete="new-password"
          class="w-full"
          :disabled="disabled || connection.clear_key"
        />
      </UFormField>
      <UCheckbox
        v-if="connection.key_configured"
        v-model="connection.clear_key"
        label="清除已保存的密钥"
        :disabled="disabled"
      />
    </template>
    <p class="text-xs leading-5 text-muted">
      接入方式在创建时确定。换服务商或改用 Codex
      时，请添加独立账号，再为模型切换账号。
    </p>
  </div>
</template>
