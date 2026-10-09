<script setup lang="ts">
import RuntimePageHeader from '../components/PageHeader.vue'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import ImageLibraryView from './ImageLibraryView.vue'
import { settingsAreas } from '../settings'
const route = useRoute()
const router = useRouter()
const selected = computed({
  get: () => route.query.tab === 'settings' ? 'settings' : 'library',
  set: (value: string | number) => { void router.replace({ query: { ...route.query, tab: String(value) } }) },
})
const tabs = [
  { label: '素材库', value: 'library', icon: 'i-tabler-library-photo' },
  { label: '图片与打包配置', value: 'settings', icon: 'i-tabler-adjustments' },
]
</script>
<template>
  <section class="min-w-0 space-y-6">
    <RuntimePageHeader eyebrow="" title="图片与打包" description="管理本地素材与图片来源" icon="i-tabler-photo" />
    <UTabs v-model="selected" :items="tabs" variant="link" :unmount-on-hide="false" :ui="{ list: 'justify-start border-b border-default', content: 'pt-5' }">
      <template #content="{ item }">
        <ImageLibraryView v-if="item.value === 'library'" />
        <RuntimeSettings v-else :sections="settingsAreas.images.sections" />
      </template>
    </UTabs>
  </section>
</template>
