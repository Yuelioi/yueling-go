<script setup lang="ts">
defineProps<{
  title: string
  dirty: boolean
  saving: boolean
  loading: boolean
}>()
defineEmits<{ save: []; reset: [] }>()
</script>
<template>
  <div
    class="sticky top-20 z-20 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-default bg-elevated/95 p-4 shadow-lg shadow-black/10 backdrop-blur-xl"
  >
    <div class="flex items-center gap-3">
      <h2 class="text-sm font-semibold text-highlighted">{{ title }}</h2>
      <UBadge :color="dirty ? 'warning' : 'neutral'" variant="soft">{{
        dirty ? '未保存' : '已保存'
      }}</UBadge>
    </div>
    <div class="flex items-center gap-2">
      <UButton
        v-if="dirty"
        color="neutral"
        variant="ghost"
        :disabled="saving || loading"
        @click="$emit('reset')"
        >撤销修改</UButton
      >
      <slot />
      <UButton
        icon="i-tabler-device-floppy"
        :loading="saving"
        :disabled="!dirty || loading"
        @click="$emit('save')"
        >保存修改</UButton
      >
    </div>
  </div>
</template>
