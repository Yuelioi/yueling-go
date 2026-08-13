<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo } from '../api'
import GroupPicker from '../components/GroupPicker.vue'
import MetricCard from '../components/MetricCard.vue'
import PageHeader from '../components/PageHeader.vue'

const groups = ref<GroupInfo[]>([])
const selectedGroupID = ref<number | null>(null)
const stylePrompt = ref('')
const savedPrompt = ref('')
const defaultPrompt = ref('')
const maxChars = ref(4000)
const custom = ref(false)
const loading = ref(false)
const loadingStyle = ref(false)
const saving = ref(false)
const resetting = ref(false)
const error = ref('')
const success = ref('')
let requestVersion = 0

const selectedGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroupID.value),
)
const charCount = computed(() => Array.from(stylePrompt.value).length)
const dirty = computed(() => stylePrompt.value !== savedPrompt.value)
const effectivePrompt = computed(() => stylePrompt.value.trim() || defaultPrompt.value)

const presets = [
  {
    name: '轻松群友',
    icon: 'i-tabler-mood-smile',
    prompt: '轻松自然，像熟悉群聊氛围的老群友；可以接梗，但不要尬聊或过度卖萌；回复尽量简短。',
  },
  {
    name: '简洁专业',
    icon: 'i-tabler-bulb',
    prompt: '冷静、可靠、直奔重点；先给结论，再补充必要信息；避免口头禅、夸张语气和冗长铺垫。',
  },
  {
    name: '温柔治愈',
    icon: 'i-tabler-heart',
    prompt: '温柔、有耐心、善于共情；语气亲近但不过分热情；不说教，优先给出具体可行的建议。',
  },
  {
    name: '克制吐槽',
    icon: 'i-tabler-message-circle-bolt',
    prompt: '带一点冷幽默和克制的吐槽，允许自然接梗；不挖苦真实弱点，不攻击群友，不重复网络烂梗。',
  },
]

function applyStyleResponse(res: Awaited<ReturnType<typeof api.groupAIStyle>>) {
  stylePrompt.value = res.style_prompt
  savedPrompt.value = res.style_prompt
  defaultPrompt.value = res.default_style_prompt
  maxChars.value = res.max_chars
  custom.value = res.custom
}

async function loadStyle(groupID: number) {
  const version = ++requestVersion
  loadingStyle.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await api.groupAIStyle(groupID)
    if (version === requestVersion && selectedGroupID.value === groupID) {
      applyStyleResponse(res)
    }
  } catch (err) {
    if (version === requestVersion) {
      error.value = err instanceof Error ? err.message : '风格设置加载失败'
    }
  } finally {
    if (version === requestVersion) loadingStyle.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.groups()
    groups.value = res.groups
    const currentAvailable = groups.value.some((group) => group.group_id === selectedGroupID.value)
    if (!currentAvailable) {
      selectedGroupID.value = groups.value[0]?.group_id ?? null
    } else if (selectedGroupID.value) {
      await loadStyle(selectedGroupID.value)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群聊加载失败'
  } finally {
    loading.value = false
  }
}

function usePreset(prompt: string) {
  stylePrompt.value = prompt
  success.value = ''
}

async function save() {
  if (!selectedGroupID.value) return
  if (charCount.value > maxChars.value) {
    error.value = `提示词不能超过 ${maxChars.value} 个字符`
    return
  }
  saving.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await api.setGroupAIStyle(selectedGroupID.value, stylePrompt.value)
    applyStyleResponse(res)
    success.value = res.custom ? '已保存，下一次 AI 对话立即生效' : '已恢复默认风格'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function reset() {
  if (!selectedGroupID.value) return
  resetting.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await api.resetGroupAIStyle(selectedGroupID.value)
    applyStyleResponse(res)
    success.value = '已恢复默认风格'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '恢复默认失败'
  } finally {
    resetting.value = false
  }
}

watch(selectedGroupID, (groupID) => {
  if (groupID) void loadStyle(groupID)
})

onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Conversation persona"
      title="AI 对话风格"
      description="为每个群设置独立的人格、语气和表达习惯。修改后无需重启 Bot。"
      icon="i-tabler-sparkles"
    >
      <UButton icon="i-tabler-refresh" :loading="loading" @click="load">刷新群聊</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="可配置群聊" :value="groups.length" detail="Bot 当前可访问" icon="i-tabler-users-group" tone="violet" />
      <MetricCard label="当前模式" :value="custom ? '独立风格' : '默认风格'" detail="仅影响所选群聊" icon="i-tabler-adjustments" tone="cyan" />
      <MetricCard label="提示词长度" :value="`${charCount}/${maxChars}`" detail="按 Unicode 字符计数" icon="i-tabler-text-size" tone="amber" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />
    <UAlert v-if="success" color="success" variant="subtle" icon="i-tabler-circle-check" :description="success" />

    <div class="grid gap-4 lg:grid-cols-[292px_minmax(0,1fr)]">
      <GroupPicker
        v-model="selectedGroupID"
        :groups="groups"
        title="风格作用域"
        description="选择要定制的群聊"
      />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择群' }}</div>
              <div class="text-xs text-zinc-500">{{ selectedGroupID ? `群号 ${selectedGroupID}` : '选择群聊后可编辑' }}</div>
            </div>
            <UBadge :color="custom ? 'primary' : 'neutral'" variant="subtle">
              {{ loadingStyle ? '读取中' : custom ? '独立风格' : '继承默认' }}
            </UBadge>
          </div>

          <div class="space-y-5 p-5">
            <UFormField
              label="群专属风格提示词"
              description="描述说话方式、人格、称呼和禁用表达；权限、安全与工具规则不受这里影响。"
            >
              <UTextarea
                v-model="stylePrompt"
                class="w-full"
                :ui="{ root: 'w-full' }"
                :rows="11"
                autoresize
                :maxlength="maxChars"
                :disabled="!selectedGroupID || loadingStyle"
                :placeholder="defaultPrompt || '例如：轻松自然，像群友一样接梗，但不要过度卖萌。'"
              />
            </UFormField>

            <div class="flex flex-wrap items-center justify-between gap-3 text-xs">
              <span class="text-zinc-500">留空保存即恢复内置默认；每个群的数据完全独立。</span>
              <span :class="charCount > maxChars ? 'text-rose-300' : 'text-zinc-500'">{{ charCount }} / {{ maxChars }}</span>
            </div>

            <div>
              <div class="mb-2 text-xs font-semibold text-zinc-300">快速模板</div>
              <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                <button
                  v-for="preset in presets"
                  :key="preset.name"
                  type="button"
                  class="surface-inset flex items-center gap-2 px-3 py-2.5 text-left text-xs text-zinc-300 transition hover:border-violet-400/40 hover:text-white"
                  :disabled="!selectedGroupID"
                  @click="usePreset(preset.prompt)"
                >
                  <UIcon :name="preset.icon" class="size-4 shrink-0 text-violet-300" />
                  <span>{{ preset.name }}</span>
                </button>
              </div>
            </div>

            <div class="surface-inset p-4">
              <div class="mb-2 flex items-center gap-2 text-xs font-semibold text-zinc-300">
                <UIcon name="i-tabler-eye" class="size-4 text-cyan-300" />
                实际生效风格
              </div>
              <p class="whitespace-pre-wrap text-sm leading-6 text-zinc-400">{{ effectivePrompt }}</p>
            </div>

            <div class="flex flex-wrap items-center justify-between gap-3 border-t border-white/5 pt-4">
              <span class="text-xs text-zinc-500">普通问答和主动插话都会使用这里的风格。</span>
              <div class="flex gap-2">
                <UButton
                  v-if="custom"
                  color="neutral"
                  variant="soft"
                  icon="i-tabler-restore"
                  :loading="resetting"
                  @click="reset"
                >
                  恢复默认
                </UButton>
                <UButton
                  icon="i-tabler-device-floppy"
                  :loading="saving"
                  :disabled="!selectedGroupID || loadingStyle || !dirty || charCount > maxChars"
                  @click="save"
                >
                  保存风格
                </UButton>
              </div>
            </div>
          </div>
        </section>

        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div>
              <div class="section-title">怎么写效果更稳定</div>
              <div class="section-caption">聚焦表达习惯，不需要重复 Bot 的能力说明</div>
            </div>
            <UIcon name="i-tabler-wand" class="size-4 text-violet-300" />
          </div>
          <div class="grid gap-3 p-4 sm:grid-cols-3">
            <div class="surface-inset p-3"><strong class="text-xs text-white">说话方式</strong><p class="mt-1 text-xs leading-5 text-zinc-500">简洁或详细、活泼或克制、是否接梗。</p></div>
            <div class="surface-inset p-3"><strong class="text-xs text-white">群内称呼</strong><p class="mt-1 text-xs leading-5 text-zinc-500">如何称呼群友，以及 Bot 自称方式。</p></div>
            <div class="surface-inset p-3"><strong class="text-xs text-white">表达边界</strong><p class="mt-1 text-xs leading-5 text-zinc-500">不使用哪些口头禅、梗或语气符号。</p></div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
