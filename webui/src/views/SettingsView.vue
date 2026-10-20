<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PageHeader from '../components/PageHeader.vue'
import RuntimeSettings from '../components/RuntimeSettings.vue'
import { settingsAreas } from '../settings'
const route = useRoute()
const area = computed(
  () => settingsAreas[route.meta.settingsArea as keyof typeof settingsAreas],
)
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      :title="String(route.meta.title)"
      :description="String(route.meta.description)"
      :icon="String(route.meta.icon)"
    />
    <RuntimeSettings
      :key="route.path"
      :sections="area.sections"
      :show-bootstrap="route.meta.settingsArea === 'system'"
    />
  </section>
</template>
