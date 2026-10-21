<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { type AIModelProfile, type AIConnection } from '../api'
import { useRuntimeConfiguration } from '../composables/useRuntimeConfiguration'
import { isAIModelField } from '../settings'
import PageHeader from '../components/PageHeader.vue'
import ConfigurationToolbar from '../components/ConfigurationToolbar.vue'
import ConfigurationGuard from '../components/ConfigurationGuard.vue'
import AIProfilesEditor from '../components/AIProfilesEditor.vue'
const {
  configuration,
  values,
  loading,
  saving,
  error,
  success,
  dirty,
  load,
  save,
  reset,
} = useRuntimeConfiguration(['ai'], undefined, isAIModelField)
const editor = ref<InstanceType<typeof AIProfilesEditor>>()
const editing = ref(false)
const busy = ref(false)
async function saveModels() {
  if (await save()) await editor.value?.saved()
}
function discard() {
  reset()
  editor.value?.discard()
}
onMounted(load)
</script>
<template>
  <section class="min-w-0 space-y-6">
    <ConfigurationGuard
      :dirty="!!dirty || editing"
      :saving="saving || busy"
      @discard="discard"
    />
    <PageHeader
      title="AI 模型"
      description="先连接服务账号，再添加该账号提供的模型。"
      icon="i-tabler-cpu"
    >
      <UButton
        icon="i-tabler-plus"
        :disabled="
          saving ||
          busy ||
          loading ||
          !configuration ||
          (values['ai.connections']?.length ?? 0) >= 32
        "
        @click="editor?.addAccount()"
        >连接 AI 服务</UButton
      >
    </PageHeader>
    <ConfigurationToolbar
      v-if="dirty"
      title="有修改尚未保存"
      :dirty="!!dirty"
      :saving="saving"
      :loading="loading || busy"
      @save="saveModels"
      @reset="discard"
    />
    <UAlert
      icon="i-tabler-alert-circle"
      variant="soft"
      v-if="error"
      color="error"
      :description="error"
    />
    <UAlert
      icon="i-tabler-circle-check"
      variant="soft"
      v-if="success"
      color="success"
      :description="success"
    />
    <USkeleton v-if="loading && !configuration" class="h-64 rounded-xl" />
    <UButton v-if="!configuration && !loading" variant="soft" @click="load"
      >重新加载</UButton
    >
    <AIProfilesEditor
      v-if="configuration"
      ref="editor"
      v-model="values['ai.profiles'] as AIModelProfile[]"
      v-model:connections="values['ai.connections'] as AIConnection[]"
      :bindings="values['ai.bindings'] || {}"
      :disabled="saving || loading"
      :saving="saving"
      :unsaved="!!dirty"
      :save-error="error"
      @save="saveModels"
      @editing="editing = $event"
      @busy="busy = $event"
    />
  </section>
</template>
