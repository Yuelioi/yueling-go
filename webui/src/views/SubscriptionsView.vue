<script setup lang="ts">
import FeatureWorkspace from '../components/FeatureWorkspace.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import { settingsAreas } from '../settings'
import FeedView from './FeedView.vue'
const tabs = [
  { label: '订阅中心', value: 'center', icon: 'i-tabler-rss' },
  { label: '订阅配置', value: 'settings', icon: 'i-tabler-settings' },
]
</script>

<template>
  <FeatureWorkspace
    title="订阅"
    description="管理群聊信息源、推送策略和 RSSHub 服务。"
    icon="i-tabler-rss"
    :tabs="tabs"
  >
    <template #center><FeedView /></template>
    <template #settings
      ><RuntimeSettings :sections="settingsAreas.feeds.sections"
    /></template>
  </FeatureWorkspace>
</template>
