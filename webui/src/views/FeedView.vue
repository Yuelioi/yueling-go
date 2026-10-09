<script setup lang="ts">
import { onMounted, watch } from 'vue'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
import PageHeader from '../components/PageHeader.vue'
import { useFeed } from '../composables/useFeed'

const {
  groups,
  feeds,
  selectedGroupID,
  feedURL,
  feedName,
  platformName,
  platform,
  platformTarget,
  loading,
  adding,
  platformAdding,
  checking,
  deleting,
  confirmOpen,
  pendingDelete,
  error,
  notice,
  settingsLoading,
  settingsSaving,
  toggling,
  translationSaving,
  pendingCount,
  strategyEditorOpen,
  platformEditorOpen,
  rawEditorOpen,
  feedSettings,
  selectedGroup,
  feedSwitchClass,
  groupFeeds,
  coveredGroups,
  activeFeeds,
  failingFeeds,
  platformOptions,
  itemLengthModeOptions,
  itemLengthMode,
  customItemMaxChars,
  itemLengthError,
  platformPlaceholder,
  sourceHost,
  formatDate,
  itemLengthLabel,
  load,
  loadSettings,
  saveSettings,
  add,
  addPlatform,
  check,
  askDelete,
  remove,
  setEnabled,
  setTranslation,
} = useFeed()

