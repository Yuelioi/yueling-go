<script setup lang="ts">
import { computed } from 'vue'
import type { GroupInfo } from '../api'

const props = withDefaults(defineProps<{
  groups: GroupInfo[]
  title?: string
  description?: string
  placeholder?: string
  zeroLabel?: string
}>(), {
  title: '群聊范围',
  description: '选择要管理的群聊',
  placeholder: '选择群聊',
  zeroLabel: '跨群范围',
})

const selected = defineModel<number | null>({ required: true })

const options = computed(() => props.groups.map((group) => ({
  label: group.group_name || String(group.group_id),
  description: group.group_id === 0 ? props.zeroLabel : `群号 ${group.group_id}`,
  value: group.group_id,
  icon: group.group_id === 0 ? 'i-tabler-world' : 'i-tabler-users-group',
})))
</script>

<template>
  <div class="group-scope-bar surface-panel">
    <div class="group-scope-copy">
      <span class="group-scope-icon">
        <UIcon name="i-tabler-adjustments-horizontal" class="size-4" />
      </span>
      <span class="min-w-0">
        <span class="group-scope-title">{{ title }}</span>
        <span class="group-scope-description">{{ description }}</span>
      </span>
      <span class="count-pill">{{ groups.length }}</span>
    </div>

    <USelectMenu
      v-model="selected"
      class="group-scope-select"
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
      :ui="{
        content: 'group-scope-menu',
        viewport: 'group-scope-menu-viewport',
        item: 'group-scope-menu-item',
        itemDescription: 'group-scope-menu-description',
      }"
    >
      <template #empty>
        没有匹配的群聊
      </template>
    </USelectMenu>
  </div>
</template>
