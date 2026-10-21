<script setup lang="ts">
import { watch } from 'vue'
import { useRoute } from 'vue-router'
import type { AIModelProfile, AIConnection } from '../api/ai.ts'
import {
  accountDestination,
  isDeepSeekAccount,
  modelCapabilities,
  modelRouteIssue,
} from '../ai-models.ts'
import { capabilities } from '../features/ai/shared.ts'
import { useModelWorkspace } from '../features/ai/useModelWorkspace.ts'
import ModelDialog from '../features/ai/components/ModelDialog.vue'
import AccountDialog from '../features/ai/components/AccountDialog.vue'
import ProviderDialog from '../features/ai/components/ProviderDialog.vue'
import CapabilityDialog from '../features/ai/components/CapabilityDialog.vue'
const profiles = defineModel<AIModelProfile[]>({ required: true })
const connections = defineModel<AIConnection[]>('connections', {
  required: true,
})
const props = defineProps<{
  saveError?: string
  disabled?: boolean
  unsaved?: boolean
  saving?: boolean
  bindings: Record<string, string>
}>()
const emit = defineEmits<{
  save: []
  editing: [dirty: boolean]
  busy: [active: boolean]
}>()
const route = useRoute()
const workspace = useModelWorkspace(
  profiles,
  connections,
  () => props,
  () => emit('save'),
  typeof route.query.model === 'string' ? route.query.model : undefined,
)
const {
  model,
  account,
  auth,
  test,
  error,
  sourceID,
  source,
  sourceProfiles,
  unassigned,
  conflicts,
  providerOpen,
  pendingDelete,
  accountExists,
  editing,
  usages,
  chooseProvider,
  addModel,
  editAccount,
  saveModel,
  saveAccount,
  saved,
  askDelete,
  deleteItem,
  discard,
} = workspace
const { statuses } = auth
watch(editing, (value) => emit('editing', value))
watch(auth.busy, (value) => emit('busy', value))
function isSearchOnly(profile: AIModelProfile) {
  return modelCapabilities(profile).join() === 'web_search'
}
function capabilityText(profile: AIModelProfile) {
  return capabilities
    .filter((c) => modelCapabilities(profile).includes(c.value))
    .map((c) => c.label)
    .join(' · ')
}
defineExpose({
  addAccount: () => {
    providerOpen.value = true
  },
  saved,
  discard,
})
</script>
<template>
  <div class="min-w-0 space-y-6">
    <UAlert
      icon="i-tabler-alert-circle"
      v-if="error"
      color="error"
      variant="soft"
      :description="error"
      closable
      @update:open="error = ''"
    />
    <UAlert
      v-if="conflicts.length"
      color="error"
      variant="soft"
      icon="i-tabler-alert-triangle"
      title="有模型的调用目标需要修正"
      :description="`${conflicts.map((p) => p.name || p.model).join('、')} 的账号与模型不匹配。打开模型，检查实际调用目标。`"
      :actions="[
        { label: '检查配置', onClick: () => model.edit(conflicts[0]!) },
      ]"
    />
    <div
      v-if="!connections.length && !profiles.length"
      class="rounded-xl border border-dashed border-default px-5 py-16 text-center"
    >
      <UIcon name="i-tabler-plug-connected" class="mb-3 size-9 text-primary" />
      <h3 class="text-lg font-semibold">先连接一个 AI 服务</h3>
      <p class="mx-auto mb-6 mt-2 max-w-md text-sm leading-6 text-muted">
        选择 Codex 订阅或 API 服务，连接账号后再添加它提供的模型。
      </p>
      <UButton
        icon="i-tabler-plus"
        :disabled="disabled"
        @click="providerOpen = true"
        >连接 AI 服务</UButton
      >
    </div>
    <div v-else class="grid min-w-0 gap-5 md:grid-cols-[13rem_minmax(0,1fr)]">
      <nav aria-label="AI 接入账号" class="space-y-1">
        <p class="mb-3 px-3 text-xs font-medium text-muted">接入账号</p>
        <button
          v-for="c in connections"
          :key="c.id"
          type="button"
          class="flex w-full items-start gap-3 rounded-lg px-3 py-3 text-left transition-colors"
          :class="
            sourceID === c.id
              ? 'bg-primary/10 text-primary'
              : 'hover:bg-elevated'
          "
          :aria-current="sourceID === c.id ? 'true' : undefined"
          @click="sourceID = c.id"
        >
          <UIcon
            :name="
              c.kind === 'codex' ? 'i-tabler-brand-openai' : 'i-tabler-api'
            "
            class="mt-0.5 size-5 shrink-0"
          />
          <span class="min-w-0"
            ><span class="block truncate text-sm font-semibold">{{
              c.name
            }}</span
            ><span class="mt-1 block text-xs text-muted"
              >{{
                c.kind === 'codex'
                  ? 'Codex 订阅'
                  : isDeepSeekAccount(c)
                    ? 'DeepSeek API'
                    : '兼容 API'
              }}
              ·
              {{ profiles.filter((p) => p.connection_id === c.id).length }}
              个模型</span
            ></span
          >
        </button>
        <button
          v-if="unassigned"
          type="button"
          class="w-full rounded-lg px-3 py-3 text-left text-sm text-warning"
          :class="sourceID === '__unassigned__' ? 'bg-warning/10' : ''"
          @click="sourceID = '__unassigned__'"
        >
          待关联账号的模型
        </button>
      </nav>
      <section class="min-w-0 overflow-hidden rounded-xl border border-default">
        <header class="space-y-4 border-b border-default bg-muted/30 p-5">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0">
              <h3 class="text-lg font-semibold">
                {{ source?.name || '待关联账号' }}
              </h3>
              <p class="mt-1 break-all text-sm text-muted">
                {{ accountDestination(source) }}
              </p>
            </div>
            <UButton
              v-if="source"
              color="neutral"
              variant="soft"
              icon="i-tabler-key"
              :disabled="disabled"
              @click="editAccount(source)"
              >{{
                source.kind === 'codex' ? '登录与账号' : '管理凭据'
              }}</UButton
            >
          </div>
          <div
            v-if="source?.kind === 'codex'"
            class="flex flex-wrap items-center gap-2 text-sm"
          >
            <span
              :class="
                statuses[source.id]?.imported ? 'text-success' : 'text-warning'
              "
              >{{ statuses[source.id]?.message || '正在读取登录状态…' }}</span
            >
          </div>
        </header>
        <div class="flex items-center justify-between gap-3 px-5 py-4">
          <h4 class="text-sm font-medium">这个账号下的模型</h4>
          <UButton
            v-if="source"
            size="sm"
            icon="i-tabler-plus"
            variant="soft"
            :disabled="disabled || profiles.length >= 32"
            @click="addModel(source)"
            >添加模型</UButton
          >
        </div>
        <div
          v-if="!sourceProfiles.length"
          class="px-5 pb-10 pt-4 text-center text-sm leading-6 text-muted"
        >
          账号连接后，在这里添加模型并选择用途。
        </div>
        <div
          v-for="p in sourceProfiles"
          :key="p.id"
          class="border-t border-default px-5 py-5"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <h4 class="break-all font-semibold">
                  {{ p.name || p.model || '未命名模型' }}
                </h4>
                <UBadge
                  v-if="p.disabled"
                  color="neutral"
                  variant="soft"
                  size="xs"
                  >已停用</UBadge
                ><UBadge
                  v-if="modelRouteIssue(p, connections)"
                  color="error"
                  variant="soft"
                  size="xs"
                  >配置冲突</UBadge
                >
              </div>
              <p class="mt-1 break-all text-sm text-muted">
                {{ isSearchOnly(p) ? 'DeepSeek Search / Fetch' : p.model }}
              </p>
              <p class="mt-2 text-xs text-muted">{{ capabilityText(p) }}</p>
            </div>
            <div class="flex shrink-0 gap-1">
              <UButton
                color="neutral"
                variant="soft"
                :disabled="disabled"
                @click="model.edit(p)"
                >配置模型</UButton
              ><UButton
                color="neutral"
                variant="ghost"
                :disabled="
                  disabled ||
                  unsaved ||
                  p.disabled ||
                  !!modelRouteIssue(p, connections)
                "
                @click="test.show(p)"
                >测试</UButton
              >
            </div>
          </div>
          <p
            v-if="modelRouteIssue(p, connections)"
            class="mt-3 text-sm leading-6 text-error"
          >
            {{ modelRouteIssue(p, connections) }}
          </p>
          <p class="mt-3 text-xs text-muted">
            {{
              usages(p).length
                ? `已分配：${usages(p).join('、')}`
                : '尚未分配给功能'
            }}
          </p>
        </div>
        <footer
          class="flex flex-wrap items-center justify-between gap-3 border-t border-default bg-muted/30 px-5 py-4"
        >
          <span class="text-xs text-muted"
            >在功能分配中决定机器人实际用哪个模型。</span
          ><UButton
            to="/ai"
            size="sm"
            color="neutral"
            variant="link"
            trailing-icon="i-tabler-arrow-right"
            >分配给功能</UButton
          >
        </footer>
      </section>
    </div>

    <ProviderDialog v-model:open="providerOpen" @choose="chooseProvider" />
    <ModelDialog
      :editor="model"
      :disabled="disabled"
      :saving="saving"
      :unsaved="unsaved"
      :save-error="saveError"
      @save="saveModel"
      @delete="askDelete('model', $event)"
    />
    <AccountDialog
      :editor="account"
      :authentication="auth"
      :profiles="profiles"
      :account-exists="accountExists"
      :disabled="disabled"
      :saving="saving"
      :unsaved="unsaved"
      :save-error="saveError"
      @save="saveAccount"
      @delete="askDelete('account', $event)"
      @add-model="addModel"
    />
    <CapabilityDialog
      :test="test"
      :connections="connections"
      :disabled="disabled"
      :unsaved="unsaved"
    />
    <UModal
      :open="!!pendingDelete"
      :title="`删除${pendingDelete?.kind === 'model' ? '模型' : '账号'}？`"
      :description="pendingDelete?.name"
      @update:open="!$event && (pendingDelete = null)"
      ><template #body
        ><p class="text-sm text-muted">
          正在使用的模型或账号需先解除关联。
        </p></template
      ><template #footer
        ><UButton color="neutral" variant="soft" @click="pendingDelete = null"
          >取消</UButton
        ><UButton color="error" :disabled="disabled" @click="deleteItem"
          >删除并保存</UButton
        ></template
      ></UModal
    >
  </div>
</template>
