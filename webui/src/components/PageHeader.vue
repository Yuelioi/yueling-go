<script setup lang="ts">
import { inject } from 'vue'
import { nestedWorkspaceKey } from './workspaceContext'
const props = defineProps<{
  title: string
  description: string
  icon: string
  standalone?: boolean
}>()
const nested = inject(nestedWorkspaceKey, false)
</script>
<template>
  <header
    class="flex min-w-0 flex-col justify-between gap-4 sm:flex-row sm:items-start sm:gap-6"
    :class="nested && !props.standalone ? 'pb-1' : 'pb-2'"
  >
    <div class="flex min-w-0 items-start gap-4">
      <div
        v-if="!nested || props.standalone"
        class="grid size-12 shrink-0 place-items-center rounded-2xl bg-primary/10 text-primary ring ring-primary/20"
      >
        <UIcon :name="icon" class="size-6" />
      </div>
      <div class="min-w-0">
        <component
          :is="nested && !props.standalone ? 'h2' : 'h1'"
          class="font-semibold tracking-tight text-highlighted"
          :class="
            nested && !props.standalone
              ? 'text-lg leading-7'
              : 'text-2xl leading-8 sm:text-[1.75rem] sm:leading-9'
          "
          >{{ title }}</component
        >
        <p class="mt-2 max-w-2xl text-sm leading-6 text-muted">
          {{ description }}
        </p>
      </div>
    </div>
    <div
      v-if="$slots.default"
      class="flex shrink-0 flex-wrap items-center gap-2 sm:justify-end"
    >
      <slot />
    </div>
  </header>
</template>
