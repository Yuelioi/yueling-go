<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type GroupInfo, type GroupMessagePayload } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import PageHeader from '../components/PageHeader.vue'

interface ImageItem {
  id: number
  kind: 'url' | 'file'
  label: string
  value: string
}

const groups = ref<GroupInfo[]>([])
const selectedGroupID = ref<number | null>(null)
const message = ref('')
const atText = ref('')
const imageURL = ref('')
const images = ref<ImageItem[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const loading = ref(false)
const sending = ref(false)
const error = ref('')
const sent = ref('')
let nextImageID = 1
const maxImages = 5
const maxImageBytes = 8 * 1024 * 1024

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)

const hasMessageContent = computed(() =>
  Boolean(message.value.trim() || atText.value.trim() || images.value.length),
)

const canSend = computed(() =>
  Boolean(selectedGroupID.value && hasMessageContent.value && !sending.value),
)

const atPreviewCount = computed(() =>
  atText.value.split(/[\s,，]+/).map((part) => part.trim()).filter(Boolean).length,
)

function parseAtUserIDs() {
  const parts = atText.value
    .split(/[\s,，]+/)
    .map((part) => part.trim())
    .filter(Boolean)
  const ids: number[] = []
  for (const part of parts) {
    if (!/^\d+$/.test(part)) {
      throw new Error('艾特 QQ 号只能填写数字')
    }
    const id = Number(part)
    if (!Number.isSafeInteger(id) || id <= 0) {
      throw new Error('艾特 QQ 号无效')
    }
    ids.push(id)
  }
  return ids
}

function fileToBase64Image(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result || '')
      const [, base64] = result.split(',', 2)
      if (!base64) {
        reject(new Error('图片读取失败'))
        return
      }
      resolve(`base64://${base64}`)
    }
    reader.onerror = () => reject(new Error('图片读取失败'))
    reader.readAsDataURL(file)
  })
}

function addImageURL() {
  error.value = ''
  sent.value = ''
  const value = imageURL.value.trim()
  if (!value) {
    error.value = '请输入图片 URL'
    return
  }
  if (images.value.length >= maxImages) {
    error.value = `每条消息最多添加 ${maxImages} 张图片`
    return
  }
  images.value.push({
    id: nextImageID++,
    kind: 'url',
    label: value,
    value,
  })
  imageURL.value = ''
}

function chooseFiles() {
  fileInput.value?.click()
}

async function addLocalImages(event: Event) {
  error.value = ''
  sent.value = ''
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  for (const file of files) {
    if (images.value.length >= maxImages) {
      error.value = `每条消息最多添加 ${maxImages} 张图片`
      break
    }
    if (!file.type.startsWith('image/')) {
      error.value = '只能选择图片文件'
      continue
    }
    if (file.size > maxImageBytes) {
      error.value = `${file.name} 超过 8MB，未添加`
      continue
    }
    try {
      images.value.push({
        id: nextImageID++,
        kind: 'file',
        label: file.name,
        value: await fileToBase64Image(file),
      })
    } catch (err) {
      error.value = err instanceof Error ? err.message : '图片读取失败'
    }
  }
}

function removeImage(id: number) {
  images.value = images.value.filter((image) => image.id !== id)
  sent.value = ''
}

async function loadGroups() {
  error.value = ''
  sent.value = ''
  loading.value = true
  try {
    const res = await api.groups()
    groups.value = res.groups
    selectedGroupID.value = res.groups[0]?.group_id ?? null
  } catch (err) {
    groups.value = []
    selectedGroupID.value = null
    error.value = err instanceof Error ? err.message : '加载群列表失败'
  } finally {
    loading.value = false
  }
}

function resetComposer() {
  message.value = ''
  atText.value = ''
  imageURL.value = ''
  images.value = []
}

async function sendMessage() {
  error.value = ''
  sent.value = ''
  if (!selectedGroupID.value) {
    error.value = '请选择要发送的群'
    return
  }
  if (!hasMessageContent.value) {
    error.value = '请输入消息内容'
    return
  }

  let atUserIDs: number[]
  try {
    atUserIDs = parseAtUserIDs()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '艾特 QQ 号无效'
    return
  }

  const payload: GroupMessagePayload = {
    text: message.value.trim(),
    at_user_ids: atUserIDs,
    images: images.value.map((image) => image.value),
  }

  sending.value = true
  try {
    const res = await api.sendGroupMessage(selectedGroupID.value, payload)
    resetComposer()
    sent.value = res.message_id ? `已发送，消息 ID ${res.message_id}` : '已发送'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '发送失败'
  } finally {
    sending.value = false
  }
}

