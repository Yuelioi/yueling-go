<script setup lang="ts">
import type { useFeed } from '../useFeed.ts'
const props = defineProps<{
  catalog: Pick<
    ReturnType<typeof useFeed>['catalog'],
    'selectedGroupID' | 'groupFeeds'
  >
  creation: Pick<
    ReturnType<typeof useFeed>['creation'],
    'feedURL' | 'feedName' | 'adding' | 'rawEditorOpen' | 'add'
  >
}>()
const { selectedGroupID, groupFeeds } = props.catalog
const { feedURL, feedName, adding, rawEditorOpen, add } = props.creation
</script>
<template>
  <section
    v-if="selectedGroupID !== null"
    class="rounded-2xl border border-default bg-default overflow-hidden"
  >
    <div
      class="flex flex-col items-stretch justify-between gap-4 border-b border-default px-5 py-4 sm:flex-row sm:items-center"
    >
      <div class="min-w-0">
        <div class="text-sm font-semibold text-highlighted">
          RSS / Atom 高级订阅
        </div>
        <div class="mt-1 text-xs leading-5 text-muted">
          适合博客、GitHub Releases 和自建 RSSHub 路由
        </div>
      </div>
      <UButton
        class="shrink-0 whitespace-nowrap"
        color="neutral"
        variant="soft"
        :icon="rawEditorOpen ? 'i-tabler-chevron-up' : 'i-tabler-rss'"
        @click="rawEditorOpen = !rawEditorOpen"
      >
        {{ rawEditorOpen ? '收起' : '展开' }}
      </UButton>
    </div>

    <form v-if="rawEditorOpen" class="space-y-4 p-4" @submit.prevent="add">
      <div
        class="grid gap-3 md:grid-cols-[minmax(0,1.6fr)_minmax(180px,0.7fr)]"
      >
        <UFormField
          label="RSS / Atom 地址"
          description="仅允许公网 HTTP/HTTPS 地址"
        >
          <UInput
            v-model="feedURL"
            class="w-full"
            :ui="{ root: 'w-full' }"
            icon="i-tabler-link"
            placeholder="https://example.com/feed.xml"
            :disabled="!selectedGroupID"
          />
        </UFormField>
        <UFormField label="显示名称" description="留空时读取信息源标题">
          <UInput
            v-model="feedName"
            class="w-full"
            :ui="{ root: 'w-full' }"
            icon="i-tabler-tag"
            placeholder="项目动态"
            :disabled="!selectedGroupID"
          />
        </UFormField>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs leading-5 text-muted">
          添加时以当前最新内容为基线，不会把历史文章一次性刷进群。
        </p>
        <UButton
          type="submit"
          icon="i-tabler-plus"
          :loading="adding"
          :disabled="!selectedGroupID || groupFeeds.length >= 10"
          >添加订阅</UButton
        >
      </div>
    </form>
  </section>
</template>
