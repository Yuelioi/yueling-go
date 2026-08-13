<script setup lang="ts">
import { computed, ref } from 'vue'
import type { GroupInfo } from '../api'

const props = withDefaults(defineProps<{
  groups: GroupInfo[]
  title?: string
  description?: string
  emptyTitle?: string
  emptyDescription?: string
}>(), {
  title: '选择群聊',
  description: '搜索群名或群号',
  emptyTitle: '暂无可用群聊',
  emptyDescription: '请确认 Bot 已连接到 NapCat',
})

const selected = defineModel<number | null>({ required: true })
const query = ref('')

const filteredGroups = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return props.groups
  return props.groups.filter((group) =>
    `${group.group_name} ${group.group_id}`.toLowerCase().includes(keyword),
  )
})
</script>

<template>
  <aside class="group-picker surface-panel">
    <div class="group-picker-head">
      <div>
        <div class="section-title">{{ title }}</div>
        <div class="section-caption">{{ description }}</div>
      </div>
      <span class="count-pill">{{ groups.length }}</span>
    </div>

    <div class="group-picker-search">
      <UInput
        v-model="query"
        class="w-full"
        :ui="{ root: 'w-full' }"
        icon="i-tabler-search"
        placeholder="搜索群聊"
      />
    </div>

    <div v-if="filteredGroups.length" class="group-picker-list">
      <button
        v-for="group in filteredGroups"
        :key="group.group_id"
        type="button"
        class="group-option"
        :class="{ 'group-option-active': selected === group.group_id }"
        @click="selected = group.group_id"
      >
        <span class="group-avatar">
          <UIcon v-if="group.group_id === 0" name="i-tabler-world" class="size-4" />
          <template v-else>{{ (group.group_name || String(group.group_id)).slice(0, 1) }}</template>
        </span>
        <span class="min-w-0 flex-1">
          <span class="group-name">{{ group.group_name || group.group_id }}</span>
          <span class="group-id">{{ group.group_id === 0 ? '共享知识空间' : group.group_id }}</span>
        </span>
        <UIcon
          v-if="selected === group.group_id"
          name="i-tabler-check"
          class="size-4 shrink-0 text-violet-200"
        />
      </button>
    </div>

    <div v-else class="empty-state group-picker-empty">
      <span class="empty-icon"><UIcon name="i-tabler-users-off" class="size-6" /></span>
      <div class="empty-title">{{ emptyTitle }}</div>
      <div class="empty-description">{{ emptyDescription }}</div>
    </div>
  </aside>
</template>