watch(selectedGroupID, (groupID) => {
  notice.value = ''
  if (groupID) loadSettings(groupID)
})
onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Signal subscriptions"
      title="订阅中心"
      description="把站点、博客、GitHub Releases 与 RSSHub 信息源聚合到群聊。"
      icon="i-tabler-rss"
    >
      <UButton
        icon="i-tabler-refresh-dot"
        :loading="checking"
        :disabled="loading"
        @click="check"
        >检查全部订阅</UButton
      >
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-refresh"
        :loading="loading"
        @click="load"
        >刷新数据</UButton
      >
    </PageHeader>

    <div class="grid grid-cols-3 gap-3 max-[860px]:grid-cols-1">
      <MetricCard
        label="活跃订阅"
        :value="activeFeeds"
        :detail="`${feeds.length - activeFeeds} 个已暂停`"
        icon="i-tabler-rss"
        tone="violet"
      />
      <MetricCard
        label="覆盖群聊"
        :value="coveredGroups"
        :detail="`${groups.length} 个可用群聊`"
        icon="i-tabler-users-group"
        tone="cyan"
      />
      <MetricCard
        label="异常源"
        :value="failingFeeds"
        :detail="`${pendingCount} 条等待推送`"
        icon="i-tabler-heart-rate-monitor"
        tone="amber"
      />
    </div>

    <UAlert
      v-if="error"
      class="error-banner"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      :description="error"
    />
    <UAlert
      v-if="notice"
      color="success"
      variant="subtle"
      icon="i-tabler-circle-check"
      :description="notice"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        v-model="selectedGroupID"
        :groups="groups"
        title="订阅群聊"
        description="选择要接收更新的群"
      />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div>
              <div class="section-title">推送策略</div>
              <div class="section-caption">
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
              <UBadge
                :color="pendingCount ? 'warning' : 'success'"
                variant="subtle"
                >{{ pendingCount }} 条待推送</UBadge
              >
              <UButton
                color="neutral"
                variant="soft"
                :icon="
                  strategyEditorOpen
                    ? 'i-tabler-chevron-up'
                    : 'i-tabler-adjustments'
                "
                @click="strategyEditorOpen = !strategyEditorOpen"
              >
                {{ strategyEditorOpen ? '收起' : '配置' }}
              </UButton>
            </div>
          </div>
          <div v-if="strategyEditorOpen" class="space-y-4 p-4">
            <UFormField
              label="检查间隔（分钟）"
              description="本群生效，5–1440 分钟"
            >
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
              <div class="surface-inset space-y-4 p-4">
                <div class="flex items-center justify-between gap-4">
                  <div class="min-w-0">
                    <div class="text-sm font-medium text-default">夜间静默</div>
                    <div class="mt-1 text-xs leading-5 text-muted">
                      静默期间继续抓取，结束后合并推送
                    </div>
                  </div>
                  <USwitch
                    v-model="feedSettings.quiet_enabled"
                    :class="feedSwitchClass"
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

              <div class="surface-inset space-y-4 p-4">
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
                  !selectedGroupID ||
                  settingsLoading ||
                  Boolean(itemLengthError)
                "
                @click="saveSettings"
                >保存推送策略</UButton
              >
            </div>
          </div>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="section-title">平台快捷订阅</div>
              <div class="section-caption">
                输入主页、UID 或用户名，不需要手写 RSSHub 路由
              </div>
            </div>
            <UButton
              class="shrink-0 whitespace-nowrap"
              color="primary"
              variant="soft"
              :icon="
                platformEditorOpen ? 'i-tabler-chevron-up' : 'i-tabler-plus'
              "
              @click="platformEditorOpen = !platformEditorOpen"
            >
              {{ platformEditorOpen ? '收起' : '新增订阅' }}
            </UButton>
          </div>
          <form
            v-if="platformEditorOpen"
            class="space-y-4 p-4"
            @submit.prevent="addPlatform"
          >
            <div
              class="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)_minmax(160px,0.55fr)]"
            >
              <UFormField label="订阅类型" description="更新只推送到当前群">
                <USelect
                  v-model="platform"
                  class="w-full"
                  :items="platformOptions"
                  value-key="value"
                  :disabled="!selectedGroupID"
                />
              </UFormField>
              <UFormField
                label="订阅对象"
                description="支持直接粘贴平台主页链接"
              >
                <UInput
                  v-model="platformTarget"
                  class="w-full"
                  :ui="{ root: 'w-full' }"
                  icon="i-tabler-link"
                  :placeholder="platformPlaceholder"
                  :disabled="!selectedGroupID"
                />
              </UFormField>
              <UFormField label="显示名称" description="可选">
                <UInput
                  v-model="platformName"
                  class="w-full"
                  :ui="{ root: 'w-full' }"
                  icon="i-tabler-tag"
                  placeholder="关注对象"
                  :disabled="!selectedGroupID"
                />
              </UFormField>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-zinc-500">
                X 路由受平台限制，稳定运行建议配置自己的 RSSHub 实例。
              </p>
              <UButton
                type="submit"
                icon="i-tabler-bell-plus"
                :loading="platformAdding"
                :disabled="!selectedGroupID || groupFeeds.length >= 10"
                >添加平台订阅</UButton
              >
            </div>
          </form>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="section-title">RSS / Atom 高级订阅</div>
              <div class="section-caption">
                适合博客、GitHub Releases 和自建 RSSHub 路由
              </div>
            </div>
            <UButton
              class="shrink-0 whitespace-nowrap"
              color="neutral"
              variant="soft"
              :icon="rawEditorOpen ? 'i-tabler-chevron-up' : 'i-tabler-rss'"
              @click="rawEditorOpen = !rawEditorOpen"
            >
              {{ rawEditorOpen ? '收起' : '展开' }}
            </UButton>
          </div>

          <form
            v-if="rawEditorOpen"
            class="space-y-4 p-4"
            @submit.prevent="add"
          >
            <div
              class="grid gap-3 md:grid-cols-[minmax(0,1.6fr)_minmax(180px,0.7fr)]"
            >
              <UFormField
                label="RSS / Atom 地址"
                description="仅允许公网 HTTP/HTTPS 地址"
              >
                <UInput
                  v-model="feedURL"
                  class="w-full"
                  :ui="{ root: 'w-full' }"
                  icon="i-tabler-link"
                  placeholder="https://example.com/feed.xml"
                  :disabled="!selectedGroupID"
                />
              </UFormField>
              <UFormField label="显示名称" description="留空时读取信息源标题">
                <UInput
                  v-model="feedName"
                  class="w-full"
                  :ui="{ root: 'w-full' }"
                  icon="i-tabler-tag"
                  placeholder="项目动态"
                  :disabled="!selectedGroupID"
                />
              </UFormField>
            </div>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs leading-5 text-zinc-500">
                添加时以当前最新内容为基线，不会把历史文章一次性刷进群。
              </p>
              <UButton
                type="submit"
                icon="i-tabler-plus"
                :loading="adding"
                :disabled="!selectedGroupID || groupFeeds.length >= 10"
                >添加订阅</UButton
              >
            </div>
          </form>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="section-title">当前信息源</div>
              <div class="section-caption">
                {{ selectedGroup?.group_name || '未选择群' }} ·
                {{ groupFeeds.length }} / 10
              </div>
            </div>
          </div>

          <div v-if="groupFeeds.length">
            <div
              v-for="row in groupFeeds"
              :key="row.id"
              class="data-row flex items-center gap-3 p-4 max-[720px]:flex-wrap"
            >
              <span
                class="grid size-[34px] shrink-0 place-items-center rounded-[10px] border border-[var(--border)] bg-[rgba(103,216,236,0.08)] text-[var(--cyan)]"
                ><UIcon name="i-tabler-rss" class="size-4"
              /></span>
              <div class="min-w-0 flex-1 max-[720px]:min-w-[calc(100%-48px)]">
                <div class="truncate text-sm font-medium text-white">
                  {{ row.name }}
                </div>
                <a
                  :href="row.url"
                  target="_blank"
                  rel="noreferrer"
                  class="mt-1 block truncate text-xs text-violet-300 hover:text-violet-200"
                  >{{ sourceHost(row.url) }}</a
                >
                <div class="mt-1 text-[11px] text-zinc-600">
                  {{
                    row.last_checked_at
                      ? `上次检查 ${formatDate(row.last_checked_at)}`
                      : `添加于 ${formatDate(row.created_at)}`
                  }}
                  <span v-if="row.next_check_at">
                    · 下次 {{ formatDate(row.next_check_at) }}</span
                  >
                </div>
                <div
                  v-if="row.enabled && row.last_error"
                  class="mt-1 truncate text-[11px] text-rose-400"
                  :title="row.last_error"
                >
                  {{ row.last_error }}
                </div>
              </div>
              <div
                class="flex shrink-0 items-center gap-3 whitespace-nowrap max-[720px]:w-full max-[720px]:justify-end max-[720px]:pl-11"
              >
                <UBadge
                  :color="
                    !row.enabled
                      ? 'neutral'
                      : row.consecutive_failures
                        ? 'error'
                        : 'success'
                  "
                  variant="subtle"
                >
                  {{
                    !row.enabled
                      ? '已暂停'
                      : row.consecutive_failures
                        ? `异常 ${row.consecutive_failures} 次`
                        : '运行正常'
                  }}
                </UBadge>
                <div
                  class="inline-flex min-h-8 items-center gap-[7px] rounded-[9px] border px-2 py-1 pl-[9px] text-[0.66rem] font-[650] transition-[border-color,background-color,color] duration-150"
                  :class="
                    row.translate_to_chinese
                      ? 'border-[rgba(125,108,200,0.34)] bg-[rgba(125,108,200,0.1)] text-[var(--violet-bright)] hover:border-[rgba(125,108,200,0.34)] hover:bg-[rgba(125,108,200,0.1)] hover:text-[var(--violet-bright)]'
                      : 'border-[rgba(147,141,168,0.2)] bg-white/[0.025] text-[var(--muted)] hover:border-[rgba(185,175,230,0.34)] hover:bg-white/[0.045] hover:text-[var(--ink-soft)]'
                  "
                >
                  <UIcon
                    name="i-tabler-language"
                    class="size-3.5"
                    aria-hidden="true"
                  />
                  <span>翻译中文</span>
                  <USwitch
                    :class="feedSwitchClass"
                    :model-value="row.translate_to_chinese"
                    color="primary"
                    :disabled="translationSaving[row.id]"
                    :aria-label="
                      row.translate_to_chinese
                        ? `关闭 ${row.name} 的中文翻译`
                        : `开启 ${row.name} 的中文翻译`
                    "
                    @update:model-value="setTranslation(row, Boolean($event))"
                  />
                </div>
                <USwitch
                  :class="feedSwitchClass"
                  :model-value="row.enabled"
                  color="primary"
                  :disabled="toggling[row.id]"
                  :aria-label="row.enabled ? '暂停订阅' : '恢复订阅'"
                  @update:model-value="setEnabled(row, Boolean($event))"
                />
                <MoreActions
                  label="订阅更多操作"
                  :items="[
                    {
                      label: '删除订阅',
                      description: '停止检查和推送',
                      icon: 'i-tabler-trash',
                      color: 'error',
                      onSelect: () => askDelete(row),
                    },
                  ]"
                />
              </div>
            </div>
          </div>
          <div v-else class="empty-state overview-empty">
            <span class="empty-icon"
              ><UIcon name="i-tabler-rss-off" class="size-6"
            /></span>
            <div class="empty-title">这个群还没有订阅</div>
            <div class="empty-description">
              添加信息源后，新内容会自动推送到群聊
            </div>
          </div>
        </section>
      </div>
    </div>

    <UModal
      v-model:open="confirmOpen"
      title="删除这个订阅？"
      description="删除后将停止检查和推送，操作无法撤销。"
      :ui="{
        overlay: 'z-40 bg-black/70 backdrop-blur-sm',
        content:
          'z-50 bg-zinc-900 text-zinc-100 ring ring-rose-500/30 divide-zinc-800 shadow-2xl',
        header: 'border-b border-zinc-800',
        body: 'bg-zinc-900',
        footer: 'border-t border-zinc-800 bg-zinc-900',
        title: 'text-white',
        description: 'text-zinc-400',
      }"
    >
      <template #body
        ><div class="surface-inset p-4 text-sm text-zinc-300">
          {{ pendingDelete?.name }}
          <div class="mt-1 truncate text-xs text-zinc-500">
            {{ pendingDelete?.url }}
          </div>
        </div></template
      >
      <template #footer
        ><div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="confirmOpen = false"
            >取消</UButton
          ><UButton
            color="error"
            icon="i-tabler-trash"
            :loading="deleting"
            @click="remove"
            >确认删除</UButton
          >
        </div></template
      >
    </UModal>
  </section>
</template>
