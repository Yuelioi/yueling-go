<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type AIModelProfile, type AIConnection } from '../api'
import { useRuntimeConfiguration } from '../composables/useRuntimeConfiguration'
import { isAIModelField } from '../settings'
import PageHeader from '../components/PageHeader.vue'
import ConfigurationToolbar from '../components/ConfigurationToolbar.vue'
import ConfigurationGuard from '../components/ConfigurationGuard.vue'
import AIProfilesEditor from '../components/AIProfilesEditor.vue'
const { configuration, values, loading, saving, error, success, dirty, load, save, reset } = useRuntimeConfiguration(['ai'], api, isAIModelField)
const editor = ref<InstanceType<typeof AIProfilesEditor>>()
async function saveModels() { if (await save()) editor.value?.saved() }
onMounted(load)
</script>
<template>
  <section class="min-w-0 space-y-5">
    <ConfigurationGuard :dirty="!!dirty" :saving="saving" @discard="reset" />
    <PageHeader eyebrow="" title="AI 模型" description="管理模型账号与能力。添加模型后，在 AI 配置中选择使用它的功能。" icon="i-tabler-cpu" />
    <ConfigurationToolbar title="模型与账号" :dirty="!!dirty" :saving="saving" :loading="loading" @save="saveModels" @reset="reset">
      <UButton icon="i-tabler-plus" variant="soft" :disabled="saving || loading || !configuration || (values['ai.profiles']?.length ?? 0) >= 32" @click="editor?.addModel()">添加模型</UButton>
    </ConfigurationToolbar>
    <UAlert variant="soft" v-if="error" color="error" :description="error" />
    <UAlert variant="soft" v-if="success" color="success" :description="success" />
    <USkeleton v-if="loading && !configuration" class="h-64 rounded-xl" />
    <UButton v-if="!configuration && !loading" variant="soft" @click="load">重新加载</UButton>
    <AIProfilesEditor v-if="configuration" ref="editor" v-model="values['ai.profiles'] as AIModelProfile[]" v-model:connections="values['ai.connections'] as AIConnection[]" :bindings="values['ai.bindings'] || {}" :conversation="values['ai.conversation_profile']" :vision="values['ai.vision_profile']" :disabled="saving || loading" :saving="saving" :unsaved="!!dirty" :save-error="error" @save="saveModels" />
  </section>
</template>
