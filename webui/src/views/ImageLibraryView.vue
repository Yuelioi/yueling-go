<script setup lang="ts">
import EmptyState from '../components/EmptyState.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { imagePreviewURL, type ImageAsset, type ImageAssetSort } from '../api'
import { useImageAssets } from '../composables/useImageAssets'

const {
  categories,
  category,
  query,
  sort,
  page,
  pageSize,
  total,
  rows,
  selected,
  loading,
  deleting,
  renaming,
  busy,
  error,
  notice,
  external,
  initialize,
  changeCategory,
  changeSort,
  load,
  search,
  rename,
  remove,
} = useImageAssets()
const preview = ref<ImageAsset | null>(null)
const previewFailed = ref(false)
const confirmOpen = ref(false)
const targets = ref<ImageAsset[]>([])
const failedPreviews = ref(new Set<string>())
const renameTarget = ref<ImageAsset | null>(null)
const renameName = ref('')
const renameError = ref('')
const sortOptions = [
  { label: '修改时间 · 最新在前', value: 'modified_desc' },
  { label: '修改时间 · 最早在前', value: 'modified_asc' },
  { label: '名称 · 正序', value: 'name_asc' },
  { label: '名称 · 倒序', value: 'name_desc' },
]
const options = computed(() =>
  categories.value.map((c) => ({
    label: `${c.name}${c.external ? ' · 外站' : ''}`,
    value: c.name,
  })),
)
const categoryGroups = computed(() => [
  {
    label: '本地素材',
    icon: 'i-tabler-folder',
    items: categories.value.filter((c) => !c.external),
  },
  {
    label: '外部来源',
    icon: 'i-tabler-world',
    items: categories.value.filter((c) => c.external),
  },
])
const selectionState = computed(() =>
  !selected.value.length
    ? false
    : selected.value.length === rows.value.length
      ? true
      : 'indeterminate',
)
const range = computed(() =>
  rows.value.length
    ? `${(page.value - 1) * pageSize.value + 1}–${(page.value - 1) * pageSize.value + rows.value.length}`
    : '0',
)
watch(preview, () => {
  previewFailed.value = false
})
watch(category, () => {
  preview.value = null
  renameTarget.value = null
  confirmOpen.value = false
  failedPreviews.value = new Set()
})
function toggle(id: string) {
  selected.value = selected.value.includes(id)
    ? selected.value.filter((x) => x !== id)
    : [...selected.value, id]
}
function selectAll() {
  selected.value =
    selectionState.value === true ? [] : rows.value.map((row) => row.id)
}
function confirm(items: ImageAsset[]) {
  targets.value = [...items]
  confirmOpen.value = true
}
async function submitDelete() {
  await remove(targets.value)
  confirmOpen.value = false
  preview.value = null
}
async function movePage(value: number) {
  page.value = value
  await load()
}
async function refresh() {
  failedPreviews.value = new Set()
  await (categories.value.length ? load() : initialize())
}
function beginRename(asset: ImageAsset) {
  renameTarget.value = asset
  let name = asset.name.replace(/\.[^.]+$/, '').replace(/_[a-f0-9]{16}$/i, '')
  if (
    categories.value.find((c) => c.name === category.value)?.naming ===
    'quotation'
  )
    name = name.replace(/^\d+_/, '')
  renameName.value = name
  renameError.value = ''
}
async function submitRename() {
  if (!renameTarget.value) return
  if (await rename(renameTarget.value, renameName.value)) {
    renameTarget.value = null
    preview.value = null
  } else renameError.value = error.value
}
function modifiedTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? '时间未知'
    : date.toLocaleString('zh-CN', { hour12: false })
}
function actions(asset: ImageAsset) {
  return [
    {
      label: '查看原图',
      icon: 'i-tabler-arrows-maximize',
      onSelect: () => {
        preview.value = asset
      },
    },
    {
      label: '重命名',
      icon: 'i-tabler-edit',
      disabled:
        busy.value ||
        !categories.value.find((c) => c.name === category.value)?.upload,
      onSelect: () => beginRename(asset),
    },
    {
      label: '移入回收站',
      icon: 'i-tabler-trash',
      color: 'error' as const,
      disabled: busy.value,
      onSelect: () => confirm([asset]),
    },
  ]
}
onMounted(initialize)
</script>

