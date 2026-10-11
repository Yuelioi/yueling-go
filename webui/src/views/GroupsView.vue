<script setup lang="ts">
import GroupAdministrationTable from '../components/GroupAdministrationTable.vue'
import PageHeader from '../components/PageHeader.vue'
</script>
<template>
  <section class="space-y-5">
    <PageHeader eyebrow="Group administration" title="群管理总览" description="查看各群配置和订阅异常，按群名或群号筛选，再进入对应功能管理。" icon="i-tabler-users-group" />
    <GroupAdministrationTable include-default>
      <template #actions="{ row }">
        <div class="flex min-w-48 flex-wrap gap-2">
          <UButton v-if="row.group_id" :to="{ path: '/chat-insights', query: { group: row.group_id } }" size="xs" variant="soft">聊天</UButton>
          <UButton v-if="row.connected" :to="{ path: '/group-actions', query: { group: row.group_id } }" size="xs" variant="soft">消息</UButton>
          <UButton v-if="row.group_id" :to="{ path: '/feeds', query: { group: row.group_id } }" size="xs" variant="soft">订阅</UButton>
          <UButton v-if="row.group_id" :to="{ path: '/plugins', query: { tab: 'groups', group: row.group_id } }" size="xs" variant="soft">插件</UButton>
          <UButton :to="{ path: '/ai', query: { tab: 'style', group: row.group_id } }" size="xs" variant="soft">风格</UButton>
          <UButton :to="{ path: '/join-review', query: { group: row.group_id } }" size="xs" variant="soft">审核</UButton>
          <UButton v-if="row.group_id" :to="{ path: '/digests', query: { group: row.group_id } }" size="xs" variant="soft">日报</UButton>
          <UButton :to="{ path: '/knowledge', query: { group: row.group_id } }" size="xs" variant="soft">知识</UButton>
        </div>
      </template>
    </GroupAdministrationTable>
  </section>
</template>
