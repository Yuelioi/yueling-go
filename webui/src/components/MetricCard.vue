<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    value: string | number
    detail: string
    icon: string
    tone?: 'violet' | 'cyan' | 'rose' | 'amber'
  }>(),
  { tone: 'violet' },
)
const tones = {
  violet: 'bg-primary/10 text-primary',
  cyan: 'bg-info/10 text-info',
  rose: 'bg-error/10 text-error',
  amber: 'bg-warning/10 text-warning',
}
</script>

<template>
  <UCard class="min-w-0" :ui="{ body: 'flex items-start gap-3' }">
    <div
      class="flex size-9 shrink-0 items-center justify-center rounded-lg"
      :class="tones[tone]"
    >
      <UIcon :name="icon" class="size-4" />
    </div>
    <div class="min-w-0">
      <p class="text-sm text-muted">{{ label }}</p>
      <p
        class="mt-1 break-words text-2xl font-semibold tracking-tight text-highlighted"
      >
        {{ value }}
      </p>
      <p class="mt-1 text-xs leading-5 text-dimmed">{{ detail }}</p>
    </div>
  </UCard>
</template>
