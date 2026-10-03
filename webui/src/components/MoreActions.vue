<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'

withDefaults(
  defineProps<{
    items: DropdownMenuItem[]
    label?: string
    showLabel?: boolean
    disabled?: boolean
  }>(),
  {
    label: '更多操作',
    showLabel: false,
    disabled: false,
  },
)
</script>

<template>
  <UDropdownMenu :items="items" :content="{ align: 'end', sideOffset: 6 }">
    <UButton
      color="neutral"
      variant="ghost"
      size="xs"
      icon="i-tabler-dots"
      :label="showLabel ? '更多' : undefined"
      :aria-label="label"
      :title="label"
      :disabled="disabled"
    />
  </UDropdownMenu>
</template>
