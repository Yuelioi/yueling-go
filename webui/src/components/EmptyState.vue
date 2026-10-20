<script setup lang="ts">
withDefaults(
  defineProps<{ title: string; description?: string; icon?: string }>(),
  { icon: 'i-tabler-inbox' },
)
</script>
<template>
  <div
    class="flex min-w-0 flex-col items-center justify-center px-6 py-12 text-center"
    role="status"
  >
    <div
      class="mb-4 grid size-12 place-items-center rounded-2xl bg-muted ring ring-default"
    >
      <UIcon :name="icon" class="size-6 text-muted" />
    </div>
    <p class="text-sm font-semibold text-highlighted">{{ title }}</p>
    <p v-if="description" class="mt-2 max-w-sm text-sm leading-6 text-muted">
      {{ description }}
    </p>
    <div v-if="$slots.default" class="mt-5 flex flex-wrap justify-center gap-2">
      <slot />
    </div>
  </div>
</template>
