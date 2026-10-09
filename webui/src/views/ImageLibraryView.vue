<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { imagePreviewURL, type ImageAsset } from '../api'
import { useImageAssets } from '../composables/useImageAssets'
const { categories, category, query, page, pages, total, rows, selected, loading, deleting, error, notice, external, initialize, changeCategory, load, search, remove } = useImageAssets()
const preview = ref<ImageAsset | null>(null)
const confirmOpen = ref(false)
const targets = ref<ImageAsset[]>([])
const failedPreviews = ref(new Set<string>())
const options = computed(() => categories.value.map(c => ({ label: `${c.name}${c.external ? ' · 外部网站' : ''}`, value: c.name })))
const allSelected = computed(() => rows.value.length > 0 && selected.value.length === rows.value.length)
function toggle(id: string) { selected.value = selected.value.includes(id) ? selected.value.filter(x => x !== id) : [...selected.value, id] }
function selectAll() { selected.value = allSelected.value ? [] : rows.value.map(row => row.id) }
function confirm(items: ImageAsset[]) { targets.value = [...items]; confirmOpen.value = true }
async function submitDelete() { await remove(targets.value); confirmOpen.value = false; preview.value = null }
async function movePage(delta: number) { page.value += delta; await load() }
onMounted(initialize)
</script>

<template>
  <section class="space-y-5">
    <div class="surface-panel flex flex-wrap items-end gap-3 p-4">
      <UFormField label="素材类目" class="min-w-52 flex-1">
        <USelect :model-value="category" :items="options" :disabled="deleting" class="w-full" aria-label="素材类目" @update:model-value="changeCategory(String($event))" />
      </UFormField>
      <UFormField label="搜索名称" class="min-w-48 flex-1">
        <UInput v-model="query" class="w-full" placeholder="语录名字、表情关键词或文件名" :disabled="external || deleting" @keyup.enter="search" />
      </UFormField>
      <UButton icon="i-tabler-search" :loading="loading" :disabled="external || deleting" @click="search">搜索</UButton>
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :disabled="deleting" @click="categories.length ? load() : initialize()">刷新</UButton>
    </div>
    <UAlert v-if="error" color="error" variant="subtle" title="操作未完成" :description="error" />
    <UAlert v-if="notice" color="info" variant="subtle" :description="notice" />
    <div v-if="external" class="surface-panel space-y-3 p-10 text-center">
      <UIcon name="i-tabler-world" class="size-10 text-zinc-400" />
      <h3 class="text-lg font-semibold">{{ category }}来自外部网站</h3>
      <p class="text-sm text-zinc-400">这类图片不保存在本地素材库，不能在此预览或删除。来源设置可在“图片与打包配置”中修改。</p>
    </div>
    <template v-else>
      <div class="flex flex-wrap items-center gap-3">
        <span class="mr-auto text-sm text-zinc-400">{{ total }} 张图片<span v-if="selected.length"> · 已选 {{ selected.length }} 张</span></span>
        <UButton color="neutral" variant="ghost" :disabled="!rows.length || deleting || loading" @click="selectAll">{{ allSelected ? '取消全选' : '选择本页' }}</UButton>
        <UButton color="error" variant="soft" icon="i-tabler-trash" :disabled="!selected.length || loading" :loading="deleting" @click="confirm(rows.filter(row => selected.includes(row.id)))">删除所选</UButton>
      </div>
      <div v-if="loading" class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4"><USkeleton v-for="i in 8" :key="i" class="h-56 rounded-xl" /></div>
      <div v-else-if="!rows.length" class="surface-panel space-y-2 p-12 text-center">
        <UIcon name="i-tabler-photo-off" class="size-10 text-zinc-500" /><p class="text-zinc-300">没有找到图片</p><p class="text-sm text-zinc-500">试试其他类目或搜索词。</p>
      </div>
      <div v-else class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
        <article v-for="asset in rows" :key="asset.id" class="surface-panel overflow-hidden" :class="selected.includes(asset.id) ? 'ring-2 ring-primary' : ''">
          <div class="relative bg-black/15">
            <button class="flex aspect-square w-full items-center justify-center p-2" :aria-label="`预览 ${asset.name}`" @click="preview = asset">
              <img v-if="!failedPreviews.has(asset.id)" :src="imagePreviewURL(category, asset.id, true)" :alt="asset.name" loading="lazy" class="max-h-full max-w-full object-contain" @error="failedPreviews.add(asset.id)" />
              <span v-else class="text-sm text-zinc-500">预览不可用</span>
            </button>
            <input type="checkbox" class="absolute left-3 top-3 size-5 accent-emerald-500" :aria-label="`选择 ${asset.name}`" :checked="selected.includes(asset.id)" :disabled="deleting" @change="toggle(asset.id)" />
          </div>
          <div class="flex items-center gap-2 border-t border-white/5 p-3">
            <p class="min-w-0 flex-1 truncate text-xs text-zinc-300" :title="asset.name">{{ asset.name }}</p>
            <UButton color="error" variant="ghost" size="xs" icon="i-tabler-trash" :aria-label="`删除 ${asset.name}`" :disabled="deleting" @click="confirm([asset])" />
          </div>
        </article>
      </div>
      <div class="flex items-center justify-center gap-4">
        <UButton color="neutral" variant="soft" :disabled="page <= 1 || loading || deleting" @click="movePage(-1)">上一页</UButton>
        <span class="text-sm text-zinc-400">{{ page }} / {{ pages }}</span>
        <UButton color="neutral" variant="soft" :disabled="page >= pages || loading || deleting" @click="movePage(1)">下一页</UButton>
      </div>
    </template>
    <UModal :open="!!preview" :title="preview?.name ?? '图片预览'" description="本地图片原图预览" :ui="{ content: 'sm:max-w-4xl' }" @update:open="!$event && (preview = null)">
      <template #body><img v-if="preview" :src="imagePreviewURL(category, preview.id)" :alt="preview.name" class="mx-auto max-h-[70vh] max-w-full object-contain" /></template>
    </UModal>
    <UModal v-model:open="confirmOpen" title="删除所选图片？" :description="`将从「${category}」移除 ${targets.length} 张图片，文件保留在服务器回收站。`" :dismissible="!deleting">
      <template #body><ul class="max-h-48 space-y-2 overflow-auto text-sm text-zinc-400"><li v-for="item in targets" :key="item.id" class="break-all">{{ item.name }}</li></ul></template>
      <template #footer><div class="flex w-full justify-end gap-3"><UButton color="neutral" variant="soft" :disabled="deleting" @click="confirmOpen = false">取消</UButton><UButton color="error" icon="i-tabler-trash" :loading="deleting" @click="submitDelete">确认删除 {{ targets.length }} 张</UButton></div></template>
    </UModal>
  </section>
</template>
