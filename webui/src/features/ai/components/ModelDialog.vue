<script setup lang="ts">
import { type AIModelProfile } from '../../../api/ai.ts'
import {
  accountDestination,
  isDeepSeekAccount,
  modelCapabilities,
} from '../../../ai-models.ts'
import { capabilities as caps } from '../shared.ts'
import type { ModelDraft } from '../modelDraft.ts'
const props = defineProps<{
  editor: ModelDraft
  disabled?: boolean
  saving?: boolean
  unsaved?: boolean
  saveError?: string
}>()
const emit = defineEmits<{ save: []; delete: [profile: AIModelProfile] }>()
const {
  selected,
  open: modelOpen,
  baseline: modelBaseline,
  custom: customCapabilities,
  account: modelAccount,
  issue: modelIssue,
  dirty: modelDirty,
  purpose,
  purposeOptions,
  accountOptions: connectionOptions,
  setPurpose,
  setCapability,
} = props.editor
function isSearchOnly(profile: AIModelProfile) {
  return modelCapabilities(profile).join() === 'web_search'
}
</script>
<template>
  <USlideover
    v-model:open="modelOpen"
    :title="modelBaseline ? '配置模型' : '添加模型'"
    description="先确认实际调用的账号，再选择模型用途。"
    :ui="{ content: 'sm:max-w-xl' }"
    :close="!disabled"
    :dismissible="!disabled && !modelDirty"
  >
    <template #body
      ><div v-if="selected" class="space-y-6">
        <UAlert
          icon="i-tabler-alert-circle"
          v-if="saveError"
          color="error"
          variant="soft"
          :description="saveError"
        />
        <section class="space-y-3">
          <UFormField label="使用哪个账号" required
            ><USelect
              v-model="selected.connection_id"
              :items="connectionOptions"
              placeholder="请选择账号"
              class="w-full"
              :disabled="disabled"
          /></UFormField>
          <div class="rounded-lg border border-default bg-muted/30 p-3">
            <p class="text-xs text-muted">实际调用目标</p>
            <p class="mt-1 break-all text-sm font-medium">
              {{ accountDestination(modelAccount) }}
            </p>
          </div>
          <UAlert
            icon="i-tabler-alert-circle"
            v-if="modelIssue"
            color="error"
            variant="soft"
            :description="modelIssue"
          />
        </section>
        <UFormField label="模型用途"
          ><USelect
            :model-value="customCapabilities ? 'custom' : purpose"
            :items="purposeOptions"
            class="w-full"
            :disabled="disabled"
            @update:model-value="setPurpose(String($event))"
        /></UFormField>
        <UFormField
          v-if="!isSearchOnly(selected)"
          label="模型 ID"
          required
          description="填写这个账号实际可用的模型名称。"
          ><UInput
            v-model="selected.model"
            class="w-full"
            :placeholder="
              modelAccount?.kind === 'codex'
                ? '填写 Codex 中可用的模型 ID'
                : isDeepSeekAccount(modelAccount)
                  ? '例如 deepseek-flash'
                  : '服务商提供的模型 ID'
            "
            :disabled="disabled"
        /></UFormField>
        <p v-else class="text-sm leading-6 text-muted">
          调用 DeepSeek Search / Fetch，无需填写推理模型
          ID。保存后在功能分配中选择“联网搜索”。
        </p>
        <UFormField
          label="显示名称（可选）"
          description="留空使用模型 ID；名称不会改变实际调用的服务。"
          ><UInput
            v-model="selected.name"
            class="w-full"
            placeholder="例如：日常对话、图片创作"
            :disabled="disabled"
        /></UFormField>
        <section
          v-if="customCapabilities || purpose === 'custom'"
          class="space-y-3"
        >
          <h4 class="text-sm font-medium">自定义能力</h4>
          <div class="grid gap-3 sm:grid-cols-2">
            <UCheckbox
              v-for="cap in caps"
              :key="cap.value"
              :model-value="modelCapabilities(selected).includes(cap.value)"
              :label="cap.label"
              :disabled="
                disabled ||
                (modelAccount?.kind === 'codex' &&
                  ['web_search', 'structured_output'].includes(cap.value) &&
                  !modelCapabilities(selected).includes(cap.value))
              "
              @update:model-value="setCapability(cap.value, $event === true)"
            />
          </div>
        </section>
        <UCollapsible class="space-y-3"
          ><UButton
            color="neutral"
            variant="link"
            trailing-icon="i-tabler-chevron-down"
            >高级参数</UButton
          ><template #content
            ><div class="grid gap-4 sm:grid-cols-2">
              <UFormField label="推理强度"
                ><USelect
                  :model-value="selected.reasoning_effort || '__default__'"
                  @update:model-value="
                    selected.reasoning_effort =
                      $event === '__default__' ? '' : String($event)
                  "
                  :items="[
                    { label: '使用默认', value: '__default__' },
                    ...[
                      'none',
                      'minimal',
                      'low',
                      'medium',
                      'high',
                      'xhigh',
                    ].map((value) => ({ label: value, value })),
                  ]"
                  class="w-full"
                  :disabled="disabled"
              /></UFormField>
              <UFormField
                v-if="modelAccount?.kind !== 'codex'"
                label="最大输出 Token"
                description="0 使用默认值。"
                ><UInput
                  v-model.number="selected.max_tokens"
                  type="number"
                  :min="0"
                  :max="200000"
                  class="w-full"
                  :disabled="disabled"
              /></UFormField>
              <UFormField label="超时（秒）" description="0 使用默认值。"
                ><UInput
                  v-model.number="selected.timeout_seconds"
                  type="number"
                  :min="0"
                  :max="600"
                  class="w-full"
                  :disabled="disabled"
              /></UFormField>
              <UFormField label="停用模型"
                ><USwitch v-model="selected.disabled" :disabled="disabled"
              /></UFormField></div></template
        ></UCollapsible></div
    ></template>
    <template #footer
      ><div class="flex w-full flex-wrap items-center justify-between gap-2">
        <UButton
          v-if="modelBaseline && selected"
          color="error"
          variant="ghost"
          :disabled="disabled"
          @click="emit('delete', selected)"
          >删除模型</UButton
        >
        <div class="ml-auto flex gap-2">
          <UButton
            color="neutral"
            variant="soft"
            :disabled="disabled"
            @click="modelOpen = false"
            >取消</UButton
          ><UButton
            :loading="saving"
            :disabled="
              disabled ||
              !selected ||
              !!modelIssue ||
              (!modelDirty && !unsaved) ||
              (!selected.model.trim() && !isSearchOnly(selected)) ||
              !modelCapabilities(selected).length
            "
            @click="emit('save')"
            >保存模型</UButton
          >
        </div>
      </div></template
    >
  </USlideover>
</template>
