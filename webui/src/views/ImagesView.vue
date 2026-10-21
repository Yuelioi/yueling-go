<script setup lang="ts">
import FeatureWorkspace from '../components/FeatureWorkspace.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import { ref } from 'vue'
import ImageUploadView from './ImageUploadView.vue'
import ImageLibraryView from './ImageLibraryView.vue'
import { settingsAreas } from '../settings'
const uploadVersion = ref(0)
const tabs = [
  { label: '素材库', value: 'library', icon: 'i-tabler-library-photo' },
  { label: '批量上传', value: 'upload', icon: 'i-tabler-cloud-upload' },
  { label: '图片与打包配置', value: 'settings', icon: 'i-tabler-adjustments' },
]
</script>
<template>
  <FeatureWorkspace
    title="图片与打包"
    description="管理本地素材、图片来源与打包设置。"
    icon="i-tabler-photo"
    :tabs="tabs"
  >
    <template #library
      ><ImageLibraryView :upload-version="uploadVersion"
    /></template>
    <template #upload><ImageUploadView @uploaded="uploadVersion++" /></template>
    <template #settings
      ><RuntimeSettings :sections="settingsAreas.images.sections"
    /></template>
  </FeatureWorkspace>
</template>
