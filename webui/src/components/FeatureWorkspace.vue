<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageHeader from './PageHeader.vue'

const props = defineProps<{
  title: string
  description: string
  icon: string
  tabs: { label: string; value: string; icon: string }[]
}>()
const route = useRoute()
const router = useRouter()
const selected = computed({
  get: () =>
    props.tabs.some((tab) => tab.value === route.query.tab)
      ? String(route.query.tab)
      : props.tabs[0]!.value,
  set: (value: string | number) => {
    void router.replace({ query: { ...route.query, tab: String(value) } })
  },
})
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      eyebrow=""
      :title="title"
      :description="description"
      :icon="icon"
    />
    <UTabs
      v-model="selected"
      :items="tabs"
      variant="link"
      :unmount-on-hide="false"
      class="min-w-0"
      :ui="{ list: 'w-fit max-w-full', content: 'pt-5' }"
    >
      <template #content="{ item }"><slot :name="item.value" /></template>
    </UTabs>
  </section>
</template>
