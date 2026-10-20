<script setup lang="ts">
withDefaults(
  defineProps<{ label?: string; tableClass?: string; loading?: boolean }>(),
  { label: '数据列表' },
)
</script>
<template>
  <div
    class="min-w-0 overflow-x-auto overscroll-x-contain focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary"
    role="region"
    :aria-label="label"
    :aria-busy="loading"
    tabindex="0"
  >
    <div
      v-if="loading"
      class="h-0.5 animate-pulse bg-primary/60 motion-reduce:animate-none"
      aria-hidden="true"
    />
    <table
      class="w-full border-collapse text-left text-sm [&_thead]:border-b [&_thead]:border-default [&_thead]:bg-muted/60 [&_th]:whitespace-nowrap [&_th]:px-5 [&_th]:py-3.5 [&_th]:text-xs [&_th]:font-medium [&_th]:text-muted [&_td]:px-5 [&_td]:py-4 [&_tbody>tr]:border-t [&_tbody>tr]:border-default [&_tbody>tr]:transition-colors [&_tbody>tr]:hover:bg-muted/40 [&_tbody>tr]:focus-within:bg-muted/40"
      :class="tableClass"
    >
      <slot />
    </table>
  </div>
</template>
