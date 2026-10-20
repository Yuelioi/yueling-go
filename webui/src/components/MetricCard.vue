<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    label: string
    value: string | number
    detail: string
    icon: string
    tone?: 'violet' | 'cyan' | 'rose' | 'amber'
  }>(),
  {
    tone: 'violet',
  },
)

const toneClasses = {
  violet: 'text-[var(--violet-bright)]',
  cyan: 'text-[var(--cyan)]',
  rose: 'text-[var(--rose)]',
  amber: 'text-[var(--amber)]',
} as const
</script>

<template>
  <article
    class="@container min-w-0 rounded-2xl border border-default bg-default"
  >
    <div class="flex items-start gap-4 p-3.5 @min-[14rem]:p-5">
      <div
        class="hidden size-10 shrink-0 place-items-center rounded-xl bg-muted @min-[14rem]:grid"
        :class="toneClasses[props.tone]"
      >
        <UIcon :name="icon" class="size-5" />
      </div>
      <div class="min-w-0">
        <p class="text-xs font-medium leading-5 text-muted">{{ label }}</p>
        <p
          class="mt-1 break-words text-lg font-semibold leading-7 tracking-tight text-highlighted tabular-nums @min-[14rem]:mt-2 @min-[14rem]:text-2xl @min-[14rem]:leading-8"
        >
          {{ value }}
        </p>
        <p class="mt-1 hidden text-xs leading-5 text-muted @min-[14rem]:block">
          {{ detail }}
        </p>
      </div>
    </div>
  </article>
</template>
