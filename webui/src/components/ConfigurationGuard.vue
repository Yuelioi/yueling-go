<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
const props = defineProps<{ dirty: boolean; saving: boolean }>()
const emit = defineEmits<{ discard: [] }>()
const open = ref(false)
let resolve: ((leave: boolean) => void) | undefined
function check() {
  if (props.saving) return false
  if (!props.dirty) return true
  resolve?.(false)
  open.value = true
  return new Promise<boolean>((done) => { resolve = done })
}
function choose(leave: boolean) {
  open.value = false
  if (leave) emit('discard')
  resolve?.(leave)
  resolve = undefined
}
onBeforeRouteLeave(check)
onBeforeRouteUpdate(check)
function beforeUnload(event: BeforeUnloadEvent) {
  if (props.dirty || props.saving) { event.preventDefault(); event.returnValue = '' }
}
window.addEventListener('beforeunload', beforeUnload)
onBeforeUnmount(() => { window.removeEventListener('beforeunload', beforeUnload); resolve?.(false) })
</script>
<template>
  <UModal :open="open" title="有修改尚未保存" description="离开会丢失当前页面的修改。" @update:open="!$event && choose(false)">
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="soft" @click="choose(false)">继续编辑</UButton>
        <UButton color="error" @click="choose(true)">放弃修改并离开</UButton>
      </div>
    </template>
  </UModal>
</template>
