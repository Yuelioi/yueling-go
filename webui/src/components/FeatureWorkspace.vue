<script setup lang="ts">
import { computed, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageHeader from './PageHeader.vue'
import { nestedWorkspaceKey } from './workspaceContext'

provide(nestedWorkspaceKey, true)

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
const visited = ref(new Set([selected.value]))
watch(selected, (value) => {
  visited.value.add(value)
})
</script>

<template>
  <section class="min-w-0 space-y-6">
    <PageHeader
      standalone
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
      :ui="{ list: 'w-full overflow-x-auto', content: 'pt-6' }"
    >
      <template #content="{ item }"
        ><slot v-if="visited.has(item.value)" :name="item.value"
      /></template>
    </UTabs>
  </section>
</template>
