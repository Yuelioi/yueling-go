<script setup lang="ts">
import type { AIConnection, AIModelProfile } from '../../../api/ai.ts'
import AIConnectionFields from '../../../components/AIConnectionFields.vue'
import type { AccountDraft } from '../accountDraft.ts'
import type { CodexAuth } from '../codexAuth.ts'
const props = defineProps<{
  editor: AccountDraft
  authentication: CodexAuth
  profiles: AIModelProfile[]
  accountExists: boolean
  disabled?: boolean
  saving?: boolean
  unsaved?: boolean
  saveError?: string
}>()
const emit = defineEmits<{
  save: []
  delete: [account: AIConnection]
  addModel: [account: AIConnection]
}>()
const {
  selected: account,
  open: accountOpen,
  file: authFile,
  replace: replacing,
  dirty: accountDirty,
} = props.editor
const { statuses: auth, busy: busyAuth, read: status } = props.authentication
</script>
<template>
  <UModal
    v-model:open="accountOpen"
    :title="accountExists ? '管理账号' : '连接账号'"
    description="账号只管理登录和凭据；模型及用途在账号下单独配置。"
    :ui="{ content: 'sm:max-w-xl' }"
    :close="!disabled && !busyAuth"
    :dismissible="!disabled && !busyAuth && !accountDirty"
  >
    <template #body
      ><div v-if="account" class="min-w-0 space-y-6">
        <UAlert
          icon="i-tabler-alert-circle"
          v-if="saveError"
          color="error"
          variant="soft"
          :description="saveError"
        />
        <AIConnectionFields
          v-model="account"
          :disabled="disabled || busyAuth"
        />
        <p
          v-if="
            accountExists &&
            profiles.some((p) => p.connection_id === account!.id)
          "
          class="text-sm leading-6 text-muted"
        >
          修改这里的凭据会影响：{{
            profiles
              .filter((p) => p.connection_id === account!.id)
              .map((p) => p.name || p.model)
              .join('、')
          }}。
        </p>
        <section
          v-if="account.kind === 'codex'"
          class="space-y-3 border-t border-default pt-4"
        >
          <div class="flex items-center justify-between gap-2">
            <h4 class="text-sm font-semibold">Codex 登录态</h4>
            <UButton
              v-if="accountExists"
              size="xs"
              color="neutral"
              variant="ghost"
              :disabled="disabled || busyAuth"
              @click="status(account)"
              >刷新状态</UButton
            >
          </div>
          <p
            v-if="auth[account.id]"
            class="text-sm"
            :class="
              auth[account.id]?.imported && !auth[account.id]?.error
                ? 'text-success'
                : 'text-warning'
            "
          >
            {{ auth[account.id]?.message }}
          </p>
          <UFileUpload
            v-model="authFile"
            label="选择 auth.json"
            description="最大 64 KB。保存账号时一并导入。"
            accept=".json,application/json"
            :disabled="disabled || busyAuth"
          />
          <UCheckbox
            v-if="auth[account.id]?.imported"
            v-model="replacing"
            label="替换已有登录态"
            :disabled="disabled || busyAuth"
          />
        </section>
        <UButton
          v-if="accountExists && !accountDirty"
          block
          variant="soft"
          trailing-icon="i-tabler-arrow-right"
          :disabled="disabled || busyAuth || profiles.length >= 32"
          @click="emit('addModel', account)"
          >为这个账号添加模型</UButton
        >
      </div></template
    >
    <template #footer
      ><div class="flex w-full flex-wrap justify-between gap-2">
        <UButton
          v-if="accountExists && account"
          color="error"
          variant="ghost"
          :disabled="disabled || busyAuth"
          @click="emit('delete', account)"
          >删除账号</UButton
        >
        <div class="ml-auto flex gap-2">
          <UButton
            color="neutral"
            variant="soft"
            :disabled="disabled || busyAuth"
            @click="accountOpen = false"
            >{{ accountDirty ? '取消' : '完成' }}</UButton
          ><UButton
            :loading="saving || busyAuth"
            :disabled="
              disabled ||
              busyAuth ||
              (!accountDirty && !unsaved) ||
              !account?.name.trim() ||
              (account.kind === 'openai' && !account.base_url.trim())
            "
            @click="emit('save')"
            >{{ authFile ? '保存并导入' : '保存账号' }}</UButton
          >
        </div>
      </div></template
    >
  </UModal>
</template>
