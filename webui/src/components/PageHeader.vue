<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
  description: string
  icon: string
}>()
</script>

<template>
  <header class="mb-6 flex items-start justify-between gap-6 max-[860px]:block">
    <div class="flex items-start gap-3.5 max-[520px]:gap-[11px]">
      <div class="grid size-[42px] shrink-0 place-items-center rounded-[13px] border border-[rgba(125,108,200,0.22)] bg-[var(--violet-soft)] text-[var(--violet-bright)] max-[520px]:size-[38px]">
        <UIcon :name="icon" class="size-5" />
      </div>
      <div class="min-w-0">
        <div class="text-[0.65rem] font-[780] uppercase tracking-[0.15em] text-[var(--violet-bright)]">{{ eyebrow }}</div>
        <h1 class="m-0 mt-1 text-[clamp(1.5rem,2vw,1.9rem)] font-[720] leading-[1.16] tracking-[-0.035em] text-[var(--ink)]">{{ title }}</h1>
        <p class="m-0 mt-[7px] max-w-[650px] text-[0.82rem] leading-[1.6] text-[var(--muted)]">{{ description }}</p>
      </div>
    </div>
    <div class="flex flex-wrap justify-end gap-[9px] max-[860px]:mt-4 max-[860px]:justify-start">
      <slot />
    </div>
  </header>
</template>
