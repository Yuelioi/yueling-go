<script setup lang="ts">
import FeatureWorkspace from '../components/FeatureWorkspace.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import { settingsAreas } from '../settings'
import BilibiliView from './BilibiliView.vue'
const tabs = [
  { label: '监控与回复', value: 'messages', icon: 'i-tabler-message-circle' },
  { label: '账号配置', value: 'account', icon: 'i-tabler-user-circle' },
]
</script>

<template>
  <FeatureWorkspace
    title="B站"
    description="管理账号登录态、私信监控和回复规则。"
    icon="i-tabler-brand-bilibili"
    :tabs="tabs"
  >
    <template #messages><BilibiliView /></template>
    <template #account
      ><RuntimeSettings :sections="settingsAreas.bilibili.sections"
    /></template>
  </FeatureWorkspace>
</template>
