<script setup lang="ts">
import type { ImageEntry } from '../api'
const entries = defineModel<ImageEntry[]>({ required: true })
const kinds = [
  { label: '随机一张', value: 'single' },
  { label: '四图网格', value: 'grid' },
  { label: '外链图片', value: 'external' },
]
const argumentModes = [
  { label: '自动', value: 'auto' },
  { label: '需要关键词', value: 'yes' },
  { label: '直接加图', value: 'no' },
]
function add() {
  entries.value.push({
    folder: '',
    call: [],
    add: '',
    kind: 'single',
    arg: null,
    url: '',
    pick: '',
    base: '',
  })
}
</script>

<template>
  <div class="space-y-4">
    <p class="text-sm text-muted">
      留空使用内置素材类目。自定义后以此列表为准，保存会自动重新加载素材命令。
    </p>
    <UCard v-for="(entry, i) in entries" :key="i" :ui="{ body: 'space-y-4' }">
      <div class="flex items-center justify-between gap-3">
        <h3 class="text-sm font-medium">类目 {{ i + 1 }}</h3>
        <UButton
          type="button"
          color="error"
          variant="ghost"
          :aria-label="`删除类目 ${i + 1}`"
          @click="entries.splice(i, 1)"
          >删除</UButton
        >
      </div>
      <div class="grid min-w-0 gap-4 md:grid-cols-2">
        <UFormField label="素材文件夹"
          ><UInput
            v-model="entry.folder"
            class="w-full"
            :aria-label="`类目 ${i + 1} 文件夹`"
        /></UFormField>
        <UFormField label="调用命令" description="多个命令用逗号分隔。"
          ><UInput
            :model-value="entry.call.join(', ')"
            class="w-full"
            :aria-label="`类目 ${i + 1} 调用命令`"
            @update:model-value="
              entry.call = String($event)
                .split(/[,，]/)
                .map((s) => s.trim())
                .filter(Boolean)
            "
        /></UFormField>
        <UFormField label="调用方式"
          ><USelect
            v-model="entry.kind"
            :items="kinds"
            class="w-full"
            :aria-label="`类目 ${i + 1} 调用方式`"
        /></UFormField>
        <UFormField label="添加命令（可留空）"
          ><UInput
            v-model="entry.add"
            class="w-full"
            :aria-label="`类目 ${i + 1} 添加命令`"
        /></UFormField>
        <UFormField label="添加规则"
          ><USelect
            :model-value="
              entry.arg === null ? 'auto' : entry.arg ? 'yes' : 'no'
            "
            :items="argumentModes"
            class="w-full"
            :aria-label="`类目 ${i + 1} 添加规则`"
            @update:model-value="
              entry.arg = $event === 'auto' ? null : $event === 'yes'
            "
        /></UFormField>
        <template v-if="entry.kind === 'external'">
          <UFormField label="图片或接口地址"
            ><UInput
              v-model="entry.url"
              class="w-full"
              :aria-label="`类目 ${i + 1} 外链地址`"
          /></UFormField>
          <UFormField label="JSON 图片路径（可留空）"
            ><UInput
              v-model="entry.pick"
              class="w-full"
              :aria-label="`类目 ${i + 1} 图片路径`"
          /></UFormField>
          <UFormField label="相对图片地址前缀（可留空）"
            ><UInput
              v-model="entry.base"
              class="w-full"
              :aria-label="`类目 ${i + 1} 地址前缀`"
          /></UFormField>
        </template>
      </div>
    </UCard>
    <UButton type="button" variant="soft" icon="i-tabler-plus" @click="add"
      >添加素材类目</UButton
    >
  </div>
</template>
