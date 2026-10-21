<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import ConfigurationGuard from '../components/ConfigurationGuard.vue'
import EmptyState from '../components/EmptyState.vue'
import { groupsApi, imagesApi, type GroupInfo } from '../api'
import {
  imageUploadPreview,
  uploadNameError,
  useImageUpload,
} from '../features/images/useImageUpload'
import { useRequestScope } from '../composables/useRequestScope'

const emit = defineEmits<{ uploaded: [category: string] }>()
const route = useRoute()
const scope = useRequestScope()
const {
  categories,
  category,
  groupID,
  items,
  selectedCategory,
  busy,
  error,
  dirty,
  remaining,
  groupError,
  canUpload,
  editable,
  add,
  remove,
  clear,
  upload,
} = useImageUpload()
const filesInput = ref<HTMLInputElement>()
const groups = ref<GroupInfo[]>([])
const groupWarning = ref('')
const loading = ref(true)
const loadError = ref('')
const dragging = ref(false)
const locked = computed(
  () => busy.value || items.value.some((item) => !editable(item)),
)
const categoryOptions = computed(() =>
  categories.value
    .filter((item) => item.upload && !item.external)
    .map((item) => ({ value: item.name, label: item.name })),
)
const groupOptions = computed(() =>
  groups.value
    .filter((item) => item.group_id > 0)
    .map((item) => ({
      value: String(item.group_id),
      label: `${item.group_name} · ${item.group_id}`,
    })),
)
const completed = computed(
  () =>
    items.value.filter(
      (item) => item.status === 'saved' || item.status === 'duplicate',
    ).length,
)
const rule = computed(() => {
  switch (selectedCategory.value?.naming) {
    case 'quotation':
      return '语录：群号 + 名称 + 哈希。请选择所属群；名称默认取本地文件名。'
    case 'label':
      return '名称类：名称 + 哈希。吃喝等菜单只显示名称；特殊符号会自动整理。'
    case 'optional_label':
      return '表情类：关键词 + 哈希。关键词用于检索，哈希后缀自动补齐。'
    default:
      return '随机类：仅使用图片内容哈希命名，无需填写名称。'
  }
})
watch(
  () => route.query.category,
  (value) => {
    if (
      route.query.tab === 'upload' &&
      !items.value.length &&
      !busy.value &&
      categoryOptions.value.some((item) => item.value === value)
    )
      category.value = String(value)
  },
)
async function initialize() {
  const current = scope.begin('categories')
  loading.value = true
  loadError.value = ''
  try {
    const result = await imagesApi.imageCategories({ signal: current.signal })
    if (!current()) return
    categories.value = result.categories
    const preferred = String(route.query.category ?? '')
    category.value =
      categoryOptions.value.find((item) => item.value === preferred)?.value ??
      categoryOptions.value[0]?.value ??
      ''
  } catch (err) {
    if (current())
      loadError.value = err instanceof Error ? err.message : '分类加载失败'
  } finally {
    if (current()) loading.value = false
  }
}
async function loadGroups() {
  const current = scope.begin('groups')
  try {
    const result = await groupsApi.managedGroups()
    if (current()) groups.value = result.groups
  } catch {
    if (current()) groupWarning.value = '群列表暂时无法加载，可直接输入群号。'
  }
}
function chooseFiles(event: Event) {
  const input = event.target as HTMLInputElement
  add(Array.from(input.files ?? []))
  input.value = ''
}
function dropFiles(event: DragEvent) {
  dragging.value = false
  if (category.value && !busy.value && event.dataTransfer)
    add(Array.from(event.dataTransfer.files))
}
async function submit() {
  const target = category.value
  if (await upload()) emit('uploaded', target)
}
onMounted(() => {
  void initialize()
  void loadGroups()
})
</script>

