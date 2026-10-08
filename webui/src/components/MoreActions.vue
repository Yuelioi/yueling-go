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
      content: '!min-w-[186px]',
      item: '!min-h-[34px] !text-[0.72rem]',
      separator: '!my-[3px]',
    }"
  >
    <UButton
      class="!min-h-7 !min-w-7 justify-center !px-[7px]"
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
