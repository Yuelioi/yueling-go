<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
  description: string
  icon: string
}>()
</script>

<template>
  <header
    class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"
  >
    <div class="flex min-w-0 gap-3">
      <div
        class="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"
      >
        <UIcon :name="icon" class="size-5" />
      </div>
      <div class="min-w-0">
        <h1 class="text-2xl font-semibold tracking-tight text-highlighted">
          {{ title }}
        </h1>
        <p class="mt-1 text-sm leading-6 text-muted">{{ description }}</p>
      </div>
    </div>
    <div
      v-if="$slots.default"
      class="flex shrink-0 flex-wrap items-center gap-2"
    >
      <slot />
    </div>
  </header>
</template>