<template>
  <ConfigurationGuard :dirty="dirty" :saving="busy" @discard="clear" />
  <section class="min-w-0 space-y-5" aria-label="批量上传图片">
    <UAlert
      v-if="loadError"
      color="error"
      :description="loadError"
      title="分类加载失败"
    >
      <template #actions
        ><UButton color="neutral" variant="soft" @click="initialize"
          >重新加载</UButton
        ></template
      >
    </UAlert>
    <UCard class="shadow-none">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 class="font-semibold text-highlighted">批量上传</h2>
          <p class="mt-1 text-sm text-muted">
            选好分类，检查名称，再一次上传。每批最多 50 张，单张不超过 16 MiB。
          </p>
        </div>
        <UBadge color="neutral" variant="subtle">本地素材</UBadge>
      </div>
      <div class="mt-5 grid gap-4 sm:grid-cols-2">
        <UFormField label="目标分类" required>
          <USelect
            v-model="category"
            :items="categoryOptions"
            :disabled="locked || loading"
            placeholder="选择可上传的分类"
            aria-label="上传分类"
            class="w-full"
          />
        </UFormField>
        <UFormField
          v-if="selectedCategory?.naming === 'quotation'"
          label="语录所属群"
          required
        >
          <USelect
            v-model="groupID"
            id="upload-quotation-group"
            :items="groupOptions"
            :disabled="locked"
            placeholder="选择群聊"
            aria-label="选择语录所属群"
            class="w-full"
          />
          <UInput
            v-model="groupID"
            id="upload-quotation-group-manual"
            :disabled="locked"
            inputmode="numeric"
            placeholder="或直接输入群号"
            aria-label="语录群号"
            class="mt-2 w-full"
          />
          <p v-if="groupWarning" class="mt-1 text-xs text-warning">
            {{ groupWarning }}
          </p>
        </UFormField>
      </div>
      <div
        v-if="selectedCategory"
        class="mt-4 flex gap-2 rounded-lg bg-primary/5 p-3 text-sm text-toned"
      >
        <UIcon
          name="i-tabler-info-circle"
          class="mt-0.5 size-4 shrink-0 text-primary"
        />
        <div>
          <p>{{ rule }}</p>
          <p class="mt-1 text-xs text-muted">
            格式后缀按真实图片内容生成，并遵循现有图片压缩配置。已存素材不覆盖；同名不同图可同时保留。
          </p>
        </div>
      </div>
      <p v-else-if="!loading && !loadError" class="mt-4 text-sm text-muted">
        当前没有允许上传的本地分类，请先在图片配置中启用添加功能。
      </p>
    </UCard>

    <input
      ref="filesInput"
      type="file"
      multiple
      accept="image/png,image/jpeg,image/gif,image/webp"
      class="hidden"
      aria-label="选择本地图片"
      :disabled="busy || !category"
      @change="chooseFiles"
    />
    <button
      type="button"
      class="flex w-full flex-col items-center gap-2 rounded-xl border-2 border-dashed p-7 text-center transition-colors enabled:hover:border-primary enabled:hover:bg-primary/5 disabled:cursor-not-allowed disabled:opacity-50"
      :class="
        dragging ? 'border-primary bg-primary/5' : 'border-default bg-muted/20'
      "
      :disabled="busy || !category"
      @click="filesInput?.click()"
      @dragover.prevent="dragging = !busy"
      @dragleave.prevent="dragging = false"
      @drop.prevent="dropFiles"
    >
      <UIcon name="i-tabler-cloud-upload" class="size-7 text-primary" />
      <span class="text-sm font-medium text-highlighted"
        >点击选择，或将多张图片拖到这里</span
      >
      <span class="text-xs text-muted"
        >PNG · JPEG · GIF · WebP，默认读取本地文件名</span
      >
    </button>
    <UAlert
      v-if="error"
      color="error"
      title="上传提示"
      :description="error"
      aria-live="polite"
    />

    <UCard
      v-if="items.length"
      :ui="{ body: 'p-0 sm:p-0' }"
      class="overflow-hidden shadow-none"
    >
      <div
        class="flex flex-wrap items-center justify-between gap-3 border-b border-default px-5 py-4"
      >
        <div>
          <h3 class="text-sm font-semibold text-highlighted">
            待上传清单 <span class="text-muted">{{ items.length }} 张</span>
          </h3>
          <p class="mt-1 text-xs text-muted">
            仅修改名称部分，群号、哈希和格式后缀自动处理。
          </p>
        </div>
        <UButton color="neutral" variant="ghost" :disabled="busy" @click="clear"
          >清空清单</UButton
        >
      </div>
      <ul class="max-h-[38rem] divide-y divide-default overflow-y-auto">
        <li
          v-for="item in items"
          :key="item.id"
          class="flex gap-3 px-4 py-4 sm:gap-4 sm:px-5"
        >
          <img
            :src="item.preview"
            loading="lazy"
            decoding="async"
            alt=""
            class="size-16 shrink-0 rounded-lg border border-default bg-muted object-contain sm:size-20"
          />
          <div class="min-w-0 flex-1 space-y-2">
            <p class="truncate text-xs text-muted" :title="item.file.name">
              {{ item.file.name }} ·
              {{ (item.file.size / 1024 / 1024).toFixed(2) }} MiB
            </p>
            <UFormField
              v-if="selectedCategory?.naming !== 'hash'"
              label="名称"
              :error="uploadNameError(item.name, selectedCategory) || undefined"
              :required="selectedCategory?.name_required"
            >
              <UInput
                v-model="item.name"
                :disabled="busy || !editable(item)"
                :aria-label="`${item.file.name} 的名称`"
                class="w-full"
              />
            </UFormField>
            <p class="break-all text-xs text-muted">
              {{ item.storedName ? '已保存：' : '命名预览：'
              }}{{
                item.storedName ||
                imageUploadPreview(item.name, selectedCategory, groupID)
              }}
            </p>
            <p
              class="text-xs"
              :class="
                item.status === 'failed' || item.status === 'uncertain'
                  ? 'text-error'
                  : 'text-primary'
              "
              role="status"
            >
              {{
                item.status === 'uploading'
                  ? '正在上传…'
                  : item.message || '待上传'
              }}
            </p>
          </div>
          <UButton
            color="neutral"
            variant="ghost"
            icon="i-tabler-x"
            :aria-label="`移除 ${item.file.name}`"
            :disabled="busy"
            class="self-start"
            @click="remove(item.id)"
          />
        </li>
      </ul>
      <div
        class="flex flex-wrap items-center justify-between gap-3 border-t border-default bg-muted/20 px-5 py-4"
      >
        <div aria-live="polite">
          <p class="text-sm text-toned">
            已完成 {{ completed }} / {{ items.length }} 张
          </p>
          <p v-if="groupError" class="mt-1 text-xs text-warning">
            {{ groupError }}
          </p>
          <p v-if="locked && !busy" class="mt-1 text-xs text-muted">
            清空清单后可更换分类或群号。
          </p>
        </div>
        <UButton
          icon="i-tabler-upload"
          :loading="busy"
          :disabled="!canUpload"
          @click="submit"
          >{{
            busy
              ? '正在上传'
              : completed === items.length
                ? '全部完成'
                : `上传剩余 ${remaining.length} 张`
          }}</UButton
        >
      </div>
    </UCard>
    <EmptyState
      v-else
      icon="i-tabler-photo-plus"
      title="从本地文件名开始"
      description="支持批量选择；名称可逐张修改，保存规则自动跟随分类。"
      variant="naked"
    />
  </section>
</template>
