<script setup lang="ts">
import { computed } from 'vue'
import type { GroupInfo } from '../api'

const props = withDefaults(
  defineProps<{
    groups: GroupInfo[]
    title?: string
    description?: string
    placeholder?: string
    zeroLabel?: string
  }>(),
  {
    title: '群聊范围',
    description: '选择要管理的群聊',
    placeholder: '选择群聊',
    zeroLabel: '跨群范围',
  },
)

const selected = defineModel<number | null>({ required: true })

const options = computed(() =>
  props.groups.map((group) => ({
    label: group.group_name || String(group.group_id),
    description:
      group.group_id === 0 ? props.zeroLabel : `群号 ${group.group_id}`,
    value: group.group_id,
    icon: group.group_id === 0 ? 'i-tabler-world' : 'i-tabler-users-group',
  })),
)
</script>

<template>
  <UCard
    :ui="{
      body: 'flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between',
    }"
  >
    <div class="flex min-w-0 items-center gap-3">
      <UIcon name="i-tabler-users-group" class="size-5 shrink-0 text-primary" />
      <div class="min-w-0">
        <p class="text-sm font-medium text-highlighted">{{ title }}</p>
        <p class="mt-1 text-xs text-muted">{{ description }}</p>
      </div>
      <UBadge color="neutral" variant="soft">{{ groups.length }}</UBadge>
    </div>
    <USelectMenu
      v-model="selected"
      class="w-full sm:w-72"
      :items="options"
      value-key="value"
      label-key="label"
      description-key="description"
      :filter-fields="['label', 'description']"
      icon="i-tabler-users-group"
      :aria-label="title"
      :placeholder="placeholder"
      :search-input="{ placeholder: '搜索群名或群号', icon: 'i-tabler-search' }"
      :content="{ align: 'end', sideOffset: 7 }"
    >
      <template #empty>没有匹配的群聊</template>
    </USelectMenu>
  </UCard>
</template>
