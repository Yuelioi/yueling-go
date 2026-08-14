<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { api, type GroupInfo } from '../api'
import GroupScopeSelect from '../components/GroupScopeSelect.vue'
import MetricCard from '../components/MetricCard.vue'
import MoreActions from '../components/MoreActions.vue'
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

const styleScopes = computed<GroupInfo[]>(() => [
  { group_id: 0, group_name: '全局默认风格' },
  ...groups.value,
])
const selectedGroup = computed(() =>
  styleScopes.value.find((group) => group.group_id === selectedGroupID.value),
)
const isDefaultScope = computed(() => selectedGroupID.value === 0)
const charCount = computed(() => Array.from(stylePrompt.value).length)
const dirty = computed(() => stylePrompt.value !== savedPrompt.value)
const modeLabel = computed(() => {
  if (isDefaultScope.value) return custom.value ? '自定义默认' : '内置默认'
  return custom.value ? '群独立风格' : '继承默认'
})

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
    const currentAvailable = styleScopes.value.some((group) => group.group_id === selectedGroupID.value)
    if (!currentAvailable) {
      selectedGroupID.value = 0
    } else if (selectedGroupID.value !== null) {
      await loadStyle(selectedGroupID.value)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '群聊加载失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  if (selectedGroupID.value === null) return
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
    success.value = isDefaultScope.value
      ? (res.custom ? '全局默认风格已保存' : '已恢复程序内置默认')
      : (res.custom ? '本群独立风格已保存' : '本群已改为继承全局默认')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function reset() {
  if (selectedGroupID.value === null) return
  resetting.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await api.resetGroupAIStyle(selectedGroupID.value)
    applyStyleResponse(res)
    success.value = isDefaultScope.value ? '已恢复程序内置默认' : '本群已改为继承全局默认'
  } catch (err) {
    error.value = err instanceof Error ? err.message : '恢复默认失败'
  } finally {
    resetting.value = false
  }
}

watch(selectedGroupID, (groupID) => {
  if (groupID !== null) void loadStyle(groupID)
})

onMounted(load)
</script>

<template>
  <section class="space-y-5">
    <PageHeader
      eyebrow="Conversation persona"
      title="AI 对话风格"
      description="维护一个全局默认提示词，需要时再为特定群设置覆盖。修改后无需重启 Bot。"
      icon="i-tabler-sparkles"
    >
      <UButton color="neutral" variant="soft" icon="i-tabler-refresh" :loading="loading" @click="load">刷新群聊</UButton>
    </PageHeader>

    <div class="metrics-grid">
      <MetricCard label="可覆盖群聊" :value="groups.length" detail="未覆盖时继承全局默认" icon="i-tabler-users-group" tone="violet" />
      <MetricCard label="当前模式" :value="modeLabel" :detail="isDefaultScope ? '默认作用于所有未覆盖群' : '仅影响所选群聊'" icon="i-tabler-adjustments" tone="cyan" />
      <MetricCard label="提示词长度" :value="`${charCount}/${maxChars}`" detail="按 Unicode 字符计数" icon="i-tabler-text-size" tone="amber" />
    </div>

    <UAlert v-if="error" class="error-banner" color="error" variant="subtle" icon="i-tabler-alert-circle" :description="error" />
    <UAlert v-if="success" color="success" variant="subtle" icon="i-tabler-circle-check" :description="success" />

    <div class="space-y-4">
      <GroupScopeSelect
        v-model="selectedGroupID"
        :groups="styleScopes"
        title="风格作用域"
        description="默认风格或指定群覆盖"
        zero-label="未覆盖群聊继承"
      />

      <div class="space-y-4">
        <section class="surface-panel overflow-hidden">
          <div class="panel-header">
            <div class="min-w-0">
              <div class="truncate font-medium text-white">{{ selectedGroup?.group_name || '未选择作用域' }}</div>
              <div class="text-xs text-zinc-500">
                {{ selectedGroupID === null ? '选择作用域后可编辑' : isDefaultScope ? '所有没有独立配置的群都会继承' : `群号 ${selectedGroupID} · 覆盖全局默认` }}
              </div>
            </div>
            <UBadge :color="custom ? 'primary' : 'neutral'" variant="subtle">
              {{ loadingStyle ? '读取中' : modeLabel }}
            </UBadge>
          </div>

          <div class="space-y-5 p-5">
            <UFormField
              :label="isDefaultScope ? '全局默认提示词' : '群专属提示词'"
              :description="isDefaultScope
                ? '控制所有未单独配置群聊的人格、语气、称呼和表达习惯。'
                : '只描述本群需要覆盖默认值的部分；权限、安全与工具规则不受影响。'"
            >
              <UTextarea
                v-model="stylePrompt"
                class="w-full"
                :ui="{ root: 'w-full' }"
                :rows="8"
                autoresize
                :maxlength="maxChars"
                :disabled="selectedGroupID === null || loadingStyle"
                :placeholder="defaultPrompt"
              />
            </UFormField>

            <div class="flex flex-wrap items-center justify-between gap-3 text-xs">
              <span class="text-zinc-500">
                {{ isDefaultScope ? '留空保存将恢复程序内置默认。' : '留空保存将删除本群覆盖并继承全局默认。' }}
              </span>
              <span :class="charCount > maxChars ? 'text-rose-300' : 'text-zinc-500'">{{ charCount }} / {{ maxChars }}</span>
            </div>

            <div class="flex flex-wrap items-center justify-between gap-3 border-t border-white/5 pt-4">
              <span class="text-xs text-zinc-500">普通问答和主动插话都会读取最终生效的风格。</span>
              <div class="flex items-center gap-2">
                <MoreActions
                  v-if="custom"
                  label="风格更多操作"
                  show-label
                  :disabled="resetting"
                  :items="[
                    { label: isDefaultScope ? '恢复内置默认' : '继承全局默认', description: isDefaultScope ? '移除自定义默认提示词' : '移除本群独立覆盖', icon: 'i-tabler-restore', color: 'warning', loading: resetting, onSelect: reset },
                  ]"
                />
                <UButton
                  icon="i-tabler-device-floppy"
                  :loading="saving"
                  :disabled="selectedGroupID === null || loadingStyle || !dirty || charCount > maxChars"
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
              <div class="section-title">继承规则</div>
              <div class="section-caption">保持配置简单，只有确实不同的群才需要覆盖</div>
            </div>
            <UIcon name="i-tabler-hierarchy-2" class="size-4 text-violet-300" />
          </div>
          <div class="grid gap-3 p-4 sm:grid-cols-3">
            <div class="surface-inset p-3"><strong class="text-xs text-white">全局默认</strong><p class="mt-1 text-xs leading-5 text-zinc-500">作为所有群的基础人格与表达习惯。</p></div>
            <div class="surface-inset p-3"><strong class="text-xs text-white">群级覆盖</strong><p class="mt-1 text-xs leading-5 text-zinc-500">只在需要不同语气或称呼的群里设置。</p></div>
            <div class="surface-inset p-3"><strong class="text-xs text-white">安全规则</strong><p class="mt-1 text-xs leading-5 text-zinc-500">风格提示词不能修改权限、真实性和工具边界。</p></div>
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
