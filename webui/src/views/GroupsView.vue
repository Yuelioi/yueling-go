<script setup lang="ts">
import GroupAdministrationTable from '../components/GroupAdministrationTable.vue'
import PageHeader from '../components/PageHeader.vue'
import MoreActions from '../components/MoreActions.vue'
import type { GroupAdministration } from '../api'

function groupActions(row: GroupAdministration) {
  const group = row.group_id
  return [
    ...(group
      ? [
          {
            label: '订阅与推送',
            icon: 'i-tabler-rss',
            to: { path: '/feeds', query: { group } },
          },
        ]
      : []),
    ...(group
      ? [
          {
            label: '插件策略',
            icon: 'i-tabler-components',
            to: { path: '/plugins', query: { tab: 'groups', group } },
          },
        ]
      : []),
    {
      label: '对话风格',
      icon: 'i-tabler-sparkles',
      to: { path: '/ai', query: { tab: 'style', group } },
    },
    {
      label: '入群审核',
      icon: 'i-tabler-user-check',
      to: { path: '/join-review', query: { group } },
    },
    ...(group
      ? [
          {
            label: '群聊日报',
            icon: 'i-tabler-notes',
            to: { path: '/digests', query: { group } },
          },
        ]
      : []),
    {
      label: '知识资料',
      icon: 'i-tabler-books',
      to: { path: '/knowledge', query: { group } },
    },
  ]
}
</script>
<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      title="群管理总览"
      description="查看各群配置和订阅异常，按群名或群号筛选，再进入对应功能管理。"
      icon="i-tabler-users-group"
    />
    <GroupAdministrationTable include-default>
      <template #actions="{ row }">
        <div class="flex items-center gap-2">
          <UButton
            v-if="row.group_id"
            :to="{ path: '/chat-insights', query: { group: row.group_id } }"
            size="xs"
            color="neutral"
            variant="soft"
            >洞察</UButton
          >
          <UButton
            v-if="row.connected"
            :to="{ path: '/group-actions', query: { group: row.group_id } }"
            size="xs"
            color="neutral"
            variant="soft"
            >消息</UButton
          >
          <MoreActions
            :label="`管理${row.group_name || '全局默认'}的功能`"
            show-label
            :items="groupActions(row)"
          />
        </div>
      </template>
    </GroupAdministrationTable>
  </section>
</template>
