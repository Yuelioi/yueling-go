<script setup lang="ts">
import FeedStrategyPanel from '../features/feeds/components/FeedStrategyPanel.vue'
import FeedPlatformPanel from '../features/feeds/components/FeedPlatformPanel.vue'
import FeedRawPanel from '../features/feeds/components/FeedRawPanel.vue'
import FeedListPanel from '../features/feeds/components/FeedListPanel.vue'
import { onMounted, watch } from 'vue'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'
import { useGroupFilter } from '../composables/useGroupFilter'
import { useFeed } from '../features/feeds/useFeed'

const state = useFeed()
const {
  groups,
  feeds,
  selectedGroupID,
  loading,
  error,
  notice,
  groupFeeds,
  coveredGroups,
  activeFeeds,
  failingFeeds,
  load,
} = state.catalog
const { settingsSaving, pendingCount, loadSettings } = state.settings
const { adding, platformAdding } = state.creation
const {
  diagnose,
  diagnosing,
  checking,
  deleting,
  confirmOpen,
  pendingDelete,
  check,
  remove,
} = state.actions

useGroupFilter(selectedGroupID)

watch(selectedGroupID, (groupID) => {
  notice.value = ''
  if (groupID) loadSettings(groupID)
})
onMounted(async () => {
  await load()
  if (selectedGroupID.value) await loadSettings(selectedGroupID.value)
})
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="订阅中心"
      description="把站点、博客、GitHub Releases 与 RSSHub 信息源聚合到群聊。"
      icon="i-tabler-rss"
    >
      <UButton
        color="neutral"
        variant="soft"
        icon="i-tabler-heart-rate-monitor"
        :loading="diagnosing"
        :disabled="loading || !groupFeeds.length"
        @click="diagnose()"
        >检测当前列表（不推送）</UButton
      >
      <UButton
        icon="i-tabler-refresh-dot"
        :loading="checking"
        :disabled="loading"
        @click="check"
        >检查并推送全部订阅</UButton
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

    <div class="grid grid-cols-3 gap-3 sm:gap-4">
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
        :detail="`${groups.length} 个群聊范围`"
        icon="i-tabler-users-group"
        tone="cyan"
      />
      <MetricCard
        label="异常源"
        :value="failingFeeds"
        :detail="
          selectedGroupID === null
            ? '全部群最近拉取异常'
            : `${pendingCount} 条等待向本群推送`
        "
        color="neutral"
        variant="soft"
        icon="i-tabler-heart-rate-monitor"
        tone="amber"
      />
    </div>

    <UAlert
      v-if="error"
      color="error"
      variant="soft"
      icon="i-tabler-alert-circle"
      :description="error"
    />
    <UAlert
      v-if="notice"
      color="success"
      variant="soft"
      icon="i-tabler-circle-check"
      :description="notice"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        allow-all
        :disabled="adding || platformAdding || settingsSaving"
        v-model="selectedGroupID"
        :groups="groups"
        title="订阅群聊"
        description="默认展示全部订阅；选择群后可新增订阅和编辑推送策略"
      />

      <div class="space-y-4">
        <FeedStrategyPanel
          :catalog="state.catalog"
          :settings="state.settings"
        />

        <FeedPlatformPanel
          :catalog="state.catalog"
          :creation="state.creation"
        />

        <FeedRawPanel :catalog="state.catalog" :creation="state.creation" />

        <FeedListPanel :catalog="state.catalog" :actions="state.actions" />
      </div>
    </div>

    <UModal
      v-model:open="confirmOpen"
      title="删除这个订阅？"
      description="删除后将停止检查和推送，操作无法撤销。"
    >
      <template #body
        ><div
          class="rounded-lg border border-default bg-muted p-4 text-sm text-toned"
        >
          {{ pendingDelete?.name }}
          <div class="mt-1 truncate text-xs text-muted">
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
