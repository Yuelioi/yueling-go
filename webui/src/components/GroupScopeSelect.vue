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

const selectUi = {
  content: 'z-[60] !w-[min(360px,calc(100vw-24px))]',
  input: 'mx-3 mb-2 mt-3 !min-h-10 !w-[calc(100%-24px)] !rounded-[10px] !border !border-[rgba(147,141,168,0.48)] !bg-[rgba(9,9,18,0.68)] !text-[var(--ink-soft)] !shadow-[inset_0_1px_rgba(255,255,255,0.025)] transition-[border-color,background-color,box-shadow] duration-150 hover:!border-[rgba(185,175,230,0.62)] hover:!bg-[rgba(18,17,32,0.92)] focus-within:!border-[rgba(149,135,212,0.9)] focus-within:!bg-[rgba(18,17,32,0.98)] focus-within:!shadow-[0_0_0_3px_rgba(125,108,200,0.16),inset_0_1px_rgba(255,255,255,0.04)] [&>input]:min-h-[38px] [&>input]:!border-0 [&>input]:!bg-transparent [&>input]:!text-[var(--ink)] [&>input]:!shadow-none [&>input]:caret-[var(--violet-bright)]',
  viewport: 'max-h-80 px-2 pb-2 pt-1',
  item: 'min-h-[43px]',
  itemDescription: 'mt-0.5 font-mono text-[0.6rem]',
}
</script>

<template>
  <div class="surface-panel flex min-h-16 items-center justify-between gap-5 px-3.5 py-[11px] max-[520px]:flex-col max-[520px]:items-stretch max-[520px]:gap-[9px]">
    <div class="flex min-w-0 items-center gap-2.5">
      <span class="grid size-8 shrink-0 place-items-center rounded-[10px] border border-[rgba(125,108,200,0.2)] bg-[var(--violet-soft)] text-[var(--violet-bright)]">
        <UIcon name="i-tabler-adjustments-horizontal" class="size-4" />
      </span>
      <span class="min-w-0">
        <span class="block text-xs font-[680] text-[var(--ink-soft)]">{{ title }}</span>
        <span class="mt-0.5 block truncate text-[0.64rem] text-[var(--dim)]">{{ description }}</span>
      </span>
      <span class="count-pill">{{ groups.length }}</span>
    </div>

    <USelectMenu
      v-model="selected"
      class="min-h-[34px] w-[min(310px,42vw)] !border-0 !bg-[var(--field)] !text-[var(--ink-soft)] !outline-0 !shadow-[inset_0_0_0_1px_var(--border-strong)] hover:!bg-[rgba(21,20,38,0.96)] hover:!shadow-[inset_0_0_0_1px_rgba(185,175,230,0.28)] focus-visible:!shadow-[inset_0_0_0_1px_var(--violet),0_0_0_3px_var(--violet-soft)] max-[520px]:w-full"
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
      :ui="selectUi"
    >
      <template #empty>
        没有匹配的群聊
      </template>
    </USelectMenu>
  </div>
</template>
