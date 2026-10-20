<script setup lang="ts">
import type { useFeed } from '../useFeed.ts'
const props = defineProps<{
  catalog: Pick<ReturnType<typeof useFeed>['catalog'], 'selectedGroupID'>
  settings: Pick<
    ReturnType<typeof useFeed>['settings'],
    | 'settingsLoading'
    | 'settingsSaving'
    | 'pendingCount'
    | 'strategyEditorOpen'
    | 'feedSettings'
    | 'itemLengthModeOptions'
    | 'itemLengthMode'
    | 'customItemMaxChars'
    | 'itemLengthError'
    | 'itemLengthLabel'
    | 'saveSettings'
  >
}>()
const { selectedGroupID } = props.catalog
const {
  settingsLoading,
  settingsSaving,
  pendingCount,
  strategyEditorOpen,
  feedSettings,
  itemLengthModeOptions,
  itemLengthMode,
  customItemMaxChars,
  itemLengthError,
  itemLengthLabel,
  saveSettings,
} = props.settings
</script>
<template>
  <section
    v-if="selectedGroupID !== null"
    class="rounded-2xl border border-default bg-default overflow-hidden"
  >
    <div
      class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
    >
      <div>
        <div class="text-sm font-semibold text-highlighted">推送策略</div>
        <div class="mt-1 text-xs leading-5 text-muted">
          设置检查间隔、内容长度与夜间静默
        </div>
      </div>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <UBadge color="neutral" variant="subtle"
          >每 {{ feedSettings.poll_interval_minutes }} 分钟检查</UBadge
        >
        <UBadge color="neutral" variant="subtle">{{
          itemLengthLabel(feedSettings.item_max_chars)
        }}</UBadge>
        <UBadge :color="pendingCount ? 'warning' : 'success'" variant="subtle"
          >{{ pendingCount }} 条待推送</UBadge
        >
        <UButton
          color="neutral"
          variant="soft"
          :icon="
            strategyEditorOpen ? 'i-tabler-chevron-up' : 'i-tabler-adjustments'
          "
          @click="strategyEditorOpen = !strategyEditorOpen"
        >
          {{ strategyEditorOpen ? '收起' : '配置' }}
        </UButton>
      </div>
    </div>
    <div v-if="strategyEditorOpen" class="space-y-4 p-4">
      <UFormField label="检查间隔（分钟）" description="本群生效，5–1440 分钟">
        <UInput
          v-model.number="feedSettings.poll_interval_minutes"
          type="number"
          :min="5"
          :max="1440"
          :step="1"
          :disabled="!selectedGroupID || settingsLoading"
        />
      </UFormField>
      <div class="grid gap-4 xl:grid-cols-2">
        <div class="rounded-lg border border-default bg-muted space-y-4 p-4">
          <div class="flex items-center justify-between gap-4">
            <div class="min-w-0">
              <div class="text-sm font-medium text-default">夜间静默</div>
              <div class="mt-1 text-xs leading-5 text-muted">
                静默期间继续抓取，结束后合并推送
              </div>
            </div>
            <USwitch
              v-model="feedSettings.quiet_enabled"
              color="primary"
              :disabled="!selectedGroupID || settingsLoading"
              aria-label="启用夜间静默"
            />
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <UFormField label="开始时间">
              <UInput
                v-model="feedSettings.quiet_start"
                class="w-full"
                type="time"
                icon="i-tabler-moon"
                :disabled="
                  !selectedGroupID ||
                  !feedSettings.quiet_enabled ||
                  settingsLoading
                "
              />
            </UFormField>
            <UFormField label="结束时间">
              <UInput
                v-model="feedSettings.quiet_end"
                class="w-full"
                type="time"
                icon="i-tabler-sun"
                :disabled="
                  !selectedGroupID ||
                  !feedSettings.quiet_enabled ||
                  settingsLoading
                "
              />
            </UFormField>
          </div>
        </div>

        <div class="rounded-lg border border-default bg-muted space-y-4 p-4">
          <div>
            <div class="text-sm font-medium text-default">单条内容</div>
            <div class="mt-1 text-xs leading-5 text-muted">
              选择完整正文，或限制最终推送的字符数
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <UFormField label="内容范围" description="最多发送 4000 字">
              <USelect
                v-model="itemLengthMode"
                class="w-full"
                :items="itemLengthModeOptions"
                value-key="value"
                icon="i-tabler-text-size"
                :disabled="!selectedGroupID || settingsLoading"
              />
            </UFormField>
            <UFormField
              v-if="itemLengthMode === 'limited'"
              label="字符上限"
              :error="itemLengthError"
              description="翻译完成后再截取，范围 80–4000"
            >
              <UInputNumber
                v-model="customItemMaxChars"
                class="w-full"
                :min="80"
                :max="4000"
                :step="20"
                :disabled="!selectedGroupID || settingsLoading"
                aria-label="订阅单条内容字符上限"
              />
            </UFormField>
          </div>
        </div>
      </div>
      <div class="flex justify-end">
        <UButton
          icon="i-tabler-device-floppy"
          :loading="settingsSaving"
          :disabled="
            !selectedGroupID || settingsLoading || Boolean(itemLengthError)
          "
          @click="saveSettings"
          >保存推送策略</UButton
        >
      </div>
    </div>
  </section>
</template>
