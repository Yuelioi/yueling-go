<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>
<template>
  <section
    class="flex min-w-0 flex-col gap-4 rounded-2xl border border-default bg-default p-5 sm:flex-row sm:flex-wrap sm:items-end"
    :aria-label="title"
  >
    <div class="min-w-0 sm:mr-auto">
      <h2 class="text-sm font-semibold text-highlighted">{{ title }}</h2>
      <p v-if="description" class="mt-1 text-xs leading-5 text-muted">
        {{ description }}
      </p>
    </div>
    <div
      class="flex min-w-0 flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end [&>*]:w-full sm:[&>*]:w-auto"
    >
      <slot />
    </div>
  </section>
</template>
