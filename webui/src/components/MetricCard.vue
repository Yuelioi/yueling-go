<script setup lang="ts">
withDefaults(defineProps<{
  label: string
  value: string | number
  detail: string
  icon: string
  tone?: 'violet' | 'cyan' | 'rose' | 'amber'
}>(), {
  tone: 'violet',
})
</script>

<template>
  <article class="metric-card" :class="`metric-${tone}`">
    <div class="metric-icon">
      <UIcon :name="icon" class="size-5" />
    </div>
    <div class="min-w-0">
      <div class="metric-label">{{ label }}</div>
      <div class="metric-value">{{ value }}</div>
      <div class="metric-detail">{{ detail }}</div>
    </div>
  </article>
</template>