onMounted(() => {
  void loadGroups()
})
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Message studio"
      title="消息中心"
      description="组合文本、图片与艾特，一次发送到指定群聊。支持 Ctrl + Enter 快速发送。"
      icon="i-tabler-send"
    >
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :loading="loading" @click="loadGroups">
        刷新群列表
      </UButton>
    </PageHeader>

    <UAlert
      v-if="error"
      class="error-banner"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      :description="error"
    />

    <div class="space-y-4">
      <GroupScopeSelect
        v-model="selectedGroupID"
        :groups="groups"
        title="发送目标"
        description="消息只会发送到所选群聊"
        @update:model-value="sent = ''"
      />

      <div class="surface-panel overflow-hidden">
        <div class="panel-header">
          <div class="min-w-0">
            <div class="truncate font-medium text-white">
              {{ selectedGroup?.group_name || '未选择群' }}
            </div>
            <div class="text-xs text-zinc-500">
              {{ selectedGroupID || '选择一个群后可发送消息' }}
            </div>
          </div>
          <UBadge v-if="selectedGroupID" color="primary" variant="subtle">组合消息</UBadge>
        </div>

        <div class="space-y-4 p-4">
          <div class="space-y-2">
            <div class="flex items-center gap-2 text-sm font-medium text-zinc-200">
              <UIcon name="i-tabler-text-caption" class="size-4 text-violet-300" />
              文本
            </div>
            <UTextarea
              v-model="message"
              class="w-full"
              :ui="{ root: 'w-full' }"
              :rows="6"
              autoresize
              placeholder="输入要发送到群里的消息"
              @keydown.ctrl.enter.prevent="sendMessage"
            />
          </div>

          <div class="grid gap-4 xl:grid-cols-2">
            <div class="space-y-2">
              <div class="flex items-center gap-2 text-sm font-medium text-zinc-200">
                <UIcon name="i-tabler-at" class="size-4 text-violet-300" />
                艾特
              </div>
              <UInput
                v-model="atText"
                class="w-full"
                :ui="{ root: 'w-full' }"
                icon="i-tabler-user-circle"
                placeholder="QQ 号，多个用空格或逗号分隔"
              />
            </div>

            <div class="space-y-2">
              <div class="flex items-center gap-2 text-sm font-medium text-zinc-200">
                <UIcon name="i-tabler-photo" class="size-4 text-violet-300" />
                图片
              </div>
              <div class="flex gap-2">
                <UInput
                  v-model="imageURL"
                  class="min-w-0 flex-1"
                  :ui="{ root: 'w-full' }"
                  icon="i-tabler-link"
                  placeholder="图片 URL"
                  @keydown.enter.prevent="addImageURL"
                />
                <UButton
                  color="primary"
                  variant="soft"
                  icon="i-tabler-plus"
                  aria-label="添加图片 URL"
                  title="添加图片 URL"
                  @click="addImageURL"
                />
                <UButton
                  color="neutral"
                  variant="soft"
                  icon="i-tabler-upload"
                  aria-label="选择本地图片"
                  title="选择本地图片"
                  @click="chooseFiles"
                />
              </div>
              <input
                ref="fileInput"
                class="sr-only"
                type="file"
                accept="image/*"
                multiple
                @change="addLocalImages"
              >
            </div>
          </div>

          <div v-if="images.length" class="message-chip-row">
            <span
              v-for="image in images"
              :key="image.id"
              class="message-chip"
              :title="image.label"
            >
              <UIcon
                :name="image.kind === 'file' ? 'i-tabler-file-image' : 'i-tabler-link'"
                class="size-4 shrink-0 text-violet-300"
              />
              <span class="truncate">{{ image.label }}</span>
              <button
                class="message-chip-remove"
                type="button"
                aria-label="移除图片"
                @click="removeImage(image.id)"
              >
                <UIcon name="i-tabler-x" class="size-3.5" />
              </button>
            </span>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3 border-t border-zinc-800/80 pt-4">
            <div class="min-h-5 text-sm">
              <span v-if="sent" class="inline-status">
                <UIcon name="i-tabler-circle-check" class="size-4" />
                {{ sent }}
              </span>
              <span v-else class="text-zinc-500">
                {{ message.trim().length }} 字 / {{ atPreviewCount }} 个艾特 / {{ images.length }} 张图片
              </span>
            </div>
            <UButton
              color="primary"
              icon="i-tabler-send-2"
              :loading="sending"
              :disabled="!canSend"
              @click="sendMessage"
            >
              发送
            </UButton>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