<template>
  <UCard
    variant="outline"
    :ui="{ body: 'p-0 sm:p-0' }"
    class="overflow-hidden shadow-none"
  >
    <div class="grid min-h-[36rem] min-w-0 xl:grid-cols-[11rem_minmax(0,1fr)]">
      <aside
        class="hidden border-r border-default bg-muted/30 p-3 xl:block"
        aria-label="素材分类"
      >
        <div
          class="flex items-center gap-2 px-3 pb-5 pt-2 text-sm font-semibold text-highlighted"
        >
          <UIcon name="i-tabler-library-photo" class="size-4 text-primary" />
          素材分类
        </div>
        <div
          v-for="group in categoryGroups"
          :key="group.label"
          class="mb-6 space-y-1"
        >
          <p
            class="flex items-center justify-between px-3 pb-2 text-xs text-muted"
          >
            {{ group.label }}<span>{{ group.items.length }}</span>
          </p>
          <UButton
            v-for="item in group.items"
            :key="item.name"
            :label="item.name"
            :icon="group.icon"
            :color="category === item.name ? 'primary' : 'neutral'"
            :variant="category === item.name ? 'soft' : 'ghost'"
            :aria-current="category === item.name ? 'true' : undefined"
            :disabled="busy"
            block
            class="justify-start"
            @click="changeCategory(item.name)"
          />
        </div>
      </aside>
      <div class="flex min-w-0 flex-col">
        <div
          class="flex flex-wrap items-center justify-between gap-4 border-b border-default p-5"
        >
          <div class="flex items-center gap-3">
            <div
              class="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary"
            >
              <UIcon
                :name="external ? 'i-tabler-world' : 'i-tabler-folder'"
                class="size-5"
              />
            </div>
            <div>
              <h2
                class="flex items-center gap-2 font-semibold text-highlighted"
              >
                {{ category || '素材库'
                }}<UBadge size="sm" color="neutral" variant="subtle">{{
                  external ? '外站' : '本地'
                }}</UBadge>
              </h2>
              <p class="mt-1 text-xs text-muted">
                {{
                  external ? '由外部网站提供图片' : '浏览图片，整理你的素材收藏'
                }}
              </p>
            </div>
          </div>
          <UTooltip text="刷新素材"
            ><UButton
              color="neutral"
              variant="ghost"
              icon="i-tabler-refresh"
              aria-label="刷新素材"
              :loading="loading"
              :disabled="busy"
              @click="refresh"
          /></UTooltip>
          <USelect
            :model-value="category"
            :items="options"
            :disabled="busy"
            class="w-full xl:hidden"
            aria-label="素材类目"
            @update:model-value="changeCategory(String($event))"
          />
        </div>
        <div
          class="space-y-3 px-5 pt-4"
          v-if="error || notice"
          aria-live="polite"
        >
          <UAlert
            v-if="error"
            color="error"
            variant="soft"
            icon="i-tabler-alert-circle"
            title="操作未完成"
            :description="error"
          />
          <UAlert
            v-if="notice"
            color="info"
            variant="soft"
            icon="i-tabler-info-circle"
            :description="notice"
          />
        </div>
        <EmptyState
          v-if="external"
          icon="i-tabler-world"
          :title="`${category}来自外部网站`"
          description="图片不保存在本地，无法在此浏览或删除。可在「图片与打包配置」中管理来源。"
          variant="naked"
          class="flex-1 py-20"
        />
        <template v-else>
          <div
            class="flex flex-wrap items-center justify-between gap-3 px-5 py-4"
          >
            <form class="w-full sm:max-w-72" @submit.prevent="search">
              <UInput
                v-model="query"
                icon="i-tabler-search"
                class="w-full"
                placeholder="搜索图片名称…"
                aria-label="搜索图片名称"
                :disabled="busy"
                :ui="{ trailing: 'pe-1' }"
              >
                <template #trailing
                  ><UButton
                    type="submit"
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    icon="i-tabler-arrow-right"
                    aria-label="搜索"
                    :disabled="busy"
                /></template>
              </UInput>
            </form>
            <div class="flex flex-wrap items-center gap-3">
              <USelect
                :model-value="sort"
                :items="sortOptions"
                :disabled="busy || loading"
                class="w-48"
                aria-label="图片排序"
                @update:model-value="changeSort($event as ImageAssetSort)"
              />
              <span class="text-xs text-muted">共 {{ total }} 张图片</span>
            </div>
          </div>
          <div
            class="mx-5 mb-4 flex min-h-10 flex-wrap items-center gap-3 rounded-lg bg-muted/50 px-3 py-2"
          >
            <UCheckbox
              :model-value="selectionState"
              label="选择本页"
              :disabled="!rows.length || busy || loading"
              @update:model-value="selectAll"
            />
            <template v-if="selected.length">
              <span class="text-xs text-muted"
                >已选 {{ selected.length }} 张</span
              >
              <UButton
                size="xs"
                color="error"
                variant="soft"
                icon="i-tabler-trash"
                class="ml-auto"
                :loading="deleting"
                :disabled="renaming"
                @click="
                  confirm(rows.filter((row) => selected.includes(row.id)))
                "
                >移入回收站</UButton
              >
            </template>
            <span v-else class="ml-auto hidden text-xs text-muted sm:inline"
              >点击图片查看原图</span
            >
          </div>
          <div
            v-if="loading"
            class="grid grid-cols-2 gap-4 px-5 pb-5 md:grid-cols-3 2xl:grid-cols-4"
          >
            <USkeleton
              v-for="i in 6"
              :key="i"
              class="aspect-[4/3] rounded-lg"
            />
          </div>
          <EmptyState
            v-else-if="!rows.length"
            icon="i-tabler-photo"
            :title="query ? '没有找到匹配的图片' : '这个类目还没有图片'"
            :description="
              query
                ? '换个关键词，或清空搜索再试试。'
                : '添加到这个类目的图片会显示在这里。'
            "
            class="flex-1 py-16"
          />
          <div
            v-else
            class="grid grid-cols-2 content-start gap-4 px-5 pb-5 md:grid-cols-3 2xl:grid-cols-4"
          >
            <UCard
              v-for="asset in rows"
              :key="asset.id"
              as="article"
              variant="outline"
              :ui="{ body: 'p-0 sm:p-0', footer: 'px-3 py-2 sm:px-3 sm:py-2' }"
              class="group min-w-0 overflow-hidden shadow-none transition-colors"
              :class="
                selected.includes(asset.id)
                  ? 'ring-2 ring-primary'
                  : 'hover:ring-accented'
              "
            >
              <div class="relative bg-muted/40">
                <button
                  class="flex aspect-[4/3] w-full items-center justify-center overflow-hidden p-3 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
                  :aria-label="`预览 ${asset.name}`"
                  @click="preview = asset"
                >
                  <img
                    v-if="!failedPreviews.has(asset.id)"
                    :src="imagePreviewURL(category, asset.id, true)"
                    :alt="asset.name"
                    loading="lazy"
                    decoding="async"
                    class="h-full w-full object-contain transition-transform duration-200 group-hover:scale-[1.03]"
                    @error="failedPreviews.add(asset.id)"
                  />
                  <span
                    v-else
                    class="flex flex-col items-center gap-2 text-xs text-muted"
                    ><UIcon
                      name="i-tabler-photo-off"
                      class="size-6"
                    />缩略图不可用</span
                  >
                </button>
                <div
                  class="absolute left-2 top-2 rounded-md bg-default/90 p-1.5 shadow-sm"
                >
                  <UCheckbox
                    :aria-label="`选择 ${asset.name}`"
                    :model-value="selected.includes(asset.id)"
                    :disabled="busy"
                    @update:model-value="toggle(asset.id)"
                  />
                </div>
              </div>
              <template #footer>
                <div class="flex items-center gap-2">
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-xs text-toned" :title="asset.name">
                      {{ asset.name }}
                    </p>
                    <p
                      class="mt-1 truncate text-[11px] text-muted"
                      :title="`文件修改时间：${modifiedTime(asset.modified_at)}`"
                    >
                      {{ modifiedTime(asset.modified_at) }}
                    </p>
                  </div>
                  <UDropdownMenu
                    :items="actions(asset)"
                    :content="{ align: 'end' }"
                    ><UButton
                      color="neutral"
                      variant="ghost"
                      size="xs"
                      icon="i-tabler-dots"
                      :aria-label="`管理 ${asset.name}`"
                  /></UDropdownMenu>
                </div>
              </template>
            </UCard>
          </div>
          <div
            class="mt-auto flex flex-wrap items-center justify-between gap-3 border-t border-default px-5 py-4"
          >
            <span class="text-xs text-muted">{{ range }} / {{ total }} 张</span>
            <UPagination
              :page="page"
              :total="total"
              :items-per-page="pageSize"
              :sibling-count="0"
              :show-edges="false"
              size="xs"
              variant="ghost"
              active-variant="soft"
              :disabled="loading || busy"
              @update:page="movePage"
            />
          </div>
        </template>
      </div>
    </div>
  </UCard>
  <UModal
    :open="!!preview"
    :title="preview?.name ?? '图片预览'"
    description="原图预览"
    :ui="{ content: 'sm:max-w-4xl' }"
    @update:open="!$event && (preview = null)"
  >
    <template #body>
      <EmptyState
        v-if="previewFailed"
        icon="i-tabler-photo-off"
        title="原图加载失败"
        description="图片可能已被移除，请刷新素材库。"
        variant="naked"
      />
      <img
        v-else-if="preview"
        :src="imagePreviewURL(category, preview.id)"
        :alt="preview.name"
        class="mx-auto max-h-[70vh] max-w-full rounded-lg object-contain"
        @error="previewFailed = true"
      />
    </template>
  </UModal>
  <UModal
    v-model:open="confirmOpen"
    title="移入回收站？"
    :description="`将从「${category}」移除 ${targets.length} 张图片，机器人将不再使用这些图片。文件仍保留在服务器回收目录。`"
    :dismissible="!deleting"
  >
    <template #body
      ><div
        class="max-h-48 divide-y divide-default overflow-auto rounded-lg border border-default"
      >
        <div
          v-for="item in targets"
          :key="item.id"
          class="flex items-center gap-3 px-3 py-2.5 text-sm text-toned"
        >
          <UIcon
            name="i-tabler-photo"
            class="size-4 shrink-0 text-muted"
          /><span class="break-all">{{ item.name }}</span>
        </div>
      </div></template
    >
    <template #footer
      ><div class="flex w-full justify-end gap-3">
        <UButton
          color="neutral"
          variant="outline"
          :disabled="deleting"
          @click="confirmOpen = false"
          >取消</UButton
        ><UButton
          color="error"
          icon="i-tabler-trash"
          :loading="deleting"
          @click="submitDelete"
          >移入回收站 · {{ targets.length }} 张</UButton
        >
      </div></template
    >
  </UModal>
  <UModal
    :open="!!renameTarget"
    title="重命名图片"
    description="输入图片的新名称，所属类目保持不变。"
    :dismissible="!renaming"
    @update:open="!$event && !renaming && (renameTarget = null)"
  >
    <template #body>
      <form id="rename-image" class="space-y-4" @submit.prevent="submitRename">
        <p class="break-all text-sm text-muted">{{ renameTarget?.name }}</p>
        <UFormField label="新名称" :error="renameError || undefined" required>
          <UInput
            v-model="renameName"
            autofocus
            class="w-full"
            :disabled="renaming"
            :maxlength="160"
            placeholder="输入新名称"
            aria-label="新名称"
          />
        </UFormField>
      </form>
    </template>
    <template #footer
      ><div class="flex w-full justify-end gap-3">
        <UButton
          color="neutral"
          variant="outline"
          :disabled="renaming"
          @click="renameTarget = null"
          >取消</UButton
        ><UButton
          type="submit"
          form="rename-image"
          icon="i-tabler-check"
          :loading="renaming"
          :disabled="!renameName.trim()"
          >保存名称</UButton
        >
      </div></template
    >
  </UModal>
</template>
