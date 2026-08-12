<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
  description: string
  icon: string
}>()
</script>

<template>
  <header class="page-hero">
    <div class="page-hero-copy">
      <div class="page-icon">
        <UIcon :name="icon" class="size-5" />
      </div>
      <div class="min-w-0">
        <div class="eyebrow">{{ eyebrow }}</div>
        <h1 class="page-title">{{ title }}</h1>
        <p class="page-subtitle">{{ description }}</p>
      </div>
    </div>
    <div class="page-actions">
      <slot />
    </div>
  </header>
</template>
