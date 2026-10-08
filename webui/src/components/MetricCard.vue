<script setup lang="ts">
const props = withDefaults(defineProps<{
  label: string
  value: string | number
  detail: string
  icon: string
  tone?: 'violet' | 'cyan' | 'rose' | 'amber'
}>(), {
  tone: 'violet',
})

const toneClasses = {
  violet: 'text-[var(--violet-bright)]',
  cyan: 'text-[var(--cyan)]',
  rose: 'text-[var(--rose)]',
  amber: 'text-[var(--amber)]',
} as const
</script>

<template>
  <article
    class="relative grid grid-cols-[42px_minmax(0,1fr)] gap-[13px] overflow-hidden rounded-[15px] border border-[var(--border)] bg-[linear-gradient(145deg,rgba(26,24,45,0.88),rgba(17,16,31,0.78))] p-[15px] shadow-[inset_0_1px_rgba(255,255,255,0.025)] after:absolute after:-right-10 after:-top-10 after:size-28 after:rounded-full after:bg-current after:opacity-[0.055] after:blur-[1px] after:content-['']"
    :class="toneClasses[props.tone]"
  >
    <div class="grid size-[42px] place-items-center rounded-xl bg-[color-mix(in_srgb,currentColor_12%,transparent)] text-inherit">
      <UIcon :name="icon" class="size-5" />
    </div>
    <div class="min-w-0">
      <div class="text-[0.68rem] text-[var(--muted)]">{{ label }}</div>
      <div class="mt-px text-[1.45rem] font-[720] leading-[1.2] tracking-[-0.04em] text-[var(--ink)]">{{ value }}</div>
      <div class="mt-0.5 truncate text-[0.66rem] text-[var(--dim)]">{{ detail }}</div>
    </div>
  </article>
</template>
