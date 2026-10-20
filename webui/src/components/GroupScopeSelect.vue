<script setup lang="ts">
import { computed } from 'vue'
import type { GroupInfo } from '../api'

const props = withDefaults(
  defineProps<{
    groups: GroupInfo[]
    title?: string
    description?: string
    placeholder?: string
    disabled?: boolean
    allowAll?: boolean
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

const selection = computed({
  get: () =>
    selected.value === null
      ? props.allowAll
        ? -1
        : undefined
      : selected.value,
  set: (value: number | undefined) => {
    selected.value = value === -1 || value === undefined ? null : value
  },
})
const options = computed(() => [
  ...(props.allowAll
    ? [
        {
          label: '全部群 / 全部范围',
          description: '查看全部，可按群筛选',
          value: -1,
          icon: 'i-tabler-world',
        },
      ]
    : []),
  ...props.groups.map((group) => ({
    label: group.group_name || String(group.group_id),
    description:
      group.group_id === 0 ? props.zeroLabel : `群号 ${group.group_id}`,
    value: group.group_id,
    icon: group.group_id === 0 ? 'i-tabler-world' : 'i-tabler-users-group',
  })),
])
</script>

<template>
  <section
    class="flex min-w-0 flex-col gap-4 rounded-2xl border border-default bg-default p-5 sm:flex-row sm:items-center sm:justify-between"
    :aria-label="title"
  >
    <div class="flex min-w-0 items-center gap-3">
      <span
        class="grid size-9 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary"
        ><UIcon name="i-tabler-adjustments-horizontal" class="size-4"
      /></span>
      <div class="min-w-0">
        <h2 class="text-sm font-semibold text-highlighted">{{ title }}</h2>
        <p class="mt-1 text-xs leading-5 text-muted">{{ description }}</p>
      </div>
      <UBadge color="neutral" variant="soft" class="shrink-0">{{
        groups.length
      }}</UBadge>
    </div>
    <USelectMenu
      v-model="selection"
      :disabled="disabled"
      class="w-full sm:w-72 sm:shrink-0"
      :items="options"
      value-key="value"
      label-key="label"
      description-key="description"
      :filter-fields="['label', 'description']"
      icon="i-tabler-users-group"
      :aria-label="title"
      :placeholder="placeholder"
      :search-input="{ placeholder: '搜索群名或群号', icon: 'i-tabler-search' }"
      :content="{ align: 'end', sideOffset: 8 }"
    >
      <template #empty>没有匹配的群聊</template>
    </USelectMenu>
  </section>
</template>
