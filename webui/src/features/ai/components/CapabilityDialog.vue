<script setup lang="ts">
import { computed } from 'vue'
import type { AIConnection } from '../../../api/ai.ts'
import { accountDestination } from '../../../ai-models.ts'
import type { CapabilityTest } from '../capabilityTest.ts'
const props = defineProps<{
  test: CapabilityTest
  connections: AIConnection[]
  unsaved?: boolean
  disabled?: boolean
}>()
const {
  target: selected,
  open: testOpen,
  busy: testing,
  capability: testCapability,
  result: testResult,
  image: testImage,
  options: testOptions,
  run,
} = props.test
const modelAccount = computed(() =>
  props.connections.find((c) => c.id === selected.value?.connection_id),
)
</script>
<template>
  <UModal
    v-model:open="testOpen"
    title="测试能力"
    :description="`${selected?.name || selected?.model} · ${accountDestination(modelAccount)}`"
    :close="!testing"
    :dismissible="!testing"
    ><template #body
      ><div class="space-y-4">
        <UFormField label="要测试的能力"
          ><USelect
            v-model="testCapability"
            :items="testOptions"
            class="w-full"
            :disabled="testing"
        /></UFormField>
        <p class="text-sm leading-6 text-muted">
          测试会发起真实请求并消耗账号额度。每次只验证所选能力；无需测试也可保存模型。
        </p>
        <UAlert
          v-if="testResult"
          color="neutral"
          variant="soft"
          :description="testResult"
        /><img
          v-if="testImage"
          :src="testImage"
          alt="测试生成的图片"
          class="max-h-80 w-full rounded-lg object-contain"
        /></div></template
    ><template #footer
      ><UButton
        :loading="testing"
        :disabled="unsaved || disabled || !testOptions.length"
        @click="run"
        >开始测试</UButton
      ></template
    ></UModal
  >
</template>
