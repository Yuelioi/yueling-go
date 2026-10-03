<script setup lang="ts">
import FeatureWorkspace from '../components/FeatureWorkspace.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import { settingsAreas } from '../settings'
import PluginGroupsView from './PluginGroupsView.vue'
const tabs = [
  { label: '群级策略', value: 'groups', icon: 'i-tabler-users-group' },
  { label: '全局开关', value: 'settings', icon: 'i-tabler-adjustments' },
]
</script>

<template>
  <FeatureWorkspace
    title="插件"
    description="管理全局开关与各个群聊的插件策略。"
    icon="i-tabler-components"
    :tabs="tabs"
  >
    <template #groups><PluginGroupsView /></template>
    <template #settings
      ><RuntimeSettings :sections="settingsAreas.plugins.sections"
    /></template>
  </FeatureWorkspace>
</template>
