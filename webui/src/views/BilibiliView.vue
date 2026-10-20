<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  bilibiliApi,
  type BilibiliResponse,
  type BilibiliSettings,
} from '../api'

const settings = ref<BilibiliSettings | null>(null)
const saved = ref('')
const recipients = ref('')
const status = ref('')
const contacts = ref<BilibiliResponse['contacts']>([])
const loginConfigured = ref(false)
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const success = ref('')
const intervals = [
  { label: '每 1 分钟', value: 60 },
  { label: '每 5 分钟', value: 300 },
  { label: '每 10 分钟', value: 600 },
  { label: '自定义', value: 0 },
]
const intervalChoice = ref(60)
const modes = [
  { label: '关闭自动回复', value: 'off' },
  { label: '关键词回复', value: 'keyword' },
  { label: 'AI 回复', value: 'ai' },
  { label: '关键词优先，AI 补充', value: 'hybrid' },
]
const dirty = computed(
  () =>
    !!settings.value &&
    JSON.stringify({ ...settings.value, notify_users: recipients.value }) !==
      saved.value,
)

function apply(res: BilibiliResponse) {
  settings.value = res.settings
  recipients.value = res.settings.notify_users.join(', ')
  saved.value = JSON.stringify({
    ...res.settings,
    notify_users: recipients.value,
  })
  intervalChoice.value = intervals.some(
    (i) => i.value === res.settings.poll_seconds,
  )
    ? res.settings.poll_seconds
    : 0
  status.value = res.status
  contacts.value = res.contacts
  loginConfigured.value = res.login_configured
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    apply(await bilibiliApi.bilibili())
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function changeInterval(value: number) {
  intervalChoice.value = value
  if (settings.value && value > 0) settings.value.poll_seconds = value
}

async function save() {
  if (!settings.value) return
  error.value = ''
  success.value = ''
  const ids = recipients.value.trim()
    ? recipients.value.trim().split(/[\s,，]+/)
    : []
  if (
    ids.some(
      (id) =>
        !/^\d+$/.test(id) ||
        !Number.isSafeInteger(Number(id)) ||
        Number(id) <= 0,
    )
  ) {
    error.value = 'QQ 接收人请填写正整数 QQ 号，多个号码用逗号分隔。'
    return
  }
  if (
    settings.value.keywords.some(
      (rule) => !rule.keyword.trim() || !rule.reply.trim(),
    )
  ) {
    error.value = '请补全关键词和回复内容，或删除空规则。'
    return
  }
  saving.value = true
  try {
    apply(
      await bilibiliApi.saveBilibili({
        ...settings.value,
        notify_users: [...new Set(ids.map(Number))],
      }),
    )
    success.value = '设置已保存并生效，重启后仍保留。'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}
onMounted(load)
</script>

<template>
  <div class="min-w-0 space-y-6">
    <div class="flex justify-end">
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="load"
        >刷新状态</UButton
      >
    </div>
    <UAlert
      icon="i-tabler-alert-circle"
      v-if="error"
      color="error"
      variant="soft"
      :description="error"
      role="alert"
    />
    <UAlert
      icon="i-tabler-circle-check"
      v-if="success"
      color="success"
      variant="soft"
      :description="success"
      role="status"
    />
    <UCard
      as="div"
      class="min-w-0"
      :ui="{ body: 'p-0 sm:p-0' }"
      v-if="loading && !settings"
      ><div class="p-5">正在读取设置…</div></UCard
    >
    <form v-if="settings" class="min-w-0 space-y-6" @submit.prevent="save">
      <fieldset :disabled="saving || loading" class="min-w-0 space-y-6">
        <UCard as="section" class="min-w-0" :ui="{ body: 'p-0 sm:p-0' }"
          ><div>
            <div
              class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
            >
              <h2 class="font-medium">自动监控</h2>
              <UBadge
                :color="settings.enabled ? 'success' : 'neutral'"
                variant="subtle"
                >{{ settings.enabled ? '已开启' : '已暂停' }}</UBadge
              >
            </div>
            <div class="space-y-5 p-5">
              <USwitch
                v-model="settings.enabled"
                label="自动轮询并转发新私信"
                description="开启后自动运行，无需发送检查命令。关闭会停止定时检查，已保存的会话和规则会保留。"
              />
              <div class="grid min-w-0 gap-5 md:grid-cols-2">
                <UFormField label="轮询间隔" description="保存后按新间隔执行。">
                  <USelect
                    :model-value="intervalChoice"
                    :items="intervals"
                    class="w-full"
                    aria-label="轮询间隔"
                    @update:model-value="changeInterval(Number($event))"
                  />
                </UFormField>
                <UFormField
                  v-if="intervalChoice === 0"
                  label="自定义间隔（秒）"
                  description="20～3600 秒。"
                >
                  <UInput
                    v-model.number="settings.poll_seconds"
                    type="number"
                    :min="20"
                    :max="3600"
                    class="w-full"
                    aria-label="自定义间隔（秒）"
                  />
                </UFormField>
                <UFormField
                  label="QQ 通知接收人"
                  description="留空使用超级管理员；多个 QQ 号用逗号分隔。"
                >
                  <UInput
                    v-model="recipients"
                    class="w-full"
                    placeholder="例如 123456, 654321"
                    aria-label="QQ 通知接收人"
                  />
                </UFormField>
              </div>
              <p class="whitespace-pre-line break-words text-sm text-muted">
                {{ status }}
              </p>
              <UAlert
                icon="i-tabler-alert-triangle"
                v-if="!loginConfigured"
                color="warning"
                variant="soft"
                description="尚未配置 B站登录态。连接账号后才能监控和回复私信。"
              />
            </div></div
        ></UCard>
        <UCard as="section" class="min-w-0" :ui="{ body: 'p-0 sm:p-0' }"
          ><div>
            <div
              class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
            >
              <h2 class="font-medium">自动回复</h2>
              <span class="text-xs text-dimmed">默认关闭</span>
            </div>
            <div class="space-y-5 p-5">
              <UFormField label="回复方式"
                ><USelect
                  v-model="settings.reply_mode"
                  :items="modes"
                  class="w-full"
                  aria-label="回复方式"
              /></UFormField>
              <div class="grid gap-5 md:grid-cols-2">
                <UFormField label="同一联系人回复冷却（秒）"
                  ><UInput
                    v-model.number="settings.reply_cooldown_seconds"
                    type="number"
                    :min="0"
                    :max="86400"
                    class="w-full"
                    aria-label="回复冷却（秒）"
                /></UFormField>
                <UFormField label="每分钟最多自动回复"
                  ><UInput
                    v-model.number="settings.reply_per_minute"
                    type="number"
                    :min="1"
                    :max="60"
                    class="w-full"
                    aria-label="每分钟最多自动回复"
                /></UFormField>
              </div>
              <UFormField
                v-if="
                  settings.reply_mode === 'ai' ||
                  settings.reply_mode === 'hybrid'
                "
                label="AI 回复提示词"
                description="使用月灵已有 AI 配置。"
              >
                <UTextarea
                  v-model="settings.ai_prompt"
                  :rows="4"
                  :maxlength="4000"
                  class="w-full"
                  aria-label="AI 回复提示词"
                />
              </UFormField>
              <div class="flex flex-wrap items-center justify-between gap-3">
                <h3 class="text-sm font-medium">关键词规则</h3>
                <UButton
                  type="button"
                  variant="soft"
                  :disabled="settings.keywords.length >= 50"
                  @click="
                    settings?.keywords.push({
                      keyword: '',
                      reply: '',
                      exact: false,
                    })
                  "
                  >添加规则</UButton
                >
              </div>
              <p v-if="!settings.keywords.length" class="text-sm text-dimmed">
                暂无关键词规则。添加后选择关键词或混合模式即可启用。
              </p>
              <div
                v-for="(rule, i) in settings.keywords"
                :key="i"
                class="space-y-3 rounded-xl border border-default p-4"
              >
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <span class="text-sm">规则 {{ i + 1 }}</span
                  ><UButton
                    type="button"
                    color="error"
                    variant="ghost"
                    :aria-label="`删除规则 ${i + 1}`"
                    @click="settings.keywords.splice(i, 1)"
                    >删除</UButton
                  >
                </div>
                <UFormField label="关键词"
                  ><UInput
                    v-model="rule.keyword"
                    class="w-full"
                    :maxlength="100"
                    :aria-label="`规则 ${i + 1} 关键词`"
                /></UFormField>
                <UFormField label="回复内容"
                  ><UTextarea
                    v-model="rule.reply"
                    class="w-full"
                    :rows="3"
                    :aria-label="`规则 ${i + 1} 回复内容`"
                /></UFormField>
                <USwitch
                  v-model="rule.exact"
                  label="完整匹配"
                  description="关闭时，只要私信包含关键词就匹配。"
                />
              </div>
            </div></div
        ></UCard>
        <UCard as="section" class="min-w-0" :ui="{ body: 'p-0 sm:p-0' }"
          ><div>
            <div
              class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
            >
              <h2 class="font-medium">视频查询默认值</h2>
            </div>
            <div class="grid gap-5 p-5 md:grid-cols-2">
              <UFormField label="默认 UP 主 UID" description="0 使用登录账号。"
                ><UInput
                  v-model.number="settings.uid"
                  type="number"
                  :min="0"
                  class="w-full"
                  aria-label="默认 UP 主 UID"
              /></UFormField>
              <UFormField label="最近视频数量"
                ><UInput
                  v-model.number="settings.recent_count"
                  type="number"
                  :min="1"
                  :max="10"
                  class="w-full"
                  aria-label="最近视频数量"
              /></UFormField>
            </div></div
        ></UCard>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <span class="text-sm text-dimmed">{{
            dirty ? '有未保存的修改' : '当前设置已保存'
          }}</span
          ><UButton
            type="submit"
            icon="i-tabler-device-floppy"
            :loading="saving"
            :disabled="!dirty || !loginConfigured"
            >保存设置</UButton
          >
        </div>
      </fieldset>
    </form>
    <UCard as="section" class="min-w-0" :ui="{ body: 'p-0 sm:p-0' }"
      ><div>
        <div
          class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
        >
          <h2 class="font-medium">QQ 回复会话</h2>
          <UButton variant="ghost" :loading="loading" @click="load"
            >刷新状态</UButton
          >
        </div>
        <div class="space-y-4 p-5">
          <p class="text-sm text-muted">
            只有一个联系人时发送「B站回复 内容」；多人时发送「B站回复 1
            内容」。会话编号固定，也可在 QQ 使用「B站会话」查询。
          </p>
          <p v-if="!contacts.length" class="text-sm text-dimmed">
            收到新私信后，这里会显示会话。首次启动不会转发旧消息。
          </p>
          <div
            v-for="contact in contacts"
            :key="contact.number"
            class="flex min-w-0 gap-3 rounded-lg border border-default p-3"
          >
            <UBadge variant="subtle" class="shrink-0 self-start">{{
              contact.number
            }}</UBadge>
            <div class="min-w-0">
              <p class="mb-1 break-words text-sm font-medium">
                {{ contact.name || `用户 ${contact.uid}` }}
              </p>
              <p class="break-words text-sm">{{ contact.preview }}</p>
              <p class="mt-1 text-xs text-dimmed">
                {{ new Date(contact.last_message_at * 1000).toLocaleString() }}
              </p>
            </div>
          </div>
        </div>
      </div></UCard
    >
  </div>
</template>
