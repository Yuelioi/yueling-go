<script setup lang="ts">
import FeatureWorkspace from '../components/FeatureWorkspace.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import ImageLibraryView from './ImageLibraryView.vue'
import { settingsAreas } from '../settings'
const tabs = [
  { label: '素材库', value: 'library', icon: 'i-tabler-library-photo' },
  { label: '图片与打包配置', value: 'settings', icon: 'i-tabler-adjustments' },
]
</script>
<template>
  <FeatureWorkspace title="图片与打包" description="浏览本地素材，选择需要移除的图片。" icon="i-tabler-photo" :tabs="tabs">
    <template #library><ImageLibraryView /></template>
    <template #settings><RuntimeSettings :sections="settingsAreas.images.sections" /></template>
  </FeatureWorkspace>
</template>
