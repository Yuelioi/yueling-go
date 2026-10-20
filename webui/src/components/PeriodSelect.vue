<script setup lang="ts" generic="T extends string | number">
defineProps<{
  modelValue: T
  options: { label: string; value: T }[]
  label: string
}>()
defineEmits<{ 'update:modelValue': [value: T] }>()
</script>
<template>
  <div
    class="inline-flex max-w-full flex-wrap gap-1 rounded-xl border border-default bg-muted p-1"
    role="group"
    :aria-label="label"
  >
    <UButton
      v-for="option in options"
      :key="option.value"
      size="sm"
      :color="modelValue === option.value ? 'primary' : 'neutral'"
      :variant="modelValue === option.value ? 'soft' : 'ghost'"
      :aria-pressed="modelValue === option.value"
      @click="$emit('update:modelValue', option.value)"
      >{{ option.label }}</UButton
    >
  </div>
</template>
