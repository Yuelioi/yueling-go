<script setup lang="ts">
import FeatureWorkspace from '../components/FeatureWorkspace.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import { settingsAreas } from '../settings'
import ModelUsageView from './ModelUsageView.vue'
import AIStyleView from './AIStyleView.vue'
const tabs = [
  { label: 'AI 配置', value: 'settings', icon: 'i-tabler-settings' },
  { label: '对话风格', value: 'style', icon: 'i-tabler-message-circle' },
  { label: '缓存与用量', value: 'usage', icon: 'i-tabler-chart-bar' },
]
</script>

<template>
  <FeatureWorkspace
    title="AI"
    description="模型、视觉、上下文与对话风格，都在这里管理。"
    icon="i-tabler-sparkles"
    :tabs="tabs"
  >
    <template #usage><ModelUsageView /></template>
    <template #style><AIStyleView /></template>
    <template #settings
      ><RuntimeSettings :sections="settingsAreas.ai.sections"
    /></template>
  </FeatureWorkspace>
</template>
