<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'

withDefaults(defineProps<{
  items: DropdownMenuItem[]
  label?: string
  showLabel?: boolean
  disabled?: boolean
}>(), {
  label: '更多操作',
  showLabel: false,
  disabled: false,
})
</script>

<template>
  <UDropdownMenu
    :items="items"
    :content="{ align: 'end', sideOffset: 6 }"
    :ui="{
      content: 'action-menu-content',
      item: 'action-menu-item',
      separator: 'action-menu-separator',
    }"
  >
    <UButton
      class="more-action-trigger"
      color="neutral"
      variant="soft"
      size="xs"
      icon="i-tabler-dots"
      :label="showLabel ? '更多' : undefined"
      :aria-label="label"
      :title="label"
      :disabled="disabled"
    />
  </UDropdownMenu>
</template>
