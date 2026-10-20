<script setup lang="ts">
import type { useFeed } from '../useFeed.ts'
const props = defineProps<{
  catalog: Pick<
    ReturnType<typeof useFeed>['catalog'],
    'selectedGroupID' | 'groupFeeds'
  >
  creation: Pick<
    ReturnType<typeof useFeed>['creation'],
    | 'platformName'
    | 'platform'
    | 'platformTarget'
    | 'platformAdding'
    | 'platformEditorOpen'
    | 'platformOptions'
    | 'platformPlaceholder'
    | 'addPlatform'
  >
}>()
const { selectedGroupID, groupFeeds } = props.catalog
const {
  platformName,
  platform,
  platformTarget,
  platformAdding,
  platformEditorOpen,
  platformOptions,
  platformPlaceholder,
  addPlatform,
} = props.creation
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
        <div class="text-sm font-semibold text-highlighted">平台快捷订阅</div>
        <div class="mt-1 text-xs leading-5 text-muted">
          输入主页、UID 或用户名，不需要手写 RSSHub 路由
        </div>
      </div>
      <UButton
        class="shrink-0 whitespace-nowrap"
        color="primary"
        variant="soft"
        :icon="platformEditorOpen ? 'i-tabler-chevron-up' : 'i-tabler-plus'"
        @click="platformEditorOpen = !platformEditorOpen"
      >
        {{ platformEditorOpen ? '收起' : '新增订阅' }}
      </UButton>
    </div>
    <form
      v-if="platformEditorOpen"
      class="space-y-4 p-4"
      @submit.prevent="addPlatform"
    >
      <div
        class="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)_minmax(160px,0.55fr)]"
      >
        <UFormField label="订阅类型" description="更新只推送到当前群">
          <USelect
            v-model="platform"
            class="w-full"
            :items="platformOptions"
            value-key="value"
            :disabled="!selectedGroupID"
          />
        </UFormField>
        <UFormField label="订阅对象" description="支持直接粘贴平台主页链接">
          <UInput
            v-model="platformTarget"
            class="w-full"
            :ui="{ root: 'w-full' }"
            icon="i-tabler-link"
            :placeholder="platformPlaceholder"
            :disabled="!selectedGroupID"
          />
        </UFormField>
        <UFormField label="显示名称" description="可选">
          <UInput
            v-model="platformName"
            class="w-full"
            :ui="{ root: 'w-full' }"
            icon="i-tabler-tag"
            placeholder="关注对象"
            :disabled="!selectedGroupID"
          />
        </UFormField>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <p class="text-xs leading-5 text-muted">
          X 路由受平台限制，稳定运行建议配置自己的 RSSHub 实例。
        </p>
        <UButton
          type="submit"
          icon="i-tabler-bell-plus"
          :loading="platformAdding"
          :disabled="!selectedGroupID || groupFeeds.length >= 10"
          >添加平台订阅</UButton
        >
      </div>
    </form>
  </section>
</template>
